---
name: rust-axum-patterns
description: Rust-Konventionen fuer den 4labscloud Upload-Service. Bei jedem Rust-Code laden.
---

# Rust - Konventionen fuer 4labscloud

## Crate-Setup (Cargo.toml)
- axum = "0.8"
- tokio = { version = "1", features = ["full"] }
- sqlx = { version = "0.8", features = ["postgres", "runtime-tokio", "uuid", "chrono"] }
- aes-gcm = "0.10"
- sha2 = "0.10"
- thiserror = "2"
- tracing = "0.1"
- serde = { version = "1", features = ["derive"] }
- jsonwebtoken = "9"

## Struktur
src/
├── main.rs
├── config.rs       # Env-Vars laden
├── error.rs        # AppError + IntoResponse
├── auth.rs         # Service-Token-Pruefung
├── routes/
│   ├── mod.rs
│   ├── health.rs
│   └── uploads.rs
├── crypto.rs       # AES-256-GCM
└── models.rs       # Serde-Structs

## Patterns
- Fehler: eigenes AppError-Enum mit thiserror, impl IntoResponse fuer HTTP-Status
- State: Arc<AppState> mit PgPool, Config, Crypto-Key
- Handlers: async fn handler(State(state): State<Arc<AppState>>, ...) -> Result<Json<T>, AppError>
- Logs: tracing::info! / tracing::error! - NIEMALS Dateiinhalte loggen
- Chunks: Bytes von axum::body, nie String
- Streaming: tokio::fs::File + tokio_util::io::ReaderStream

## Crypto
- AES-256-GCM mit zufaelligem Nonce (12 Bytes) pro Datei
- Key aus Env-Variable STORAGE_KEY (Base64, 32 Bytes)
- Nonce wird VOR dem Ciphertext gespeichert (Format: nonce || ciphertext)
- SHA-256 Pruefsumme der KLARTEXT-Datei (nicht der verschluesselten)

## Verboten
- unwrap() in Handlern (nur in main erlaubt)
- println! (immer tracing)
- String fuer Binaerdaten
- Datei-Inhalte in Logs
