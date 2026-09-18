package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool initialisiert den PostgreSQL Connection-Pool mit Timeouts und Limits.
func NewPool(ctx context.Context, connString string) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("fehler beim parsen der datenbank-url: %w", err)
	}

	// Verbindungseinstellungen
	poolConfig.MaxConns = 25
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = 1 * time.Hour
	poolConfig.MaxConnIdleTime = 15 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("fehler beim erstellen des pgx-pools: %w", err)
	}

	// Verbindung mit Timeout pruefen
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("datenbank nicht erreichbar: %w", err)
	}

	return pool, nil
}

// SetUserContext setzt die lokale Postgres-Variable fuer Row Level Security (RLS).
// Dies muss innerhalb einer Transaktion aufgerufen werden.
func SetUserContext(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	_, err := tx.Exec(ctx, "SELECT set_config('app.user_id', $1, true)", userID.String())
	if err != nil {
		return fmt.Errorf("fehler beim setzen des rls-kontexts: %w", err)
	}
	return nil
}

// WithUserRLS fuehrt eine Funktion innerhalb einer Transaktion mit gesetztem RLS-User-Kontext aus.
func WithUserRLS(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, fn func(tx pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("transaktion konnte nicht gestartet werden: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := SetUserContext(ctx, tx, userID); err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("transaktion konnte nicht bestaetigt werden: %w", err)
	}

	return nil
}
