package routes_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/4labscloud/4labscloud/services/app-server/internal/auth"
	"github.com/4labscloud/4labscloud/services/app-server/internal/routes"
	"github.com/gin-gonic/gin"
)

// TestRegister_FirstUser prueft, dass der erste Benutzer ohne Einladung registriert
// werden kann, automatisch is_admin=true erhaelt und folgende Registrierungen
// ohne Einladung mit HTTP 403 abgewiesen werden.
func TestRegister_FirstUser(t *testing.T) {
	pool := getRoutesTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()

	// Sicherheitspruefung: felix@4labs.local darf NIEMALS geloescht werden
	var felixExists bool
	_ = pool.QueryRow(ctx, "SELECT true FROM users WHERE email = 'felix@4labs.local'").Scan(&felixExists)
	if felixExists {
		t.Skip("Skipping FirstUser test on DB with existing felix@4labs.local admin account")
		return
	}

	// Bereinige bestehende Test-Benutzer fuer saubere Ausgangslage (ohne felix@4labs.local zu gefaehrden)
	_, err := pool.Exec(ctx, "DELETE FROM invitations")
	if err != nil {
		t.Fatalf("fehler beim bereinigen der einladungen: %v", err)
	}
	_, err = pool.Exec(ctx, "DELETE FROM users WHERE email != 'felix@4labs.local'")
	if err != nil {
		t.Fatalf("fehler beim bereinigen der benutzer: %v", err)
	}

	testEmail1 := fmt.Sprintf("first_admin_%d@test.internal", time.Now().UnixNano())
	testEmail2 := fmt.Sprintf("second_user_%d@test.internal", time.Now().UnixNano())
	password := "SicheresInitialPasswort123!"

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE email IN ($1, $2)", testEmail1, testEmail2)
	}()

	limiter := auth.NewInMemoryRateLimiter()
	testMFAKey := []byte("01234567890123456789012345678901")
	handler := routes.NewAuthHandler(pool, limiter, nil, "test-jwt-secret-xyz", 15, testMFAKey)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/auth/register", handler.Register)

	// 1. Erster User ohne Einladung -> MUSS 201 Created und is_admin=true liefern
	body1, _ := json.Marshal(map[string]string{
		"email":    testEmail1,
		"password": password,
	})
	req1, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(body1))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("erster User muss HTTP 201 liefern, erhalten: %d (Body: %s)", w1.Code, w1.Body.String())
	}

	var resp1 map[string]any
	if err := json.Unmarshal(w1.Body.Bytes(), &resp1); err != nil {
		t.Fatalf("fehler beim parsen der antwort: %v", err)
	}

	isAdmin, ok := resp1["is_admin"].(bool)
	if !ok || !isAdmin {
		t.Fatalf("erster User muss is_admin=true haben, erhalten: %v", resp1["is_admin"])
	}

	// In der Datenbank verifizieren
	var dbAdmin bool
	err = pool.QueryRow(ctx, "SELECT is_admin FROM users WHERE email = $1", testEmail1).Scan(&dbAdmin)
	if err != nil {
		t.Fatalf("datenbankfehler beim pruefen des admin-status: %v", err)
	}
	if !dbAdmin {
		t.Fatalf("in der datenbank muss is_admin=true sein fuer den ersten user")
	}

	// 2. Zweiter User ohne Einladung -> MUSS HTTP 403 Forbidden liefern
	body2, _ := json.Marshal(map[string]string{
		"email":    testEmail2,
		"password": password,
	})
	req2, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusForbidden {
		t.Fatalf("zweiter User ohne Einladung muss HTTP 403 liefern, erhalten: %d (Body: %s)", w2.Code, w2.Body.String())
	}
}

// TestRegister_FirstUser_RaceCondition prueft, dass bei paralleler Registrierung
// auf leerer Datenbank nur exakt ein Benutzer Admin werden kann.
func TestRegister_FirstUser_RaceCondition(t *testing.T) {
	pool := getRoutesTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	// Sicherheitspruefung: felix@4labs.local darf NIEMALS geloescht werden
	ctx := context.Background()
	var felixExists bool
	_ = pool.QueryRow(ctx, "SELECT true FROM users WHERE email = 'felix@4labs.local'").Scan(&felixExists)
	if felixExists {
		t.Skip("Skipping FirstUser race test on DB with existing felix@4labs.local admin account")
		return
	}

	// Bereinige Tabellen
	_, _ = pool.Exec(ctx, "DELETE FROM invitations")
	_, _ = pool.Exec(ctx, "DELETE FROM users WHERE email != 'felix@4labs.local'")

	limiter := auth.NewInMemoryRateLimiter()
	testMFAKey := []byte("01234567890123456789012345678901")
	handler := routes.NewAuthHandler(pool, limiter, nil, "test-jwt-secret-xyz", 15, testMFAKey)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/auth/register", handler.Register)

	concurrency := 6
	var wg sync.WaitGroup
	wg.Add(concurrency)

	statusCodes := make([]int, concurrency)
	adminFlags := make([]bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			email := fmt.Sprintf("race_user_%d_%d@test.internal", time.Now().UnixNano(), idx)
			body, _ := json.Marshal(map[string]string{
				"email":    email,
				"password": "PasswordRace123!",
			})
			req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			statusCodes[idx] = w.Code
			if w.Code == http.StatusCreated {
				var resp map[string]any
				_ = json.Unmarshal(w.Body.Bytes(), &resp)
				if adm, ok := resp["is_admin"].(bool); ok && adm {
					adminFlags[idx] = true
				}
			}
		}(i)
	}

	wg.Wait()

	// Aufraeumen
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE email LIKE 'race_user_%'")
	}()

	// Zaehlen wie viele 201 Created waren
	createdCount := 0
	adminCount := 0
	for i := 0; i < concurrency; i++ {
		if statusCodes[i] == http.StatusCreated {
			createdCount++
		}
		if adminFlags[i] {
			adminCount++
		}
	}

	if createdCount != 1 {
		t.Fatalf("exakt 1 paralleler Request durfte 201 Created erhalten, erhalten: %d (Codes: %v)", createdCount, statusCodes)
	}
	if adminCount != 1 {
		t.Fatalf("exakt 1 Benutzer durfte Admin werden, erhalten: %d", adminCount)
	}

	// In der Datenbank verifizieren, wie viele Admins existieren
	var dbAdminCount int
	err := pool.QueryRow(ctx, "SELECT count(*) FROM users WHERE is_admin = true").Scan(&dbAdminCount)
	if err != nil {
		t.Fatalf("datenbankfehler: %v", err)
	}
	if dbAdminCount != 1 {
		t.Fatalf("exakt 1 Admin darf in der DB existieren, gefunden: %d", dbAdminCount)
	}
}
