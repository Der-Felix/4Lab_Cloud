package routes_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/4labscloud/4labscloud/services/app-server/internal/routes"
	"github.com/gin-gonic/gin"
)

type mockPinger struct {
	err error
}

func (m *mockPinger) Ping(ctx context.Context) error {
	return m.err
}

func init() {
	gin.SetMode(gin.TestMode)
}

func TestHealthHandler_OK(t *testing.T) {
	r := gin.New()
	r.GET("/health", routes.HealthHandler(&mockPinger{err: nil}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("erwartet HTTP 200, erhalten %d", w.Code)
	}

	var res map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("fehler beim de-serialisieren der antwort: %v", err)
	}

	if res["status"] != "ok" || res["db"] != "ok" {
		t.Errorf("erwartet status: ok, db: ok; erhalten: %v", res)
	}
}

func TestHealthHandler_DBDown(t *testing.T) {
	r := gin.New()
	r.GET("/health", routes.HealthHandler(&mockPinger{err: errors.New("db down")}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("erwartet HTTP 503 bei DB down, erhalten %d", w.Code)
	}

	var res map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("fehler beim de-serialisieren der antwort: %v", err)
	}

	if res["status"] != "degraded" || res["db"] != "down" {
		t.Errorf("erwartet status: degraded, db: down; erhalten: %v", res)
	}
}
