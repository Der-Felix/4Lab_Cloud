package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/4labscloud/4labscloud/services/app-server/internal/audit"
	"github.com/4labscloud/4labscloud/services/app-server/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PublicShareHandler verwaltet oeffentliche Freigaben ohne zwingende Benutzeranmeldung.
type PublicShareHandler struct {
	dbPool           *pgxpool.Pool
	uploadServiceURL string
	serviceToken     string
	auditLogger      *audit.Logger
	httpClient       *http.Client
}

// NewPublicShareHandler initialisiert den PublicShareHandler.
func NewPublicShareHandler(dbPool *pgxpool.Pool, uploadServiceURL, serviceToken string, auditLogger *audit.Logger) *PublicShareHandler {
	return &PublicShareHandler{
		dbPool:           dbPool,
		uploadServiceURL: uploadServiceURL,
		serviceToken:     serviceToken,
		auditLogger:      auditLogger,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// GetShareInfo liefert oeffentliche Metadaten einer Freigabe (ohne JWT).
func (h *PublicShareHandler) GetShareInfo(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Freigabe-Token fehlt"})
		return
	}

	var (
		shareID      uuid.UUID
		fileID       uuid.UUID
		ownerID      uuid.UUID
		passwordHash *string
		expiresAt    *time.Time
		filename     string
		sizeBytes    int64
		mimeType     string
	)

	// Oeffentliche Abfrage (kein User-RLS, da oeffentlicher Link via unerratbarem Token)
	err := h.dbPool.QueryRow(c.Request.Context(),
		`SELECT s.id, s.file_id, s.owner_id, s.password_hash, s.expires_at,
		        f.filename, f.size_bytes, coalesce(f.mime_type, 'application/octet-stream')
		 FROM shares s
		 JOIN files f ON s.file_id = f.id
		 WHERE s.token = $1`,
		token,
	).Scan(&shareID, &fileID, &ownerID, &passwordHash, &expiresAt, &filename, &sizeBytes, &mimeType)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Freigabelink nicht gefunden"})
		return
	}

	// Ablauf pruefen
	if expiresAt != nil && expiresAt.Before(time.Now()) {
		c.JSON(http.StatusGone, gin.H{"error": "Dieser Freigabelink ist abgelaufen"})
		return
	}

	requiresPassword := passwordHash != nil && *passwordHash != ""

	c.JSON(http.StatusOK, gin.H{
		"token":             token,
		"filename":          filename,
		"size_bytes":        sizeBytes,
		"mime_type":         mimeType,
		"requires_password": requiresPassword,
		"expires_at":        expiresAt,
	})
}

// DownloadShare liefert eine signierte Download-URL fuer die freigegebene Datei (optional passwortgeschuetzt).
func (h *PublicShareHandler) DownloadShare(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Freigabe-Token fehlt"})
		return
	}

	providedPassword := c.Query("password")
	if providedPassword == "" {
		providedPassword = c.GetHeader("X-Share-Password")
	}

	var (
		shareID      uuid.UUID
		fileID       uuid.UUID
		ownerID      uuid.UUID
		passwordHash *string
		expiresAt    *time.Time
		filename     string
		storagePath  string
	)

	err := h.dbPool.QueryRow(c.Request.Context(),
		`SELECT s.id, s.file_id, s.owner_id, s.password_hash, s.expires_at,
		        f.filename, f.storage_path
		 FROM shares s
		 JOIN files f ON s.file_id = f.id
		 WHERE s.token = $1`,
		token,
	).Scan(&shareID, &fileID, &ownerID, &passwordHash, &expiresAt, &filename, &storagePath)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Freigabelink nicht gefunden"})
		return
	}

	// Ablauf pruefen
	if expiresAt != nil && expiresAt.Before(time.Now()) {
		c.JSON(http.StatusGone, gin.H{"error": "Dieser Freigabelink ist abgelaufen"})
		return
	}

	// Passwort pruefen
	if passwordHash != nil && *passwordHash != "" {
		if providedPassword == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Passwort erforderlich", "requires_password": true})
			return
		}
		match, err := auth.VerifyPassword(providedPassword, *passwordHash)
		if err != nil || !match {
			h.logAudit(c.Request.Context(), nil, "share_download_unauthorized", &shareID, c.ClientIP(), "failed_wrong_password")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Falsches Passwort"})
			return
		}
	}

	// Rust intern aufrufen: GET /internal/files/:id/download?storage_path=...&filename=...
	rustURL := fmt.Sprintf("%s/internal/files/%s/download?storage_path=%s&filename=%s",
		h.uploadServiceURL,
		fileID.String(),
		url.QueryEscape(storagePath),
		url.QueryEscape(filename),
	)

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, rustURL, nil)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "fehler beim erstellen der download-anfrage an rust", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Interner Serverfehler"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+h.serviceToken)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		slog.WarnContext(c.Request.Context(), "upload-service bei share-download nicht erreichbar", "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Storage-Service nicht erreichbar"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		slog.WarnContext(c.Request.Context(), "upload-service meldet fehler bei share-download",
			"status", resp.StatusCode, "body", string(respBody))
		c.JSON(http.StatusBadGateway, gin.H{"error": "Download-Token konnte nicht erstellt werden"})
		return
	}

	var rustResp struct {
		DownloadURL string `json:"download_url"`
		ExpiresAt   int64  `json:"expires_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rustResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ungueltige Antwort vom Storage-Service"})
		return
	}

	h.logAudit(c.Request.Context(), nil, "share_download", &shareID, c.ClientIP(), "ok")

	c.JSON(http.StatusOK, gin.H{
		"download_url": rustResp.DownloadURL,
		"filename":     filename,
		"expires_at":   rustResp.ExpiresAt,
	})
}

func (h *PublicShareHandler) logAudit(ctx context.Context, userID *uuid.UUID, action string, targetID *uuid.UUID, ip, result string) {
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
