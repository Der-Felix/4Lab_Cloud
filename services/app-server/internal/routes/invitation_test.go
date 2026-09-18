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
	"github.com/4labscloud/4labscloud/services/app-server/internal/routes"
	"github.com/gin-gonic/gin"
)

func TestInvitations_Workflow(t *testing.T) {
	pool := getRoutesTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()

	// Sicherstellen, dass mindestens ein Benutzer existiert, damit Einladungszwang greift
	_, err := pool.Exec(ctx,
		`INSERT INTO users (email, password_hash, is_admin)
		 VALUES ('existing_admin@test.internal', 'hash', true)
		 ON CONFLICT (email) DO NOTHING`,
	)
	if err != nil {
		t.Fatalf("fehler beim anlegen des basis-benutzers: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE email = 'existing_admin@test.internal'")
	}()

	limiter := auth.NewInMemoryRateLimiter()
	testMFAKey := []byte("01234567890123456789012345678901")
	handler := routes.NewAuthHandler(pool, limiter, nil, "test-jwt-secret", 15, testMFAKey)

	r := gin.New()
	r.POST("/api/v1/auth/register", handler.Register)

	testEmail := fmt.Sprintf("inv_test_%d@test.internal", time.Now().UnixNano())
	testPassword := "SicheresPasswort123!"

	// 1. Versuch ohne Einladung -> MUSS 403 Forbidden liefern
	noInvBody, _ := json.Marshal(map[string]string{
		"email":    testEmail,
		"password": testPassword,
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(noInvBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("erwartet HTTP 403 ohne Einladung, erhalten: %d (Body: %s)", w.Code, w.Body.String())
	}

	// 2. Versuch mit abgelaufener Einladung -> MUSS 403 Forbidden liefern
	expiredToken, expiredHash, _ := auth.GenerateRefreshToken()
	_, err = pool.Exec(ctx,
		`INSERT INTO invitations (email, token_hash, expires_at)
		 VALUES ($1, $2, now() - interval '1 day')`,
		testEmail, expiredHash,
	)
	if err != nil {
		t.Fatalf("fehler beim anlegen der abgelaufenen einladung: %v", err)
	}

	expBody, _ := json.Marshal(map[string]string{
		"email":            testEmail,
		"password":         testPassword,
		"invitation_token": expiredToken,
	})
	req, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(expBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("erwartet HTTP 403 bei abgelaufener Einladung, erhalten: %d", w.Code)
	}

	// 3. Versuch mit gueltiger Einladung -> MUSS 201 Created liefern
	validToken, validHash, _ := auth.GenerateRefreshToken()
	_, err = pool.Exec(ctx,
		`INSERT INTO invitations (email, token_hash, expires_at)
		 VALUES ($1, $2, now() + interval '7 days')`,
		testEmail, validHash,
	)
	if err != nil {
		t.Fatalf("fehler beim anlegen der gueltigen einladung: %v", err)
	}

	validBody, _ := json.Marshal(map[string]string{
		"email":            testEmail,
		"password":         testPassword,
		"invitation_token": validToken,
	})
	req, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(validBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("erwartet HTTP 201 mit gueltiger Einladung, erhalten: %d (Body: %s)", w.Code, w.Body.String())
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE email = $1", testEmail)
		_, _ = pool.Exec(ctx, "DELETE FROM invitations WHERE email = $1", testEmail)
	}()

	// 4. Erneuter Versuch mit derselben Einladung (bereits genutzt) -> MUSS 403 liefern
	req, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(validBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("erwartet HTTP 403 bei bereits genutzter Einladung, erhalten: %d", w.Code)
	}
}
