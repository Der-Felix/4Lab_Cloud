package db

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient initialisiert die Verbindung zu Redis und prueft die Erreichbarkeit.
func NewRedisClient(ctx context.Context, host string, port int, password string) (*redis.Client, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           0,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis nicht erreichbar (%s): %w", addr, err)
	}

	return client, nil
}
