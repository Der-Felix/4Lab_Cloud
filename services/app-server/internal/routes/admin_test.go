package routes_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/4labscloud/4labscloud/services/app-server/internal/audit"
	"github.com/4labscloud/4labscloud/services/app-server/internal/auth"
	"github.com/4labscloud/4labscloud/services/app-server/internal/routes"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TestAudit_NotPublic_Forbidden prueft, dass nicht-autorisierte und Standard-Nutzer keinen Zugriff auf /api/v1/admin/audit haben.
func TestAudit_NotPublic_Forbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	jwtSecret := "test-secret-32-chars-long-security-bsi"

	pool := getRoutesTestDB(t)
	if pool == nil {
		t.Skip("PostgreSQL Test-DB nicht erreichbar")
	}

	auditLogger := audit.NewLogger(pool)
	adminHandler := routes.NewAdminHandler(pool, auditLogger)

	router := gin.New()
	v1 := router.Group("/api/v1")
	v1.Use(auth.Middleware(jwtSecret, nil))
	{
		admin := v1.Group("")
		admin.Use(auth.RequireAdmin(pool))
		{
			admin.GET("/admin/audit", adminHandler.ListAuditLogs)
			admin.GET("/admin/users", adminHandler.ListUsers)
		}
	}

	// 1. Unauthentifizierter Request -> 401
	reqAnon, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/audit", nil)
	wAnon := httptest.NewRecorder()
	router.ServeHTTP(wAnon, reqAnon)
	if wAnon.Code != http.StatusUnauthorized {
		t.Errorf("erwartete 401 fuer anonymen Zugriff, erhielt %d", wAnon.Code)
	}

	// 2. Normaler (Nicht-Admin) Benutzer -> 403 Forbidden
	normalUserID := uuid.New()
	// User in DB einfuegen mit is_admin = false
	_, err := pool.Exec(t.Context(),
		"INSERT INTO users (id, email, password_hash, is_admin) VALUES ($1, $2, 'hash', false) ON CONFLICT (id) DO NOTHING",
		normalUserID, "normal_user@4labs.test",
	)
	if err != nil {
		t.Fatalf("fehler beim anlegen des normal-users: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(t.Context(), "DELETE FROM users WHERE id = $1", normalUserID)
	}()

	normalToken, _ := auth.GenerateToken(normalUserID, jwtSecret, 15*time.Minute)
	reqNormal, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/audit", nil)
	reqNormal.Header.Set("Authorization", "Bearer "+normalToken)
	wNormal := httptest.NewRecorder()
	router.ServeHTTP(wNormal, reqNormal)

	if wNormal.Code != http.StatusForbidden {
		t.Errorf("audit_not_public_test fehlgeschlagen: erwartete 403 fuer Nicht-Admin, erhielt %d", wNormal.Code)
	}

	// 3. Admin-Benutzer -> 200 OK
	adminUserID := uuid.New()
	_, err = pool.Exec(t.Context(),
		"INSERT INTO users (id, email, password_hash, is_admin) VALUES ($1, $2, 'hash', true) ON CONFLICT (id) DO NOTHING",
		adminUserID, "admin_user@4labs.test",
	)
	if err != nil {
		t.Fatalf("fehler beim anlegen des admin-users: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(t.Context(), "DELETE FROM users WHERE id = $1", adminUserID)
	}()

	adminToken, _ := auth.GenerateToken(adminUserID, jwtSecret, 15*time.Minute)
	reqAdmin, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/audit", nil)
	reqAdmin.Header.Set("Authorization", "Bearer "+adminToken)
	wAdmin := httptest.NewRecorder()
	router.ServeHTTP(wAdmin, reqAdmin)

	if wAdmin.Code != http.StatusOK {
		t.Errorf("erwartete 200 fuer Admin-Zugriff, erhielt %d", wAdmin.Code)
	}

	var resp struct {
		Logs []routes.AuditLogListItem `json:"logs"`
	}
	if err := json.Unmarshal(wAdmin.Body.Bytes(), &resp); err != nil {
		t.Errorf("ungueltige json antwort: %v", err)
	}
}
