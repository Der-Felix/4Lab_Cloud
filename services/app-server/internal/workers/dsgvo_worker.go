package workers

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/4labscloud/4labscloud/services/app-server/internal/audit"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// DSGVOWorker verarbeitet asynchrone Datenexporte und fuehrt taegliche Bereinigungen durch.
type DSGVOWorker struct {
	dbPool           *pgxpool.Pool
	redisClient      *redis.Client
	auditLogger      *audit.Logger
	uploadServiceURL string
	serviceToken     string
	retentionDays    int
	httpClient       *http.Client
}

// NewDSGVOWorker erstellt eine neue Instanz des Hintergrundarbeiters.
func NewDSGVOWorker(
	dbPool *pgxpool.Pool,
	redisClient *redis.Client,
	auditLogger *audit.Logger,
	uploadServiceURL string,
	serviceToken string,
	retentionDays int,
) *DSGVOWorker {
	if retentionDays <= 0 {
		retentionDays = 90
	}
	return &DSGVOWorker{
		dbPool:           dbPool,
		redisClient:      redisClient,
		auditLogger:      auditLogger,
		uploadServiceURL: uploadServiceURL,
		serviceToken:     serviceToken,
		retentionDays:    retentionDays,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Start startet die Hintergrund-Goroutinen fuer Export-Verarbeitung und Bereinigung.
func (w *DSGVOWorker) Start(ctx context.Context) {
	slog.Info("dsgvo-hintergrundarbeiter gestartet")

	// Einmalige Ausfuehrung der Aufbewahrungspruefung beim Serverstart
	w.RunRetentionCleanup(ctx)
	w.RunExportCleanup(ctx)

	// 1. Worker fuer anstehende Export-Auftraege (kurzes Intervall fuer schnelle Antwortzeiten)
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.ProcessPendingExports(ctx)
			}
		}
	}()

	// 2. Taeglicher Bereinigungs-Job (Aufbewahrungsfristen & abgelaufene ZIP-Exporte)
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.RunRetentionCleanup(ctx)
				w.RunExportCleanup(ctx)
			}
		}
	}()
}

// ProcessPendingExports sucht nach anstehenden Exporten und stoesst die ZIP-Generierung im Rust-Service an.
func (w *DSGVOWorker) ProcessPendingExports(ctx context.Context) {
	if w.dbPool == nil {
		return
	}

	rows, err := w.dbPool.Query(ctx,
		`SELECT id, user_id
		 FROM export_jobs
		 WHERE status = 'pending'
		 ORDER BY created_at ASC
		 LIMIT 5`,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	type jobItem struct {
		id     uuid.UUID
		userID uuid.UUID
	}
	var jobs []jobItem
	for rows.Next() {
		var j jobItem
		if err := rows.Scan(&j.id, &j.userID); err == nil {
			jobs = append(jobs, j)
		}
	}

	for _, job := range jobs {
		w.processSingleExport(ctx, job.id, job.userID)
	}
}

func (w *DSGVOWorker) processSingleExport(ctx context.Context, jobID, userID uuid.UUID) {
	// Status auf 'processing' setzen
	_, err := w.dbPool.Exec(ctx, "UPDATE export_jobs SET status = 'processing' WHERE id = $1", jobID)
	if err != nil {
		return
	}

	// 0. Key-Salt aus DB und temporaeres Passwort aus Redis laden
	var keySalt []byte
	_ = w.dbPool.QueryRow(ctx, "SELECT key_salt FROM export_jobs WHERE id = $1", jobID).Scan(&keySalt)
	saltHex := hex.EncodeToString(keySalt)

	password := ""
	if w.redisClient != nil {
		password, _ = w.redisClient.Get(ctx, "export:pw:"+jobID.String()).Result()
	}
	defer func() {
		if w.redisClient != nil {
			_ = w.redisClient.Del(ctx, "export:pw:"+jobID.String())
		}
	}()

	// 1. Profil laden
	var (
		email      string
		isAdmin    bool
		mfaEnabled bool
		createdAt  time.Time
	)
	err = w.dbPool.QueryRow(ctx,
		"SELECT email, is_admin, mfa_enabled, created_at FROM users WHERE id = $1",
		userID,
	).Scan(&email, &isAdmin, &mfaEnabled, &createdAt)
	if err != nil {
		_, _ = w.dbPool.Exec(ctx, "UPDATE export_jobs SET status = 'failed' WHERE id = $1", jobID)
		return
	}

	profileData := map[string]any{
		"user_id":     userID,
		"email":       email,
		"is_admin":    isAdmin,
		"mfa_enabled": mfaEnabled,
		"created_at":  createdAt.Format(time.RFC3339),
	}

	// 2. Shares laden
	var sharesList []map[string]any
	shRows, err := w.dbPool.Query(ctx,
		"SELECT id, file_id, token, expires_at, created_at FROM shares WHERE owner_id = $1",
		userID,
	)
	if err == nil {
		for shRows.Next() {
			var (
				sID   uuid.UUID
				fID   uuid.UUID
				token string
				expAt *time.Time
				cAt   time.Time
			)
			if err := shRows.Scan(&sID, &fID, &token, &expAt, &cAt); err == nil {
				item := map[string]any{
					"share_id":   sID,
					"file_id":    fID,
					"token":      token,
					"created_at": cAt.Format(time.RFC3339),
				}
				if expAt != nil {
					item["expires_at"] = expAt.Format(time.RFC3339)
				}
				sharesList = append(sharesList, item)
			}
		}
		shRows.Close()
	}

	// 3. Dateien laden
	type fileItem struct {
		Filename    string `json:"filename"`
		StoragePath string `json:"storage_path"`
	}
	var filesList []fileItem
	fRows, err := w.dbPool.Query(ctx,
		"SELECT filename, storage_path FROM files WHERE user_id = $1",
		userID,
	)
	if err == nil {
		for fRows.Next() {
			var fi fileItem
			if err := fRows.Scan(&fi.Filename, &fi.StoragePath); err == nil {
				filesList = append(filesList, fi)
			}
		}
		fRows.Close()
	}

	// 4. Rust-Anfrage zusammenbauen (inklusive Passwort und Salt fuer AES-GCM/Argon2id)
	rustReq := map[string]any{
		"user_id":   userID,
		"job_id":    jobID,
		"password":  password,
		"key_salt":  saltHex,
		"profile":   profileData,
		"shares":    sharesList,
		"files":     filesList,
	}

	bodyBytes, _ := json.Marshal(rustReq)
	url := fmt.Sprintf("%s/internal/exports", w.uploadServiceURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		_, _ = w.dbPool.Exec(ctx, "UPDATE export_jobs SET status = 'failed' WHERE id = $1", jobID)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+w.serviceToken)

	resp, err := w.httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		slog.ErrorContext(ctx, "export-erstellung in rust fehlgeschlagen", "job_id", jobID, "error", err)
		_, _ = w.dbPool.Exec(ctx, "UPDATE export_jobs SET status = 'failed' WHERE id = $1", jobID)
		if resp != nil {
			_ = resp.Body.Close()
		}
		return
	}
	defer resp.Body.Close()

	var rustResp struct {
		Path      string `json:"path"`
		SizeBytes int64  `json:"size_bytes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rustResp); err != nil {
		_, _ = w.dbPool.Exec(ctx, "UPDATE export_jobs SET status = 'failed' WHERE id = $1", jobID)
		return
	}

	// 5. Erfolgreichen Export in DB speichern (7 Tage Gueltigkeit)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	_, err = w.dbPool.Exec(ctx,
		`UPDATE export_jobs
		 SET status = 'completed', storage_path = $1, size_bytes = $2, completed_at = now(), expires_at = $3
		 WHERE id = $4`,
		rustResp.Path, rustResp.SizeBytes, expiresAt, jobID,
	)
	if err != nil {
		slog.ErrorContext(ctx, "fehler beim aktualisieren des export-status", "job_id", jobID, "error", err)
	}
}

// RunRetentionCleanup fuehrt die Loeschung von Audit-Logs aelter als retentionDays durch.
func (w *DSGVOWorker) RunRetentionCleanup(ctx context.Context) {
	if w.dbPool == nil {
		return
	}

	count, err := audit.CleanupRetention(ctx, w.dbPool, w.retentionDays)
	if err != nil {
		slog.ErrorContext(ctx, "fehler bei audit-log bereinigung", "error", err)
		return
	}

	slog.InfoContext(ctx, "audit-log retention cleanup durchgefuehrt",
		"deleted_records", count,
		"retention_days", w.retentionDays,
	)

	if w.auditLogger != nil {
		w.auditLogger.Log(ctx, audit.Entry{
			Action: "audit_retention_cleanup",
			Result: fmt.Sprintf("%d", count),
		})
	}
}

// RunExportCleanup loescht abgelaufene Export-Auftraege und deren Dateien.
func (w *DSGVOWorker) RunExportCleanup(ctx context.Context) {
	if w.dbPool == nil {
		return
	}

	rows, err := w.dbPool.Query(ctx,
		`SELECT id FROM export_jobs WHERE expires_at IS NOT NULL AND expires_at < now()`,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	var expiredIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err == nil {
			expiredIDs = append(expiredIDs, id)
		}
	}

	for _, id := range expiredIDs {
		// Datei in Rust loeschen
		delURL := fmt.Sprintf("%s/internal/exports/%s", w.uploadServiceURL, id)
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, delURL, nil)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+w.serviceToken)
			resp, err := w.httpClient.Do(req)
			if err == nil && resp != nil {
				_ = resp.Body.Close()
			}
		}

		// Eintrag aus DB loeschen
		_, _ = w.dbPool.Exec(ctx, "DELETE FROM export_jobs WHERE id = $1", id)
	}

	if len(expiredIDs) > 0 && w.auditLogger != nil {
		w.auditLogger.Log(ctx, audit.Entry{
			Action: "export_cleanup",
			Result: fmt.Sprintf("%d", len(expiredIDs)),
		})
	}
}
