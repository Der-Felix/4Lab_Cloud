use axum::{
    http::StatusCode,
    response::{IntoResponse, Response},
    Json,
};
use serde_json::json;
use thiserror::Error;

/// Fehlerarten im Upload-Service.
#[derive(Debug, Error)]
pub enum AppError {
    #[error("Authentifizierungsfehler: {0}")]
    Auth(String),

    #[error("Ungueltige Anfrage: {0}")]
    BadRequest(String),

    #[error("Kryptografiefehler: {0}")]
    Crypto(String),

    #[error("Konfigurationsfehler: {0}")]
    Config(String),

    #[error("Zugriff verweigert: {0}")]
    Forbidden(String),

    #[error("Speicherfehler: {0}")]
    Storage(String),

    #[error("Nicht gefunden: {0}")]
    NotFound(String),

    #[error("Interner Fehler: {0}")]
    Internal(String),
}

impl IntoResponse for AppError {
    fn into_response(self) -> Response {
        let (status, message) = match &self {
            AppError::Auth(msg) => (StatusCode::UNAUTHORIZED, msg.clone()),
            AppError::Forbidden(msg) => (StatusCode::FORBIDDEN, msg.clone()),
            AppError::BadRequest(msg) => (StatusCode::BAD_REQUEST, msg.clone()),
            AppError::Crypto(_) => (
                StatusCode::INTERNAL_SERVER_ERROR,
                "Verschluesselungsfehler aufgetreten".to_string(),
            ),
            AppError::Config(msg) => (StatusCode::INTERNAL_SERVER_ERROR, msg.clone()),
            AppError::Storage(msg) => (StatusCode::INTERNAL_SERVER_ERROR, msg.clone()),
            AppError::NotFound(msg) => (StatusCode::NOT_FOUND, msg.clone()),
            AppError::Internal(_) => (
                StatusCode::INTERNAL_SERVER_ERROR,
                "Interner Serverfehler".to_string(),
            ),
        };

        // Bei internen Fehlern strukturiertes Warning/Error loggen
        if status.is_server_error() {
            tracing::error!(error = %self, "interner fehler bei anfrage");
        }

        (status, Json(json!({ "error": message }))).into_response()
    }
}
