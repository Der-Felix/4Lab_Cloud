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

func TestFiles_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := routes.NewFilesHandler(nil, "http://127.0.0.1:9999", "secret-token", nil)
	r.GET("/api/v1/files", auth.Middleware("jwt-secret"), handler.List)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/files", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("erwartet HTTP 401 ohne JWT, erhalten %d", w.Code)
	}
}

func TestFiles_RoutesLifecycle(t *testing.T) {
	pool := getRoutesTestDB(t)
	if pool == nil {
		t.Skip("Postgres nicht verfuegbar")
		return
	}
	defer pool.Close()

	ctx := context.Background()
	jwtSecret := "files-test-secret-123"
	serviceToken := "rust-service-token-xyz"

	// Test-Benutzer anlegen
	userID := uuid.New()
	userEmail := fmt.Sprintf("files_user_%d@test.internal", time.Now().UnixNano())
	pwHash, _ := auth.HashPassword("TestPasswort123!")
	_, err := pool.Exec(ctx, "INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)", userID, userEmail, pwHash)
	if err != nil {
		t.Fatalf("test-user konnte nicht angelegt werden: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
	}()

	// Mock Rust Upload-Service
	mockRustServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+serviceToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/internal/files/%s/download", r.URL.Query().Get("file_id")) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"download_url": "/api/v1/uploads/tus/mock/download?token=mocktoken123",
				"expires_at":   time.Now().Add(5 * time.Minute).Unix(),
			})
			return
		}

		if r.Method == http.MethodGet && (r.URL.Path == fmt.Sprintf("/internal/files/%s/download", r.URL.Path[len("/internal/files/"):len(r.URL.Path)-len("/download")])) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"download_url": "/api/v1/uploads/tus/mock/download?token=mocktoken123",
				"expires_at":   time.Now().Add(5 * time.Minute).Unix(),
			})
			return
		}

		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer mockRustServer.Close()

	handler := routes.NewFilesHandler(pool, mockRustServer.URL, serviceToken, nil)

	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(auth.Middleware(jwtSecret))
	{
		v1.GET("/files", handler.List)
		v1.GET("/files/:id/download", handler.Download)
		v1.DELETE("/files/:id", handler.Delete)
		v1.PATCH("/files/:id", handler.Rename)
		v1.POST("/files/:id/share", handler.CreateShare)
	}

	token, _ := auth.GenerateToken(userID, jwtSecret, 15*time.Minute)

	// 1. Testdatei fuer User in DB anlegen
	var fileID uuid.UUID
	err = db.WithUserRLS(ctx, pool, userID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`INSERT INTO files (user_id, filename, size_bytes, mime_type, checksum_sha256, storage_path)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 RETURNING id`,
			userID, "beispiel.pdf", 1024, "application/pdf", "checksum123", "ab/test.enc",
		).Scan(&fileID)
	})
	if err != nil {
		t.Fatalf("fehler beim anlegen der datei: %v", err)
	}

	// 2. Dateiliste abfragen
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/files?sort=date&order=desc", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("erwartet HTTP 200 bei Dateiliste, erhalten %d", w.Code)
	}

	var listResp routes.FileListResponse
	_ = json.Unmarshal(w.Body.Bytes(), &listResp)
	if listResp.Total != 1 || len(listResp.Files) != 1 || listResp.Files[0].Filename != "beispiel.pdf" {
		t.Fatalf("unerwartete Dateiliste: %+v", listResp)
	}

	// 3. Download-URL anfordern
	req, _ = http.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/files/%s/download", fileID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("erwartet HTTP 200 bei Download-URL, erhalten %d (Body: %s)", w.Code, w.Body.String())
	}

	var dlResp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &dlResp)
	if dlResp["download_url"] == nil || dlResp["filename"] != "beispiel.pdf" {
		t.Fatalf("ungueltige Download-Antwort: %+v", dlResp)
	}

	// 4. Datei umbenennen
	renameBody := []byte(`{"filename": "neuer_name.pdf"}`)
	req, _ = http.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/files/%s", fileID), bytes.NewBuffer(renameBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("erwartet HTTP 200 bei Rename, erhalten %d", w.Code)
	}

	// 5. Share-Link erstellen
	shareBody := []byte(`{"expires_days": 14, "password": "SharePasswort123!"}`)
	req, _ = http.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/files/%s/share", fileID), bytes.NewBuffer(shareBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("erwartet HTTP 201 bei Share, erhalten %d (Body: %s)", w.Code, w.Body.String())
	}

	var shareResp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &shareResp)
	if shareResp["share_token"] == nil || shareResp["share_url"] == nil {
		t.Fatalf("ungueltige Share-Antwort: %+v", shareResp)
	}

	// 6. Datei loeschen
	req, _ = http.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/files/%s", fileID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("erwartet HTTP 204 bei Delete, erhalten %d", w.Code)
	}

	// 7. Pruefen, dass Datei nicht mehr in der Liste ist
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/files", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var listAfter routes.FileListResponse
	_ = json.Unmarshal(w.Body.Bytes(), &listAfter)
	if listAfter.Total != 0 || len(listAfter.Files) != 0 {
		t.Fatalf("datei nach Delete immer noch vorhanden: %+v", listAfter)
	}
}
