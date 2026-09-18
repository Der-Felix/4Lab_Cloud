package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config haelt alle Konfigurationswerte der Anwendung.
type Config struct {
	AppPort          int    `env:"APP_PORT" envDefault:"8080"`
	PostgresHost     string `env:"POSTGRES_HOST" envDefault:"localhost"`
	PostgresPort     int    `env:"POSTGRES_PORT" envDefault:"5432"`
	PostgresUser     string `env:"POSTGRES_USER" envDefault:"4labs"`
	PostgresPassword string `env:"POSTGRES_PASSWORD"`
	PostgresDB       string `env:"POSTGRES_DB" envDefault:"4labscloud"`
	RedisHost        string `env:"REDIS_HOST" envDefault:"localhost"`
	RedisPort        int    `env:"REDIS_PORT" envDefault:"6379"`
	RedisPassword    string `env:"REDIS_PASSWORD"`
	ServiceToken     string `env:"SERVICE_TOKEN"`
	JWTSecret        string `env:"JWT_SECRET"`
	JWTTTLMinutes    int    `env:"JWT_TTL_MINUTES" envDefault:"15"`
	MFAKey             string `env:"MFA_KEY"`
	AuditHMACKey       string `env:"AUDIT_HMAC_KEY"`
	AuditRetentionDays int    `env:"AUDIT_RETENTION_DAYS" envDefault:"90"`
	DefaultQuotaGB     int    `env:"DEFAULT_QUOTA_GB" envDefault:"50"`
	UploadServiceURL   string `env:"UPLOAD_SERVICE_URL" envDefault:"http://upload-service:8081"`
	GinMode            string `env:"GIN_MODE" envDefault:"release"`
	InContainer        string `env:"IN_CONTAINER" envDefault:""`
}

// Load laedt Umgebungsvariablen und validiert notwendige Werte.
func Load() (*Config, error) {
	// .env nur laden, wenn nicht im Container ausgefuehrt
	if os.Getenv("IN_CONTAINER") != "1" {
		// Suche nach .env im aktuellen Verzeichnis oder Workspace-Root
		if err := godotenv.Load(); err != nil {
			// Falls im Unterordner gestartet, zwei Ebenen hoeher suchen
			_ = godotenv.Load("../../.env")
		}
	}

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("fehler beim parsen der umgebungsvariablen: %w", err)
	}

	// Basis-Validierung wichtiger Sicherheitskonfigurationen
	if cfg.ServiceToken == "" {
		slog.Warn("SERVICE_TOKEN ist nicht gesetzt")
	}
	if cfg.JWTSecret == "" {
		slog.Warn("JWT_SECRET ist nicht gesetzt")
	}

	return cfg, nil
}

// DatabaseURL erzeugt den Verbindungsstring fuer PostgreSQL.
func (c *Config) DatabaseURL() string {
	userInfo := url.UserPassword(c.PostgresUser, c.PostgresPassword)
	return fmt.Sprintf("postgres://%s@%s:%d/%s?sslmode=disable",
		userInfo.String(),
		c.PostgresHost,
		c.PostgresPort,
		c.PostgresDB,
	)
}
