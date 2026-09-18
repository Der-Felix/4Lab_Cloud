package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/4labscloud/4labscloud/services/app-server/internal/audit"
	"github.com/4labscloud/4labscloud/services/app-server/internal/auth"
	"github.com/4labscloud/4labscloud/services/app-server/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UploadInitRequest definiert die Eingabedaten fuer das Initialisieren eines Uploads.
type UploadInitRequest struct {
	Filename  string `json:"filename" binding:"required"`
	SizeBytes int64  `json:"size_bytes" binding:"required,gt=0"`
}

// RustUploadInitRequest ist das Payload fuer den internen Rust-Upload-Service.
type RustUploadInitRequest struct {
	UserID    string `json:"user_id"`
	Filename  string `json:"filename"`
	SizeBytes int64  `json:"size_bytes"`
}

// UploadCompleteRequest definiert die Eingabedaten fuer den Complete-Aufruf.
type UploadCompleteRequest struct {
	UploadID uuid.UUID `json:"upload_id" binding:"required"`
}

// UploadCompleteResponse enthaelt die Metadaten der erstellten Datei.
type UploadCompleteResponse struct {
	FileID   uuid.UUID `json:"file_id"`
	Filename string    `json:"filename"`
	Status   string    `json:"status"`
}

// RustUploadStatusResponse entspricht der Rueckgabe von Rust GET /internal/uploads/{id}/status.
type RustUploadStatusResponse struct {
	UploadID    uuid.UUID `json:"upload_id"`
	UserID      uuid.UUID `json:"user_id"`
	Filename    string    `json:"filename"`
	SizeBytes   int64     `json:"size_bytes"`
	Checksum    *string   `json:"checksum"`
	StoragePath string    `json:"storage_path"`
	Status      string    `json:"status"`
}

// RustThumbnailResponse enthaelt die Rueckgabe der Thumbnail-Generierung.
type RustThumbnailResponse struct {
	ThumbnailPath string          `json:"thumbnail_path"`
	Width         int             `json:"width"`
	Height        int             `json:"height"`
	TakenAt       *time.Time      `json:"taken_at"`
	ExifJSON      json.RawMessage `json:"exif_json"`
}

// detectMimeType ermittelt den MIME-Typ anhand der Dateiendung.
func detectMimeType(filename string) string {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lower, ".mp4"):
		return "video/mp4"
	case strings.HasSuffix(lower, ".webm"):
		return "video/webm"
	case strings.HasSuffix(lower, ".pdf"):
		return "application/pdf"
	case strings.HasSuffix(lower, ".zip"):
		return "application/zip"
	case strings.HasSuffix(lower, ".txt"):
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}

// UploadsHandler verwaltet Upload-Aktionen und die Kommunikation mit dem Rust-Service.
type UploadsHandler struct {
	uploadServiceURL string
	serviceToken     string
	dbPool           *pgxpool.Pool
	httpClient       *http.Client
	auditLogger      *audit.Logger
}

// NewUploadsHandler erstellt einen neuen Handler mit konfiguriertem Timeout.
func NewUploadsHandler(uploadServiceURL, serviceToken string, dbPool *pgxpool.Pool, auditLogger *audit.Logger) *UploadsHandler {
	return &UploadsHandler{
		uploadServiceURL: uploadServiceURL,
		serviceToken:     serviceToken,
		dbPool:           dbPool,
		httpClient: &http.Client{
			Timeout: 5 * time.Second, // 5s Timeout fuer Metadaten gemaess Konvention
		},
		auditLogger: auditLogger,
	}
}

// Init startet den Upload-Prozess, ruft den Rust-Service auf und loggt das Ereignis.
func (h *UploadsHandler) Init(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	var req UploadInitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungueltige Anfrage: " + err.Error()})
		return
	}

	// Payload fuer Rust vorbereiten
	rustReq := RustUploadInitRequest{
		UserID:    userID.String(),
		Filename:  req.Filename,
		SizeBytes: req.SizeBytes,
	}

	reqBytes, err := json.Marshal(rustReq)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "fehler beim serialisieren der upload-anfrage", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Interner Serverfehler"})
		return
	}

	// Internen Aufruf an Rust Storage-Service durchfuehren
	targetURL := h.uploadServiceURL + "/internal/uploads/init"
	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, targetURL, bytes.NewBuffer(reqBytes))
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "fehler beim erstellen der anfrage an upload-service", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Interner Serverfehler"})
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+h.serviceToken)

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		slog.WarnContext(c.Request.Context(), "upload-service nicht erreichbar", "url", targetURL, "error", err)
		h.logAudit(c.Request.Context(), &userID, "upload_init", nil, c.ClientIP(), "failed_service_unreachable")
		c.JSON(http.StatusBadGateway, gin.H{"error": "Upload-Service nicht erreichbar"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		slog.WarnContext(c.Request.Context(), "upload-service gab fehlerhaften status zurueck",
			"status", resp.StatusCode,
			"response", string(respBody),
		)
		h.logAudit(c.Request.Context(), &userID, "upload_init", nil, c.ClientIP(), "failed_service_error")
		c.JSON(http.StatusBadGateway, gin.H{"error": "Upload-Service meldet Fehler"})
		return
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		slog.ErrorContext(c.Request.Context(), "ungueltige antwort vom upload-service", "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Ungueltige Antwort vom Upload-Service"})
		return
	}

	// Audit-Log erfolgreich erfassen
	h.logAudit(c.Request.Context(), &userID, "upload_init", nil, c.ClientIP(), "ok")

	c.JSON(http.StatusOK, result)
}

// Complete schliesst einen Datei-Upload ab, verifiziert diesen bei Rust und traegt Metadaten in files ein.
func (h *UploadsHandler) Complete(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	var req UploadCompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungueltige Anfrage: " + err.Error()})
		return
	}

	// 1. Rust-Upload-Service Status abfragen
	targetURL := fmt.Sprintf("%s/internal/uploads/%s/status", h.uploadServiceURL, req.UploadID.String())
	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, targetURL, nil)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "fehler beim erstellen der anfrage an upload-service", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Interner Serverfehler"})
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+h.serviceToken)

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		slog.WarnContext(c.Request.Context(), "upload-service nicht erreichbar", "url", targetURL, "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Upload-Service nicht erreichbar"})
		return
	}
	defer resp.Body.Close()

	// Falls Rust 404 liefert: Idempotenz-Pruefung in DB durchfuehren
	if resp.StatusCode == http.StatusNotFound {
		if h.dbPool == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Upload-Session nicht gefunden"})
			return
		}

		var existingFileID uuid.UUID
		var existingFilename string
		err := db.WithUserRLS(c.Request.Context(), h.dbPool, userID, func(tx pgx.Tx) error {
			return tx.QueryRow(c.Request.Context(),
				"SELECT id, filename FROM files WHERE upload_id = $1", req.UploadID).
				Scan(&existingFileID, &existingFilename)
		})
		if err == nil {
			// Idempotenz-Treffer: Gleiche file_id mit 200 zurueckgeben
			h.logAudit(c.Request.Context(), &userID, "upload_complete_retry", &existingFileID, c.ClientIP(), "ok")
			c.JSON(http.StatusOK, UploadCompleteResponse{
				FileID:   existingFileID,
				Filename: existingFilename,
				Status:   "ok",
			})
			return
		}

		c.JSON(http.StatusNotFound, gin.H{"error": "Upload-Session nicht gefunden"})
		return
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		slog.WarnContext(c.Request.Context(), "upload-service fehlerhafter status",
			"status", resp.StatusCode, "response", string(respBody))
		c.JSON(http.StatusBadGateway, gin.H{"error": "Upload-Service meldet Fehler"})
		return
	}

	var statusResp RustUploadStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		slog.ErrorContext(c.Request.Context(), "ungueltige status-antwort vom upload-service", "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Ungueltige Antwort vom Upload-Service"})
		return
	}

	// 2. Sicherheitspruefung: Gehoert der Upload dem authentifizierten Benutzer?
	if statusResp.UserID != userID {
		slog.WarnContext(c.Request.Context(), "unberechtigter abschlussversuch fuer fremden upload",
			"upload_id", req.UploadID, "owner_id", statusResp.UserID, "user_id", userID)
		// Session sofort loeschen via Rust DELETE inkl. Chunks auf Disk (delete_file=true)
		h.deleteRustUpload(c.Request.Context(), req.UploadID, true)
		h.logAudit(c.Request.Context(), &userID, "upload_complete_denied", nil, c.ClientIP(), "forbidden_wrong_user")
		c.JSON(http.StatusForbidden, gin.H{"error": "Zugriff verweigert"})
		return
	}

	// 3. Statuspruefung: Wurde der Upload vollstaendig hochgeladen?
	if statusResp.Status != "completed" || statusResp.Checksum == nil {
		slog.WarnContext(c.Request.Context(), "upload noch nicht abgeschlossen",
			"upload_id", req.UploadID, "status", statusResp.Status)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Upload ist noch nicht abgeschlossen"})
		return
	}

	if h.dbPool == nil {
		slog.ErrorContext(c.Request.Context(), "datenbankpool nicht verfuegbar")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbank nicht verfuegbar"})
		return
	}

	// 4. MIME-Typ erkennen und ggf. Thumbnail bei Bildern generieren
	mimeType := detectMimeType(statusResp.Filename)
	var (
		thumbnailPath *string
		width         *int
		height        *int
		takenAt       *time.Time
		exifJSON      []byte
	)

	if strings.HasPrefix(mimeType, "image/") {
		thumbURL := fmt.Sprintf("%s/internal/thumbs/%s", h.uploadServiceURL, req.UploadID.String())
		thumbReqPayload := map[string]any{
			"storage_path": statusResp.StoragePath,
			"extract_exif": false, // DSGVO Art. 5 Datensparsamkeit: EXIF standardmaessig deaktiviert
		}
		if reqBody, err := json.Marshal(thumbReqPayload); err == nil {
			if tReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, thumbURL, bytes.NewBuffer(reqBody)); err == nil {
				tReq.Header.Set("Content-Type", "application/json")
				tReq.Header.Set("Authorization", "Bearer "+h.serviceToken)
				if tResp, err := h.httpClient.Do(tReq); err == nil {
					defer tResp.Body.Close()
					if tResp.StatusCode == http.StatusOK {
						var tData RustThumbnailResponse
						if err := json.NewDecoder(tResp.Body).Decode(&tData); err == nil {
							thumbnailPath = &tData.ThumbnailPath
							width = &tData.Width
							height = &tData.Height
							takenAt = tData.TakenAt
							if len(tData.ExifJSON) > 0 && string(tData.ExifJSON) != "null" {
								exifJSON = tData.ExifJSON
							}
						}
					}
				}
			}
		}
	}

	// 5. Metadaten in files eintragen mit RLS
	var newFileID uuid.UUID
	insertErr := db.WithUserRLS(c.Request.Context(), h.dbPool, userID, func(tx pgx.Tx) error {
		return tx.QueryRow(c.Request.Context(),
			`INSERT INTO files (user_id, filename, size_bytes, mime_type, checksum_sha256, storage_path, upload_id, thumbnail_path, width, height, taken_at, exif_json)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			 RETURNING id`,
			userID, statusResp.Filename, statusResp.SizeBytes, mimeType, *statusResp.Checksum, statusResp.StoragePath, req.UploadID,
			thumbnailPath, width, height, takenAt, exifJSON,
		).Scan(&newFileID)
	})

	// 5. Race Condition Behandlung (Postgres Fehler 23505 unique_violation)
	if insertErr != nil {
		var pgErr *pgconn.PgError
		if errors.As(insertErr, &pgErr) && pgErr.Code == "23505" {
			var existingID uuid.UUID
			var existingFilename string
			queryErr := db.WithUserRLS(c.Request.Context(), h.dbPool, userID, func(tx pgx.Tx) error {
				return tx.QueryRow(c.Request.Context(),
					"SELECT id, filename FROM files WHERE upload_id = $1", req.UploadID).
					Scan(&existingID, &existingFilename)
			})
			if queryErr == nil {
				h.logAudit(c.Request.Context(), &userID, "upload_complete_race", &existingID, c.ClientIP(), "ok")
				h.deleteRustUpload(c.Request.Context(), req.UploadID, false)
				c.JSON(http.StatusOK, UploadCompleteResponse{
					FileID:   existingID,
					Filename: existingFilename,
					Status:   "ok",
				})
				return
			}
		}

		slog.ErrorContext(c.Request.Context(), "datenbankfehler beim speichern der metadaten", "error", insertErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Metadaten konnten nicht gespeichert werden"})
		return
	}

	// 6. Erfolgreicher Abschluss: Audit-Log schreiben und Rust-Session aufraeumen (Datei bleibt erhalten)
	h.logAudit(c.Request.Context(), &userID, "upload_complete", &newFileID, c.ClientIP(), "ok")
	h.deleteRustUpload(c.Request.Context(), req.UploadID, false)

	c.JSON(http.StatusOK, UploadCompleteResponse{
		FileID:   newFileID,
		Filename: statusResp.Filename,
		Status:   "ok",
	})
}

// deleteRustUpload ruft DELETE /internal/uploads/{id} am Rust-Service auf.
func (h *UploadsHandler) deleteRustUpload(ctx context.Context, uploadID uuid.UUID, deleteFile bool) {
	targetURL := fmt.Sprintf("%s/internal/uploads/%s", h.uploadServiceURL, uploadID.String())
	if deleteFile {
		targetURL += "?delete_file=true"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, targetURL, nil)
	if err != nil {
		slog.Warn("fehler beim erstellen der delete-anfrage an rust", "upload_id", uploadID, "error", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+h.serviceToken)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		slog.Warn("fehler beim aufruf von delete an rust", "upload_id", uploadID, "error", err)
		return
	}
	defer resp.Body.Close()
}

func (h *UploadsHandler) logAudit(ctx context.Context, userID *uuid.UUID, action string, targetID *uuid.UUID, ip, result string) {
	if h.auditLogger != nil {
		h.auditLogger.Log(ctx, audit.Entry{
			UserID:   userID,
			Action:   action,
			TargetID: targetID,
			IP:       ip,
			Result:   result,
		})
	}
}
