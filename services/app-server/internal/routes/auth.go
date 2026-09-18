package routes

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/4labscloud/4labscloud/services/app-server/internal/audit"
	"github.com/4labscloud/4labscloud/services/app-server/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AuthHandler verwaltet Authentifizierung, MFA, Token-Rotation und Einladungen.
type AuthHandler struct {
	dbPool        *pgxpool.Pool
	rateLimiter   auth.RateLimiter
	auditLogger   *audit.Logger
	jwtSecret     string
	jwtTTLMinutes int
	mfaKey        []byte
}

// NewAuthHandler initialisiert den Auth-Handler mit allen Abhaengigkeiten.
func NewAuthHandler(dbPool *pgxpool.Pool, rateLimiter auth.RateLimiter, auditLogger *audit.Logger, jwtSecret string, jwtTTLMinutes int, mfaKey []byte) *AuthHandler {
	if jwtTTLMinutes <= 0 {
		jwtTTLMinutes = 15
	}
	return &AuthHandler{
		dbPool:        dbPool,
		rateLimiter:   rateLimiter,
		auditLogger:   auditLogger,
		jwtSecret:     jwtSecret,
		jwtTTLMinutes: jwtTTLMinutes,
		mfaKey:        mfaKey,
	}
}

// RegisterRequest definiert die Eingabedaten fuer die Registrierung.
type RegisterRequest struct {
	Email           string `json:"email" binding:"required,email"`
	Password        string `json:"password" binding:"required,min=8"`
	InvitationToken string `json:"invitation_token"`
}

// Register fuehrt die Registrierung aus. Erster Nutzer wird Admin; danach nur per Einladung.
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungueltige Eingabedaten: " + err.Error()})
		return
	}

	normEmail := strings.ToLower(strings.TrimSpace(req.Email))

	if h.dbPool == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbank nicht verfuegbar"})
		return
	}

	// 1. Passwort vorab mit Argon2id hashen (CPU-intensiv, daher ausserhalb der DB-Transaktion)
	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "fehler beim hashen des passworts", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Interner Serverfehler"})
		return
	}

	// 2. Transaktion mit SERIALIZABLE Isolation starten (Schutz vor Race-Conditions bei parallelen Erstregistrierungen)
	tx, err := h.dbPool.BeginTx(c.Request.Context(), pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "transaktionsfehler bei registrierung", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Interner Serverfehler"})
		return
	}
	defer func() {
		_ = tx.Rollback(c.Request.Context())
	}()

	// 3. ZUERST pruefen, wie viele Benutzer existieren (Reihenfolge: count(users) -> invitation -> insert)
	var userCount int
	err = tx.QueryRow(c.Request.Context(), "SELECT count(*) FROM users").Scan(&userCount)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "datenbankfehler beim ermitteln der benutzeranzahl", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Interner Serverfehler"})
		return
	}

	var (
		isAdmin      bool
		invitationID *uuid.UUID
		newUserID    uuid.UUID
	)

	if userCount == 0 {
		// ERSTER BENUTZER: Keine Einladung erforderlich, wird Administrator
		isAdmin = true

		// Race-Condition Absicherung: INSERT nur wenn weiterhin kein User existiert
		err = tx.QueryRow(c.Request.Context(),
			`INSERT INTO users (email, password_hash, is_admin)
			 SELECT $1, $2, true
			 WHERE NOT EXISTS (SELECT 1 FROM users)
			 RETURNING id`,
			normEmail, hashedPassword,
		).Scan(&newUserID)

		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Race condition: Ein anderer User hat sich parallel als Erstnutzer registriert!
				h.logAudit(c.Request.Context(), nil, "register", nil, c.ClientIP(), "invalid_invitation")
				c.JSON(http.StatusForbidden, gin.H{"error": "Registrierung nur mit gueltiger Einladung moeglich"})
				return
			}
			slog.ErrorContext(c.Request.Context(), "fehler beim anlegen des ersten benutzers", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Interner Serverfehler"})
			return
		}
	} else {
		// NICHT LEER: Einladung ist zwingend erforderlich
		isAdmin = false

		if strings.TrimSpace(req.InvitationToken) == "" {
			h.logAudit(c.Request.Context(), nil, "register", nil, c.ClientIP(), "invalid_invitation")
			c.JSON(http.StatusForbidden, gin.H{"error": "Registrierung nur mit gueltiger Einladung moeglich"})
			return
		}

		tokenHash := auth.HashToken(req.InvitationToken)
		var (
			invID     uuid.UUID
			invEmail  string
			expiresAt time.Time
			usedAt    *time.Time
		)

		err = tx.QueryRow(c.Request.Context(),
			`SELECT id, email, expires_at, used_at
			 FROM invitations
			 WHERE token_hash = $1
			 FOR UPDATE`,
			tokenHash,
		).Scan(&invID, &invEmail, &expiresAt, &usedAt)

		if err != nil || usedAt != nil || time.Now().After(expiresAt) || !strings.EqualFold(invEmail, normEmail) {
			h.logAudit(c.Request.Context(), nil, "register", nil, c.ClientIP(), "invalid_invitation")
			c.JSON(http.StatusForbidden, gin.H{"error": "Einladung ist ungueltig, abgelaufen oder gehoert zu einer anderen E-Mail"})
			return
		}

		// Pruefen, ob die E-Mail-Adresse bereits registriert ist
		var existingCount int
		err = tx.QueryRow(c.Request.Context(), "SELECT count(*) FROM users WHERE email = $1", normEmail).Scan(&existingCount)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "datenbankfehler bei email-duplikatspruefung", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Interner Serverfehler"})
			return
		}
		if existingCount > 0 {
			h.logAudit(c.Request.Context(), nil, "register", nil, c.ClientIP(), "email_exists")
			c.JSON(http.StatusConflict, gin.H{"error": "E-Mail-Adresse bereits registriert"})
			return
		}

		invitationID = &invID

		// Benutzer anlegen
		err = tx.QueryRow(c.Request.Context(),
			`INSERT INTO users (email, password_hash, is_admin)
			 VALUES ($1, $2, false)
			 RETURNING id`,
			normEmail, hashedPassword,
		).Scan(&newUserID)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "fehler beim anlegen des benutzers", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Interner Serverfehler"})
			return
		}

		// Einladung als verbraucht markieren
		_, err = tx.Exec(c.Request.Context(),
			`UPDATE invitations
			 SET used_at = now()
			 WHERE id = $1`,
			*invitationID,
		)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "fehler beim aktualisieren der einladung", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Interner Serverfehler"})
			return
		}
	}

	// 5. Transaktion bestaetigen
	if err := tx.Commit(c.Request.Context()); err != nil {
		slog.ErrorContext(c.Request.Context(), "fehler beim bestaetigen der registrierung", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Interner Serverfehler"})
		return
	}

	// 6. Audit-Log erfassen
	h.logAudit(c.Request.Context(), &newUserID, "register", &newUserID, c.ClientIP(), "ok")

	c.JSON(http.StatusCreated, gin.H{
		"user_id":  newUserID,
		"email":    normEmail,
		"is_admin": isAdmin,
	})
}

// LoginRequest definiert die Eingabedaten fuer die Anmeldung.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Login validiert Benutzerdaten, fuehrt Rate-Limiting aus und behandelt MFA-Pflicht.
func (h *AuthHandler) Login(c *gin.Context) {
	start := time.Now()
	const minDuration = 200 * time.Millisecond
	ensureConstantTime := func() {
		elapsed := time.Since(start)
		if elapsed < minDuration {
			time.Sleep(minDuration - elapsed)
		}
	}

	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ensureConstantTime()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungueltige Anfrage: " + err.Error()})
		return
	}

	normEmail := strings.ToLower(strings.TrimSpace(req.Email))

	// 1. Rate-Limiting pruefen (IP & Email)
	if h.rateLimiter != nil {
		allowed, err := h.rateLimiter.CheckLoginLimit(c.Request.Context(), c.ClientIP(), normEmail)
		if err != nil {
			slog.WarnContext(c.Request.Context(), "fehler bei rate-limit-pruefung", "error", err)
		}
		if !allowed {
			h.logAudit(c.Request.Context(), nil, "login", nil, c.ClientIP(), "rate_limited")
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Zu viele Login-Versuche, bitte spaeter erneut versuchen"})
			return
		}
	}

	// 2. Benutzer abfragen
	if h.dbPool == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbank nicht verfuegbar"})
		return
	}

	var (
		userID       uuid.UUID
		passwordHash string
		isAdmin      bool
		mfaEnabled   bool
	)

	err := h.dbPool.QueryRow(c.Request.Context(),
		`SELECT id, password_hash, is_admin, mfa_enabled
		 FROM users
		 WHERE email = $1`,
		normEmail,
	).Scan(&userID, &passwordHash, &isAdmin, &mfaEnabled)

	if err != nil {
		auth.VerifyDummyPassword(req.Password)
		ensureConstantTime()
		h.logAudit(c.Request.Context(), nil, "login", nil, c.ClientIP(), "invalid_credentials")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Ungueltige Anmeldedaten"})
		return
	}

	// 3. Passwort mit Argon2id verifizieren
	valid, err := auth.VerifyPassword(req.Password, passwordHash)
	if err != nil || !valid {
		ensureConstantTime()
		h.logAudit(c.Request.Context(), &userID, "login", nil, c.ClientIP(), "invalid_credentials")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Ungueltige Anmeldedaten"})
		return
	}

	// 4. MFA-Pruefung: MFA ist OPTIONAL (Self-Hosted, kein Zwang)
	// Nutzer mit aktiviertem MFA -> 2. Faktor anfordern
	if mfaEnabled {
		mfaToken, err := auth.GenerateMFAPendingToken(userID, h.jwtSecret, 5*time.Minute)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Interner Serverfehler"})
			return
		}

		h.logAudit(c.Request.Context(), &userID, "login", &userID, c.ClientIP(), "mfa_required")

		c.JSON(http.StatusOK, gin.H{
			"mfa_required": true,
			"mfa_token":    mfaToken,
		})
		return
	}


	// 5. Regulärer Login ohne MFA: Access- & Refresh-Tokens ausstellen
	h.issueSessionTokens(c, userID, isAdmin)
}

// issueSessionTokens erzeugt Access- und Refresh-Tokens, setzt httpOnly- und CSRF-Cookies.
func (h *AuthHandler) issueSessionTokens(c *gin.Context, userID uuid.UUID, isAdmin bool) {
	accessToken, err := auth.GenerateToken(userID, h.jwtSecret, time.Duration(h.jwtTTLMinutes)*time.Minute)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "fehler beim erstellen des access-tokens", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Interner Serverfehler"})
		return
	}

	refreshToken, err := auth.CreateRefreshToken(c.Request.Context(), h.dbPool, userID, 30*24*time.Hour)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "fehler beim erstellen des refresh-tokens", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Interner Serverfehler"})
		return
	}

	csrfToken, err := auth.GenerateCSRFToken()
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "fehler beim erstellen des csrf-tokens", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Interner Serverfehler"})
		return
	}

	h.setAuthCookies(c, refreshToken, csrfToken)
	h.logAudit(c.Request.Context(), &userID, "login", &userID, c.ClientIP(), "ok")

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"csrf_token":    csrfToken,
		"expires_in":    h.jwtTTLMinutes * 60,
		"token_type":    "Bearer",
		"is_admin":      isAdmin,
	})
}

// RefreshRequest definiert das optionale Payload fuer die Token-Erneuerung.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Refresh tauscht ein Refresh-Token gegen ein neues Paar aus und validiert CSRF.
func (h *AuthHandler) Refresh(c *gin.Context) {
	// CSRF Double-Submit Validierung
	if !h.validateCSRF(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Ungueltiges oder fehlendes CSRF-Token"})
		return
	}

	var req RefreshRequest
	_ = c.ShouldBindJSON(&req)

	tokenToUse := req.RefreshToken
	if cookieToken, err := c.Cookie("refresh_token"); err == nil && cookieToken != "" {
		tokenToUse = cookieToken
	}

	if tokenToUse == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Refresh-Token ist erforderlich"})
		return
	}

	if h.dbPool == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbank nicht verfuegbar"})
		return
	}

	userID, newRefreshToken, err := auth.ValidateAndRotateRefreshToken(c.Request.Context(), h.dbPool, tokenToUse, 30*24*time.Hour)
	if err != nil {
		if errors.Is(err, auth.ErrReuseDetected) {
			h.clearAuthCookies(c)
			h.logAudit(c.Request.Context(), &userID, "refresh", nil, c.ClientIP(), "reuse_detected")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token-Wiederverwendung erkannt. Alle Sitzungen wurden beendet."})
			return
		}

		h.logAudit(c.Request.Context(), nil, "refresh", nil, c.ClientIP(), "invalid_token")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Ungueltiges oder abgelaufenes Refresh-Token"})
		return
	}

	accessToken, err := auth.GenerateToken(userID, h.jwtSecret, time.Duration(h.jwtTTLMinutes)*time.Minute)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "fehler beim erzeugen des neuen access-tokens", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Interner Serverfehler"})
		return
	}

	newCSRF, err := auth.GenerateCSRFToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Interner Serverfehler"})
		return
	}

	h.setAuthCookies(c, newRefreshToken, newCSRF)
	h.logAudit(c.Request.Context(), &userID, "refresh", nil, c.ClientIP(), "ok")

	var (
		userEmail string
		isAdmin   bool
	)
	_ = h.dbPool.QueryRow(c.Request.Context(), "SELECT email, is_admin FROM users WHERE id = $1", userID).Scan(&userEmail, &isAdmin)

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": newRefreshToken,
		"csrf_token":    newCSRF,
		"expires_in":    h.jwtTTLMinutes * 60,
		"token_type":    "Bearer",
		"email":         userEmail,
		"is_admin":      isAdmin,
	})
}

// LogoutRequest enthaelt optionale Abmeldedaten.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Logout invalidiert das Refresh-Token, loescht Cookies und prueft CSRF.
func (h *AuthHandler) Logout(c *gin.Context) {
	if !h.validateCSRF(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Ungueltiges oder fehlendes CSRF-Token"})
		return
	}

	var req LogoutRequest
	_ = c.ShouldBindJSON(&req)

	tokenToRevoke := req.RefreshToken
	if cookieToken, err := c.Cookie("refresh_token"); err == nil && cookieToken != "" {
		tokenToRevoke = cookieToken
	}

	userIDPtr := (*uuid.UUID)(nil)
	if userID, exists := auth.GetUserID(c); exists {
		userIDPtr = &userID
		if h.dbPool != nil {
			_ = auth.RevokeAllForUser(c.Request.Context(), h.dbPool, userID)
		}
	}

	if tokenToRevoke != "" && h.dbPool != nil {
		_ = auth.RevokeToken(c.Request.Context(), h.dbPool, tokenToRevoke)
	}

	h.clearAuthCookies(c)
	h.logAudit(c.Request.Context(), userIDPtr, "logout", nil, c.ClientIP(), "ok")

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// MFA-Endpunkte

// MFASetupRequest erlaubt optional die Uebergabe des mfa_token im Request-Body.
type MFASetupRequest struct {
	MFAToken string `json:"mfa_token"`
}

// SetupMFA generiert ein neues TOTP-Secret, verschluesselt es via AES-256-GCM und liefert otpauth:// URI.
func (h *AuthHandler) SetupMFA(c *gin.Context) {
	userID, err := h.getUserIDFromAccessOrMFA(c)
	if err != nil || userID == uuid.Nil {
		var req MFASetupRequest
		if c.ShouldBindJSON(&req) == nil && req.MFAToken != "" {
			userID, err = auth.ParseMFAPendingToken(req.MFAToken, h.jwtSecret)
		}
	}
	if err != nil || userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentifizierung erforderlich"})
		return
	}

	if h.dbPool == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbank nicht verfuegbar"})
		return
	}

	var email string
	err = h.dbPool.QueryRow(c.Request.Context(), "SELECT email FROM users WHERE id = $1", userID).Scan(&email)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Benutzer nicht gefunden"})
		return
	}

	secret, uri, err := auth.GenerateTOTPKey(email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Generieren des TOTP-Schluessels"})
		return
	}

	encryptedSecret, err := auth.EncryptMFASecret(h.mfaKey, secret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Verschluesseln des Secrets"})
		return
	}

	_, err = h.dbPool.Exec(c.Request.Context(),
		"UPDATE users SET mfa_secret_encrypted = $1 WHERE id = $2",
		encryptedSecret, userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Speichern des Secrets"})
		return
	}

	h.logAudit(c.Request.Context(), &userID, "mfa_setup", &userID, c.ClientIP(), "ok")

	c.JSON(http.StatusOK, gin.H{
		"secret": secret,
		"qr_uri": uri,
	})
}

// MFAActivateRequest enthaelt den ersten Bestaetigungscode zur Aktivierung.
type MFAActivateRequest struct {
	Code     string `json:"code" binding:"required"`
	MFAToken string `json:"mfa_token"`
}

// ActivateMFA validiert den ersten Code, aktiviert mfa_enabled=true und erzeugt 10 Recovery-Codes.
func (h *AuthHandler) ActivateMFA(c *gin.Context) {
	var req MFAActivateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Code ist erforderlich"})
		return
	}

	userID, err := h.getUserIDFromAccessOrMFA(c)
	if (err != nil || userID == uuid.Nil) && req.MFAToken != "" {
		userID, err = auth.ParseMFAPendingToken(req.MFAToken, h.jwtSecret)
	}
	if err != nil || userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentifizierung erforderlich"})
		return
	}

	var (
		encryptedSecret []byte
		isAdmin         bool
	)
	err = h.dbPool.QueryRow(c.Request.Context(),
		"SELECT mfa_secret_encrypted, is_admin FROM users WHERE id = $1",
		userID,
	).Scan(&encryptedSecret, &isAdmin)

	if err != nil || len(encryptedSecret) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "MFA wurde noch nicht initialisiert"})
		return
	}

	plainSecret, err := auth.DecryptMFASecret(h.mfaKey, encryptedSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Entschluesselung fehlgeschlagen"})
		return
	}

	if !auth.ValidateTOTPCode(req.Code, plainSecret) {
		h.logAudit(c.Request.Context(), &userID, "mfa_activate", &userID, c.ClientIP(), "invalid_code")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungueltiger TOTP-Code"})
		return
	}

	// 1. MFA als aktiv markieren
	_, err = h.dbPool.Exec(c.Request.Context(), "UPDATE users SET mfa_enabled = true WHERE id = $1", userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Aktivierung fehlgeschlagen"})
		return
	}

	// 2. 10 Recovery-Codes erzeugen und gehasht speichern
	recoveryCodes, err := auth.GenerateRecoveryCodes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Generieren der Recovery-Codes"})
		return
	}

	if err := auth.SaveRecoveryCodes(c.Request.Context(), h.dbPool, userID, recoveryCodes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Speichern der Recovery-Codes"})
		return
	}

	// 3. Tokens ausstellen (insb. fuer initialen Admin-Login)
	accessToken, _ := auth.GenerateToken(userID, h.jwtSecret, time.Duration(h.jwtTTLMinutes)*time.Minute)
	refreshToken, _ := auth.CreateRefreshToken(c.Request.Context(), h.dbPool, userID, 30*24*time.Hour)
	csrfToken, _ := auth.GenerateCSRFToken()
	h.setAuthCookies(c, refreshToken, csrfToken)

	h.logAudit(c.Request.Context(), &userID, "mfa_activate", &userID, c.ClientIP(), "ok")

	c.JSON(http.StatusOK, gin.H{
		"status":         "mfa_activated",
		"recovery_codes": recoveryCodes,
		"access_token":   accessToken,
		"refresh_token":  refreshToken,
		"csrf_token":     csrfToken,
		"expires_in":     h.jwtTTLMinutes * 60,
		"token_type":     "Bearer",
		"is_admin":       isAdmin,
	})
}

// MFAVerifyRequest enthaelt das temporaere MFA-Token und den 6-stelligen TOTP-Code.
type MFAVerifyRequest struct {
	MFAToken string `json:"mfa_token" binding:"required"`
	Code     string `json:"code" binding:"required"`
}

// VerifyMFA prueft den TOTP-Code waehrend des Logins und gibt Session-Tokens aus.
func (h *AuthHandler) VerifyMFA(c *gin.Context) {
	var req MFAVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mfa_token und code sind erforderlich"})
		return
	}

	userID, err := auth.ParseMFAPendingToken(req.MFAToken, h.jwtSecret)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Ungueltiges oder abgelaufenes MFA-Token"})
		return
	}

	// Rate-Limiting pruefen (max 5 pro mfa_token, max 10 pro User / Stunde)
	if h.rateLimiter != nil {
		tokenHash := auth.HashToken(req.MFAToken)
		allowed, err := h.rateLimiter.CheckMFAVerifyLimit(c.Request.Context(), tokenHash, userID.String())
		if err != nil {
			slog.WarnContext(c.Request.Context(), "fehler bei mfa-rate-limit-pruefung", "error", err)
		}
		if !allowed {
			h.logAudit(c.Request.Context(), &userID, "mfa_verify_rate_limited", &userID, c.ClientIP(), "rate_limited")
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Zu viele Verifikationsversuche, bitte spaeter erneut versuchen"})
			return
		}
	}

	if h.dbPool == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbank nicht verfuegbar"})
		return
	}

	var (
		encryptedSecret []byte
		isAdmin         bool
	)
	err = h.dbPool.QueryRow(c.Request.Context(),
		"SELECT mfa_secret_encrypted, is_admin FROM users WHERE id = $1 AND mfa_enabled = true",
		userID,
	).Scan(&encryptedSecret, &isAdmin)

	if err != nil || len(encryptedSecret) == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "MFA nicht konfiguriert"})
		return
	}

	plainSecret, err := auth.DecryptMFASecret(h.mfaKey, encryptedSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Entschluesselung fehlgeschlagen"})
		return
	}

	if !auth.ValidateTOTPCode(req.Code, plainSecret) {
		h.logAudit(c.Request.Context(), &userID, "mfa_verify", &userID, c.ClientIP(), "invalid_code")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Ungueltiger TOTP-Code"})
		return
	}

	h.logAudit(c.Request.Context(), &userID, "mfa_verify", &userID, c.ClientIP(), "ok")
	h.issueSessionTokens(c, userID, isAdmin)
}

// GenerateRecoveryCodesHandler erzeugt 10 neue Recovery-Codes fuer den authentifizierten Benutzer.
func (h *AuthHandler) GenerateRecoveryCodesHandler(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	codes, err := auth.GenerateRecoveryCodes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Generieren der Codes"})
		return
	}

	if err := auth.SaveRecoveryCodes(c.Request.Context(), h.dbPool, userID, codes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Speichern der Codes"})
		return
	}

	h.logAudit(c.Request.Context(), &userID, "mfa_recovery_regenerate", &userID, c.ClientIP(), "ok")

	c.JSON(http.StatusOK, gin.H{
		"recovery_codes": codes,
	})
}

// MFARecoveryVerifyRequest enthaelt das mfa_token und den Wiederherstellungscode.
type MFARecoveryVerifyRequest struct {
	MFAToken     string `json:"mfa_token" binding:"required"`
	RecoveryCode string `json:"recovery_code" binding:"required"`
}

// VerifyMFARecovery validiert einen einmaligen Recovery-Code bei Geraeteverlust.
func (h *AuthHandler) VerifyMFARecovery(c *gin.Context) {
	var req MFARecoveryVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mfa_token und recovery_code sind erforderlich"})
		return
	}

	userID, err := auth.ParseMFAPendingToken(req.MFAToken, h.jwtSecret)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Ungueltiges oder abgelaufenes MFA-Token"})
		return
	}

	// Rate-Limiting pruefen
	if h.rateLimiter != nil {
		tokenHash := auth.HashToken(req.MFAToken)
		allowed, err := h.rateLimiter.CheckMFAVerifyLimit(c.Request.Context(), tokenHash, userID.String())
		if err != nil {
			slog.WarnContext(c.Request.Context(), "fehler bei mfa-recovery-rate-limit-pruefung", "error", err)
		}
		if !allowed {
			h.logAudit(c.Request.Context(), &userID, "mfa_verify_rate_limited", &userID, c.ClientIP(), "rate_limited")
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Zu viele Verifikationsversuche, bitte spaeter erneut versuchen"})
			return
		}
	}

	if h.dbPool == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbank nicht verfuegbar"})
		return
	}

	var isAdmin bool
	err = h.dbPool.QueryRow(c.Request.Context(), "SELECT is_admin FROM users WHERE id = $1", userID).Scan(&isAdmin)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Benutzer nicht gefunden"})
		return
	}

	valid, err := auth.ValidateAndConsumeRecoveryCode(c.Request.Context(), h.dbPool, userID, req.RecoveryCode)
	if err != nil || !valid {
		h.logAudit(c.Request.Context(), &userID, "mfa_recovery_used", &userID, c.ClientIP(), "invalid_code")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Ungueltiger oder bereits verwendeter Recovery-Code"})
		return
	}

	h.logAudit(c.Request.Context(), &userID, "mfa_recovery_used", &userID, c.ClientIP(), "ok")
	h.issueSessionTokens(c, userID, isAdmin)
}

// MFADeactivateRequest enthaelt den Bestaetigungscode (TOTP oder Recovery) sowie eine optionale Ziel-User-ID.
type MFADeactivateRequest struct {
	Code   string     `json:"code" binding:"required"`
	UserID *uuid.UUID `json:"user_id"`
}

// DeactivateMFA deaktiviert die Zwei-Faktor-Authentifizierung nach Bestaetigung mit TOTP- oder Recovery-Code.
// Ein Administrator darf sein eigenes MFA nicht selbst deaktivieren (nur ein anderer Administrator).
func (h *AuthHandler) DeactivateMFA(c *gin.Context) {
	callerID := auth.MustGetUserID(c)
	if callerID == uuid.Nil {
		return
	}

	if h.dbPool == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbank nicht verfuegbar"})
		return
	}

	var req MFADeactivateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bestaetigungscode ist erforderlich"})
		return
	}

	var callerIsAdmin bool
	err := h.dbPool.QueryRow(c.Request.Context(), "SELECT is_admin FROM users WHERE id = $1", callerID).Scan(&callerIsAdmin)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Benutzer nicht gefunden"})
		return
	}

	targetID := callerID
	if req.UserID != nil && *req.UserID != uuid.Nil && *req.UserID != callerID {
		if !callerIsAdmin {
			h.logAudit(c.Request.Context(), &callerID, "mfa_deactivate", req.UserID, c.ClientIP(), "forbidden_not_admin")
			c.JSON(http.StatusForbidden, gin.H{"error": "Nur Administratoren koennen MFA fuer andere Benutzer deaktivieren"})
			return
		}
		targetID = *req.UserID
	}

	// Ein Administrator darf sein EIGENES MFA nicht deaktivieren
	if callerIsAdmin && targetID == callerID {
		h.logAudit(c.Request.Context(), &callerID, "mfa_deactivate", &callerID, c.ClientIP(), "admin_self_deactivation_forbidden")
		c.JSON(http.StatusForbidden, gin.H{"error": "Administratoren koennen ihr eigenes MFA nicht selbst deaktivieren"})
		return
	}

	var (
		mfaEnabled      bool
		encryptedSecret []byte
	)
	err = h.dbPool.QueryRow(c.Request.Context(),
		"SELECT mfa_enabled, mfa_secret_encrypted FROM users WHERE id = $1",
		targetID,
	).Scan(&mfaEnabled, &encryptedSecret)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Zielbenutzer nicht gefunden"})
		return
	}

	if !mfaEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "MFA ist fuer diesen Benutzer nicht aktiviert"})
		return
	}

	// Code pruefen: Entweder aktueller TOTP-Code ODER ungenutzter Recovery-Code
	codeValid := false

	// 1. TOTP-Pruefung
	if len(encryptedSecret) > 0 {
		plainSecret, err := auth.DecryptMFASecret(h.mfaKey, encryptedSecret)
		if err == nil && auth.ValidateTOTPCode(req.Code, plainSecret) {
			codeValid = true
		}
	}

	// 2. Recovery-Code-Pruefung
	if !codeValid {
		recoveryValid, err := auth.ValidateAndConsumeRecoveryCode(c.Request.Context(), h.dbPool, targetID, req.Code)
		if err == nil && recoveryValid {
			codeValid = true
		}
	}

	// Falls ein anderer Admin deaktivert, kann alternativ der TOTP-Code des Admins akzeptiert werden
	if !codeValid && callerIsAdmin && targetID != callerID {
		var adminSecretEnc []byte
		_ = h.dbPool.QueryRow(c.Request.Context(), "SELECT mfa_secret_encrypted FROM users WHERE id = $1", callerID).Scan(&adminSecretEnc)
		if len(adminSecretEnc) > 0 {
			adminPlainSecret, err := auth.DecryptMFASecret(h.mfaKey, adminSecretEnc)
			if err == nil && auth.ValidateTOTPCode(req.Code, adminPlainSecret) {
				codeValid = true
			}
		}
	}

	if !codeValid {
		h.logAudit(c.Request.Context(), &callerID, "mfa_deactivate", &targetID, c.ClientIP(), "invalid_code")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungueltiger TOTP- oder Recovery-Code"})
		return
	}

	// MFA deaktivieren und veraltete Recovery-Codes loeschen
	_, err = h.dbPool.Exec(c.Request.Context(),
		"UPDATE users SET mfa_enabled = false, mfa_secret_encrypted = NULL WHERE id = $1",
		targetID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Deaktivieren von MFA"})
		return
	}

	_, _ = h.dbPool.Exec(c.Request.Context(),
		"DELETE FROM mfa_recovery_codes WHERE user_id = $1",
		targetID,
	)

	h.logAudit(c.Request.Context(), &callerID, "mfa_deactivate", &targetID, c.ClientIP(), "ok")

	c.JSON(http.StatusOK, gin.H{
		"status":  "mfa_deactivated",
		"user_id": targetID,
	})
}

// InviteRequest definiert die Daten fuer die Erstellung einer Einladung.
type InviteRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// Invite erstellt eine neue Einladung (nur fuer Administratoren zugaenglich).
func (h *AuthHandler) Invite(c *gin.Context) {
	adminID := auth.MustGetUserID(c)
	if adminID == uuid.Nil {
		return
	}

	if h.dbPool == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbank nicht verfuegbar"})
		return
	}

	var req InviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungueltige E-Mail: " + err.Error()})
		return
	}

	normEmail := strings.ToLower(strings.TrimSpace(req.Email))

	plainToken, tokenHash, err := auth.GenerateRefreshToken()
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "fehler beim generieren des einladungs-tokens", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Interner Serverfehler"})
		return
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	var inviteID uuid.UUID
	err = h.dbPool.QueryRow(c.Request.Context(),
		`INSERT INTO invitations (email, token_hash, created_by, expires_at)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		normEmail, tokenHash, adminID, expiresAt,
	).Scan(&inviteID)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "fehler beim speichern der einladung", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Interner Serverfehler"})
		return
	}

	h.logAudit(c.Request.Context(), &adminID, "invitation_create", &inviteID, c.ClientIP(), "ok")

	c.JSON(http.StatusCreated, gin.H{
		"invitation_token": plainToken,
		"email":            normEmail,
		"expires_at":       expiresAt.Format(time.RFC3339),
	})
}

// Hilfsfunktionen fuer Cookies und CSRF

func (h *AuthHandler) setAuthCookies(c *gin.Context, refreshToken, csrfToken string) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("refresh_token", refreshToken, 30*24*3600, "/api/v1/auth", "", true, true)
	c.SetCookie("csrf_token", csrfToken, 30*24*3600, "/", "", true, false)
}

func (h *AuthHandler) clearAuthCookies(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("refresh_token", "", -1, "/api/v1/auth", "", true, true)
	c.SetCookie("csrf_token", "", -1, "/", "", true, false)
}

func (h *AuthHandler) validateCSRF(c *gin.Context) bool {
	headerCSRF := c.GetHeader("X-CSRF-Token")
	cookieCSRF, _ := c.Cookie("csrf_token")
	if headerCSRF == "" || cookieCSRF == "" {
		return false
	}
	return auth.ConstantTimeCompare(headerCSRF, cookieCSRF)
}

func (h *AuthHandler) getUserIDFromAccessOrMFA(c *gin.Context) (uuid.UUID, error) {
	if userID, ok := auth.GetUserID(c); ok && userID != uuid.Nil {
		return userID, nil
	}
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		// Zuerst als regulaeres Access-Token versuchen
		if uid, err := auth.ParseToken(tokenStr, h.jwtSecret); err == nil && uid != uuid.Nil {
			return uid, nil
		}
		// Falls kein Access-Token, als mfa_pending Token pruefen
		return auth.ParseMFAPendingToken(tokenStr, h.jwtSecret)
	}
	mfaHeader := c.GetHeader("X-MFA-Token")
	if mfaHeader != "" {
		return auth.ParseMFAPendingToken(mfaHeader, h.jwtSecret)
	}
	return uuid.Nil, errors.New("keine gueltige authentifizierung vorhanden")
}

func (h *AuthHandler) logAudit(ctx context.Context, userID *uuid.UUID, action string, targetID *uuid.UUID, ip, result string) {
	if h.auditLogger != nil {
		h.auditLogger.Log(ctx, audit.Entry{
			UserID:   userID,
			Action:   action,
			TargetID: targetID,
			IP:       ip,
			Result:   result,
		})
	}
}
