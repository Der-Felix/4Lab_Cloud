---
name: go-gin-conventions
description: Go-Konventionen fuer den 4labscloud App-Server. Bei jedem Go-Code laden.
---

# Go - Konventionen fuer 4labscloud

## Module
- Go 1.24+
- Router: Gin (github.com/gin-gonic/gin)
- DB: pgx v5 (github.com/jackc/pgx/v5/pgxpool)
- JWT: github.com/golang-jwt/jwt/v5
- TOTP: github.com/pquerna/otp
- Config: github.com/caarlos0/env/v11
- Logging: log/slog (Standard, strukturiert)

## Struktur
cmd/server/main.go
internal/
  config/config.go
  db/db.go
  auth/
    jwt.go
    mfa.go
    middleware.go
  users/handler.go
  shares/handler.go
  files/handler.go
  storage/client.go
  audit/log.go

## Patterns
- Handler: func (h *Handler) UploadInit(c *gin.Context)
- Auth-Middleware: setzt c.Set("user_id", ...), danach c.MustGet("user_id").(uuid.UUID)
- Service-Token zu Rust: Middleware internalAuth() prueft Authorization: Bearer
- DB-Queries: IMMER mit pgx Placeholder $1, $2 - keine String-Concatenation
- RLS aktivieren: SET LOCAL app.user_id = $1 vor jeder Query
- Fehler: c.JSON(http.StatusX, gin.H{"error": "..."}) - keine Stacktraces

## Rust-Client (HTTP)
type StorageClient struct {
    baseURL string
    token   string
    http    *http.Client
}
func (c *StorageClient) InitUpload(ctx context.Context, req UploadRequest) (*UploadResponse, error)
- Timeouts: 5s fuer Metadaten, KEIN Timeout fuer Upload-Streams
- Retry: 3x mit exponential Backoff bei 5xx

## Audit-Log
audit.Log(ctx, audit.Entry{
    UserID: userID,
    Action: "upload_init",
    TargetID: fileID,
    IP: c.ClientIP(),
    Result: "ok",
})

## Verboten
- panic() in Handlern
- fmt.Println (nur slog)
- SQL-Strings ohne Parameter
- Passwoerter oder Tokens loggen
