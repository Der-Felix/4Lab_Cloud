package routes_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/4labscloud/4labscloud/services/app-server/internal/audit"
	"github.com/4labscloud/4labscloud/services/app-server/internal/auth"
	"github.com/4labscloud/4labscloud/services/app-server/internal/routes"
	"github.com/4labscloud/4labscloud/services/app-server/internal/workers"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func getRoutesTestRedis() *redis.Client {
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := os.Getenv("REDIS_PORT")
	if port == "" {
		port = "6379"
	}
	pass := os.Getenv("REDIS_PASSWORD")
	if pass == "" {
		pass = "nCWjCMAO9MYsI3unBAA6Vxbnnan7Ue"
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     host + ":" + port,
		Password: pass,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil
	}
	return rdb
}

// setupMockRustService simuliert den Rust-Upload- und Export-Service fuer Tests.
func setupMockRustService(t *testing.T, failDelete bool) *httptest.Server {
	mux := http.NewServeMux()

	// DELETE /internal/files/...
	mux.HandleFunc("/internal/files/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if failDelete {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"disk failure"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"deleted"}`))
	})

	// POST /internal/exports & DELETE /internal/exports/... & GET /internal/exports/.../download
	mux.HandleFunc("/internal/exports", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"path":"exports/test/job.zip","size_bytes":2048}`))
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/internal/exports/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			if failDelete {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"deleted"}`))
			return
		}
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/zip")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("PK\x03\x04mockzipcontent"))
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	return httptest.NewServer(mux)
}

func setupTestUser(t *testing.T, dbPool *pgxpool.Pool, email string, password string, isAdmin bool) (uuid.UUID, string) {
	ctx := context.Background()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword fehlgeschlagen: %v", err)
	}

	var userID uuid.UUID
	err = dbPool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, is_admin, mfa_enabled)
		 VALUES ($1, $2, $3, false)
		 RETURNING id`,
		email, hash, isAdmin,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("User anlegen fehlgeschlagen: %v", err)
	}

	token, err := auth.GenerateToken(userID, "test-jwt-secret-1234567890-32bytes!", 15*time.Minute)
	if err != nil {
		t.Fatalf("GenerateToken fehlgeschlagen: %v", err)
	}

	return userID, token
}

// TestDSGVO_UserSelfDelete testet DELETE /api/v1/users/me inkl. Datenloeschung und Pseudonymisierung.
func TestDSGVO_UserSelfDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dbPool := getRoutesTestDB(t)
	if dbPool == nil {
		return
	}
	defer dbPool.Close()

	redisClient := getRoutesTestRedis()
	rustServer := setupMockRustService(t, false)
	defer rustServer.Close()

	jwtSecret := "test-jwt-secret-1234567890-32bytes!"
	mfaKey := make([]byte, 32)
	auditKey := make([]byte, 32)
	for i := range auditKey {
		auditKey[i] = byte(i + 42)
	}

	auditLogger := audit.NewLogger(dbPool)
	userHandler := routes.NewUserHandler(dbPool, redisClient, auditLogger, rustServer.URL, "test-service-token", mfaKey, auditKey, 50)

	router := gin.New()
	v1 := router.Group("/api/v1")
	v1.Use(auth.Middleware(jwtSecret, redisClient))
	v1.DELETE("/users/me", userHandler.DeleteMe)

	email := fmt.Sprintf("selfdelete_%d@4labs.example", time.Now().UnixNano())
	userID, token := setupTestUser(t, dbPool, email, "SuperSecurePass123!", false)

	// Datei und Audit-Log fuer den User anlegen
	ctx := context.Background()
	_, err := dbPool.Exec(ctx,
		`INSERT INTO files (user_id, filename, size_bytes, checksum_sha256, storage_path)
		 VALUES ($1, 'geheim.pdf', 1024, 'abc123', 'storage/geheim.enc')`,
		userID,
	)
	if err != nil {
		t.Fatalf("Datei anlegen fehlgeschlagen: %v", err)
	}

	auditLogger.Log(ctx, audit.Entry{
		UserID: &userID,
		Action: "file_upload",
		Result: "ok",
	})

	// 1. Falsches Passwort -> 401
	bodyBad, _ := json.Marshal(map[string]string{"password": "WrongPassword!"})
	reqBad, _ := http.NewRequest(http.MethodDelete, "/api/v1/users/me", bytes.NewReader(bodyBad))
	reqBad.Header.Set("Authorization", "Bearer "+token)
	reqBad.Header.Set("Content-Type", "application/json")
	wBad := httptest.NewRecorder()
	router.ServeHTTP(wBad, reqBad)
	if wBad.Code != http.StatusUnauthorized {
		t.Fatalf("Erwartet 401 bei falschem Passwort, erhalten: %d", wBad.Code)
	}

	// 2. Richtiges Passwort -> 204 No Content
	bodyGood, _ := json.Marshal(map[string]string{"password": "SuperSecurePass123!"})
	reqGood, _ := http.NewRequest(http.MethodDelete, "/api/v1/users/me", bytes.NewReader(bodyGood))
	reqGood.Header.Set("Authorization", "Bearer "+token)
	reqGood.Header.Set("Content-Type", "application/json")
	wGood := httptest.NewRecorder()
	router.ServeHTTP(wGood, reqGood)
	if wGood.Code != http.StatusNoContent {
		t.Fatalf("Erwartet 204 bei Selbstloeschung, erhalten: %d (%s)", wGood.Code, wGood.Body.String())
	}

	// 3. Pruefen: User existiert nicht mehr
	var exists bool
	_ = dbPool.QueryRow(ctx, "SELECT true FROM users WHERE id = $1", userID).Scan(&exists)
	if exists {
		t.Fatal("User sollte nach Selbstloeschung nicht mehr in der DB existieren")
	}

	// 4. Pruefen: Dateien wurden geloescht (Kaskade)
	var fileExists bool
	_ = dbPool.QueryRow(ctx, "SELECT true FROM files WHERE user_id = $1", userID).Scan(&fileExists)
	if fileExists {
		t.Fatal("Dateien des Users sollten geloescht worden sein")
	}

	// 5. Pruefen: Audit-Log wurde pseudonymisiert (user_id IS NULL und pseudonym_hash gesetzt)
	expectedPseudo := audit.CalculatePseudonym(userID, auditKey)
	var (
		loggedPseudo *string
		loggedUserID *uuid.UUID
	)
	err = dbPool.QueryRow(ctx,
		"SELECT user_id, pseudonym_hash FROM audit_log WHERE pseudonym_hash = $1 LIMIT 1",
		expectedPseudo,
	).Scan(&loggedUserID, &loggedPseudo)
	if err != nil {
		t.Fatalf("Pseudonymisiertes Audit-Log nicht gefunden: %v", err)
	}
	if loggedUserID != nil {
		t.Fatalf("user_id im pseudonymisierten Audit-Log muss NULL sein, ist: %v", loggedUserID)
	}
	if loggedPseudo == nil || *loggedPseudo != expectedPseudo {
		t.Fatalf("Pseudonym-Hash stimmt nicht ueberein: %v vs %s", loggedPseudo, expectedPseudo)
	}
}

// TestDSGVO_AdminDelete testet DELETE /api/v1/admin/users/:id inkl. Selbstloeschungsverbot.
func TestDSGVO_AdminDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dbPool := getRoutesTestDB(t)
	if dbPool == nil {
		return
	}
	defer dbPool.Close()

	redisClient := getRoutesTestRedis()
	rustServer := setupMockRustService(t, false)
	defer rustServer.Close()

	jwtSecret := "test-jwt-secret-1234567890-32bytes!"
	mfaKey := make([]byte, 32)
	auditKey := make([]byte, 32)

	auditLogger := audit.NewLogger(dbPool)
	userHandler := routes.NewUserHandler(dbPool, redisClient, auditLogger, rustServer.URL, "test-service-token", mfaKey, auditKey, 50)

	router := gin.New()
	admin := router.Group("/api/v1/admin")
	admin.Use(auth.Middleware(jwtSecret, redisClient))
	admin.Use(auth.RequireAdmin(dbPool))
	admin.DELETE("/users/:id", userHandler.AdminDeleteUser)

	adminEmail := fmt.Sprintf("admin_%d@4labs.example", time.Now().UnixNano())
	adminID, adminToken := setupTestUser(t, dbPool, adminEmail, "AdminSecure123!", true)

	targetEmail := fmt.Sprintf("target_%d@4labs.example", time.Now().UnixNano())
	targetID, _ := setupTestUser(t, dbPool, targetEmail, "UserSecure123!", false)

	// 1. Admin versucht sich selbst zu loeschen -> 403 Forbidden
	reqSelf, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/admin/users/%s", adminID), nil)
	reqSelf.Header.Set("Authorization", "Bearer "+adminToken)
	wSelf := httptest.NewRecorder()
	router.ServeHTTP(wSelf, reqSelf)
	if wSelf.Code != http.StatusForbidden {
		t.Fatalf("Erwartet 403 bei Admin-Selbstloeschung, erhalten: %d", wSelf.Code)
	}

	// 2. Admin loescht fremden User -> 204 No Content
	reqTarget, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/admin/users/%s", targetID), nil)
	reqTarget.Header.Set("Authorization", "Bearer "+adminToken)
	wTarget := httptest.NewRecorder()
	router.ServeHTTP(wTarget, reqTarget)
	if wTarget.Code != http.StatusNoContent {
		t.Fatalf("Erwartet 204 bei Admin-Loeschung von Fremduser, erhalten: %d", wTarget.Code)
	}

	// 3. Ziel-User existiert nicht mehr
	var exists bool
	ctx := context.Background()
	_ = dbPool.QueryRow(ctx, "SELECT true FROM users WHERE id = $1", targetID).Scan(&exists)
	if exists {
		t.Fatal("Ziel-User sollte nach Admin-Loeschung nicht mehr existieren")
	}
}

// TestDSGVO_SessionManagement testet GET und DELETE auf /api/v1/auth/sessions.
func TestDSGVO_SessionManagement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dbPool := getRoutesTestDB(t)
	if dbPool == nil {
		return
	}
	defer dbPool.Close()

	jwtSecret := "test-jwt-secret-1234567890-32bytes!"
	mfaKey := make([]byte, 32)
	auditLogger := audit.NewLogger(dbPool)
	authHandler := routes.NewAuthHandler(dbPool, nil, auditLogger, jwtSecret, 15, mfaKey)

	router := gin.New()
	v1 := router.Group("/api/v1")
	v1.Use(auth.Middleware(jwtSecret))
	v1.GET("/auth/sessions", authHandler.ListSessions)
	v1.DELETE("/auth/sessions/:id", authHandler.RevokeSession)
	v1.DELETE("/auth/sessions", authHandler.RevokeOtherSessions)

	email := fmt.Sprintf("session_user_%d@4labs.example", time.Now().UnixNano())
	userID, token := setupTestUser(t, dbPool, email, "Pass12345678!", false)

	ctx := context.Background()

	// 3 Sessions erzeugen
	tok1, err := auth.CreateRefreshTokenWithMeta(ctx, dbPool, userID, 7*24*time.Hour, "Mozilla/5.0 Firefox", "192.168.1.50")
	if err != nil {
		t.Fatalf("Session 1 anlegen fehlgeschlagen: %v", err)
	}
	_, err = auth.CreateRefreshTokenWithMeta(ctx, dbPool, userID, 7*24*time.Hour, "Chrome/120.0", "10.0.0.12")
	if err != nil {
		t.Fatalf("Session 2 anlegen fehlgeschlagen: %v", err)
	}
	_, err = auth.CreateRefreshTokenWithMeta(ctx, dbPool, userID, 7*24*time.Hour, "Safari/17.0", "172.16.0.8")
	if err != nil {
		t.Fatalf("Session 3 anlegen fehlgeschlagen: %v", err)
	}

	// 1. Alle Sessions listen -> 3 Stueck
	reqList, _ := http.NewRequest(http.MethodGet, "/api/v1/auth/sessions", nil)
	reqList.Header.Set("Authorization", "Bearer "+token)
	wList := httptest.NewRecorder()
	router.ServeHTTP(wList, reqList)
	if wList.Code != http.StatusOK {
		t.Fatalf("Erwartet 200 bei ListSessions, erhalten: %d", wList.Code)
	}

	var sessions []map[string]any
	if err := json.Unmarshal(wList.Body.Bytes(), &sessions); err != nil {
		t.Fatalf("Fehler beim Dekodieren der Sessions: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("Erwartet 3 aktive Sessions, erhalten: %d", len(sessions))
	}

	// IP-Kuerzung pruefen (192.168.1.0/24)
	foundTruncatedIP := false
	for _, s := range sessions {
		if s["ip"] == "192.168.1.0/24" {
			foundTruncatedIP = true
			break
		}
	}
	if !foundTruncatedIP {
		t.Fatal("IP-Kuerzung auf /24 nicht gefunden")
	}

	// 2. Einzelne Session revoken -> 204
	session2ID := sessions[1]["id"].(string)
	reqRevoke, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/auth/sessions/%s", session2ID), nil)
	reqRevoke.Header.Set("Authorization", "Bearer "+token)
	wRevoke := httptest.NewRecorder()
	router.ServeHTTP(wRevoke, reqRevoke)
	if wRevoke.Code != http.StatusNoContent {
		t.Fatalf("Erwartet 204 bei RevokeSession, erhalten: %d", wRevoke.Code)
	}

	// 3. Alle ANDEREN Sessions beenden, mit Cookie tok1
	reqRevokeOthers, _ := http.NewRequest(http.MethodDelete, "/api/v1/auth/sessions", nil)
	reqRevokeOthers.Header.Set("Authorization", "Bearer "+token)
	reqRevokeOthers.AddCookie(&http.Cookie{Name: "refresh_token", Value: tok1})
	wRevokeOthers := httptest.NewRecorder()
	router.ServeHTTP(wRevokeOthers, reqRevokeOthers)
	if wRevokeOthers.Code != http.StatusNoContent {
		t.Fatalf("Erwartet 204 bei RevokeOtherSessions, erhalten: %d", wRevokeOthers.Code)
	}

	// Pruefen: Nur noch 1 Session ist aktiv
	activeSessions, err := auth.ListActiveSessions(ctx, dbPool, userID)
	if err != nil {
		t.Fatalf("ListActiveSessions fehlgeschlagen: %v", err)
	}
	if len(activeSessions) != 1 {
		t.Fatalf("Erwartet genau 1 verbleibende Session, erhalten: %d", len(activeSessions))
	}
}

// TestDSGVO_ExportJob testet den asynchronen Ablauf: Request mit Passwort -> Pending -> Worker -> Completed -> Download.
func TestDSGVO_ExportJob(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dbPool := getRoutesTestDB(t)
	if dbPool == nil {
		return
	}
	defer dbPool.Close()

	redisClient := getRoutesTestRedis()
	rustServer := setupMockRustService(t, false)
	defer rustServer.Close()

	jwtSecret := "test-jwt-secret-1234567890-32bytes!"
	mfaKey := make([]byte, 32)
	auditKey := make([]byte, 32)

	auditLogger := audit.NewLogger(dbPool)
	userHandler := routes.NewUserHandler(dbPool, redisClient, auditLogger, rustServer.URL, "test-service-token", mfaKey, auditKey, 50)
	dsgvoWorker := workers.NewDSGVOWorker(dbPool, redisClient, auditLogger, rustServer.URL, "test-service-token", 90)

	router := gin.New()
	v1 := router.Group("/api/v1")
	v1.Use(auth.Middleware(jwtSecret, redisClient))
	v1.POST("/users/me/export", userHandler.RequestExport)
	v1.GET("/users/me/export/:job_id", userHandler.GetExportStatus)
	v1.GET("/users/me/export/:job_id/download", userHandler.DownloadExport)

	password := "Pass12345678!"
	email := fmt.Sprintf("export_user_%d@4labs.example", time.Now().UnixNano())
	userID, token := setupTestUser(t, dbPool, email, password, false)

	// 1. Export beantragen mit Passwort -> 202 Accepted + job_id
	exportReqBody, _ := json.Marshal(map[string]string{"password": password})
	reqInit, _ := http.NewRequest(http.MethodPost, "/api/v1/users/me/export", bytes.NewReader(exportReqBody))
	reqInit.Header.Set("Authorization", "Bearer "+token)
	reqInit.Header.Set("Content-Type", "application/json")
	wInit := httptest.NewRecorder()
	router.ServeHTTP(wInit, reqInit)
	if wInit.Code != http.StatusAccepted {
		t.Fatalf("Erwartet 202 bei RequestExport, erhalten: %d (%s)", wInit.Code, wInit.Body.String())
	}

	var initResp struct {
		JobID  string `json:"job_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(wInit.Body.Bytes(), &initResp); err != nil {
		t.Fatalf("Fehler beim Dekodieren der Export-Antwort: %v", err)
	}
	if initResp.JobID == "" || initResp.Status != "pending" {
		t.Fatalf("Ungueltige Initial-Antwort: %+v", initResp)
	}

	// 2. Status abfragen -> pending
	reqStatus, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/users/me/export/%s", initResp.JobID), nil)
	reqStatus.Header.Set("Authorization", "Bearer "+token)
	wStatus := httptest.NewRecorder()
	router.ServeHTTP(wStatus, reqStatus)
	if wStatus.Code != http.StatusOK {
		t.Fatalf("Erwartet 200 bei GetExportStatus, erhalten: %d", wStatus.Code)
	}

	// 3. Worker stoesst Verarbeitung an
	ctx := context.Background()
	dsgvoWorker.ProcessPendingExports(ctx)

	// 4. Status erneut abfragen -> completed + download_url
	wStatus2 := httptest.NewRecorder()
	router.ServeHTTP(wStatus2, reqStatus)
	if wStatus2.Code != http.StatusOK {
		t.Fatalf("Erwartet 200 bei GetExportStatus nach Worker, erhalten: %d", wStatus2.Code)
	}

	var statusResp struct {
		Status      string `json:"status"`
		DownloadURL string `json:"download_url"`
		SizeBytes   int64  `json:"size_bytes"`
	}
	if err := json.Unmarshal(wStatus2.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("Fehler beim Dekodieren von Status: %v", err)
	}
	if statusResp.Status != "completed" || statusResp.DownloadURL == "" {
		t.Fatalf("Export nicht completed: %+v", statusResp)
	}

	// 5. Download abrufen -> 200 OK + ZIP
	reqDl, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/users/me/export/%s/download", initResp.JobID), nil)
	reqDl.Header.Set("Authorization", "Bearer "+token)
	wDl := httptest.NewRecorder()
	router.ServeHTTP(wDl, reqDl)
	if wDl.Code != http.StatusOK {
		t.Fatalf("Erwartet 200 bei DownloadExport, erhalten: %d", wDl.Code)
	}
	if wDl.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("Content-Type sollte application/zip sein, ist: %s", wDl.Header().Get("Content-Type"))
	}
	if !bytes.HasPrefix(wDl.Body.Bytes(), []byte("PK")) {
		t.Fatal("Heruntergeladene Daten beginnen nicht mit ZIP-Magic-Bytes PK")
	}

	_ = userID
}

// TestDSGVO_JWTBlacklist prueft, dass Tokens nach User-Loeschung sofort geblacklistet sind.
func TestDSGVO_JWTBlacklist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dbPool := getRoutesTestDB(t)
	if dbPool == nil {
		return
	}
	defer dbPool.Close()

	redisClient := getRoutesTestRedis()
	if redisClient == nil {
		t.Skip("Redis nicht verfuegbar, JWT Blacklist Test wird uebersprungen")
		return
	}

	rustServer := setupMockRustService(t, false)
	defer rustServer.Close()

	jwtSecret := "test-jwt-secret-1234567890-32bytes!"
	mfaKey := make([]byte, 32)
	auditKey := make([]byte, 32)

	auditLogger := audit.NewLogger(dbPool)
	userHandler := routes.NewUserHandler(dbPool, redisClient, auditLogger, rustServer.URL, "test-service-token", mfaKey, auditKey, 50)
	authHandler := routes.NewAuthHandler(dbPool, nil, auditLogger, jwtSecret, 15, mfaKey)

	router := gin.New()
	v1 := router.Group("/api/v1")
	v1.Use(auth.Middleware(jwtSecret, redisClient))
	v1.GET("/auth/sessions", authHandler.ListSessions)
	v1.DELETE("/users/me", userHandler.DeleteMe)

	password := "BlacklistPass123!"
	email := fmt.Sprintf("blacklist_%d@4labs.example", time.Now().UnixNano())
	_, token := setupTestUser(t, dbPool, email, password, false)

	// 1. Token ist gueltig -> Sessions liefert 200
	reqSessions, _ := http.NewRequest(http.MethodGet, "/api/v1/auth/sessions", nil)
	reqSessions.Header.Set("Authorization", "Bearer "+token)
	wSessions := httptest.NewRecorder()
	router.ServeHTTP(wSessions, reqSessions)
	if wSessions.Code != http.StatusOK {
		t.Fatalf("Erwartet 200 vor Loeschung, erhalten: %d", wSessions.Code)
	}

	// 2. User loescht sich selbst -> 204
	delBody, _ := json.Marshal(map[string]string{"password": password})
	reqDel, _ := http.NewRequest(http.MethodDelete, "/api/v1/users/me", bytes.NewReader(delBody))
	reqDel.Header.Set("Authorization", "Bearer "+token)
	reqDel.Header.Set("Content-Type", "application/json")
	wDel := httptest.NewRecorder()
	router.ServeHTTP(wDel, reqDel)
	if wDel.Code != http.StatusNoContent {
		t.Fatalf("Erwartet 204 bei Selbstloeschung, erhalten: %d", wDel.Code)
	}

	// 3. Gleicher Token muss SOFORT 401 liefern (Blacklist Treffer)
	reqAfter, _ := http.NewRequest(http.MethodGet, "/api/v1/auth/sessions", nil)
	reqAfter.Header.Set("Authorization", "Bearer "+token)
	wAfter := httptest.NewRecorder()
	router.ServeHTTP(wAfter, reqAfter)
	if wAfter.Code != http.StatusUnauthorized {
		t.Fatalf("Erwartet 401 nach Loeschung (JWT-Blacklist), erhalten: %d", wAfter.Code)
	}
}

// TestDSGVO_DeleteRustFailure prueft: Wenn Rust DELETE fehlschlaegt, liefert Go 500 und aendert die DB NICHT.
func TestDSGVO_DeleteRustFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dbPool := getRoutesTestDB(t)
	if dbPool == nil {
		return
	}
	defer dbPool.Close()

	redisClient := getRoutesTestRedis()
	// Mock Rust Server der bei DELETE fehlschlaegt
	rustServer := setupMockRustService(t, true)
	defer rustServer.Close()

	jwtSecret := "test-jwt-secret-1234567890-32bytes!"
	mfaKey := make([]byte, 32)
	auditKey := make([]byte, 32)

	auditLogger := audit.NewLogger(dbPool)
	userHandler := routes.NewUserHandler(dbPool, redisClient, auditLogger, rustServer.URL, "test-service-token", mfaKey, auditKey, 50)

	router := gin.New()
	v1 := router.Group("/api/v1")
	v1.Use(auth.Middleware(jwtSecret, redisClient))
	v1.DELETE("/users/me", userHandler.DeleteMe)

	password := "RustFailPass123!"
	email := fmt.Sprintf("rustfail_%d@4labs.example", time.Now().UnixNano())
	userID, token := setupTestUser(t, dbPool, email, password, false)

	// Datei in DB anlegen
	ctx := context.Background()
	_, err := dbPool.Exec(ctx,
		`INSERT INTO files (user_id, filename, size_bytes, checksum_sha256, storage_path)
		 VALUES ($1, 'kritisch.pdf', 2048, 'hash123', 'storage/kritisch.enc')`,
		userID,
	)
	if err != nil {
		t.Fatalf("Datei anlegen fehlgeschlagen: %v", err)
	}

	// Selbstloeschung aufrufen -> Rust DELETE schlaegt fehl -> Go MUSS 500 liefern
	delBody, _ := json.Marshal(map[string]string{"password": password})
	reqDel, _ := http.NewRequest(http.MethodDelete, "/api/v1/users/me", bytes.NewReader(delBody))
	reqDel.Header.Set("Authorization", "Bearer "+token)
	reqDel.Header.Set("Content-Type", "application/json")
	wDel := httptest.NewRecorder()
	router.ServeHTTP(wDel, reqDel)
	if wDel.Code != http.StatusInternalServerError {
		t.Fatalf("Erwartet 500 bei Rust-Fehler, erhalten: %d", wDel.Code)
	}

	// User MUSS noch in der Datenbank existieren (KEINE DB-Aenderung)
	var userStillExists bool
	_ = dbPool.QueryRow(ctx, "SELECT true FROM users WHERE id = $1", userID).Scan(&userStillExists)
	if !userStillExists {
		t.Fatal("User darf bei Rust-Fehler NICHT aus der DB geloescht werden")
	}

	// Datei MUSS noch in der Datenbank existieren
	var fileStillExists bool
	_ = dbPool.QueryRow(ctx, "SELECT true FROM files WHERE user_id = $1", userID).Scan(&fileStillExists)
	if !fileStillExists {
		t.Fatal("Datei darf bei Rust-Fehler NICHT aus der DB geloescht werden")
	}
}

// TestDSGVO_AuditRetention prueft die Loeschung von Logs aelter als 90 Tage.
func TestDSGVO_AuditRetention(t *testing.T) {
	dbPool := getRoutesTestDB(t)
	if dbPool == nil {
		return
	}
	defer dbPool.Close()

	ctx := context.Background()

	// 1. Alten Eintrag (vor 100 Tagen) und frischen Eintrag einfuegen
	oldAction := fmt.Sprintf("test_old_action_%d", time.Now().UnixNano())
	freshAction := fmt.Sprintf("test_fresh_action_%d", time.Now().UnixNano())

	_, err := dbPool.Exec(ctx,
		`INSERT INTO audit_log (action, result, created_at)
		 VALUES ($1, 'ok', now() - interval '100 days')`,
		oldAction,
	)
	if err != nil {
		t.Fatalf("Alten Audit-Eintrag einfuegen fehlgeschlagen: %v", err)
	}

	_, err = dbPool.Exec(ctx,
		`INSERT INTO audit_log (action, result, created_at)
		 VALUES ($1, 'ok', now())`,
		freshAction,
	)
	if err != nil {
		t.Fatalf("Frischen Audit-Eintrag einfuegen fehlgeschlagen: %v", err)
	}

	// 2. Retention Cleanup fuer 90 Tage ausfuehren
	deleted, err := audit.CleanupRetention(ctx, dbPool, 90)
	if err != nil {
		t.Fatalf("CleanupRetention fehlgeschlagen: %v", err)
	}
	if deleted < 1 {
		t.Fatalf("Mindestens 1 alter Eintrag haette geloescht werden muessen, geloescht: %d", deleted)
	}

	// 3. Pruefen: Alter Eintrag ist weg, frischer ist noch da
	var oldExists bool
	_ = dbPool.QueryRow(ctx, "SELECT true FROM audit_log WHERE action = $1", oldAction).Scan(&oldExists)
	if oldExists {
		t.Fatal("Alter Audit-Eintrag haette geloescht werden muessen")
	}

	var freshExists bool
	_ = dbPool.QueryRow(ctx, "SELECT true FROM audit_log WHERE action = $1", freshAction).Scan(&freshExists)
	if !freshExists {
		t.Fatal("Frischer Audit-Eintrag haette erhalten bleiben muessen")
	}
}

// TestDSGVO_Pseudonymization prueft die Pseudonymisierung vorhandener Logs eines Users in der DB.
func TestDSGVO_Pseudonymization(t *testing.T) {
	dbPool := getRoutesTestDB(t)
	if dbPool == nil {
		return
	}
	defer dbPool.Close()

	ctx := context.Background()
	testUserID := uuid.New()
	auditKey := make([]byte, 32)
	for i := range auditKey {
		auditKey[i] = byte(i + 15)
	}

	action := fmt.Sprintf("test_pseudo_%d", time.Now().UnixNano())
	_, err := dbPool.Exec(ctx,
		`INSERT INTO audit_log (user_id, action, result)
		 VALUES ($1, $2, 'ok')`,
		testUserID, action,
	)
	if err != nil {
		t.Fatalf("Audit-Log einfuegen fehlgeschlagen: %v", err)
	}

	// Pseudonymisieren
	pseudo, err := audit.PseudonymizeUser(ctx, dbPool, testUserID, auditKey)
	if err != nil {
		t.Fatalf("PseudonymizeUser fehlgeschlagen: %v", err)
	}

	expectedPseudo := audit.CalculatePseudonym(testUserID, auditKey)
	if pseudo != expectedPseudo {
		t.Fatalf("Erwartetes Pseudonym %s, erhalten: %s", expectedPseudo, pseudo)
	}

	// Pruefen: user_id IS NULL und pseudonym_hash gesetzt
	var (
		resUserID *uuid.UUID
		resPseudo *string
	)
	err = dbPool.QueryRow(ctx,
		"SELECT user_id, pseudonym_hash FROM audit_log WHERE action = $1",
		action,
	).Scan(&resUserID, &resPseudo)
	if err != nil {
		t.Fatalf("Audit-Log abfragen fehlgeschlagen: %v", err)
	}

	if resUserID != nil {
		t.Fatalf("user_id muss nach Pseudonymisierung NULL sein, ist: %v", resUserID)
	}
	if resPseudo == nil || *resPseudo != expectedPseudo {
		t.Fatalf("pseudonym_hash stimmt nicht: %v", resPseudo)
	}
}
