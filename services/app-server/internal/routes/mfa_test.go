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
	"github.com/google/uuid"
)

// TestMFA_RecoveryRoundtrip prueft: 10 Codes generieren, einen nutzen -> 200, Wiederverwendung -> 401.
func TestMFA_RecoveryRoundtrip(t *testing.T) {
	pool := getRoutesTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	secret := "test-mfa-jwt-secret-very-secure-32chars"
	mfaKey := []byte("01234567890123456789012345678901")

	// Testbenutzer anlegen
	testEmail := fmt.Sprintf("mfa_recovery_%d@test.internal", time.Now().UnixNano())
	pwHash, _ := auth.HashPassword("TestPass123!")

	var userID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, is_admin, mfa_enabled)
		 VALUES ($1, $2, false, true)
		 RETURNING id`,
		testEmail, pwHash,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("fehler beim anlegen des testnutzers: %v", err)
	}

	// 10 Recovery-Codes generieren und in DB speichern
	codes, err := auth.GenerateRecoveryCodes()
	if err != nil {
		t.Fatalf("fehler beim generieren der recovery-codes: %v", err)
	}
	if len(codes) != 10 {
		t.Fatalf("erwartet 10 recovery-codes, erhalten: %d", len(codes))
	}

	if err := auth.SaveRecoveryCodes(ctx, pool, userID, codes); err != nil {
		t.Fatalf("fehler beim speichern der recovery-codes: %v", err)
	}

	// mfa_token fuer Benutzer generieren
	mfaToken, err := auth.GenerateMFAPendingToken(userID, secret, 5*time.Minute)
	if err != nil {
		t.Fatalf("fehler beim generieren des mfa-tokens: %v", err)
	}

	limiter := auth.NewInMemoryRateLimiter()
	handler := routes.NewAuthHandler(pool, limiter, nil, secret, 15, mfaKey)

	r := gin.New()
	r.POST("/api/v1/auth/mfa/verify-recovery", handler.VerifyMFARecovery)

	// Ersten Code verwenden -> MUSS 200 OK liefern
	firstCode := codes[0]
	reqBody, _ := json.Marshal(map[string]string{
		"mfa_token":     mfaToken,
		"recovery_code": firstCode,
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/mfa/verify-recovery", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("erster aufruf mit recovery_code erwartet 200 OK, erhalten: %d (Body: %s)", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("ungueltige json-antwort: %v", err)
	}
	if resp["access_token"] == nil || resp["refresh_token"] == nil {
		t.Fatalf("access_token oder refresh_token fehlen in der antwort")
	}

	// Zweiten Versuch mit DEMSELBEN Code -> MUSS 401 Unauthorized liefern (einmalige Nutzung)
	req2, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/mfa/verify-recovery", bytes.NewBuffer(reqBody))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("wiederverwendung des codes erwartet 401 Unauthorized, erhalten: %d (Body: %s)", w2.Code, w2.Body.String())
	}

	// Einen anderen, ungenutzten Code verwenden -> MUSS 200 OK liefern
	secondCode := codes[1]
	reqBody2, _ := json.Marshal(map[string]string{
		"mfa_token":     mfaToken,
		"recovery_code": secondCode,
	})
	req3, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/mfa/verify-recovery", bytes.NewBuffer(reqBody2))
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Fatalf("weiterer ungenutzter code erwartet 200 OK, erhalten: %d", w3.Code)
	}
}

// TestMFA_TokenPurpose prueft: mfa_token als Access-Token an geschuetztem Endpunkt -> 401.
func TestMFA_TokenPurpose(t *testing.T) {
	secret := "test-purpose-secret-123456789012"
	userID := uuid.New()

	// 1. Temporaeres MFA-Token generieren (purpose=mfa_pending)
	mfaToken, err := auth.GenerateMFAPendingToken(userID, secret, 5*time.Minute)
	if err != nil {
		t.Fatalf("fehler beim generieren des mfa-tokens: %v", err)
	}

	// 2. Regulaeres Access-Token generieren (purpose=access)
	accessToken, err := auth.GenerateToken(userID, secret, 15*time.Minute)
	if err != nil {
		t.Fatalf("fehler beim generieren des access-tokens: %v", err)
	}

	// Geschuetzten Router aufsetzen
	r := gin.New()
	r.Use(auth.Middleware(secret))
	r.GET("/api/v1/files", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Versuch 1: mfa_pending Token an geschuetztem Endpunkt -> MUSS 401 liefern!
	req1, _ := http.NewRequest(http.MethodGet, "/api/v1/files", nil)
	req1.Header.Set("Authorization", "Bearer "+mfaToken)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusUnauthorized {
		t.Fatalf("mfa_pending token als access-token erwartet 401 Unauthorized, erhalten: %d", w1.Code)
	}

	// Versuch 2: Regulaeres Access-Token an geschuetztem Endpunkt -> MUSS 200 liefern
	req2, _ := http.NewRequest(http.MethodGet, "/api/v1/files", nil)
	req2.Header.Set("Authorization", "Bearer "+accessToken)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("regulaeres access-token erwartet 200 OK, erhalten: %d", w2.Code)
	}
}

// TestMFA_CSRF prueft: /refresh ohne oder mit falschem X-CSRF-Token -> 403.
func TestMFA_CSRF(t *testing.T) {
	pool := getRoutesTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	secret := "test-csrf-secret-123456789012345"
	mfaKey := []byte("01234567890123456789012345678901")

	// Benutzer & gueltigen Refresh-Token erzeugen
	testEmail := fmt.Sprintf("csrf_%d@test.internal", time.Now().UnixNano())
	pwHash, _ := auth.HashPassword("TestPass123!")

	var userID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, is_admin)
		 VALUES ($1, $2, false)
		 RETURNING id`,
		testEmail, pwHash,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("fehler beim anlegen des nutzers: %v", err)
	}

	refreshToken, err := auth.CreateRefreshToken(ctx, pool, userID, 24*time.Hour)
	if err != nil {
		t.Fatalf("fehler beim erzeugen des refresh-tokens: %v", err)
	}
	csrfToken, _ := auth.GenerateCSRFToken()

	limiter := auth.NewInMemoryRateLimiter()
	handler := routes.NewAuthHandler(pool, limiter, nil, secret, 15, mfaKey)

	r := gin.New()
	r.POST("/api/v1/auth/refresh", handler.Refresh)
	r.POST("/api/v1/auth/logout", handler.Logout)

	// Fall 1: /refresh OHNE X-CSRF-Token Header -> MUSS 403 liefern
	req1, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req1.AddCookie(&http.Cookie{Name: "refresh_token", Value: refreshToken})
	req1.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusForbidden {
		t.Fatalf("refresh ohne X-CSRF-Token header erwartet 403 Forbidden, erhalten: %d", w1.Code)
	}

	// Fall 2: /refresh mit FALSCHEM X-CSRF-Token Header -> MUSS 403 liefern
	req2, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req2.AddCookie(&http.Cookie{Name: "refresh_token", Value: refreshToken})
	req2.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
	req2.Header.Set("X-CSRF-Token", "manipuliertes-csrf-token")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusForbidden {
		t.Fatalf("refresh mit falschem X-CSRF-Token erwartet 403 Forbidden, erhalten: %d", w2.Code)
	}

	// Fall 3: /refresh mit PASSENDEM X-CSRF-Token Header -> MUSS 200 liefern
	req3, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req3.AddCookie(&http.Cookie{Name: "refresh_token", Value: refreshToken})
	req3.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
	req3.Header.Set("X-CSRF-Token", csrfToken)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Fatalf("refresh mit gueltigem CSRF-Token erwartet 200 OK, erhalten: %d (Body: %s)", w3.Code, w3.Body.String())
	}

	// Fall 4: /logout mit falschem CSRF -> 403
	req4, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req4.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
	req4.Header.Set("X-CSRF-Token", "wrong-csrf")
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, req4)

	if w4.Code != http.StatusForbidden {
		t.Fatalf("logout mit falschem CSRF erwartet 403 Forbidden, erhalten: %d", w4.Code)
	}
}

// TestMFA_AdminMFARequired prueft: Admin ohne MFA -> Login 200 + Tokens, KEIN Zwang (MFA ist optional).
func TestMFA_AdminMFARequired(t *testing.T) {

	pool := getRoutesTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	secret := "test-admin-mfa-secret-1234567890"
	mfaKey := []byte("01234567890123456789012345678901")

	// Admin ohne MFA anlegen
	adminEmail := fmt.Sprintf("admin_mfa_req_%d@test.internal", time.Now().UnixNano())
	rawPassword := "SuperAdminPass123!"
	pwHash, _ := auth.HashPassword(rawPassword)

	var adminID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, is_admin, mfa_enabled)
		 VALUES ($1, $2, true, false)
		 RETURNING id`,
		adminEmail, pwHash,
	).Scan(&adminID)
	if err != nil {
		t.Fatalf("fehler beim anlegen des admins: %v", err)
	}

	limiter := auth.NewInMemoryRateLimiter()
	handler := routes.NewAuthHandler(pool, limiter, nil, secret, 15, mfaKey)

	r := gin.New()
	r.POST("/api/v1/auth/login", handler.Login)

	loginBody, _ := json.Marshal(map[string]string{
		"email":    adminEmail,
		"password": rawPassword,
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Status MUSS 200 OK sein
	if w.Code != http.StatusOK {
		t.Fatalf("admin-login ohne MFA erwartet 200 OK, erhalten: %d (Body: %s)", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("ungueltiges JSON: %v", err)
	}

	// mfa_setup_required darf NICHT true sein (MFA ist rein optional)
	if resp["mfa_setup_required"] == true {
		t.Fatalf("MFA darf nicht erzwungen werden: mfa_setup_required ist true!")
	}

	// Access-Token MUSS vorhanden sein
	accessToken, ok := resp["access_token"].(string)
	if !ok || accessToken == "" {
		t.Fatalf("access_token fehlt oder ist leer in der Antwort fuer Admin ohne MFA")
	}

	// is_admin MUSS true sein
	if resp["is_admin"] != true {
		t.Fatalf("is_admin muss true sein, erhalten: %v", resp["is_admin"])
	}

	// refresh_token Cookie MUSS gesetzt sein!
	foundRefreshCookie := false
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "refresh_token" && cookie.Value != "" {
			foundRefreshCookie = true
			break
		}
	}
	if !foundRefreshCookie {
		t.Fatalf("refresh_token Cookie muss bei erfolgreichem Login gesetzt sein!")
	}


	// Das erhaltene Access-Token MUSS valide sein und fuer adminID ausgestellt sein
	parsedUserID, err := auth.ParseToken(accessToken, secret)
	if err != nil {
		t.Fatalf("access_token konnte nicht geparst werden: %v", err)
	}
	if parsedUserID != adminID {
		t.Fatalf("user-id im token stimmt nicht: %s != %s", parsedUserID, adminID)
	}
}



// TestMFA_VerifyRateLimit prueft: 6. Versuch mit mfa_token -> HTTP 429 Too Many Requests.
func TestMFA_VerifyRateLimit(t *testing.T) {
	secret := "test-verify-rl-secret-1234567890"
	userID := uuid.New()

	mfaToken, err := auth.GenerateMFAPendingToken(userID, secret, 5*time.Minute)
	if err != nil {
		t.Fatalf("mfa-token konnte nicht generiert werden: %v", err)
	}

	limiter := auth.NewInMemoryRateLimiter()
	mfaKey := []byte("01234567890123456789012345678901")
	handler := routes.NewAuthHandler(nil, limiter, nil, secret, 15, mfaKey)

	r := gin.New()
	r.POST("/api/v1/auth/mfa/verify", handler.VerifyMFA)

	verifyBody, _ := json.Marshal(map[string]string{
		"mfa_token": mfaToken,
		"code":      "000000",
	})

	// Versuche 1 bis 5 duerfen kein 429 liefern (sie scheitern mit 500 wegen fehlender DB oder 401)
	for i := 1; i <= 5; i++ {
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/mfa/verify", bytes.NewBuffer(verifyBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code == http.StatusTooManyRequests {
			t.Fatalf("versuch %d wurde faelschlicherweise vorzeitig rate-limited", i)
		}
	}

	// 6. Versuch mit demselben mfa_token MUSS HTTP 429 liefern!
	req6, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/mfa/verify", bytes.NewBuffer(verifyBody))
	req6.Header.Set("Content-Type", "application/json")
	w6 := httptest.NewRecorder()
	r.ServeHTTP(w6, req6)

	if w6.Code != http.StatusTooManyRequests {
		t.Fatalf("erwartet HTTP 429 beim 6. Verifikationsversuch, erhalten: %d (Body: %s)", w6.Code, w6.Body.String())
	}
}

// TestMFA_DeactivateUser prueft: Ein regulaerer User kann sein MFA mit TOTP-Code deaktivieren.
func TestMFA_DeactivateUser(t *testing.T) {
	pool := getRoutesTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	secret := "test-deactivate-user-secret-1234"
	mfaKey := []byte("01234567890123456789012345678901")

	testEmail := fmt.Sprintf("deact_user_%d@test.internal", time.Now().UnixNano())
	pwHash, _ := auth.HashPassword("UserPassword123!")

	// TOTP-Secret generieren und verschluesseln
	totpSecret, _, err := auth.GenerateTOTPKey(testEmail)
	if err != nil {
		t.Fatalf("fehler beim generieren des totp-keys: %v", err)
	}
	encryptedSecret, _ := auth.EncryptMFASecret(mfaKey, totpSecret)

	var userID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, is_admin, mfa_enabled, mfa_secret_encrypted)
		 VALUES ($1, $2, false, true, $3)
		 RETURNING id`,
		testEmail, pwHash, encryptedSecret,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("fehler beim anlegen des users: %v", err)
	}

	// Access-Token fuer den User erstellen
	accessToken, err := auth.GenerateToken(userID, secret, 15*time.Minute)
	if err != nil {
		t.Fatalf("fehler beim generieren des access-tokens: %v", err)
	}

	// Aktuellen TOTP-Code erzeugen
	totpCode, err := auth.GenerateCode(totpSecret, time.Now())
	if err != nil {
		t.Fatalf("fehler beim generieren des totp-codes: %v", err)
	}

	limiter := auth.NewInMemoryRateLimiter()
	handler := routes.NewAuthHandler(pool, limiter, nil, secret, 15, mfaKey)

	r := gin.New()
	r.POST("/api/v1/auth/mfa/deactivate", auth.Middleware(secret), handler.DeactivateMFA)

	deactBody, _ := json.Marshal(map[string]string{
		"code": totpCode,
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/mfa/deactivate", bytes.NewBuffer(deactBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("deaktivierung durch user erwartet 200 OK, erhalten: %d (Body: %s)", w.Code, w.Body.String())
	}

	// Pruefen, dass mfa_enabled in DB auf false steht
	var (
		mfaEnabled      bool
		secEncrypted    []byte
	)
	err = pool.QueryRow(ctx, "SELECT mfa_enabled, mfa_secret_encrypted FROM users WHERE id = $1", userID).Scan(&mfaEnabled, &secEncrypted)
	if err != nil {
		t.Fatalf("fehler beim abfragen des users: %v", err)
	}
	if mfaEnabled {
		t.Fatalf("mfa_enabled ist nach deaktivierung immer noch true!")
	}
	if len(secEncrypted) > 0 {
		t.Fatalf("mfa_secret_encrypted wurde nicht geleert!")
	}
}

// TestMFA_DeactivateAdmin_Forbidden prueft: Admin darf sein eigenes MFA nicht selbst deaktivieren (HTTP 403).
func TestMFA_DeactivateAdmin_Forbidden(t *testing.T) {
	pool := getRoutesTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	secret := "test-deact-admin-secret-12345678"
	mfaKey := []byte("01234567890123456789012345678901")

	adminEmail := fmt.Sprintf("deact_admin_%d@test.internal", time.Now().UnixNano())
	pwHash, _ := auth.HashPassword("AdminPassword123!")

	totpSecret, _, _ := auth.GenerateTOTPKey(adminEmail)
	encryptedSecret, _ := auth.EncryptMFASecret(mfaKey, totpSecret)

	var adminID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, is_admin, mfa_enabled, mfa_secret_encrypted)
		 VALUES ($1, $2, true, true, $3)
		 RETURNING id`,
		adminEmail, pwHash, encryptedSecret,
	).Scan(&adminID)
	if err != nil {
		t.Fatalf("fehler beim anlegen des admins: %v", err)
	}

	adminAccessToken, err := auth.GenerateToken(adminID, secret, 15*time.Minute)
	if err != nil {
		t.Fatalf("fehler beim generieren des access-tokens: %v", err)
	}

	totpCode, _ := auth.GenerateCode(totpSecret, time.Now())

	limiter := auth.NewInMemoryRateLimiter()
	handler := routes.NewAuthHandler(pool, limiter, nil, secret, 15, mfaKey)

	r := gin.New()
	r.POST("/api/v1/auth/mfa/deactivate", auth.Middleware(secret), handler.DeactivateMFA)

	deactBody, _ := json.Marshal(map[string]string{
		"code": totpCode,
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/mfa/deactivate", bytes.NewBuffer(deactBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminAccessToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Admin-Selbst-Deaktivierung MUSS HTTP 403 Forbidden liefern!
	if w.Code != http.StatusForbidden {
		t.Fatalf("admin-selbst-deaktivierung erwartet 403 Forbidden, erhalten: %d (Body: %s)", w.Code, w.Body.String())
	}

	// Pruefen, dass mfa_enabled in DB weiterhin true ist
	var mfaEnabled bool
	err = pool.QueryRow(ctx, "SELECT mfa_enabled FROM users WHERE id = $1", adminID).Scan(&mfaEnabled)
	if err != nil {
		t.Fatalf("fehler beim abfragen des admins: %v", err)
	}
	if !mfaEnabled {
		t.Fatalf("mfa_enabled des admins wurde faelschlicherweise deaktiviert!")
	}
}
