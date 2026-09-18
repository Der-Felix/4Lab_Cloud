use axum::{
    extract::{Request, State},
    http::header,
    middleware::Next,
    response::Response,
};
use subtle::ConstantTimeEq;
use std::sync::Arc;

use crate::{error::AppError, AppState};

/// Validiert ein Service-Token zeitkonstant gegen das erwartete Geheimnis.
pub fn validate_service_token(provided: &str, expected: &str) -> Result<(), AppError> {
    let provided_bytes = provided.as_bytes();
    let expected_bytes = expected.as_bytes();

    if provided_bytes.ct_eq(expected_bytes).into() {
        Ok(())
    } else {
        Err(AppError::Auth("Ungueltiges Service-Token".to_string()))
    }
}

/// Axum-Middleware zum Schutz interner Endpunkte mittels Service-Token.
pub async fn require_service_token(
    State(state): State<Arc<AppState>>,
    req: Request,
    next: Next,
) -> Result<Response, AppError> {
    let auth_header = req
        .headers()
        .get(header::AUTHORIZATION)
        .and_then(|h| h.to_str().ok())
        .ok_or_else(|| AppError::Auth("Fehlender Authorization-Header".to_string()))?;

    let token = auth_header
        .strip_prefix("Bearer ")
        .ok_or_else(|| AppError::Auth("Ungueltiges Authorization-Format".to_string()))?;

    validate_service_token(token, &state.config.service_token)?;

    Ok(next.run(req).await)
}
