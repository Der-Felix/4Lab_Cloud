package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidToken  = errors.New("ungueltiges refresh-token")
	ErrExpiredToken  = errors.New("abgelaufenes refresh-token")
	ErrReuseDetected = errors.New("token-wiederverwendung erkannt")
)

// GenerateRefreshToken erzeugt ein kryptografisch sicheres, opakes 32-Byte-Token
// im Base64url-Format sowie dessen SHA-256-Hash.
func GenerateRefreshToken() (plainToken string, tokenHash string, err error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", fmt.Errorf("fehler beim generieren von zufallsdaten: %w", err)
	}

	plainToken = base64.RawURLEncoding.EncodeToString(bytes)
	tokenHash = HashToken(plainToken)
	return plainToken, tokenHash, nil
}

// HashToken erzeugt die SHA-256 Hex-Repraesentation eines Tokens fuer die DB-Speicherung.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// TruncateIP kuerzt eine IP-Adresse gemaess DSGVO Art. 5 (Datensparsamkeit) auf /24 (IPv4) bzw. /48 (IPv6).
func TruncateIP(ipStr string) string {
	parsed := net.ParseIP(strings.TrimSpace(ipStr))
	if parsed == nil {
		return ""
	}
	if v4 := parsed.To4(); v4 != nil {
		return fmt.Sprintf("%d.%d.%d.0/24", v4[0], v4[1], v4[2])
	}
	mask := net.CIDRMask(48, 128)
	masked := parsed.Mask(mask)
	return fmt.Sprintf("%s/48", masked.String())
}

// Session repraesentiert eine aktive Login-Sitzung.
type Session struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UserAgent string    `json:"user_agent"`
	IP        string    `json:"ip"`
	ExpiresAt time.Time `json:"expires_at"`
}

// CreateRefreshToken speichert den Hash eines neuen Refresh-Tokens in der Datenbank.
func CreateRefreshToken(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, ttl time.Duration) (string, error) {
	return CreateRefreshTokenWithMeta(ctx, pool, userID, ttl, "", "")
}

// CreateRefreshTokenWithMeta speichert ein Refresh-Token inklusive User-Agent und gekuerzter IP-Adresse.
func CreateRefreshTokenWithMeta(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, ttl time.Duration, userAgent, rawIP string) (string, error) {
	plainToken, tokenHash, err := GenerateRefreshToken()
	if err != nil {
		return "", err
	}

	expiresAt := time.Now().Add(ttl)
	truncatedIP := TruncateIP(rawIP)

	_, err = pool.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, user_agent, ip_address, expires_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		userID, tokenHash, userAgent, truncatedIP, expiresAt,
	)
	if err != nil {
		return "", fmt.Errorf("fehler beim speichern des refresh-tokens: %w", err)
	}

	return plainToken, nil
}

// ValidateAndRotateRefreshToken prueft das uebergebene Refresh-Token und stellt atomar ein neues aus.
func ValidateAndRotateRefreshToken(ctx context.Context, pool *pgxpool.Pool, oldPlainToken string, ttl time.Duration) (userID uuid.UUID, newPlainToken string, err error) {
	return ValidateAndRotateRefreshTokenWithMeta(ctx, pool, oldPlainToken, ttl, "", "")
}

// ValidateAndRotateRefreshTokenWithMeta prueft das Token und speichert beim neuen Token User-Agent und IP.
func ValidateAndRotateRefreshTokenWithMeta(ctx context.Context, pool *pgxpool.Pool, oldPlainToken string, ttl time.Duration, userAgent, rawIP string) (userID uuid.UUID, newPlainToken string, err error) {
	oldHash := HashToken(oldPlainToken)

	// Transaktion starten, um Race Conditions bei simultanem Refresh abzufangen
	tx, err := pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("transaktion konnte nicht gestartet werden: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var (
		tokenID   uuid.UUID
		dbUserID  uuid.UUID
		expiresAt time.Time
		revokedAt *time.Time
	)

	err = tx.QueryRow(ctx,
		`SELECT id, user_id, expires_at, revoked_at
		 FROM refresh_tokens
		 WHERE token_hash = $1
		 FOR UPDATE`,
		oldHash,
	).Scan(&tokenID, &dbUserID, &expiresAt, &revokedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, "", ErrInvalidToken
		}
		return uuid.Nil, "", fmt.Errorf("datenbankfehler bei token-abfrage: %w", err)
	}

	// Wiederverwendungs-Erkennung (Reuse-Detection)
	if revokedAt != nil {
		// Token wurde bereits frueher revokt -> Alarm: Alle aktiven Tokens des Users widerrufen!
		_, _ = tx.Exec(ctx,
			`UPDATE refresh_tokens
			 SET revoked_at = now()
			 WHERE user_id = $1 AND revoked_at IS NULL`,
			dbUserID,
		)
		_ = tx.Commit(ctx)
		return dbUserID, "", ErrReuseDetected
	}

	// Ablaufpruefung
	if time.Now().After(expiresAt) {
		return uuid.Nil, "", ErrExpiredToken
	}

	// 1. Alten Token als widerrufen markieren
	_, err = tx.Exec(ctx,
		`UPDATE refresh_tokens
		 SET revoked_at = now()
		 WHERE id = $1`,
		tokenID,
	)
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("fehler beim widerrufen des alten tokens: %w", err)
	}

	// 2. Neues Token generieren und atomar eintragen
	newPlain, newHash, err := GenerateRefreshToken()
	if err != nil {
		return uuid.Nil, "", err
	}

	newExpiresAt := time.Now().Add(ttl)
	truncatedIP := TruncateIP(rawIP)

	_, err = tx.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, user_agent, ip_address, expires_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		dbUserID, newHash, userAgent, truncatedIP, newExpiresAt,
	)
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("fehler beim anlegen des rotierten tokens: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, "", fmt.Errorf("transaktion konnte nicht bestaetigt werden: %w", err)
	}

	return dbUserID, newPlain, nil
}

// ListActiveSessions gibt alle aktuell aktiven Login-Sitzungen eines Benutzers zurueck.
func ListActiveSessions(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID) ([]Session, error) {
	if pool == nil {
		return nil, errors.New("datenbankpool nicht verfuegbar")
	}

	rows, err := pool.Query(ctx,
		`SELECT id, created_at, COALESCE(user_agent, ''), COALESCE(ip_address, ''), expires_at
		 FROM refresh_tokens
		 WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > now()
		 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("fehler beim abfragen der sitzungen: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.CreatedAt, &s.UserAgent, &s.IP, &s.ExpiresAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// RevokeSession beendet gezielt eine einzelne Sitzung eines Benutzers anhand ihrer ID.
func RevokeSession(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, sessionID uuid.UUID) error {
	if pool == nil {
		return errors.New("datenbankpool nicht verfuegbar")
	}
	_, err := pool.Exec(ctx,
		`UPDATE refresh_tokens
		 SET revoked_at = now()
		 WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL`,
		sessionID, userID,
	)
	if err != nil {
		return fmt.Errorf("fehler beim beenden der sitzung: %w", err)
	}
	return nil
}

// RevokeOtherSessions beendet alle aktiven Sitzungen des Nutzers mit Ausnahme der aktuellen.
func RevokeOtherSessions(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, currentTokenHash string) error {
	if pool == nil {
		return errors.New("datenbankpool nicht verfuegbar")
	}
	if currentTokenHash != "" {
		_, err := pool.Exec(ctx,
			`UPDATE refresh_tokens
			 SET revoked_at = now()
			 WHERE user_id = $1 AND token_hash != $2 AND revoked_at IS NULL`,
			userID, currentTokenHash,
		)
		if err != nil {
			return fmt.Errorf("fehler beim beenden anderer sitzungen: %w", err)
		}
		return nil
	}
	return RevokeAllForUser(ctx, pool, userID)
}

// RevokeAllForUser widerruft saemtliche noch aktiven Refresh-Tokens eines Nutzers.
func RevokeAllForUser(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID) error {
	_, err := pool.Exec(ctx,
		`UPDATE refresh_tokens
		 SET revoked_at = now()
		 WHERE user_id = $1 AND revoked_at IS NULL`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("fehler beim widerrufen aller benutzer-tokens: %w", err)
	}
	return nil
}

// RevokeToken widerruft gezielt ein einzelnes Refresh-Token anhand seines Klartextwerts.
func RevokeToken(ctx context.Context, pool *pgxpool.Pool, plainToken string) error {
	tokenHash := HashToken(plainToken)
	_, err := pool.Exec(ctx,
		`UPDATE refresh_tokens
		 SET revoked_at = now()
		 WHERE token_hash = $1 AND revoked_at IS NULL`,
		tokenHash,
	)
	if err != nil {
		return fmt.Errorf("fehler beim widerrufen des tokens: %w", err)
	}
	return nil
}
