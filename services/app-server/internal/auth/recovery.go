package auth

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	RecoveryCodesCount = 10
)

// GenerateRecoveryCodes generiert 10 einmalige Wiederherstellungscodes im Format XXXX-XXXX (Base32).
func GenerateRecoveryCodes() ([]string, error) {
	codes := make([]string, RecoveryCodesCount)
	encoder := base32.StdEncoding.WithPadding(base32.NoPadding)

	for i := 0; i < RecoveryCodesCount; i++ {
		// 5 Bytes ergeben genau 8 Base32-Zeichen
		randomBytes := make([]byte, 5)
		if _, err := rand.Read(randomBytes); err != nil {
			return nil, fmt.Errorf("fehler beim generieren von zufallsdaten fuer recovery-code: %w", err)
		}

		raw := strings.ToUpper(encoder.EncodeToString(randomBytes))
		// Format XXXX-XXXX
		formatted := fmt.Sprintf("%s-%s", raw[:4], raw[4:8])
		codes[i] = formatted
	}

	return codes, nil
}

// SaveRecoveryCodes invalidiert alte Codes des Nutzers und speichert die neuen Codes gehasht mit Argon2id.
func SaveRecoveryCodes(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, plainCodes []string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("transaktionsstart fehlgeschlagen: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// 1. Vorherige Codes entfernen
	_, err = tx.Exec(ctx, "DELETE FROM mfa_recovery_codes WHERE user_id = $1", userID)
	if err != nil {
		return fmt.Errorf("fehler beim loeschen alter recovery-codes: %w", err)
	}

	// 2. Neue Codes mit Argon2id hashen und speichern
	for _, code := range plainCodes {
		normCode := strings.ToUpper(strings.TrimSpace(code))
		hash, err := HashPassword(normCode)
		if err != nil {
			return fmt.Errorf("fehler beim hashen des recovery-codes: %w", err)
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO mfa_recovery_codes (user_id, code_hash)
			 VALUES ($1, $2)`,
			userID, hash,
		)
		if err != nil {
			return fmt.Errorf("fehler beim speichern des recovery-codes: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// ValidateAndConsumeRecoveryCode prueft den eingegebenen Code gegen alle noch unbenutzten Hashes des Nutzers
// und markiert ihn bei Treffer atomar als used_at = now().
func ValidateAndConsumeRecoveryCode(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, inputCode string) (bool, error) {
	normCode := strings.ToUpper(strings.TrimSpace(inputCode))

	tx, err := pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("transaktionsstart fehlgeschlagen: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	rows, err := tx.Query(ctx,
		`SELECT id, code_hash
		 FROM mfa_recovery_codes
		 WHERE user_id = $1 AND used_at IS NULL
		 FOR UPDATE`,
		userID,
	)
	if err != nil {
		return false, fmt.Errorf("fehler beim abfragen unbenutzter recovery-codes: %w", err)
	}
	defer rows.Close()

	type codeEntry struct {
		id   uuid.UUID
		hash string
	}
	var entries []codeEntry

	for rows.Next() {
		var e codeEntry
		if err := rows.Scan(&e.id, &e.hash); err != nil {
			return false, err
		}
		entries = append(entries, e)
	}
	rows.Close()

	var matchedID *uuid.UUID
	for _, e := range entries {
		valid, err := VerifyPassword(normCode, e.hash)
		if err == nil && valid {
			id := e.id
			matchedID = &id
			break
		}
	}

	if matchedID == nil {
		// Kein Treffer -> Dummy-Verifikation gegen Timing-Leaks
		VerifyDummyPassword(normCode)
		return false, nil
	}

	// Code als verwendet markieren
	_, err = tx.Exec(ctx,
		`UPDATE mfa_recovery_codes
		 SET used_at = now()
		 WHERE id = $1`,
		*matchedID,
	)
	if err != nil {
		return false, fmt.Errorf("fehler beim markieren des recovery-codes: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("fehler beim bestaetigen der transaktion: %w", err)
	}

	return true, nil
}
