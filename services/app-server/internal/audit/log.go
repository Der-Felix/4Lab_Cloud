package audit

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Entry beschreibt einen Eintrag im revisionssicheren Audit-Log.
type Entry struct {
	UserID        *uuid.UUID
	PseudonymHash *string
	Action        string
	TargetID      *uuid.UUID
	IP            string
	Result        string
}

// Logger stellt Methoden zum Schreiben von Audit-Logs bereit.
type Logger struct {
	pool *pgxpool.Pool
}

// NewLogger instanziiert einen neuen Audit-Logger.
func NewLogger(pool *pgxpool.Pool) *Logger {
	return &Logger{pool: pool}
}

// Log schreibt einen strukturierten Eintrag in die audit_log-Tabelle.
func (l *Logger) Log(ctx context.Context, entry Entry) {
	if l.pool == nil {
		slog.WarnContext(ctx, "audit-log uebersprungen: datenbankpool ist nicht verfuegbar",
			"action", entry.Action,
			"result", entry.Result,
		)
		return
	}

	var ipParam any
	if parsedIP := net.ParseIP(entry.IP); parsedIP != nil {
		ipParam = entry.IP
	}

	query := `
		INSERT INTO audit_log (user_id, pseudonym_hash, action, target_id, ip_address, result)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := l.pool.Exec(ctx, query,
		entry.UserID,
		entry.PseudonymHash,
		entry.Action,
		entry.TargetID,
		ipParam,
		entry.Result,
	)

	if err != nil {
		slog.ErrorContext(ctx, "fehler beim schreiben in audit_log",
			"error", err,
			"action", entry.Action,
			"result", entry.Result,
		)
		return
	}

	slog.InfoContext(ctx, "audit_log erfasst",
		"action", entry.Action,
		"result", entry.Result,
	)
}

// DecodeHMACKey dekodiert den Base64-codierten 32-Byte Audit-HMAC-Schluessel.
func DecodeHMACKey(keyB64 string) ([]byte, error) {
	if keyB64 == "" {
		return nil, errors.New("audit hmac key darf nicht leer sein")
	}
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return nil, fmt.Errorf("ungueltiges base64 im hmac-key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("audit hmac key muss genau 32 bytes lang sein, hat %d bytes", len(key))
	}
	return key, nil
}

// CalculatePseudonym berechnet den deterministischen HMAC-SHA256 Hash fuer eine User-ID.
func CalculatePseudonym(userID uuid.UUID, hmacKey []byte) string {
	h := hmac.New(sha256.New, hmacKey)
	h.Write([]byte(userID.String()))
	return hex.EncodeToString(h.Sum(nil))
}

// PseudonymizeUser ersetzt user_id in audit_log durch den HMAC-Hash und setzt user_id auf NULL.
// Gemaess DSGVO Art. 17 Abs. 3 lit. e fuer Sicherheitsanalysen ohne Re-Identifikation.
func PseudonymizeUser(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, hmacKey []byte) (string, error) {
	if pool == nil {
		return "", errors.New("datenbankpool nicht verfuegbar")
	}

	pseudonym := CalculatePseudonym(userID, hmacKey)

	_, err := pool.Exec(ctx,
		`UPDATE audit_log
		 SET pseudonym_hash = $1, user_id = NULL
		 WHERE user_id = $2`,
		pseudonym, userID,
	)
	if err != nil {
		return "", fmt.Errorf("fehler beim pseudonymisieren der audit-logs: %w", err)
	}

	return pseudonym, nil
}

// CleanupRetention loescht Audit-Log-Eintraege, die aelter als die konfigurierte Aufbewahrungsfrist sind.
func CleanupRetention(ctx context.Context, pool *pgxpool.Pool, retentionDays int) (int64, error) {
	if pool == nil {
		return 0, errors.New("datenbankpool nicht verfuegbar")
	}
	if retentionDays <= 0 {
		retentionDays = 90
	}

	intervalStr := fmt.Sprintf("%d days", retentionDays)
	tag, err := pool.Exec(ctx,
		`DELETE FROM audit_log
		 WHERE created_at < now() - $1::interval`,
		intervalStr,
	)
	if err != nil {
		return 0, fmt.Errorf("fehler beim bereinigen abgelaufener audit-logs: %w", err)
	}

	return tag.RowsAffected(), nil
}
