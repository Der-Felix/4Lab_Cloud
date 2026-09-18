use axum::{extract::State, http::StatusCode, Json};
use chrono::{Duration, Utc};
use std::sync::Arc;
use uuid::Uuid;

use crate::{
    error::AppError,
    models::{UploadInitRequest, UploadInitResponse},
    AppState,
};

/// Initialisiert eine Upload-Session fuer eine Datei.
/// Wird intern vom Go-App-Server aufgerufen.
pub async fn init_upload(
    State(_state): State<Arc<AppState>>,
    Json(payload): Json<UploadInitRequest>,
) -> Result<Json<UploadInitResponse>, AppError> {
    // Validierung der Eingabeparameter
    if payload.filename.trim().is_empty() {
        return Err(AppError::BadRequest("Dateiname darf nicht leer sein".to_string()));
    }

    if payload.size_bytes <= 0 {
        return Err(AppError::BadRequest(
            "Dateigroesse muss groesser als 0 sein".to_string(),
        ));
    }

    let upload_id = Uuid::new_v4();

    // In Datenbank registrieren
    crate::db::create_session(
        &_state.db_pool,
        upload_id,
        payload.user_id,
        &payload.filename,
        payload.size_bytes,
    )
    .await?;

    let expires_at = Utc::now() + Duration::minutes(15);
    let presigned_url = format!("/api/v1/uploads/tus/{upload_id}");

    tracing::info!(
        upload_id = %upload_id,
        user_id = %payload.user_id,
        filename = %payload.filename,
        size_bytes = payload.size_bytes,
        "upload-session erfolgreich initialisiert"
    );

    Ok(Json(UploadInitResponse {
        upload_id,
        presigned_url,
        expires_at,
    }))
}

/// Liefert den Status einer Upload-Session fuer den Go-Server (intern).
pub async fn get_upload_status(
    State(state): State<Arc<AppState>>,
    axum::extract::Path(upload_id): axum::extract::Path<Uuid>,
) -> Result<Json<crate::models::UploadStatusResponse>, AppError> {
    let session = crate::db::get_session(&state.db_pool, upload_id)
        .await?
        .ok_or_else(|| AppError::NotFound("Upload-Session nicht gefunden".to_string()))?;

    let storage_path = crate::storage::get_relative_storage_path(upload_id);

    Ok(Json(crate::models::UploadStatusResponse {
        upload_id: session.id,
        user_id: session.user_id,
        filename: session.filename,
        size_bytes: session.size_bytes,
        checksum: session.checksum,
        storage_path,
        status: session.status,
    }))
}

#[derive(serde::Deserialize)]
pub struct DeleteUploadQuery {
    pub delete_file: Option<bool>,
}

/// Loescht eine Upload-Session und optional die Datei auf Platte (intern vom Go-Server aufgerufen).
pub async fn delete_upload(
    State(state): State<Arc<AppState>>,
    axum::extract::Path(upload_id): axum::extract::Path<Uuid>,
    axum::extract::Query(query): axum::extract::Query<DeleteUploadQuery>,
) -> Result<StatusCode, AppError> {
    let session = crate::db::get_session(&state.db_pool, upload_id).await?;

    let should_delete_file = match query.delete_file {
        Some(val) => val,
        None => {
            // Standard: Datei nur loeschen, wenn der Upload noch nicht abgeschlossen war
            match session.as_ref() {
                Some(s) => s.status != "completed",
                None => false,
            }
        }
    };

    // 1. Datei auf Disk loeschen, falls gefordert
    if should_delete_file {
        let storage_path = crate::storage::get_storage_path(&state.config.storage_dir, upload_id);
        crate::storage::delete_file(&storage_path).await?;
    }

    // 2. Aktiven Hasher aus dem RAM entfernen
    {
        let mut active = state.active_sessions.lock().await;
        active.remove(&upload_id);
    }

    // 3. Eintrag aus upload_sessions loeschen
    crate::db::delete_session(&state.db_pool, upload_id).await?;

    tracing::info!(
        upload_id = %upload_id,
        file_deleted = should_delete_file,
        "upload-session erfolgreich bereinigt"
    );

    Ok(StatusCode::NO_CONTENT)
}
