package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/4labscloud/4labscloud/services/app-server/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestJWTTokenAndMiddleware(t *testing.T) {
	secret := "test-geheimer-schluessel-1234567890"
	userID := uuid.New()

	// Token generieren
	token, err := auth.GenerateToken(userID, secret, 15*time.Minute)
	if err != nil {
		t.Fatalf("fehler beim generieren des tokens: %v", err)
	}

	// Router aufsetzen
	r := gin.New()
	r.Use(auth.Middleware(secret))
	r.GET("/protected", func(c *gin.Context) {
		extractedID := auth.MustGetUserID(c)
		c.JSON(http.StatusOK, gin.H{"user_id": extractedID.String()})
	})

	// Testfall 1: Ohne Authorization-Header -> 401
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("erwartet HTTP 401 ohne Header, erhalten %d", w.Code)
	}

	// Testfall 2: Ungueltiges Token -> 401
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer ungueltigestoken")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("erwartet HTTP 401 bei falschem Token, erhalten %d", w.Code)
	}

	// Testfall 3: Gueltiges Token -> 200
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("erwartet HTTP 200 bei gueltigem Token, erhalten %d", w.Code)
	}
}

func TestInternalAuthMiddleware(t *testing.T) {
	expectedToken := "internes-service-geheimnis"

	r := gin.New()
	r.Use(auth.InternalAuthMiddleware(expectedToken))
	r.GET("/internal", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Testfall 1: Ohne Token -> 401
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/internal", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("erwartet HTTP 401 ohne Header, erhalten %d", w.Code)
	}

	// Testfall 2: Falsches Token -> 401
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/internal", nil)
	req.Header.Set("Authorization", "Bearer falsches-token")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("erwartet HTTP 401 bei falschem Token, erhalten %d", w.Code)
	}

	// Testfall 3: Korrektes Token -> 200
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/internal", nil)
	req.Header.Set("Authorization", "Bearer "+expectedToken)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("erwartet HTTP 200 bei korrektem Token, erhalten %d", w.Code)
	}
}
