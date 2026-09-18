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

func TestFiles_RLS_UserIsolation(t *testing.T) {
	pool := getRoutesTestDB(t)
	if pool == nil {
		t.Skip("Postgres nicht verfuegbar")
		return
	}
	defer pool.Close()

	ctx := context.Background()
	jwtSecret := "rls-test-secret-456"
	pwHash, _ := auth.HashPassword("TestPasswort123!")

	// 1. Zwei unterschiedliche Benutzer anlegen
	userA := uuid.New()
	emailA := fmt.Sprintf("user_a_%d@test.internal", time.Now().UnixNano())
	_, err := pool.Exec(ctx, "INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)", userA, emailA, pwHash)
	if err != nil {
		t.Fatalf("fehler beim anlegen von userA: %v", err)
	}
	defer func() { _, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userA) }()

	userB := uuid.New()
	emailB := fmt.Sprintf("user_b_%d@test.internal", time.Now().UnixNano())
	_, err = pool.Exec(ctx, "INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)", userB, emailB, pwHash)
	if err != nil {
		t.Fatalf("fehler beim anlegen von userB: %v", err)
	}
	defer func() { _, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userB) }()

	// 2. Dateien fuer beide Benutzer in files hinterlegen
	var (
		fileID_A uuid.UUID
		fileID_B uuid.UUID
	)

	err = db.WithUserRLS(ctx, pool, userA, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`INSERT INTO files (user_id, filename, size_bytes, mime_type, checksum_sha256, storage_path)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 RETURNING id`,
			userA, "dokument_a.pdf", 2048, "application/pdf", "hash_a", "aa/doc.enc",
		).Scan(&fileID_A)
	})
	if err != nil {
		t.Fatalf("datei A konnte nicht erstellt werden: %v", err)
	}

	err = db.WithUserRLS(ctx, pool, userB, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`INSERT INTO files (user_id, filename, size_bytes, mime_type, checksum_sha256, storage_path)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 RETURNING id`,
			userB, "geheimnis_b.docx", 4096, "application/vnd.openxmlformats", "hash_b", "bb/secret.enc",
		).Scan(&fileID_B)
	})
	if err != nil {
		t.Fatalf("datei B konnte nicht erstellt werden: %v", err)
	}

	handler := routes.NewFilesHandler(pool, "http://127.0.0.1:9999", "secret", nil)

	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(auth.Middleware(jwtSecret))
	{
		v1.GET("/files", handler.List)
		v1.GET("/files/:id/download", handler.Download)
		v1.DELETE("/files/:id", handler.Delete)
		v1.PATCH("/files/:id", handler.Rename)
	}

	tokenA, _ := auth.GenerateToken(userA, jwtSecret, 15*time.Minute)
	tokenB, _ := auth.GenerateToken(userB, jwtSecret, 15*time.Minute)

	// 3. User A darf NUR datei A sehen, NICHT datei B
	reqA, _ := http.NewRequest(http.MethodGet, "/api/v1/files", nil)
	reqA.Header.Set("Authorization", "Bearer "+tokenA)
	wA := httptest.NewRecorder()
	r.ServeHTTP(wA, reqA)

	if wA.Code != http.StatusOK {
		t.Fatalf("erwartet HTTP 200 fuer User A, erhalten %d", wA.Code)
	}

	var listA routes.FileListResponse
	_ = json.Unmarshal(wA.Body.Bytes(), &listA)
	if listA.Total != 1 || listA.Files[0].ID != fileID_A {
		t.Fatalf("RLS-Verletzung: User A sieht unerwartete Daten: %+v", listA)
	}

	// 4. User B darf NUR datei B sehen, NICHT datei A
	reqB, _ := http.NewRequest(http.MethodGet, "/api/v1/files", nil)
	reqB.Header.Set("Authorization", "Bearer "+tokenB)
	wB := httptest.NewRecorder()
	r.ServeHTTP(wB, reqB)

	var listB routes.FileListResponse
	_ = json.Unmarshal(wB.Body.Bytes(), &listB)
	if listB.Total != 1 || listB.Files[0].ID != fileID_B {
		t.Fatalf("RLS-Verletzung: User B sieht unerwartete Daten: %+v", listB)
	}

	// 5. User A darf Datei B weder herunterladen, umbenennen noch loeschen -> MUSS HTTP 404 sein!
	// Download-Versuch von A auf B
	dlReq, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/files/%s/download", fileID_B), nil)
	dlReq.Header.Set("Authorization", "Bearer "+tokenA)
	dlRec := httptest.NewRecorder()
	r.ServeHTTP(dlRec, dlReq)
	if dlRec.Code != http.StatusNotFound {
		t.Fatalf("RLS-Sicherheitsluecke: User A kann fremde Datei B abfragen! Status: %d", dlRec.Code)
	}

	// Rename-Versuch von A auf B
	renameReq, _ := http.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/files/%s", fileID_B), bytes.NewBufferString(`{"filename": "gehackt.docx"}`))
	renameReq.Header.Set("Content-Type", "application/json")
	renameReq.Header.Set("Authorization", "Bearer "+tokenA)
	renameRec := httptest.NewRecorder()
	r.ServeHTTP(renameRec, renameReq)
	if renameRec.Code != http.StatusNotFound {
		t.Fatalf("RLS-Sicherheitsluecke: User A kann fremde Datei B umbenennen! Status: %d", renameRec.Code)
	}

	// Delete-Versuch von A auf B
	delReq, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/files/%s", fileID_B), nil)
	delReq.Header.Set("Authorization", "Bearer "+tokenA)
	delRec := httptest.NewRecorder()
	r.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusNotFound {
		t.Fatalf("RLS-Sicherheitsluecke: User A kann fremde Datei B loeschen! Status: %d", delRec.Code)
	}
}
