package routes

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/4labscloud/4labscloud/services/app-server/internal/audit"
	"github.com/4labscloud/4labscloud/services/app-server/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AdminHandler verwaltet administrative Aufgaben wie Nutzerliste, Einladungen und Audit-Logs.
type AdminHandler struct {
	dbPool      *pgxpool.Pool
	auditLogger *audit.Logger
}

// NewAdminHandler instanziiert den Admin-Handler.
func NewAdminHandler(dbPool *pgxpool.Pool, auditLogger *audit.Logger) *AdminHandler {
	return &AdminHandler{
		dbPool:      dbPool,
		auditLogger: auditLogger,
	}
}

// UserListItem repraesentiert einen Benutzer in der administrativen Uebersicht.
type UserListItem struct {
	ID         string    `json:"id"`
	Email      string    `json:"email"`
	IsAdmin    bool      `json:"is_admin"`
	MFAEnabled bool      `json:"mfa_enabled"`
	CreatedAt  time.Time `json:"created_at"`
}

// ListUsers liefert alle registrierten Benutzer geordnet nach Erstellungsdatum.
func (h *AdminHandler) ListUsers(c *gin.Context) {
	if h.dbPool == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbank nicht verfuegbar"})
		return
	}

	rows, err := h.dbPool.Query(c.Request.Context(),
		`SELECT id, email, is_admin, mfa_enabled, created_at
		 FROM users
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "fehler beim laden der benutzerliste", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Benutzerliste konnte nicht geladen werden"})
		return
	}
	defer rows.Close()

	users := make([]UserListItem, 0)
	for rows.Next() {
		var u UserListItem
		var uid uuid.UUID
		if err := rows.Scan(&uid, &u.Email, &u.IsAdmin, &u.MFAEnabled, &u.CreatedAt); err != nil {
			slog.ErrorContext(c.Request.Context(), "fehler beim parsen eines benutzers", "error", err)
			continue
		}
		u.ID = uid.String()
		users = append(users, u)
	}

	c.JSON(http.StatusOK, gin.H{"users": users})
}

// InvitationListItem repraesentiert eine offene Einladung.
type InvitationListItem struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ListInvitations gibt alle noch ungenutzten und nicht abgelaufenen Einladungen zurueck.
func (h *AdminHandler) ListInvitations(c *gin.Context) {
	if h.dbPool == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbank nicht verfuegbar"})
		return
	}

	rows, err := h.dbPool.Query(c.Request.Context(),
		`SELECT id, email, created_at, expires_at
		 FROM invitations
		 WHERE used_at IS NULL AND expires_at > now()
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "fehler beim laden der einladungen", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Einladungen konnten nicht geladen werden"})
		return
	}
	defer rows.Close()

	invitations := make([]InvitationListItem, 0)
	for rows.Next() {
		var inv InvitationListItem
		var iid uuid.UUID
		if err := rows.Scan(&iid, &inv.Email, &inv.CreatedAt, &inv.ExpiresAt); err != nil {
			slog.ErrorContext(c.Request.Context(), "fehler beim parsen einer einladung", "error", err)
			continue
		}
		inv.ID = iid.String()
		invitations = append(invitations, inv)
	}

	c.JSON(http.StatusOK, gin.H{"invitations": invitations})
}

// AuditLogListItem repraesentiert einen Eintrag im Audit-Protokoll.
type AuditLogListItem struct {
	ID            int64     `json:"id"`
	UserID        *string   `json:"user_id,omitempty"`
	UserEmail     *string   `json:"user_email,omitempty"`
	PseudonymHash *string   `json:"pseudonym_hash,omitempty"`
	Action        string    `json:"action"`
	TargetID      *string   `json:"target_id,omitempty"`
	IPAddress     string    `json:"ip_address"`
	Result        string    `json:"result"`
	CreatedAt     time.Time `json:"created_at"`
}

// ListAuditLogs liefert die letzten 100 Eintraege des Audit-Logs mit optionalem Filter.
func (h *AdminHandler) ListAuditLogs(c *gin.Context) {
	if h.dbPool == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbank nicht verfuegbar"})
		return
	}

	actionFilter := c.Query("action")
	userFilter := c.Query("user")

	query := `
		SELECT a.id, a.user_id, a.pseudonym_hash, a.action, a.target_id,
		       COALESCE(a.ip_address::text, '') as ip_text, a.result, a.created_at,
		       u.email
		FROM audit_log a
		LEFT JOIN users u ON a.user_id = u.id
		WHERE ($1 = '' OR a.action = $1)
		  AND ($2 = '' OR u.email ILIKE '%' || $2 || '%' OR a.user_id::text = $2)
		ORDER BY a.created_at DESC
		LIMIT 100`

	rows, err := h.dbPool.Query(c.Request.Context(), query, actionFilter, userFilter)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "fehler beim abfragen des audit-logs", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Audit-Log konnte nicht geladen werden"})
		return
	}
	defer rows.Close()

	logs := make([]AuditLogListItem, 0)
	for rows.Next() {
		var item AuditLogListItem
		var uid *uuid.UUID
		var tid *uuid.UUID
		var uEmail *string

		if err := rows.Scan(
			&item.ID,
			&uid,
			&item.PseudonymHash,
			&item.Action,
			&tid,
			&item.IPAddress,
			&item.Result,
			&item.CreatedAt,
			&uEmail,
		); err != nil {
			slog.ErrorContext(c.Request.Context(), "fehler beim parsen eines audit-logs", "error", err)
			continue
		}

		if uid != nil {
			s := uid.String()
			item.UserID = &s
		}
		if tid != nil {
			s := tid.String()
			item.TargetID = &s
		}
		item.UserEmail = uEmail

		logs = append(logs, item)
	}

	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

// GetMyAuditLogs liefert Audit-Eintraege des aktuell angemeldeten Benutzers (DSGVO Auskunftsrecht).
func (h *AdminHandler) GetMyAuditLogs(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	if h.dbPool == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbank nicht verfuegbar"})
		return
	}

	query := `
		SELECT id, user_id, pseudonym_hash, action, target_id,
		       COALESCE(ip_address::text, '') as ip_text, result, created_at
		FROM audit_log
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 50`

	rows, err := h.dbPool.Query(c.Request.Context(), query, userID)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "fehler beim abfragen des persoenlichen audit-logs", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Audit-Log konnte nicht geladen werden"})
		return
	}
	defer rows.Close()

	logs := make([]AuditLogListItem, 0)
	for rows.Next() {
		var item AuditLogListItem
		var uid *uuid.UUID
		var tid *uuid.UUID

		if err := rows.Scan(
			&item.ID,
			&uid,
			&item.PseudonymHash,
			&item.Action,
			&tid,
			&item.IPAddress,
			&item.Result,
			&item.CreatedAt,
		); err != nil {
			continue
		}

		if uid != nil {
			s := uid.String()
			item.UserID = &s
		}
		if tid != nil {
			s := tid.String()
			item.TargetID = &s
		}

		logs = append(logs, item)
	}

	c.JSON(http.StatusOK, gin.H{"logs": logs})
}
