package auth_test

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/4labscloud/4labscloud/services/app-server/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func getTestDB(t *testing.T) *pgxpool.Pool {
	host := os.Getenv("POSTGRES_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := os.Getenv("POSTGRES_PORT")
	if port == "" {
		port = "5432"
	}
	pass := os.Getenv("POSTGRES_PASSWORD")
	if pass == "" {
		pass = "ZW2rz93oet3JM4TH8Va71Y6U2XRsCMQ5"
	}
	user := os.Getenv("POSTGRES_USER")
	if user == "" {
		user = "4labs"
	}
	dbName := os.Getenv("POSTGRES_DB")
	if dbName == "" {
		dbName = "4labscloud"
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, dbName)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Skipf("DB-Pool konnte nicht initialisiert werden: %v", err)
		return nil
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("PostgreSQL nicht erreichbar (%s:%s): %v", host, port, err)
		return nil
	}
	return pool
}

func TestRefreshToken_GenerateAndHash(t *testing.T) {
	token, hash, err := auth.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("fehler beim generieren: %v", err)
	}

	if len(token) == 0 {
		t.Fatal("token darf nicht leer sein")
	}

	rawBytes, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("token ist kein gueltiges Base64url: %v", err)
	}
	if len(rawBytes) != 32 {
		t.Fatalf("erwartet 32 Bytes Zufallsdaten, erhalten %d", len(rawBytes))
	}

	expectedHash := auth.HashToken(token)
	if hash != expectedHash {
		t.Fatalf("hash-inkonsistenz: %s != %s", hash, expectedHash)
	}
}

func TestRefreshToken_Rotate(t *testing.T) {
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
		testUserID, fmt.Sprintf("refresh_%s@test.internal", testUserID.String()[:8]), "dummy_hash",
	)
	if err != nil {
		t.Fatalf("test-benutzer konnte nicht angelegt werden: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", testUserID)
	}()

	// 1. Refresh-Token erstellen
	token1, err := auth.CreateRefreshToken(ctx, pool, testUserID, 1*time.Hour)
	if err != nil {
		t.Fatalf("fehler bei CreateRefreshToken: %v", err)
	}

	// 2. Token rotieren
	returnedUserID, token2, err := auth.ValidateAndRotateRefreshToken(ctx, pool, token1, 1*time.Hour)
	if err != nil {
		t.Fatalf("fehler bei ValidateAndRotateRefreshToken: %v", err)
	}

	if returnedUserID != testUserID {
		t.Fatalf("falsche user-id: erwartet %s, erhalten %s", testUserID, returnedUserID)
	}
	if token2 == "" || token2 == token1 {
		t.Fatalf("rotiertes token ungueltig oder identisch zu vorherigem token")
	}

	// 3. Alter Token muss nun ungueltig sein (weil revoked)
	_, _, err = auth.ValidateAndRotateRefreshToken(ctx, pool, token1, 1*time.Hour)
	if !errors.Is(err, auth.ErrReuseDetected) {
		t.Fatalf("erwartet ErrReuseDetected bei Verwendung des alten Tokens, erhalten: %v", err)
	}
}
