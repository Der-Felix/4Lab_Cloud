package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// InternalAuthMiddleware schuetzt interne Endpunkte mittels Service-Token.
// Verwendet konstante Zeit fuer den String-Vergleich gegen Timing-Angriffe.
func InternalAuthMiddleware(expectedToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if expectedToken == "" {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Internes Service-Token ist nicht konfiguriert"})
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Fehlender Authorization-Header"})
			return
		}

		if !strings.HasPrefix(authHeader, BearerPrefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Ungueltiges Authorization-Format"})
			return
		}

		token := strings.TrimPrefix(authHeader, BearerPrefix)

		// Zeitkonstanter Vergleich
		if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Ungueltiges Service-Token"})
			return
		}

		c.Next()
	}
}
