package routes

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/4labscloud/4labscloud/services/app-server/internal/audit"
	"github.com/4labscloud/4labscloud/services/app-server/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// UserHandler verwaltet Account-Loeschung und Datenexport gemaess DSGVO.
type UserHandler struct {
	dbPool           *pgxpool.Pool
	redisClient      *redis.Client
	auditLogger      *audit.Logger
	uploadServiceURL string
	serviceToken     string
	mfaKey           []byte
	auditHMACKey     []byte
	defaultQuotaGB   int
	httpClient       *http.Client
}

// NewUserHandler instanziiert den User-Handler mit allen Compliance-Abhaengigkeiten.
func NewUserHandler(
	dbPool *pgxpool.Pool,
	redisClient *redis.Client,
	auditLogger *audit.Logger,
	uploadServiceURL string,
	serviceToken string,
	mfaKey []byte,
	auditHMACKey []byte,
	defaultQuotaGB int,
) *UserHandler {
	return &UserHandler{
		dbPool:           dbPool,
		redisClient:      redisClient,
		auditLogger:      auditLogger,
		uploadServiceURL: uploadServiceURL,
		serviceToken:     serviceToken,
		mfaKey:           mfaKey,
		auditHMACKey:     auditHMACKey,
		defaultQuotaGB:   defaultQuotaGB,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// DeleteMeRequest enthaelt das Kennwort und optional den TOTP-Code zur Loeschbestaetigung.
type DeleteMeRequest struct {
	Password string `json:"password" binding:"required"`
	TOTPCode string `json:"totp_code"`
}

// DeleteMe fuehrt die vollstaendige Selbstloeschung des Benutzers durch (DSGVO Art. 17).
func (h *UserHandler) DeleteMe(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	if h.dbPool == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbank nicht verfuegbar"})
		return
	}

	var req DeleteMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Passwort ist fuer die Loeschung erforderlich"})
		return
	}

	// 1. Benutzerdaten und Passwort-Hash laden
	var (
		passwordHash    string
		mfaEnabled      bool
		encryptedSecret []byte
	)
	err := h.dbPool.QueryRow(c.Request.Context(),
		"SELECT password_hash, mfa_enabled, mfa_secret_encrypted FROM users WHERE id = $1",
		userID,
	).Scan(&passwordHash, &mfaEnabled, &encryptedSecret)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Benutzer nicht gefunden"})
		return
	}

	// 2. Passwort pruefen
	valid, err := auth.VerifyPassword(req.Password, passwordHash)
	if err != nil || !valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Ungueltiges Passwort"})
		return
	}

	// 3. Falls MFA aktiv ist, TOTP-Code oder Recovery-Code pruefen
	if mfaEnabled {
		if req.TOTPCode == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "MFA-Bestaetigungscode erforderlich"})
			return
		}

		mfaValid := false
		if len(encryptedSecret) > 0 {
			plainSecret, err := auth.DecryptMFASecret(h.mfaKey, encryptedSecret)
			if err == nil && auth.ValidateTOTPCode(req.TOTPCode, plainSecret) {
				mfaValid = true
			}
		}

		if !mfaValid {
			recoveryValid, err := auth.ValidateAndConsumeRecoveryCode(c.Request.Context(), h.dbPool, userID, req.TOTPCode)
			if err == nil && recoveryValid {
				mfaValid = true
			}
		}

		if !mfaValid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Ungueltiger MFA-Code"})
			return
		}
	}

	// 4. Kaskadierende physische und datenbankseitige Loeschung ausfuehren (Rust zuerst)
	pseudonym, err := h.executeUserDeletion(c.Request.Context(), userID)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "fehler bei benutzerloeschung", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Loeschen des Kontos"})
		return
	}

	// Cookies leeren
	c.SetCookie("refresh_token", "", -1, "/api/v1/auth", "", true, true)
	c.SetCookie("csrf_token", "", -1, "/", "", true, false)

	// Pseudonymisiertes Audit-Log schreiben
	if h.auditLogger != nil {
		h.auditLogger.Log(c.Request.Context(), audit.Entry{
			PseudonymHash: &pseudonym,
			Action:        "user_self_delete",
			IP:            c.ClientIP(),
			Result:        "ok",
		})
	}

	c.Status(http.StatusNoContent)
}

// AdminDeleteUser erlaubt Administratoren das Loeschen fremder Accounts.
func (h *UserHandler) AdminDeleteUser(c *gin.Context) {
	adminID := auth.MustGetUserID(c)
	if adminID == uuid.Nil {
		return
	}

	targetIDStr := c.Param("id")
	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungueltige Benutzer-ID"})
		return
	}

	// Admin kann sich selbst NICHT ueber diesen Endpunkt loeschen
	if targetID == adminID {
		if h.auditLogger != nil {
			h.auditLogger.Log(c.Request.Context(), audit.Entry{
				UserID:   &adminID,
				Action:   "user_admin_delete",
				TargetID: &targetID,
				IP:       c.ClientIP(),
				Result:   "forbidden_self_delete",
			})
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "Administratoren koennen sich nicht selbst loeschen"})
		return
	}

	if h.dbPool == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbank nicht verfuegbar"})
		return
	}

	var exists bool
	err = h.dbPool.QueryRow(c.Request.Context(), "SELECT true FROM users WHERE id = $1", targetID).Scan(&exists)
	if err != nil || !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Zielbenutzer nicht gefunden"})
		return
	}

	_, err = h.executeUserDeletion(c.Request.Context(), targetID)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "fehler bei admin-loeschung", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Loeschen des Benutzers"})
		return
	}

	if h.auditLogger != nil {
		h.auditLogger.Log(c.Request.Context(), audit.Entry{
			UserID:   &adminID,
			Action:   "user_admin_delete",
			TargetID: &targetID,
			IP:       c.ClientIP(),
			Result:   "ok",
		})
	}

	c.Status(http.StatusNoContent)
}

// executeUserDeletion fuehrt die Schritte der DSGVO-konformen Loeschung durch.
// STRIKTE REIHENFOLGE:
// 1. Rust DELETE fuer Dateien und Exporte zuerst
// 2. Bei Rust-Fehler: Sofortiger Abbruch, KEINE DB-Aenderung
// 3. Erst nach erfolgreicher physischer Loeschung: DB-Datensatz entfernen & Audit pseudonymisieren
// 4. User-ID in Redis blacklisten
func (h *UserHandler) executeUserDeletion(ctx context.Context, userID uuid.UUID) (string, error) {
	// 1. Alle Speicherdateien des Nutzers ermitteln und physisch in Rust loeschen
	rows, err := h.dbPool.Query(ctx, "SELECT storage_path FROM files WHERE user_id = $1", userID)
	if err != nil {
		return "", fmt.Errorf("dateien konnten nicht ermittelt werden: %w", err)
	}
	var storagePaths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err == nil {
			storagePaths = append(storagePaths, path)
		}
	}
	rows.Close()

	for _, p := range storagePaths {
		if err := h.deleteRustStorageFile(ctx, p); err != nil {
			return "", fmt.Errorf("physische dateiloeschung in rust fehlgeschlagen: %w", err)
		}
	}

	// 2. Export-ZIPs auf Disk via Rust loeschen
	expRows, err := h.dbPool.Query(ctx, "SELECT id FROM export_jobs WHERE user_id = $1", userID)
	if err != nil {
		return "", fmt.Errorf("export-jobs konnten nicht ermittelt werden: %w", err)
	}
	var jobIDs []uuid.UUID
	for expRows.Next() {
		var jID uuid.UUID
		if err := expRows.Scan(&jID); err == nil {
			jobIDs = append(jobIDs, jID)
		}
	}
	expRows.Close()

	for _, jID := range jobIDs {
		if err := h.deleteRustExportFile(ctx, jID); err != nil {
			return "", fmt.Errorf("physische exportloeschung in rust fehlgeschlagen: %w", err)
		}
	}

	// 3. Alle Refresh-Tokens und Sitzungen widerrufen
	_ = auth.RevokeAllForUser(ctx, h.dbPool, userID)

	// 4. Audit-Log pseudonymisieren (user_id -> NULL, pseudonym_hash = HMAC(user_id))
	pseudonym := ""
	if len(h.auditHMACKey) == 32 {
		pseudonym, _ = audit.PseudonymizeUser(ctx, h.dbPool, userID, h.auditHMACKey)
	}

	// 5. Benutzerdatensatz loeschen (kaskadiert files, shares, export_jobs, refresh_tokens)
	_, err = h.dbPool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
	if err != nil {
		return pseudonym, fmt.Errorf("fehler beim loeschen des datensatzes: %w", err)
	}

	// 6. User-ID in Redis auf Blacklist setzen
	if h.redisClient != nil {
		_ = auth.BlacklistUser(ctx, h.redisClient, userID, 15*time.Minute)
	}

	return pseudonym, nil
}

func (h *UserHandler) deleteRustStorageFile(ctx context.Context, storagePath string) error {
	url := fmt.Sprintf("%s/internal/files/%s", h.uploadServiceURL, storagePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("anfrage konnte nicht erstellt werden: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+h.serviceToken)
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("storage-service nicht erreichbar: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("storage-service meldete status %d", resp.StatusCode)
	}
	return nil
}

func (h *UserHandler) deleteRustExportFile(ctx context.Context, jobID uuid.UUID) error {
	url := fmt.Sprintf("%s/internal/exports/%s", h.uploadServiceURL, jobID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("anfrage konnte nicht erstellt werden: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+h.serviceToken)
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("storage-service nicht erreichbar: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("storage-service meldete status %d", resp.StatusCode)
	}
	return nil
}

// RequestExportBody verlangt die Bestaetigung durch das Benutzerkennwort.
type RequestExportBody struct {
	Password string `json:"password" binding:"required"`
}

// RequestExport erstellt einen asynchronen Datenexport-Auftrag mit Passwort-Validierung (DSGVO Art. 20).
func (h *UserHandler) RequestExport(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	if h.dbPool == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbank nicht verfuegbar"})
		return
	}

	var req RequestExportBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Passwort ist fuer den Datenexport erforderlich"})
		return
	}

	// 1. Passwort pruefen
	var passwordHash string
	err := h.dbPool.QueryRow(c.Request.Context(), "SELECT password_hash FROM users WHERE id = $1", userID).Scan(&passwordHash)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Benutzer nicht gefunden"})
		return
	}

	valid, err := auth.VerifyPassword(req.Password, passwordHash)
	if err != nil || !valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Ungueltiges Passwort"})
		return
	}

	// 2. Kryptografisches 16-Byte-Salt generieren
	keySalt := make([]byte, 16)
	if _, err := rand.Read(keySalt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler bei Salt-Erstellung"})
		return
	}

	var jobID uuid.UUID
	err = h.dbPool.QueryRow(c.Request.Context(),
		`INSERT INTO export_jobs (user_id, key_salt, status)
		 VALUES ($1, $2, 'pending')
		 RETURNING id`,
		userID, keySalt,
	).Scan(&jobID)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "fehler beim erstellen des export-jobs", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Exportauftrag konnte nicht angelegt werden"})
		return
	}

	// 3. Passwort temporaer in Redis fuer Hintergrund-Worker hinterlegen (10 Min TTL)
	if h.redisClient != nil {
		_ = h.redisClient.Set(c.Request.Context(), "export:pw:"+jobID.String(), req.Password, 10*time.Minute).Err()
	}

	if h.auditLogger != nil {
		h.auditLogger.Log(c.Request.Context(), audit.Entry{
			UserID:   &userID,
			Action:   "export_request",
			TargetID: &jobID,
			IP:       c.ClientIP(),
			Result:   "ok",
		})
	}

	c.JSON(http.StatusAccepted, gin.H{
		"job_id": jobID,
		"status": "pending",
	})
}

// GetExportStatus gibt den Status und bei Fertigstellung den Download-Link zurueck.
func (h *UserHandler) GetExportStatus(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	jobIDStr := c.Param("job_id")
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungueltige Auftrags-ID"})
		return
	}

	if h.dbPool == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbank nicht verfuegbar"})
		return
	}

	var (
		status      string
		sizeBytes   *int64
		completedAt *time.Time
		expiresAt   *time.Time
	)
	err = h.dbPool.QueryRow(c.Request.Context(),
		`SELECT status, size_bytes, completed_at, expires_at
		 FROM export_jobs
		 WHERE id = $1 AND user_id = $2`,
		jobID, userID,
	).Scan(&status, &sizeBytes, &completedAt, &expiresAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Exportauftrag nicht gefunden"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Laden des Auftrags"})
		return
	}

	resp := gin.H{
		"job_id": jobID,
		"status": status,
	}

	if status == "completed" {
		resp["download_url"] = fmt.Sprintf("/api/v1/users/me/export/%s/download", jobID)
		if sizeBytes != nil {
			resp["size_bytes"] = *sizeBytes
		}
		if completedAt != nil {
			resp["completed_at"] = completedAt.Format(time.RFC3339)
		}
		if expiresAt != nil {
			resp["expires_at"] = expiresAt.Format(time.RFC3339)
		}
	}

	c.JSON(http.StatusOK, resp)
}

// DownloadExport streamt das fertige ZIP-Archiv ueber den Rust-Upload-Service an den Client.
func (h *UserHandler) DownloadExport(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	jobIDStr := c.Param("job_id")
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungueltige Auftrags-ID"})
		return
	}

	if h.dbPool == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbank nicht verfuegbar"})
		return
	}

	var (
		status    string
		expiresAt *time.Time
	)
	err = h.dbPool.QueryRow(c.Request.Context(),
		`SELECT status, expires_at
		 FROM export_jobs
		 WHERE id = $1 AND user_id = $2`,
		jobID, userID,
	).Scan(&status, &expiresAt)

	if err != nil || status != "completed" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Exportdatei nicht verfuegbar oder noch nicht fertiggestellt"})
		return
	}

	if expiresAt != nil && time.Now().After(*expiresAt) {
		c.JSON(http.StatusGone, gin.H{"error": "Exportdatei ist abgelaufen"})
		return
	}

	// Stream von Rust abrufen
	rustURL := fmt.Sprintf("%s/internal/exports/%s/download", h.uploadServiceURL, jobID)
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, rustURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Interner Fehler"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+h.serviceToken)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Storage-Service nicht erreichbar"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "Fehler beim Abrufen der Exportdatei"})
		return
	}

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"export-%s.zip\"", jobID))
	c.Status(http.StatusOK)

	_, _ = io.Copy(c.Writer, resp.Body)

	if h.auditLogger != nil {
		h.auditLogger.Log(c.Request.Context(), audit.Entry{
			UserID:   &userID,
			Action:   "export_download",
			TargetID: &jobID,
			IP:       c.ClientIP(),
			Result:   "ok",
		})
	}
}

// QuotaResponse liefert belegten Speicher, Gesamtkapazitaet und prozentuale Auslastung.
type QuotaResponse struct {
	UsedBytes  int64   `json:"used_bytes"`
	TotalBytes int64   `json:"total_bytes"`
	Percent    float64 `json:"percent"`
}

// GetQuota berechnet den Speicherplatzverbrauch des aktuellen Nutzers.
func (h *UserHandler) GetQuota(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	if h.dbPool == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbank nicht verfuegbar"})
		return
	}

	// 1. Individuelles Quota aus users abfragen
	var customQuota *int64
	err := h.dbPool.QueryRow(c.Request.Context(),
		"SELECT quota_bytes FROM users WHERE id = $1",
		userID,
	).Scan(&customQuota)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		slog.ErrorContext(c.Request.Context(), "fehler beim abfragen des benutzer-quotas", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Quota konnte nicht geladen werden"})
		return
	}

	// Gesamtkapazitaet: users.quota_bytes falls vorhanden, sonst DEFAULT_QUOTA_GB (Default: 50 GB)
	defaultGB := h.defaultQuotaGB
	if defaultGB <= 0 {
		defaultGB = 50
	}
	totalBytes := int64(defaultGB) * 1024 * 1024 * 1024
	if customQuota != nil && *customQuota > 0 {
		totalBytes = *customQuota
	}

	// 2. Belegten Speicher aus files berechnen
	var usedBytes int64
	err = h.dbPool.QueryRow(c.Request.Context(),
		"SELECT COALESCE(SUM(size_bytes), 0) FROM files WHERE user_id = $1",
		userID,
	).Scan(&usedBytes)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "fehler beim berechnen des speicherplatzes", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Speicherplatz konnte nicht ermittelt werden"})
		return
	}

	var percent float64
	if totalBytes > 0 {
		percent = math.Round((float64(usedBytes)/float64(totalBytes)*100)*10) / 10
	}

	c.JSON(http.StatusOK, QuotaResponse{
		UsedBytes:  usedBytes,
		TotalBytes: totalBytes,
		Percent:    percent,
	})
}

