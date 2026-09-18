package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/4labscloud/4labscloud/services/app-server/internal/audit"
	"github.com/4labscloud/4labscloud/services/app-server/internal/auth"
	"github.com/4labscloud/4labscloud/services/app-server/internal/config"
	"github.com/4labscloud/4labscloud/services/app-server/internal/db"
	"github.com/4labscloud/4labscloud/services/app-server/internal/routes"
	"github.com/4labscloud/4labscloud/services/app-server/internal/workers"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// Strukturierten JSON-Logger initialisieren
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("4labscloud App-Server startet")

	// Konfiguration laden
	cfg, err := config.Load()
	if err != nil {
		slog.Error("konfiguration konnte nicht geladen werden", "error", err)
		os.Exit(1)
	}

	// Gin-Modus konfigurieren
	gin.SetMode(cfg.GinMode)

	// Hauptkontext mit Signalbehandlung fuer Graceful Shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// PostgreSQL-Verbindungspool aufbauen (tolerant bei verzögertem DB-Start)
	var dbPool *pgxpool.Pool
	dbPool, err = db.NewPool(ctx, cfg.DatabaseURL())
	if err != nil {
		slog.Warn("datenbankverbindung konnte beim start nicht hergestellt werden", "error", err)
	} else {
		defer dbPool.Close()
		slog.Info("datenbankverbindung erfolgreich hergestellt")
	}

	// Redis-Verbindung aufbauen
	redisClient, err := db.NewRedisClient(ctx, cfg.RedisHost, cfg.RedisPort, cfg.RedisPassword)
	if err != nil {
		slog.Warn("redis-verbindung konnte beim start nicht hergestellt werden", "error", err)
	} else {
		defer redisClient.Close()
		slog.Info("redis-verbindung erfolgreich hergestellt")
	}
	rateLimiter := auth.NewRedisRateLimiter(redisClient)

	// MFA-Schluessel fuer at-rest Verschluesselung der TOTP-Secrets dekodieren
	mfaKey, err := auth.DecodeMFAKey(cfg.MFAKey)
	if err != nil {
		slog.Error("ungueltiger mfa-key konfiguriert", "error", err)
		os.Exit(1)
	}

	// HMAC-Schluessel fuer Pseudonymisierung von Audit-Logs (DSGVO Art. 17)
	auditHMACKey, err := audit.DecodeHMACKey(cfg.AuditHMACKey)
	if err != nil {
		slog.Error("ungueltiger audit-hmac-key konfiguriert", "error", err)
		os.Exit(1)
	}

	// Audit-Logger und Handler initialisieren
	auditLogger := audit.NewLogger(dbPool)
	uploadsHandler := routes.NewUploadsHandler(cfg.UploadServiceURL, cfg.ServiceToken, dbPool, auditLogger)
	filesHandler := routes.NewFilesHandler(dbPool, cfg.UploadServiceURL, cfg.ServiceToken, auditLogger)
	publicShareHandler := routes.NewPublicShareHandler(dbPool, cfg.UploadServiceURL, cfg.ServiceToken, auditLogger)
	tagsHandler := routes.NewTagsHandler(dbPool, auditLogger)
	authHandler := routes.NewAuthHandler(dbPool, rateLimiter, auditLogger, cfg.JWTSecret, cfg.JWTTTLMinutes, mfaKey)
	userHandler := routes.NewUserHandler(dbPool, redisClient, auditLogger, cfg.UploadServiceURL, cfg.ServiceToken, mfaKey, auditHMACKey, cfg.DefaultQuotaGB)
	adminHandler := routes.NewAdminHandler(dbPool, auditLogger)

	// DSGVO Hintergrund-Worker fuer asynchrone Exporte und Retention-Cleanup starten
	dsgvoWorker := workers.NewDSGVOWorker(dbPool, redisClient, auditLogger, cfg.UploadServiceURL, cfg.ServiceToken, cfg.AuditRetentionDays)
	dsgvoWorker.Start(ctx)

	// Router aufsetzen
	router := gin.New()
	router.Use(gin.Recovery())

	// Strukturierte Zugriffs-Protokollierung
	router.Use(func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		c.Next()

		// Healthchecks nicht im Access-Log spammen
		if path != "/health" {
			slog.InfoContext(c.Request.Context(), "http-anfrage",
				"method", c.Request.Method,
				"path", path,
				"status", c.Writer.Status(),
				"duration_ms", time.Since(start).Milliseconds(),
				"client_ip", c.ClientIP(),
			)
		}
	})

	// Oeffentliche Endpunkte (Healthcheck)
	router.GET("/health", routes.HealthHandler(dbPool))

	// Oeffentliche Freigabe-Endpunkte (ohne JWT)
	router.GET("/api/v1/shares/:token", publicShareHandler.GetShareInfo)
	router.GET("/api/v1/shares/:token/download", publicShareHandler.DownloadShare)

	// Auth-Endpunkte
	authGroup := router.Group("/api/v1/auth")
	{
		authGroup.POST("/register", authHandler.Register)
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/refresh", authHandler.Refresh)
		authGroup.POST("/logout", authHandler.Logout)

		// MFA-Endpunkte
		authGroup.POST("/mfa/setup", authHandler.SetupMFA)
		authGroup.POST("/mfa/activate", authHandler.ActivateMFA)
		authGroup.POST("/mfa/verify", authHandler.VerifyMFA)
		authGroup.POST("/mfa/verify-recovery", authHandler.VerifyMFARecovery)
		authGroup.POST("/mfa/recovery-codes", auth.Middleware(cfg.JWTSecret, redisClient), authHandler.GenerateRecoveryCodesHandler)
		authGroup.POST("/mfa/deactivate", auth.Middleware(cfg.JWTSecret, redisClient), authHandler.DeactivateMFA)
	}

	// Authentifizierte API v1 Endpunkte
	v1 := router.Group("/api/v1")
	v1.Use(auth.Middleware(cfg.JWTSecret, redisClient))
	{
		v1.POST("/uploads/init", uploadsHandler.Init)
		v1.POST("/uploads/complete", uploadsHandler.Complete)

		// Dateiverwaltung & Shares
		v1.GET("/files", filesHandler.List)
		v1.GET("/files/:id/download", filesHandler.Download)
		v1.GET("/files/:id/thumbnail", filesHandler.Thumbnail)
		v1.GET("/files/:id/thumb", filesHandler.Thumbnail)
		v1.GET("/files/:id/exif", filesHandler.GetFileExif)
		v1.GET("/photos/map", filesHandler.GetPhotosMap)
		v1.DELETE("/files/:id", filesHandler.Delete)
		v1.PATCH("/files/:id", filesHandler.Rename)
		v1.POST("/files/:id/share", filesHandler.CreateShare)
		v1.GET("/shares", filesHandler.ListShares)
		v1.DELETE("/shares/:id", filesHandler.DeleteShare)

		// Tags & Alben
		v1.GET("/tags", tagsHandler.ListTags)
		v1.POST("/tags", tagsHandler.CreateTag)
		v1.GET("/tags/:id/files", tagsHandler.ListFilesByTag)
		v1.POST("/files/:id/tags", tagsHandler.AssignTag)
		v1.DELETE("/files/:id/tags/:tag_id", tagsHandler.RemoveTag)

		// Session-Management (DSGVO Art. 5 Datensparsamkeit)
		v1.GET("/auth/sessions", authHandler.ListSessions)
		v1.DELETE("/auth/sessions/:id", authHandler.RevokeSession)
		v1.DELETE("/auth/sessions", authHandler.RevokeOtherSessions)

		// DSGVO Account-Loeschung, Quota, Einstellungen und Datenexport
		v1.DELETE("/users/me", userHandler.DeleteMe)
		v1.GET("/users/me/quota", userHandler.GetQuota)
		v1.GET("/users/me/preferences", userHandler.GetPreferences)
		v1.PATCH("/users/me/preferences", userHandler.UpdatePreferences)
		v1.POST("/users/me/export", userHandler.RequestExport)
		v1.GET("/users/me/export/:job_id", userHandler.GetExportStatus)
		v1.GET("/users/me/export/:job_id/download", userHandler.DownloadExport)
		v1.GET("/audit/me", adminHandler.GetMyAuditLogs)

		// Admin-Endpunkte (ausschliesslich RequireAdmin)
		admin := v1.Group("")
		admin.Use(auth.RequireAdmin(dbPool))
		{
			admin.POST("/auth/invite", authHandler.Invite)
			admin.DELETE("/admin/users/:id", userHandler.AdminDeleteUser)
			admin.DELETE("/users/:id", userHandler.AdminDeleteUser)
			admin.GET("/admin/users", adminHandler.ListUsers)
			admin.GET("/admin/invitations", adminHandler.ListInvitations)
			admin.GET("/admin/audit", adminHandler.ListAuditLogs)
		}
	}

	// HTTP-Server konfigurieren
	serverAddr := fmt.Sprintf(":%d", cfg.AppPort)
	server := &http.Server{
		Addr:              serverAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Server in Goroutine starten
	go func() {
		slog.Info("server lauscht", "port", cfg.AppPort)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server unerwartet beendet", "error", err)
			stop()
		}
	}()

	// Warten auf Beendigungssignal
	<-ctx.Done()
	slog.Info("beendigungssignal empfangen, fahre server herunter")

	// Shutdown-Timeout fuer laufende Anfragen
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("fehler beim sauberen herunterfahren", "error", err)
	}

	slog.Info("4labscloud App-Server beendet")
}
