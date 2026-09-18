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

	"github.com/4labscloud/4labscloud/services/app-server/internal/auth"
	"github.com/4labscloud/4labscloud/services/app-server/internal/routes"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func getRoutesTestDB(t *testing.T) *pgxpool.Pool {
	host := os.Getenv("POSTGRES_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := os.Getenv("POSTGRES_PORT")
	if port == "" {
		port = "5432"
	}
	pass := os.Getenv("POSTGRES_PASSWORD")
	if pass == "" {
		pass = "ZW2rz93oet3JM4TH8Va71Y6U2XRsCMQ5"
	}
	user := os.Getenv("POSTGRES_USER")
	if user == "" {
		user = "4labs"
	}
	dbName := os.Getenv("POSTGRES_DB")
	if dbName == "" {
		dbName = "4labscloud"
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, dbName)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Skipf("DB-Pool nicht initialisierbar: %v", err)
		return nil
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("PostgreSQL nicht verfuegbar (%s:%s): %v", host, port, err)
		return nil
	}
	return pool
}

func TestAuthRoutes_LoginRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// In-Memory Rate Limiter fuer isolierte Tests verwenden
	limiter := auth.NewInMemoryRateLimiter()
	testMFAKey := []byte("01234567890123456789012345678901")
	handler := routes.NewAuthHandler(nil, limiter, nil, "jwt-secret", 15, testMFAKey)
	r.POST("/api/v1/auth/login", handler.Login)

	body := []byte(`{"email": "rate@test.internal", "password": "wrong-password"}`)

	// Die ersten 5 Versuche duerfen kein 429 liefern (sie scheitern ggf. mit 401 oder 500)
	for i := 1; i <= 5; i++ {
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.100:12345"

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code == http.StatusTooManyRequests {
			t.Fatalf("versuch %d wurde faelschlicherweise vorzeitig rate-limited", i)
		}
	}

	// 6. Versuch von der gleichen IP MUSS HTTP 429 Too Many Requests liefern
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.100:12345"

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("erwartet HTTP 429 beim 6. Versuch, erhalten: %d (Body: %s)", w.Code, w.Body.String())
	}
}

func TestAuthRoutes_FullLifecycle(t *testing.T) {
	pool := getRoutesTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	email := fmt.Sprintf("lifecycle_%d@test.internal", time.Now().UnixNano())
	password := "SehrSicheresPW123!"

	limiter := auth.NewInMemoryRateLimiter()
	testMFAKey := []byte("01234567890123456789012345678901")
	handler := routes.NewAuthHandler(pool, limiter, nil, "test-jwt-secret-xyz", 15, testMFAKey)

	r := gin.New()
	r.POST("/api/v1/auth/register", handler.Register)
	r.POST("/api/v1/auth/login", handler.Login)
	r.POST("/api/v1/auth/refresh", handler.Refresh)
	r.POST("/api/v1/auth/logout", handler.Logout)

	// Sicherstellen, dass mindestens ein Admin existiert, damit dieser Test einen regulaeren User via Einladung prueft
	var userCount int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&userCount)
	if userCount == 0 {
		_, _ = pool.Exec(ctx, "INSERT INTO users (email, password_hash, is_admin) VALUES ('system_admin@test.internal', 'hash', true)")
		defer func() {
			_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE email = 'system_admin@test.internal'")
		}()
	}

	// Einladung fuer den regulaeren Testnutzer erzeugen
	inviteToken, tokenHash, err := auth.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("fehler bei Token-Generierung: %v", err)
	}
	_, err = pool.Exec(ctx,
		"INSERT INTO invitations (email, token_hash, expires_at) VALUES ($1, $2, now() + interval '1 hour')",
		email, tokenHash,
	)
	if err != nil {
		t.Fatalf("fehler beim einrichten der einladung: %v", err)
	}

	// 1. Registrieren
	regBody, _ := json.Marshal(map[string]string{
		"email":            email,
		"password":         password,
		"invitation_token": inviteToken,
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(regBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("erwartet HTTP 201 bei Registrierung, erhalten: %d (Body: %s)", w.Code, w.Body.String())
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE email = $1", email)
	}()

	// 2. Login mit falschem Passwort (Timing-Pruefung >= 200ms)
	start := time.Now()
	wrongLoginBody, _ := json.Marshal(map[string]string{
		"email":    email,
		"password": "FalschesPasswort123!",
	})
	req, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(wrongLoginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	elapsed := time.Since(start)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("erwartet HTTP 401 bei falschem PW, erhalten %d", w.Code)
	}
	if elapsed < 180*time.Millisecond {
		t.Fatalf("timing leak: antwort kam in zu kurzer zeit (%v < 180ms)", elapsed)
	}

	// 3. Login mit korrektem Passwort
	loginBody, _ := json.Marshal(map[string]string{
		"email":    email,
		"password": password,
	})
	req, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("erwartet HTTP 200 bei Login, erhalten: %d (Body: %s)", w.Code, w.Body.String())
	}

	var loginResp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &loginResp)
	refreshToken, ok := loginResp["refresh_token"].(string)
	if !ok || refreshToken == "" {
		t.Fatalf("kein Refresh-Token in Login-Antwort: %v", loginResp)
	}
	csrfToken, _ := loginResp["csrf_token"].(string)

	// 4. Token Refresh (mit CSRF-Schutz)
	refreshBody, _ := json.Marshal(map[string]string{
		"refresh_token": refreshToken,
	})
	req, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBuffer(refreshBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: refreshToken})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("erwartet HTTP 200 bei Refresh, erhalten: %d (Body: %s)", w.Code, w.Body.String())
	}

	var refreshResp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &refreshResp)
	newRefreshToken, ok := refreshResp["refresh_token"].(string)
	if !ok || newRefreshToken == "" || newRefreshToken == refreshToken {
		t.Fatalf("neues Refresh-Token fehlt oder ist identisch: %v", refreshResp)
	}
	newCSRF, _ := refreshResp["csrf_token"].(string)

	// 5. Logout (mit CSRF-Schutz)
	logoutBody, _ := json.Marshal(map[string]string{
		"refresh_token": newRefreshToken,
	})
	req, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/logout", bytes.NewBuffer(logoutBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", newCSRF)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: newCSRF})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("erwartet HTTP 200 bei Logout, erhalten: %d", w.Code)
	}
}

func TestAuthRoutes_FirstUserIsAdmin(t *testing.T) {
	pool := getRoutesTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()

	var initialCount int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&initialCount)
	if initialCount > 0 {
		t.Skip("DB ist nicht leer, Test fuer initialen Admin-User wird uebersprungen")
		return
	}

	limiter := auth.NewInMemoryRateLimiter()
	testMFAKey := []byte("01234567890123456789012345678901")
	handler := routes.NewAuthHandler(pool, limiter, nil, "test-jwt-secret-xyz", 15, testMFAKey)

	r := gin.New()
	r.POST("/api/v1/auth/register", handler.Register)

	regBody, _ := json.Marshal(map[string]string{
		"email":    "first_admin@test.internal",
		"password": "AdminPassword123!",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(regBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("erwartet HTTP 201 fuer ersten User, erhalten: %d", w.Code)
	}

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	isAdmin, ok := resp["is_admin"].(bool)
	if !ok || !isAdmin {
		t.Fatalf("erster User muss is_admin=true haben, erhalten: %v", resp)
	}
}

