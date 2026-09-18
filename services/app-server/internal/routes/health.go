package routes

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Pinger abstrahiert den Healthcheck-Ping fuer die Datenbank.
type Pinger interface {
	Ping(ctx context.Context) error
}

// HealthHandler prueft die Verfuegbarkeit des Servers und der Datenbank.
func HealthHandler(db Pinger) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if db == nil {
			slog.WarnContext(c.Request.Context(), "healthcheck fehlgeschlagen: kein datenbanktreiber konfiguriert")
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "degraded",
				"db":     "down",
			})
			return
		}

		if err := db.Ping(ctx); err != nil {
			slog.WarnContext(c.Request.Context(), "healthcheck fehlgeschlagen: datenbank nicht erreichbar", "error", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "degraded",
				"db":     "down",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"db":     "ok",
		})
	}
}
