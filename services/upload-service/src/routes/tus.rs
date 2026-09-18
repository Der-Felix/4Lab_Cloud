use axum::{
    body::Bytes,
    extract::{Path, State},
    http::{header, HeaderMap, HeaderValue, StatusCode},
    response::{IntoResponse, Response},
};
use base64::prelude::*;
use sha2::Digest;
use std::{sync::Arc, time::Instant};
use uuid::Uuid;

use crate::{
    db,
    error::AppError,
    storage,
    AppState,
    SessionHasher,
};

pub const TUS_RESUMABLE: &str = "1.0.0";
pub const TUS_VERSION: &str = "1.0.0";
pub const TUS_EXTENSION: &str = "creation,termination";

/// Erzeugt die Standard-Header fuer jede Tus-Antwort.
fn tus_headers() -> HeaderMap {
    let mut headers = HeaderMap::new();
    headers.insert("Tus-Resumable", HeaderValue::from_static(TUS_RESUMABLE));
    headers
}

/// Validiert, dass eine Upload-Session aktiv und nicht abgelaufen ist.
/// Liefert 410 Gone bei Ablauf oder 403 Forbidden bei falschem Status.
fn check_session_active(session: &crate::db::UploadSession) -> Result<(), StatusCode> {
    if session.expires_at <= chrono::Utc::now() {
        return Err(StatusCode::GONE);
    }
    if session.status != "uploading" {
        return Err(StatusCode::FORBIDDEN);
    }
    Ok(())
}

/// OPTIONS /api/v1/uploads/tus: Gibt Faehigkeiten des Tus-Servers zurueck.
pub async fn tus_options(State(state): State<Arc<AppState>>) -> Response {
    let mut headers = tus_headers();
    headers.insert("Tus-Version", HeaderValue::from_static(TUS_VERSION));
    headers.insert("Tus-Extension", HeaderValue::from_static(TUS_EXTENSION));
    headers.insert(
        "Tus-Max-Size",
        HeaderValue::from_str(&state.config.max_upload_size.to_string())
            .unwrap_or_else(|_| HeaderValue::from_static("5368709120")),
    );

    (StatusCode::NO_CONTENT, headers).into_response()
}

/// POST /api/v1/uploads/tus: Erstellt eine neue Upload-Ressource.
pub async fn tus_create(
    State(state): State<Arc<AppState>>,
    headers: HeaderMap,
) -> Result<Response, AppError> {
    let upload_length = headers
        .get("Upload-Length")
        .and_then(|v| v.to_str().ok())
        .and_then(|s| s.parse::<i64>().ok())
        .ok_or_else(|| AppError::BadRequest("Upload-Length Header fehlt oder ist ungueltig".to_string()))?;

    if upload_length <= 0 {
        return Err(AppError::BadRequest("Upload-Length muss groesser als 0 sein".to_string()));
    }

    if upload_length > state.config.max_upload_size {
        return Err(AppError::BadRequest(format!(
            "Upload-Length ueberschreitet maximale Dateigroesse von {} Bytes",
            state.config.max_upload_size
        )));
    }

    // Optional Dateiname aus Upload-Metadata extrahieren
    let filename = extract_filename_from_metadata(&headers).unwrap_or_else(|| "unnamed_file".to_string());
    let user_id = extract_user_id_from_metadata(&headers).unwrap_or_else(Uuid::nil);

    let upload_id = Uuid::new_v4();

    // Session in der Datenbank anlegen
    db::create_session(&state.db_pool, upload_id, user_id, &filename, upload_length).await?;

    // In-Memory Session initialisieren
    {
        let mut active = state.active_sessions.lock().await;
        active.insert(upload_id, SessionHasher {
            hasher: sha2::Sha256::new(),
            last_patch: Instant::now() - std::time::Duration::from_secs(2),
        });
    }

    let mut response_headers = tus_headers();
    let location = format!("/api/v1/uploads/tus/{upload_id}");
    response_headers.insert(
        header::LOCATION,
        HeaderValue::from_str(&location)
            .map_err(|e| AppError::Internal(format!("Ungueltiger Location-Header: {e}")))?,
    );
    response_headers.insert(
        "Upload-Length",
        HeaderValue::from_str(&upload_length.to_string())
            .map_err(|e| AppError::Internal(format!("Ungueltiger Upload-Length-Header: {e}")))?,
    );

    Ok((StatusCode::CREATED, response_headers).into_response())
}

/// HEAD /api/v1/uploads/tus/{id}: Gibt den aktuellen Offset einer Upload-Ressource zurueck.
pub async fn tus_head(
    State(state): State<Arc<AppState>>,
    Path(upload_id): Path<Uuid>,
) -> Result<Response, AppError> {
    let session = db::get_session(&state.db_pool, upload_id)
        .await?
        .ok_or_else(|| AppError::NotFound("Upload-Session nicht gefunden".to_string()))?;

    if let Err(status) = check_session_active(&session) {
        let headers = tus_headers();
        return Ok((status, headers).into_response());
    }

    let mut headers = tus_headers();
    headers.insert(
        "Upload-Offset",
        HeaderValue::from_str(&session.upload_offset.to_string())
            .map_err(|e| AppError::Internal(format!("Ungueltiger Upload-Offset: {e}")))?,
    );
    headers.insert(
        "Upload-Length",
        HeaderValue::from_str(&session.size_bytes.to_string())
            .map_err(|e| AppError::Internal(format!("Ungueltiges Upload-Length: {e}")))?,
    );
    headers.insert(header::CACHE_CONTROL, HeaderValue::from_static("no-store"));

    Ok((StatusCode::OK, headers).into_response())
}

/// PATCH /api/v1/uploads/tus/{id}: Haengt Datenbytes an den aktuellen Offset an.
pub async fn tus_patch(
    State(state): State<Arc<AppState>>,
    Path(upload_id): Path<Uuid>,
    headers: HeaderMap,
    body: Bytes,
) -> Result<Response, AppError> {
    let client_offset = headers
        .get("Upload-Offset")
        .and_then(|v| v.to_str().ok())
        .and_then(|s| s.parse::<i64>().ok())
        .ok_or_else(|| AppError::BadRequest("Upload-Offset Header fehlt oder ist ungueltig".to_string()))?;

    let session = db::get_session(&state.db_pool, upload_id)
        .await?
        .ok_or_else(|| AppError::NotFound("Upload-Session nicht gefunden".to_string()))?;

    if let Err(status) = check_session_active(&session) {
        let headers = tus_headers();
        return Ok((status, headers).into_response());
    }

    // Pruefung auf Offset-Konflikt -> 409 Conflict
    if client_offset != session.upload_offset {
        let mut conflict_headers = tus_headers();
        conflict_headers.insert(
            "Upload-Offset",
            HeaderValue::from_str(&session.upload_offset.to_string())
                .map_err(|e| AppError::Internal(format!("Ungueltiger Upload-Offset: {e}")))?,
        );
        return Ok((StatusCode::CONFLICT, conflict_headers).into_response());
    }

    // In-Memory Rate-Limiting und Hasher-State pruefen
    let now = Instant::now();
    let mut active_sessions = state.active_sessions.lock().await;

    if let Some(entry) = active_sessions.get(&upload_id) {
        let elapsed = now.duration_since(entry.last_patch);
        if elapsed < std::time::Duration::from_millis(1000) {
            let mut rate_headers = tus_headers();
            rate_headers.insert("Retry-After", HeaderValue::from_static("1"));
            return Ok((StatusCode::TOO_MANY_REQUESTS, rate_headers).into_response());
        }
    }

    let chunk_len = body.len() as i64;
    if chunk_len == 0 {
        let mut response_headers = tus_headers();
        response_headers.insert(
            "Upload-Offset",
            HeaderValue::from_str(&session.upload_offset.to_string())
                .map_err(|e| AppError::Internal(format!("Ungueltiger Upload-Offset: {e}")))?,
        );
        return Ok((StatusCode::NO_CONTENT, response_headers).into_response());
    }

    if session.upload_offset + chunk_len > session.size_bytes {
        return Err(AppError::BadRequest("Upload ueberschreitet die angekuendigte Dateigroesse".to_string()));
    }

    // Hasher bereitstellen (oder bei Neustart aus existierenden Klartext-Chunks rekonstruieren)
    let entry = match active_sessions.get_mut(&upload_id) {
        Some(entry) => {
            entry.last_patch = now;
            entry
        }
        None => {
            let mut hasher = sha2::Sha256::new();
            if session.upload_offset > 0 {
                let storage_path = storage::get_storage_path(&state.config.storage_dir, upload_id);
                let prev_plain = storage::read_and_decrypt_all(&storage_path, &state.config.storage_key).await?;
                hasher.update(&prev_plain);
            }
            active_sessions.insert(upload_id, SessionHasher {
                hasher,
                last_patch: now,
            });
            active_sessions.get_mut(&upload_id).unwrap()
        }
    };

    // Inkrementeller Hash ueber den empfangenen Klartext-Chunk
    entry.hasher.update(&body);

    // Verschluesselung und Speicherung des Chunks auf der Festplatte
    let storage_path = storage::get_storage_path(&state.config.storage_dir, upload_id);
    storage::append_chunk(&storage_path, &state.config.storage_key, &body).await?;

    let new_offset = session.upload_offset + chunk_len;

    // Finalisierung wenn vollstaendig uebertragen, ansonsten Offset aktualisieren
    if new_offset == session.size_bytes {
        let hash_result = entry.hasher.clone().finalize();
        let mut checksum = String::with_capacity(64);
        for byte in hash_result {
            use std::fmt::Write;
            let _ = write!(checksum, "{:02x}", byte);
        }

        db::complete_session(&state.db_pool, upload_id, &checksum, new_offset).await?;
        tracing::info!(
            upload_id = %upload_id,
            checksum = %checksum,
            offset = new_offset,
            "upload-session erfolgreich vollstaendig uebertragen und finalisiert"
        );
    } else {
        db::update_offset(&state.db_pool, upload_id, new_offset).await?;
    }

    let mut response_headers = tus_headers();
    response_headers.insert(
        "Upload-Offset",
        HeaderValue::from_str(&new_offset.to_string())
            .map_err(|e| AppError::Internal(format!("Ungueltiger Upload-Offset: {e}")))?,
    );

    Ok((StatusCode::NO_CONTENT, response_headers).into_response())
}

/// DELETE /api/v1/uploads/tus/{id}: Bricht einen Upload ab und raeumt Ressourcen auf.
pub async fn tus_delete(
    State(state): State<Arc<AppState>>,
    Path(upload_id): Path<Uuid>,
) -> Result<Response, AppError> {
    let storage_path = storage::get_storage_path(&state.config.storage_dir, upload_id);
    let _ = storage::delete_file(&storage_path).await;

    // Aus aktiven Sitzungen entfernen
    {
        let mut active = state.active_sessions.lock().await;
        active.remove(&upload_id);
    }

    let deleted = db::delete_session(&state.db_pool, upload_id).await?;
    if !deleted {
        return Err(AppError::NotFound("Upload-Session nicht gefunden".to_string()));
    }

    let response_headers = tus_headers();
    Ok((StatusCode::NO_CONTENT, response_headers).into_response())
}

/// Hilfsfunktion: Extrahiert Dateinamen aus dem Tus Upload-Metadata Header.
fn extract_filename_from_metadata(headers: &HeaderMap) -> Option<String> {
    let metadata_header = headers.get("Upload-Metadata")?.to_str().ok()?;
    for pair in metadata_header.split(',') {
        let mut parts = pair.trim().splitn(2, ' ');
        if let (Some(key), Some(encoded_val)) = (parts.next(), parts.next()) {
            if key == "filename" {
                if let Ok(decoded) = base64::prelude::BASE64_STANDARD.decode(encoded_val.trim()) {
                    if let Ok(name) = String::from_utf8(decoded) {
                        return Some(name);
                    }
                }
            }
        }
    }
    None
}

/// Hilfsfunktion: Extrahiert User-ID aus dem Tus Upload-Metadata Header.
fn extract_user_id_from_metadata(headers: &HeaderMap) -> Option<Uuid> {
    let metadata_header = headers.get("Upload-Metadata")?.to_str().ok()?;
    for pair in metadata_header.split(',') {
        let mut parts = pair.trim().splitn(2, ' ');
        if let (Some(key), Some(encoded_val)) = (parts.next(), parts.next()) {
            if key == "user_id" {
                if let Ok(decoded) = base64::prelude::BASE64_STANDARD.decode(encoded_val.trim()) {
                    if let Ok(id_str) = String::from_utf8(decoded) {
                        if let Ok(id) = Uuid::parse_str(&id_str) {
                            return Some(id);
                        }
                    }
                }
            }
        }
    }
    None
}
