package routes

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/4labscloud/4labscloud/services/app-server/internal/audit"
	"github.com/4labscloud/4labscloud/services/app-server/internal/auth"
	"github.com/4labscloud/4labscloud/services/app-server/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FileItem repraesentiert die Datei-Metadaten fuer die Rueckgabe.
type FileItem struct {
	ID            uuid.UUID  `json:"id"`
	Filename      string     `json:"filename"`
	SizeBytes     int64      `json:"size_bytes"`
	MimeType      string     `json:"mime_type"`
	ThumbnailPath *string    `json:"thumbnail_path,omitempty"`
	Width         *int       `json:"width,omitempty"`
	Height        *int       `json:"height,omitempty"`
	TakenAt       *time.Time `json:"taken_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	LocationName  *string    `json:"location_name,omitempty"`
	GPSLat        *float64   `json:"gps_lat,omitempty"`
	GPSLon        *float64   `json:"gps_lon,omitempty"`
}

// FileListResponse enthaelt paginierte Dateilisten.
type FileListResponse struct {
	Files []FileItem `json:"files"`
	Total int64      `json:"total"`
	Page  int        `json:"page"`
	Limit int        `json:"limit"`
}

// RenameFileRequest definiert den neuen Dateinamen.
type RenameFileRequest struct {
	Filename string `json:"filename" binding:"required,min=1,max=255"`
}

// CreateShareRequest definiert Parameter zur Freigabe-Erstellung.
type CreateShareRequest struct {
	ExpiresDays int    `json:"expires_days"`
	Password    string `json:"password"`
}

// FilesHandler verwaltet Datei-Metadaten, Downloads, Umbenennungen und Freigaben.
type FilesHandler struct {
	dbPool           *pgxpool.Pool
	uploadServiceURL string
	serviceToken     string
	auditLogger      *audit.Logger
	httpClient       *http.Client
}

// NewFilesHandler initialisiert den Files-Handler.
func NewFilesHandler(dbPool *pgxpool.Pool, uploadServiceURL, serviceToken string, auditLogger *audit.Logger) *FilesHandler {
	return &FilesHandler{
		dbPool:           dbPool,
		uploadServiceURL: uploadServiceURL,
		serviceToken:     serviceToken,
		auditLogger:      auditLogger,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// List liefert die Dateien des authentifizierten Benutzers mit Pagination und Sortierung.
func (h *FilesHandler) List(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	page := 1
	limit := 50
	if p := c.Query("page"); p != "" {
		_, _ = fmt.Sscan(p, &page)
		if page < 1 {
			page = 1
		}
	}
	if l := c.Query("limit"); l != "" {
		_, _ = fmt.Sscan(l, &limit)
		if limit < 1 || limit > 100 {
			limit = 50
		}
	}

	sortField := "created_at"
	switch c.Query("sort") {
	case "name":
		sortField = "filename"
	case "size":
		sortField = "size_bytes"
	case "date":
		sortField = "created_at"
	}

	orderDir := "DESC"
	if strings.ToLower(c.Query("order")) == "asc" {
		orderDir = "ASC"
	}

	typeFilter := strings.ToLower(strings.TrimSpace(c.Query("type")))
	filter := strings.TrimSpace(c.Query("filter"))
	offset := (page - 1) * limit

	var (
		files []FileItem
		total int64
	)

	err := db.WithUserRLS(c.Request.Context(), h.dbPool, userID, func(tx pgx.Tx) error {
		// 1. Where-Bedingungen aufbauen (Benutzer-Isolation sicherstellen)
		whereClauses := []string{"user_id = $1"}
		args := []any{userID}
		argIdx := 2

		if filter != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("filename ILIKE $%d", argIdx))
			args = append(args, "%"+filter+"%")
			argIdx++
		}

		if typeFilter == "image" || typeFilter == "images" {
			whereClauses = append(whereClauses, fmt.Sprintf("mime_type LIKE $%d", argIdx))
			args = append(args, "image/%")
			argIdx++
		} else if typeFilter == "video" || typeFilter == "videos" {
			whereClauses = append(whereClauses, fmt.Sprintf("mime_type LIKE $%d", argIdx))
			args = append(args, "video/%")
			argIdx++
		} else if typeFilter == "media" {
			whereClauses = append(whereClauses, fmt.Sprintf("(mime_type LIKE $%d OR mime_type LIKE $%d)", argIdx, argIdx+1))
			args = append(args, "image/%", "video/%")
			argIdx += 2
		}

		whereSQL := strings.Join(whereClauses, " AND ")

		// 2. Gesamtzahl ermitteln
		countQuery := fmt.Sprintf("SELECT count(*) FROM files WHERE %s", whereSQL)
		if err := tx.QueryRow(c.Request.Context(), countQuery, args...).Scan(&total); err != nil {
			return err
		}

		// 3. Daten abfragen inkl. Thumbnail- und EXIF-Spalten
		dataQuery := fmt.Sprintf(
			"SELECT id, filename, size_bytes, coalesce(mime_type, 'application/octet-stream'), thumbnail_path, width, height, taken_at, created_at, location_name, gps_lat, gps_lon FROM files WHERE %s ORDER BY %s %s LIMIT %d OFFSET %d",
			whereSQL, sortField, orderDir, limit, offset,
		)

		rows, err := tx.Query(c.Request.Context(), dataQuery, args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var f FileItem
			if err := rows.Scan(&f.ID, &f.Filename, &f.SizeBytes, &f.MimeType, &f.ThumbnailPath, &f.Width, &f.Height, &f.TakenAt, &f.CreatedAt, &f.LocationName, &f.GPSLat, &f.GPSLon); err != nil {
				return err
			}
			files = append(files, f)
		}

		return rows.Err()
	})

	if err != nil {
		slog.ErrorContext(c.Request.Context(), "fehler beim abfragen der dateien", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Dateiliste konnte nicht geladen werden"})
		return
	}

	if files == nil {
		files = []FileItem{}
	}

	c.JSON(http.StatusOK, FileListResponse{
		Files: files,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

// Download fordert eine signierte Download-URL vom Rust-Service an und liefert diese zurueck.
func (h *FilesHandler) Download(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	fileIDStr := c.Param("id")
	fileID, err := uuid.Parse(fileIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungueltige Datei-ID"})
		return
	}

	var (
		filename    string
		storagePath string
	)

	err = db.WithUserRLS(c.Request.Context(), h.dbPool, userID, func(tx pgx.Tx) error {
		return tx.QueryRow(c.Request.Context(),
			"SELECT filename, storage_path FROM files WHERE id = $1 AND user_id = $2", fileID, userID).
			Scan(&filename, &storagePath)
	})

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Datei nicht gefunden"})
		return
	}

	// Rust intern aufrufen: GET /internal/files/:id/download?storage_path=...&filename=...&user_id=...
	rustURL := fmt.Sprintf("%s/internal/files/%s/download?storage_path=%s&filename=%s&user_id=%s",
		h.uploadServiceURL,
		fileID.String(),
		url.QueryEscape(storagePath),
		url.QueryEscape(filename),
		userID.String(),
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
		slog.WarnContext(c.Request.Context(), "upload-service bei download-token nicht erreichbar", "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Storage-Service nicht erreichbar"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		slog.WarnContext(c.Request.Context(), "upload-service meldet fehler bei download-token",
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

	h.logAudit(c.Request.Context(), &userID, "file_download", &fileID, c.ClientIP(), "ok")

	c.JSON(http.StatusOK, gin.H{
		"download_url": rustResp.DownloadURL,
		"filename":     filename,
		"expires_at":   rustResp.ExpiresAt,
	})
}

// Thumbnail streamt das Thumbnail einer Datei mit Authentifizierung und RLS-Pruefung.
func (h *FilesHandler) Thumbnail(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	fileIDStr := c.Param("id")
	fileID, err := uuid.Parse(fileIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungueltige Datei-ID"})
		return
	}

	var (
		thumbnailPath *string
		uploadID      *uuid.UUID
	)
	err = db.WithUserRLS(c.Request.Context(), h.dbPool, userID, func(tx pgx.Tx) error {
		return tx.QueryRow(c.Request.Context(),
			"SELECT thumbnail_path, upload_id FROM files WHERE id = $1 AND user_id = $2", fileID, userID).
			Scan(&thumbnailPath, &uploadID)
	})

	if err != nil || thumbnailPath == nil || *thumbnailPath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Thumbnail nicht gefunden"})
		return
	}

	targetID := fileID
	if uploadID != nil && *uploadID != uuid.Nil {
		targetID = *uploadID
	}

	// Rust intern aufrufen: GET /internal/thumbs/:id?thumbnail_path=...
	rustURL := fmt.Sprintf("%s/internal/thumbs/%s?thumbnail_path=%s",
		h.uploadServiceURL,
		targetID.String(),
		url.QueryEscape(*thumbnailPath),
	)

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, rustURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler bei interner Anfrage"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+h.serviceToken)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		slog.Warn("upload-service bei thumbnail nicht erreichbar", "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Storage-Dienst nicht erreichbar"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": "Thumbnail nicht gefunden"})
		return
	}
	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Storage-Dienst meldete Fehler"})
		return
	}

	c.Header("Content-Type", resp.Header.Get("Content-Type"))
	c.Header("Cache-Control", "private, max-age=86400")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, resp.Body)
}

// Delete loescht eine Datei (DSGVO Art. 17: Hartes Loeschen).
func (h *FilesHandler) Delete(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	fileIDStr := c.Param("id")
	fileID, err := uuid.Parse(fileIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungueltige Datei-ID"})
		return
	}

	var storagePath string
	err = db.WithUserRLS(c.Request.Context(), h.dbPool, userID, func(tx pgx.Tx) error {
		return tx.QueryRow(c.Request.Context(),
			"SELECT storage_path FROM files WHERE id = $1 AND user_id = $2", fileID, userID).
			Scan(&storagePath)
	})

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Datei nicht gefunden"})
		return
	}

	// 1. Rust Storage loeschen (Reihenfolge: Rust zuerst)
	rustURL := fmt.Sprintf("%s/internal/files/%s", h.uploadServiceURL, storagePath)
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodDelete, rustURL, nil)
	if err == nil {
		req.Header.Set("Authorization", "Bearer "+h.serviceToken)
		if resp, err := h.httpClient.Do(req); err == nil {
			_ = resp.Body.Close()
		} else {
			slog.WarnContext(c.Request.Context(), "warnung: rust delete fehlgeschlagen", "error", err)
		}
	}

	// 2. Metadaten loeschen
	err = db.WithUserRLS(c.Request.Context(), h.dbPool, userID, func(tx pgx.Tx) error {
		_, err := tx.Exec(c.Request.Context(), "DELETE FROM files WHERE id = $1 AND user_id = $2", fileID, userID)
		return err
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Metadaten konnten nicht gelöscht werden"})
		return
	}

	h.logAudit(c.Request.Context(), &userID, "file_delete", &fileID, c.ClientIP(), "ok")
	c.Status(http.StatusNoContent)
}

// Rename aendert den Dateinamen.
func (h *FilesHandler) Rename(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	fileIDStr := c.Param("id")
	fileID, err := uuid.Parse(fileIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungueltige Datei-ID"})
		return
	}

	var req RenameFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungueltiger Dateiname"})
		return
	}

	cleanName := strings.TrimSpace(req.Filename)
	if cleanName == "" || strings.Contains(cleanName, "..") || strings.Contains(cleanName, "/") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungueltiger Dateiname"})
		return
	}

	var updated bool
	err = db.WithUserRLS(c.Request.Context(), h.dbPool, userID, func(tx pgx.Tx) error {
		cmdTag, err := tx.Exec(c.Request.Context(),
			"UPDATE files SET filename = $1 WHERE id = $2 AND user_id = $3", cleanName, fileID, userID)
		if err != nil {
			return err
		}
		updated = cmdTag.RowsAffected() > 0
		return nil
	})

	if err != nil || !updated {
		c.JSON(http.StatusNotFound, gin.H{"error": "Datei nicht gefunden oder Umbenennung fehlgeschlagen"})
		return
	}

	h.logAudit(c.Request.Context(), &userID, "file_rename", &fileID, c.ClientIP(), "ok")

	c.JSON(http.StatusOK, gin.H{
		"id":       fileID,
		"filename": cleanName,
		"status":   "ok",
	})
}

// CreateShare erstellt einen Freigabelink mit Ablaufdatum und optionalem Passwortschutz.
func (h *FilesHandler) CreateShare(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	fileIDStr := c.Param("id")
	fileID, err := uuid.Parse(fileIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungueltige Datei-ID"})
		return
	}

	var req CreateShareRequest
	_ = c.ShouldBindJSON(&req)

	if req.ExpiresDays <= 0 {
		req.ExpiresDays = 7 // Standard: 7 Tage
	}
	expiresAt := time.Now().Add(time.Duration(req.ExpiresDays) * 24 * time.Hour)

	var pwHash *string
	if req.Password != "" {
		hashed, err := auth.HashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Passwort konnte nicht verschluesselt werden"})
			return
		}
		pwHash = &hashed
	}

	// Token generieren (kryptographisch sicher)
	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token konnte nicht generiert werden"})
		return
	}
	token := hex.EncodeToString(tokenBytes)

	var shareID uuid.UUID
	err = db.WithUserRLS(c.Request.Context(), h.dbPool, userID, func(tx pgx.Tx) error {
		// Pruefen, ob Datei existiert und dem User gehoert
		var count int
		if err := tx.QueryRow(c.Request.Context(), "SELECT count(*) FROM files WHERE id = $1", fileID).Scan(&count); err != nil || count == 0 {
			return pgx.ErrNoRows
		}

		return tx.QueryRow(c.Request.Context(),
			`INSERT INTO shares (file_id, owner_id, token, password_hash, expires_at)
			 VALUES ($1, $2, $3, $4, $5)
			 RETURNING id`,
			fileID, userID, token, pwHash, expiresAt,
		).Scan(&shareID)
	})

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Datei nicht gefunden"})
		return
	}

	h.logAudit(c.Request.Context(), &userID, "share_create", &shareID, c.ClientIP(), "ok")

	c.JSON(http.StatusCreated, gin.H{
		"share_token": token,
		"share_url":   "/share/" + token,
		"expires_at":  expiresAt.Format(time.RFC3339),
	})
}

// ShareItem repraesentiert eine aktive Dateifreigabe.
type ShareItem struct {
	ID          uuid.UUID  `json:"id"`
	FileID      uuid.UUID  `json:"file_id"`
	Filename    string     `json:"filename"`
	SizeBytes   int64      `json:"size_bytes"`
	Token       string     `json:"token"`
	ShareURL    string     `json:"share_url"`
	HasPassword bool       `json:"has_password"`
	ExpiresAt   *time.Time `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ListShares liefert alle vom Benutzer erstellten Freigaben.
func (h *FilesHandler) ListShares(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	var shares []ShareItem
	err := db.WithUserRLS(c.Request.Context(), h.dbPool, userID, func(tx pgx.Tx) error {
		query := `SELECT s.id, s.file_id, f.filename, f.size_bytes, s.token,
		                 (s.password_hash IS NOT NULL) AS has_password,
		                 s.expires_at, s.created_at
		          FROM shares s
		          JOIN files f ON s.file_id = f.id
		          WHERE s.owner_id = $1
		          ORDER BY s.created_at DESC`
		rows, err := tx.Query(c.Request.Context(), query, userID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var item ShareItem
			if err := rows.Scan(
				&item.ID, &item.FileID, &item.Filename, &item.SizeBytes,
				&item.Token, &item.HasPassword, &item.ExpiresAt, &item.CreatedAt,
			); err != nil {
				return err
			}
			item.ShareURL = "/share/" + item.Token
			shares = append(shares, item)
		}
		return rows.Err()
	})

	if err != nil {
		slog.Error("fehler beim abrufen der shares", "error", err, "user_id", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Freigaben konnten nicht geladen werden"})
		return
	}

	if shares == nil {
		shares = []ShareItem{}
	}

	c.JSON(http.StatusOK, gin.H{"shares": shares})
}

// DeleteShare widerruft eine Freigabe.
func (h *FilesHandler) DeleteShare(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	shareIDStr := c.Param("id")
	shareID, err := uuid.Parse(shareIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Share-ID"})
		return
	}


	err = db.WithUserRLS(c.Request.Context(), h.dbPool, userID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(c.Request.Context(), "DELETE FROM shares WHERE id = $1 AND owner_id = $2", shareID, userID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return nil
	})

	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Freigabe nicht gefunden"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Freigabe konnte nicht gelöscht werden"})
		return
	}

	h.logAudit(c.Request.Context(), &userID, "share_delete", &shareID, c.ClientIP(), "ok")
	c.JSON(http.StatusOK, gin.H{"status": "deleted", "id": shareID})
}

func (h *FilesHandler) logAudit(ctx context.Context, userID *uuid.UUID, action string, targetID *uuid.UUID, ip, result string) {
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

// ExifResponse liefert die extrahierten EXIF- und Standort-Metadaten.
type ExifResponse struct {
	FileID          uuid.UUID       `json:"file_id"`
	ExifJSON        json.RawMessage `json:"exif_json"`
	LocationName    *string         `json:"location_name"`
	LocationAddress json.RawMessage `json:"location_address"`
	GPSLat          *float64        `json:"gps_lat,omitempty"`
	GPSLon          *float64        `json:"gps_lon,omitempty"`
}

// GetFileExif liefert EXIF- und Geocoding-Metadaten einer Datei (JWT & RLS geschuetzt).
func (h *FilesHandler) GetFileExif(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}
	fileIDStr := c.Param("id")
	fileID, err := uuid.Parse(fileIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Datei-ID"})
		return
	}

	var (
		exifJSON        []byte
		locationName    *string
		locationAddress []byte
		gpsLat          *float64
		gpsLon          *float64
	)

	err = db.WithUserRLS(c.Request.Context(), h.dbPool, userID, func(tx pgx.Tx) error {
		return tx.QueryRow(c.Request.Context(),
			`SELECT exif_json, location_name, location_address, gps_lat, gps_lon 
			 FROM files WHERE id = $1 AND user_id = $2`,
			fileID, userID,
		).Scan(&exifJSON, &locationName, &locationAddress, &gpsLat, &gpsLon)
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Datei nicht gefunden"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Abrufen der EXIF-Daten"})
		return
	}

	var exifRaw json.RawMessage
	if len(exifJSON) > 0 && string(exifJSON) != "null" {
		exifRaw = json.RawMessage(exifJSON)
	}
	var addrRaw json.RawMessage
	if len(locationAddress) > 0 && string(locationAddress) != "null" {
		addrRaw = json.RawMessage(locationAddress)
	}

	c.JSON(http.StatusOK, ExifResponse{
		FileID:          fileID,
		ExifJSON:        exifRaw,
		LocationName:    locationName,
		LocationAddress: addrRaw,
		GPSLat:          gpsLat,
		GPSLon:          gpsLon,
	})
}

// PhotoMapItem repraesentiert ein Foto mit Standortdaten fuer die Kartenansicht.
type PhotoMapItem struct {
	ID           uuid.UUID  `json:"id"`
	Filename     string     `json:"filename"`
	ThumbURL     string     `json:"thumb_url"`
	GPSLat       float64    `json:"gps_lat"`
	GPSLon       float64    `json:"gps_lon"`
	LocationName *string    `json:"location_name"`
	TakenAt      *time.Time `json:"taken_at,omitempty"`
}

// GetPhotosMap liefert alle Fotos des Benutzers mit GPS-Koordinaten fuer die Kartenansicht.
func (h *FilesHandler) GetPhotosMap(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	items := make([]PhotoMapItem, 0)
	err := db.WithUserRLS(c.Request.Context(), h.dbPool, userID, func(tx pgx.Tx) error {
		rows, err := tx.Query(c.Request.Context(),
			`SELECT id, filename, gps_lat, gps_lon, location_name, taken_at
			 FROM files
			 WHERE user_id = $1 AND gps_lat IS NOT NULL AND gps_lon IS NOT NULL
			 ORDER BY taken_at DESC NULLS LAST, created_at DESC`,
			userID,
		)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var item PhotoMapItem
			var lat, lon *float64
			if err := rows.Scan(&item.ID, &item.Filename, &lat, &lon, &item.LocationName, &item.TakenAt); err != nil {
				return err
			}
			if lat != nil && lon != nil {
				item.GPSLat = *lat
				item.GPSLon = *lon
				item.ThumbURL = fmt.Sprintf("/api/v1/files/%s/thumb", item.ID.String())
				items = append(items, item)
			}
		}
		return rows.Err()
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Laden der Karten-Fotos"})
		return
	}

	c.JSON(http.StatusOK, items)
}

