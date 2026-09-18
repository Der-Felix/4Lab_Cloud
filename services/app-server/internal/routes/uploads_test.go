package routes_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/4labscloud/4labscloud/services/app-server/internal/auth"
	"github.com/4labscloud/4labscloud/services/app-server/internal/routes"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestUploadInit_WithoutAuth(t *testing.T) {
	r := gin.New()
	handler := routes.NewUploadsHandler("http://127.0.0.1:9999", "secret-token", nil, nil)
	r.POST("/api/v1/uploads/init", auth.Middleware("jwt-secret"), handler.Init)

	body := []byte(`{"filename": "test.txt", "size_bytes": 1024}`)
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/uploads/init", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("erwartet HTTP 401 ohne JWT, erhalten %d", w.Code)
	}
}

func TestUploadInit_RustUnreachable(t *testing.T) {
	jwtSecret := "jwt-test-secret"
	userID := uuid.New()
	token, err := auth.GenerateToken(userID, jwtSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("token konnte nicht erstellt werden: %v", err)
	}

	// Ungueltige Zieladresse fuer Rust-Service simulieren
	handler := routes.NewUploadsHandler("http://127.0.0.1:64321", "service-token", nil, nil)

	r := gin.New()
	r.POST("/api/v1/uploads/init", auth.Middleware(jwtSecret), handler.Init)

	body := []byte(`{"filename": "test.txt", "size_bytes": 1024}`)
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/uploads/init", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("erwartet HTTP 502 bei unerreichbarem Rust-Service, erhalten %d", w.Code)
	}
}

func TestUploadInit_SuccessWithMockRust(t *testing.T) {
	serviceToken := "valid-service-token"

	mockRust := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+serviceToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"upload_id": "up-12345",
			"url":       "http://upload-service:8081/internal/uploads/up-12345",
		})
	}))
	defer mockRust.Close()

	jwtSecret := "jwt-test-secret"
	userID := uuid.New()
	token, _ := auth.GenerateToken(userID, jwtSecret, 15*time.Minute)

	handler := routes.NewUploadsHandler(mockRust.URL, serviceToken, nil, nil)

	r := gin.New()
	r.POST("/api/v1/uploads/init", auth.Middleware(jwtSecret), handler.Init)

	body := []byte(`{"filename": "test.txt", "size_bytes": 1024}`)
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/uploads/init", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("erwartet HTTP 200, erhalten %d (Body: %s)", w.Code, w.Body.String())
	}
}

func TestUploadComplete_WithoutAuth(t *testing.T) {
	r := gin.New()
	handler := routes.NewUploadsHandler("http://127.0.0.1:9999", "secret-token", nil, nil)
	r.POST("/api/v1/uploads/complete", auth.Middleware("jwt-secret"), handler.Complete)

	body := []byte(fmt.Sprintf(`{"upload_id": "%s"}`, uuid.New().String()))
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/uploads/complete", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("erwartet HTTP 401 ohne JWT, erhalten %d", w.Code)
	}
}

func TestUploadComplete_WrongUser_403AndCleanup(t *testing.T) {
	serviceToken := "valid-service-token"
	uploadID := uuid.New()
	realOwnerID := uuid.New()
	attackerID := uuid.New()

	var deleteCalled int32

	mockRust := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+serviceToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/internal/uploads/%s/status", uploadID.String()) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"upload_id":    uploadID.String(),
				"user_id":      realOwnerID.String(), // Gehoert realOwnerID
				"filename":     "secret.txt",
				"size_bytes":   500,
				"checksum":     "dummy-sha",
				"storage_path": "ab/secret.enc",
				"status":       "completed",
			})
			return
		}

		if r.Method == http.MethodDelete && r.URL.Path == fmt.Sprintf("/internal/uploads/%s", uploadID.String()) {
			atomic.AddInt32(&deleteCalled, 1)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockRust.Close()

	jwtSecret := "jwt-test-secret"
	// Token fuer attackerID generieren
	token, _ := auth.GenerateToken(attackerID, jwtSecret, 15*time.Minute)

	handler := routes.NewUploadsHandler(mockRust.URL, serviceToken, nil, nil)

	r := gin.New()
	r.POST("/api/v1/uploads/complete", auth.Middleware(jwtSecret), handler.Complete)

	body := []byte(fmt.Sprintf(`{"upload_id": "%s"}`, uploadID.String()))
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/uploads/complete", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("erwartet HTTP 403 Forbidden bei falschem User, erhalten %d", w.Code)
	}

	if atomic.LoadInt32(&deleteCalled) != 1 {
		t.Fatalf("erwartet, dass Rust DELETE genau 1x aufgerufen wird, erhalten: %d", deleteCalled)
	}
}

func TestUploadComplete_IncompleteUpload_400(t *testing.T) {
	serviceToken := "valid-service-token"
	uploadID := uuid.New()
	userID := uuid.New()

	mockRust := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+serviceToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/internal/uploads/%s/status", uploadID.String()) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"upload_id":    uploadID.String(),
				"user_id":      userID.String(),
				"filename":     "partial.txt",
				"size_bytes":   1000,
				"checksum":     nil,
				"storage_path": "ab/partial.enc",
				"status":       "in_progress", // Noch nicht abgeschlossen
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockRust.Close()

	jwtSecret := "jwt-test-secret"
	token, _ := auth.GenerateToken(userID, jwtSecret, 15*time.Minute)

	handler := routes.NewUploadsHandler(mockRust.URL, serviceToken, nil, nil)

	r := gin.New()
	r.POST("/api/v1/uploads/complete", auth.Middleware(jwtSecret), handler.Complete)

	body := []byte(fmt.Sprintf(`{"upload_id": "%s"}`, uploadID.String()))
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/uploads/complete", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("erwartet HTTP 400 Bad Request bei unvollstaendigem Upload, erhalten %d", w.Code)
	}
}

func TestUploadComplete_Success_200(t *testing.T) {
	pool := getRoutesTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	userID := uuid.New()
	email := fmt.Sprintf("upload_user_%d@test.internal", time.Now().UnixNano())
	pwHash, _ := auth.HashPassword("TestPW123!")
	_, err := pool.Exec(ctx, "INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)", userID, email, pwHash)
	if err != nil {
		t.Fatalf("fehler beim anlegen des benutzers: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", userID)
	}()

	serviceToken := "valid-service-token"
	uploadID := uuid.New()
	shaChecksum := "648c445ea1a157ff97e57d314eba636ea1b22119f2b9895abf3eae2aa77465a7"

	mockRust := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+serviceToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/internal/uploads/%s/status", uploadID.String()) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"upload_id":    uploadID.String(),
				"user_id":      userID.String(),
				"filename":     "testbild.jpeg",
				"size_bytes":   1024,
				"checksum":     shaChecksum,
				"storage_path": "ab/testbild.enc",
				"status":       "completed",
			})
			return
		}

		if r.Method == http.MethodDelete && r.URL.Path == fmt.Sprintf("/internal/uploads/%s", uploadID.String()) {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockRust.Close()

	jwtSecret := "jwt-test-secret"
	token, _ := auth.GenerateToken(userID, jwtSecret, 15*time.Minute)

	handler := routes.NewUploadsHandler(mockRust.URL, serviceToken, pool, nil)

	r := gin.New()
	r.POST("/api/v1/uploads/complete", auth.Middleware(jwtSecret), handler.Complete)

	body := []byte(fmt.Sprintf(`{"upload_id": "%s"}`, uploadID.String()))
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/uploads/complete", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("erwartet HTTP 200 bei erfolgreichem Complete, erhalten %d (Body: %s)", w.Code, w.Body.String())
	}

	var resp routes.UploadCompleteResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("fehler beim parsen der antwort: %v", err)
	}
	if resp.Status != "ok" || resp.Filename != "testbild.jpeg" {
		t.Fatalf("unerwartete Antwort: %+v", resp)
	}

	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM files WHERE id = $1", resp.FileID)
	}()
}
