package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const (
	ContextUserIDKey = "user_id"
	BearerPrefix     = "Bearer "
)

// Middleware prueft das JWT im Authorization-Header und legt die user_id im Gin-Kontext ab.
// Optional kann ein Redis-Client zur Ueberpruefung der User-Blacklist uebergeben werden.
func Middleware(secret string, rdb ...*redis.Client) gin.HandlerFunc {
	var redisClient *redis.Client
	if len(rdb) > 0 {
		redisClient = rdb[0]
	}

	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Fehlender Authorization-Header"})
			return
		}

		if !strings.HasPrefix(authHeader, BearerPrefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Ungueltiges Authorization-Format, 'Bearer <token>' erwartet"})
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, BearerPrefix)
		userID, err := ParseToken(tokenStr, secret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Ungueltiges oder abgelaufenes Token"})
			return
		}

		// Blacklist-Pruefung fuer geloeschte Benutzerkonten (DSGVO Art. 17)
		if redisClient != nil {
			blacklisted, err := IsUserBlacklisted(c.Request.Context(), redisClient, userID)
			if err == nil && blacklisted {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Benutzerkonto wurde geloescht"})
				return
			}
		}

		// User-ID fuer nachfolgende Handler im Kontext speichern
		c.Set(ContextUserIDKey, userID)
		c.Next()
	}
}

// GetUserID liest die User-ID sicher aus dem Gin-Kontext aus.
func GetUserID(c *gin.Context) (uuid.UUID, bool) {
	val, exists := c.Get(ContextUserIDKey)
	if !exists {
		return uuid.Nil, false
	}
	id, ok := val.(uuid.UUID)
	return id, ok
}

// MustGetUserID liest die User-ID aus oder bricht mit 500 ab, falls die Middleware fehlte.
func MustGetUserID(c *gin.Context) uuid.UUID {
	id, ok := GetUserID(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Benutzerkontext fehlt"})
		return uuid.Nil
	}
	return id
}

// RequireAdmin stellt sicher, dass der authentifizierte Benutzer Admin-Rechte besitzt.
func RequireAdmin(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := MustGetUserID(c)
		if userID == uuid.Nil {
			return
		}

		if pool == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Datenbank nicht verfuegbar"})
			return
		}

		var isAdmin bool
		err := pool.QueryRow(c.Request.Context(), "SELECT is_admin FROM users WHERE id = $1", userID).Scan(&isAdmin)
		if err != nil || !isAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Administrator-Rechte erforderlich"})
			return
		}

		c.Set("is_admin", true)
		c.Next()
	}
}
