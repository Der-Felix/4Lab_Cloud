use axum::{extract::State, Json};
use std::sync::Arc;

use crate::{models::HealthResponse, AppState};

/// Healthcheck-Handler zur Pruefung des Dienstes und der Storage-Verfuegbarkeit.
pub async fn health_check(State(state): State<Arc<AppState>>) -> Json<HealthResponse> {
    // Pruefen, ob das Speicherverzeichnis existiert und zugaenglich ist
    let storage_status = match tokio::fs::metadata(&state.config.storage_dir).await {
        Ok(meta) if meta.is_dir() => "ok",
        Ok(_) => "degraded",
        Err(_) => {
            // Pruefen, ob es angelegt werden kann
            if tokio::fs::create_dir_all(&state.config.storage_dir).await.is_ok() {
                "ok"
            } else {
                "degraded"
            }
        }
    };

    Json(HealthResponse {
        status: "ok".to_string(),
        storage: storage_status.to_string(),
    })
}
