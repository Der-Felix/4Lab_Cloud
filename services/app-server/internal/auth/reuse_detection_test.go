package auth_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/4labscloud/4labscloud/services/app-server/internal/auth"
	"github.com/google/uuid"
)

func TestReuseDetection_RevokesAllTokens(t *testing.T) {
	pool := getTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	testUserID := uuid.New()

	// Benutzer fuer Test anlegen
	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (id) DO NOTHING`,
		testUserID, fmt.Sprintf("reuse_%s@test.internal", testUserID.String()[:8]), "dummy_hash",
	)
	if err != nil {
		t.Fatalf("test-benutzer konnte nicht angelegt werden: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", testUserID)
	}()

	// Zwei unabhaengige Sessions anlegen (z.B. Desktop und Mobiltelefon)
	tokenA, err := auth.CreateRefreshToken(ctx, pool, testUserID, 1*time.Hour)
	if err != nil {
		t.Fatalf("fehler beim anlegen von Token A: %v", err)
	}

	tokenB, err := auth.CreateRefreshToken(ctx, pool, testUserID, 1*time.Hour)
	if err != nil {
		t.Fatalf("fehler beim anlegen von Token B: %v", err)
	}

	// Token A wird normal rotiert -> Token A ist nun revoked, Token A2 aktiv
	_, tokenA2, err := auth.ValidateAndRotateRefreshToken(ctx, pool, tokenA, 1*time.Hour)
	if err != nil {
		t.Fatalf("fehler bei normaler Rotation von Token A: %v", err)
	}

	// Angreifer versucht, den alten Token A erneut einzuloesen!
	_, _, err = auth.ValidateAndRotateRefreshToken(ctx, pool, tokenA, 1*time.Hour)
	if !errors.Is(err, auth.ErrReuseDetected) {
		t.Fatalf("erwartet ErrReuseDetected bei reuse von Token A, erhalten: %v", err)
	}

	// Pruefung: Saemtliche aktiven Tokens des Nutzers (Token A2 und Token B) muessen nun widerrufen sein!
	var activeCount int
	err = pool.QueryRow(ctx,
		"SELECT count(*) FROM refresh_tokens WHERE user_id = $1 AND revoked_at IS NULL",
		testUserID,
	).Scan(&activeCount)

	if err != nil {
		t.Fatalf("fehler beim zaehlen aktiver tokens: %v", err)
	}

	if activeCount != 0 {
		t.Fatalf("erwartet 0 aktive Tokens nach Reuse-Detection, noch aktiv: %d", activeCount)
	}

	// Auch Versuch mit A2 oder B muss jetzt fehlschlagen
	_, _, err = auth.ValidateAndRotateRefreshToken(ctx, pool, tokenA2, 1*time.Hour)
	if !errors.Is(err, auth.ErrReuseDetected) {
		t.Fatalf("erwartet ErrReuseDetected fuer ehemals gueltiges Token A2, erhalten: %v", err)
	}

	_, _, err = auth.ValidateAndRotateRefreshToken(ctx, pool, tokenB, 1*time.Hour)
	if !errors.Is(err, auth.ErrReuseDetected) {
		t.Fatalf("erwartet ErrReuseDetected fuer ehemals gueltiges Token B, erhalten: %v", err)
	}
}
