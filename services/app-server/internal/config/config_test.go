package config_test

import (
	"testing"

	"github.com/4labscloud/4labscloud/services/app-server/internal/config"
)

func TestConfigLoadDefaults(t *testing.T) {
	// Container-Flag setzen, um lokale .env Datei-Ueberschreibungen im Test zu verhindern
	t.Setenv("IN_CONTAINER", "1")
	t.Setenv("SERVICE_TOKEN", "test-token")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("POSTGRES_HOST", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("konfiguration konnte nicht geladen werden: %v", err)
	}

	if cfg.AppPort != 8080 {
		t.Errorf("erwartet AppPort 8080, erhalten %d", cfg.AppPort)
	}
	if cfg.PostgresHost != "localhost" {
		t.Errorf("erwartet PostgresHost localhost, erhalten %s", cfg.PostgresHost)
	}
	if cfg.ServiceToken != "test-token" {
		t.Errorf("erwartet ServiceToken test-token, erhalten %s", cfg.ServiceToken)
	}
	if cfg.JWTSecret != "test-secret" {
		t.Errorf("erwartet JWTSecret test-secret, erhalten %s", cfg.JWTSecret)
	}
}

func TestDatabaseURL(t *testing.T) {
	t.Setenv("IN_CONTAINER", "1")
	t.Setenv("POSTGRES_USER", "dbuser")
	t.Setenv("POSTGRES_PASSWORD", "dbpass")
	t.Setenv("POSTGRES_HOST", "postgres.internal")
	t.Setenv("POSTGRES_PORT", "5432")
	t.Setenv("POSTGRES_DB", "4labscloud")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("fehler beim laden: %v", err)
	}

	expected := "postgres://dbuser:dbpass@postgres.internal:5432/4labscloud?sslmode=disable"
	if cfg.DatabaseURL() != expected {
		t.Errorf("erwartet %s, erhalten %s", expected, cfg.DatabaseURL())
	}
}
