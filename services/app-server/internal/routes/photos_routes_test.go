package routes_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/4labscloud/4labscloud/services/app-server/internal/auth"
	"github.com/4labscloud/4labscloud/services/app-server/internal/db"
	"github.com/4labscloud/4labscloud/services/app-server/internal/routes"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestPhotos_UnauthorizedThumbnail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := routes.NewFilesHandler(nil, "http://127.0.0.1:9999", "secret", nil)
	r.GET("/api/v1/files/:id/thumbnail", auth.Middleware("jwt-secret"), handler.Thumbnail)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/files/00000000-0000-0000-0000-000000000001/thumbnail", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("erwartet HTTP 401 ohne JWT, erhalten %d", w.Code)
	}
}

func TestTags_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := routes.NewTagsHandler(nil, nil)
	r.GET("/api/v1/tags", auth.Middleware("jwt-secret"), handler.ListTags)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/tags", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("erwartet HTTP 401 ohne JWT, erhalten %d", w.Code)
	}
}

func TestPhotosAndTags_Lifecycle(t *testing.T) {
	pool := getRoutesTestDB(t)
	if pool == nil {
		t.Skip("Postgres nicht verfuegbar")
		return
	}
	defer pool.Close()

	ctx := context.Background()
	jwtSecret := "photos-test-secret-456"
	serviceToken := "service-secret-token-789"

	// 1. Test-Benutzer anlegen
	userID := uuid.New()
	userEmail := fmt.Sprintf("photo_user_%d@test.internal", time.Now().UnixNano())
	pwHash, _ := auth.HashPassword("SicheresPasswort123!")
	_, err := pool.Exec(ctx, "INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)", userID, userEmail, pwHash)
	if err != nil {
		t.Fatalf("test-user konnte nicht angelegt werden: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
	}()

	// 2. Mock Rust Upload-Service fuer Thumbnails & Downloads
	mockRust := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+serviceToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Thumbnail Abfrage
		if r.Method == http.MethodGet && len(r.URL.Path) > len("/internal/thumbs/") {
			w.Header().Set("Content-Type", "image/jpeg")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("fake-jpeg-thumbnail-bytes"))
			return
		}

		// Download Token
		if r.Method == http.MethodGet && r.URL.Path == "/internal/files/download" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"download_url": "/api/v1/uploads/tus/mock/download?token=mocktoken",
				"expires_at":   time.Now().Add(5 * time.Minute).Unix(),
			})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockRust.Close()

	// 3. Router initialisieren
	gin.SetMode(gin.TestMode)
	r := gin.New()
	filesHandler := routes.NewFilesHandler(pool, mockRust.URL, serviceToken, nil)
	tagsHandler := routes.NewTagsHandler(pool, nil)
	publicShareHandler := routes.NewPublicShareHandler(pool, mockRust.URL, serviceToken, nil)

	v1 := r.Group("/api/v1")
	v1.Use(auth.Middleware(jwtSecret))
	{
		v1.GET("/files", filesHandler.List)
		v1.GET("/files/:id/thumbnail", filesHandler.Thumbnail)
		v1.POST("/files/:id/tags", tagsHandler.AssignTag)
		v1.DELETE("/files/:id/tags/:tag_id", tagsHandler.RemoveTag)
		v1.GET("/tags", tagsHandler.ListTags)
		v1.POST("/tags", tagsHandler.CreateTag)
		v1.GET("/tags/:id/files", tagsHandler.ListFilesByTag)
	}

	r.GET("/api/v1/shares/:token", publicShareHandler.GetShareInfo)
	r.GET("/api/v1/shares/:token/download", publicShareHandler.DownloadShare)

	// Test-Dateien anlegen (Bild und Dokument)
	photoID := uuid.New()
	thumbPath := "00/photo.enc"
	origPath := "00/photo_orig.enc"
	width := 1920
	height := 1080

	err = db.WithUserRLS(ctx, pool, userID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO files (id, user_id, filename, size_bytes, mime_type, checksum_sha256, storage_path, thumbnail_path, width, height)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			photoID, userID, "urlaub.jpg", 1024000, "image/jpeg", "fakehash1", origPath, thumbPath, width, height,
		)
		return err
	})
	if err != nil {
		t.Fatalf("test-bild konnte nicht eingefuegt werden: %v", err)
	}

	docID := uuid.New()
	err = db.WithUserRLS(ctx, pool, userID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO files (id, user_id, filename, size_bytes, mime_type, checksum_sha256, storage_path)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			docID, userID, "vertrag.pdf", 500000, "application/pdf", "fakehash2", "00/doc.enc",
		)
		return err
	})
	if err != nil {
		t.Fatalf("test-dokument konnte nicht eingefuegt werden: %v", err)
	}

	token, _ := auth.GenerateToken(userID, jwtSecret, 15*time.Minute)

	// 4. Test: Filter nach Bildern (type=images)
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/files?type=images", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("erwartet HTTP 200 bei files?type=images, erhalten %d", w.Code)
	}

	var listResp routes.FileListResponse
	_ = json.Unmarshal(w.Body.Bytes(), &listResp)
	if listResp.Total != 1 || len(listResp.Files) != 1 {
		t.Fatalf("erwartet 1 Bild, erhalten %d", listResp.Total)
	}
	if listResp.Files[0].ID != photoID || listResp.Files[0].ThumbnailPath == nil {
		t.Fatalf("ungueltige Bild-Metadaten: %+v", listResp.Files[0])
	}

	// 5. Test: Thumbnail abrufen (GET /api/v1/files/:id/thumbnail)
	req, _ = http.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/files/%s/thumbnail", photoID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("erwartet HTTP 200 bei Thumbnail, erhalten %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("erwartet Content-Type image/jpeg, erhalten %s", w.Header().Get("Content-Type"))
	}
	if w.Header().Get("Cache-Control") != "private, max-age=86400" {
		t.Fatalf("erwartet Cache-Control private, erhalten %s", w.Header().Get("Cache-Control"))
	}

	// 6. Test: Tag erstellen und Album zuweisen
	tagReqBody := map[string]any{
		"name":  "Sommer 2025",
		"color": "#3B82F6",
	}
	tagBytes, _ := json.Marshal(tagReqBody)
	req, _ = http.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/files/%s/tags", photoID), bytes.NewBuffer(tagBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("erwartet HTTP 200 bei AssignTag, erhalten %d (Body: %s)", w.Code, w.Body.String())
	}

	var assignResp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &assignResp)
	tagIDStr, ok := assignResp["tag_id"].(string)
	if !ok || tagIDStr == "" {
		t.Fatalf("ungueltige Tag-ID in Antwort: %+v", assignResp)
	}

	// 7. Test: Tags auflisten (GET /api/v1/tags)
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/tags", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("erwartet HTTP 200 bei ListTags, erhalten %d", w.Code)
	}

	// 8. Test: Album abfragen (GET /api/v1/tags/:id/files)
	req, _ = http.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/tags/%s/files", tagIDStr), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("erwartet HTTP 200 bei ListFilesByTag, erhalten %d", w.Code)
	}
	var albumResp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &albumResp)
	filesInAlbum, _ := albumResp["files"].([]any)
	if len(filesInAlbum) != 1 {
		t.Fatalf("erwartet 1 Datei im Album, erhalten %d", len(filesInAlbum))
	}

	// 9. Test: Tag von Datei entfernen (DELETE /api/v1/files/:id/tags/:tag_id)
	req, _ = http.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/files/%s/tags/%s", photoID, tagIDStr), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("erwartet HTTP 200 bei RemoveTag, erhalten %d", w.Code)
	}
}
