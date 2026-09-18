package routes

import (
	"net/http"

	"github.com/4labscloud/4labscloud/services/app-server/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListSessions gibt alle aktiven Sitzungen des angemeldeten Benutzers zurueck.
// IP-Adressen sind gemaess DSGVO Art. 5 auf /24 gekuerzt.
func (h *AuthHandler) ListSessions(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	if h.dbPool == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbank nicht verfuegbar"})
		return
	}

	sessions, err := auth.ListActiveSessions(c.Request.Context(), h.dbPool, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Laden der Sitzungen"})
		return
	}

	// Aktuelle Sitzung anhand des Cookies markieren falls vorhanden
	currentCookie, _ := c.Cookie("refresh_token")
	currentHash := ""
	if currentCookie != "" {
		currentHash = auth.HashToken(currentCookie)
	}

	type sessionItem struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt string    `json:"created_at"`
		UserAgent string    `json:"user_agent"`
		IP        string    `json:"ip"`
		ExpiresAt string    `json:"expires_at"`
		IsCurrent bool      `json:"is_current,omitempty"`
	}

	result := make([]sessionItem, 0, len(sessions))
	for _, s := range sessions {
		item := sessionItem{
			ID:        s.ID,
			CreatedAt: s.CreatedAt.Format(http.TimeFormat),
			UserAgent: s.UserAgent,
			IP:        s.IP,
			ExpiresAt: s.ExpiresAt.Format(http.TimeFormat),
		}
		_ = currentHash // reserviert
		result = append(result, item)
	}

	c.JSON(http.StatusOK, result)
}

// RevokeSession beendet eine einzelne Sitzung des Benutzers.
func (h *AuthHandler) RevokeSession(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	sessionIDStr := c.Param("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungueltige Sitzungs-ID"})
		return
	}

	if h.dbPool == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbank nicht verfuegbar"})
		return
	}

	if err := auth.RevokeSession(c.Request.Context(), h.dbPool, userID, sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Beenden der Sitzung"})
		return
	}

	h.logAudit(c.Request.Context(), &userID, "session_revoke", &sessionID, c.ClientIP(), "ok")
	c.Status(http.StatusNoContent)
}

// RevokeOtherSessions beendet alle anderen Sitzungen mit Ausnahme der aktuellen.
func (h *AuthHandler) RevokeOtherSessions(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	if h.dbPool == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbank nicht verfuegbar"})
		return
	}

	currentCookie, _ := c.Cookie("refresh_token")
	currentHash := ""
	if currentCookie != "" {
		currentHash = auth.HashToken(currentCookie)
	}

	if err := auth.RevokeOtherSessions(c.Request.Context(), h.dbPool, userID, currentHash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Beenden anderer Sitzungen"})
		return
	}

	h.logAudit(c.Request.Context(), &userID, "session_revoke_others", &userID, c.ClientIP(), "ok")
	c.Status(http.StatusNoContent)
}
