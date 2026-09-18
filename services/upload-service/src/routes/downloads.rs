use axum::{
    body::Body,
    extract::{Path, Query, State},
    http::{header, HeaderMap, HeaderValue, StatusCode},
    response::{IntoResponse, Response},
    Json,
};
use base64::prelude::*;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::sync::Arc;
use subtle::ConstantTimeEq;
use uuid::Uuid;

use crate::{error::AppError, storage, AppState};

#[derive(Debug, Deserialize)]
pub struct DownloadTokenRequest {
    pub file_id: Option<Uuid>,
    pub user_id: Option<Uuid>,
    pub storage_path: String,
    pub filename: String,
}

#[derive(Debug, Serialize)]
pub struct DownloadTokenResponse {
    pub download_url: String,
    pub token: String,
    pub expires_at: i64,
}

#[derive(Debug, Deserialize)]
pub struct DownloadQueryParams {
    pub token: String,
}

/// Erzeugt einen HMAC-SHA256 Hash gemaess RFC 2104.
fn hmac_sha256(key: &[u8; 32], data: &[u8]) -> [u8; 32] {
    let mut k_ipad = [0x36u8; 64];
    let mut k_opad = [0x5cu8; 64];

    for i in 0..32 {
        k_ipad[i] ^= key[i];
        k_opad[i] ^= key[i];
    }

    let mut inner = Sha256::new();
    inner.update(k_ipad);
    inner.update(data);
    let inner_hash = inner.finalize();

    let mut outer = Sha256::new();
    outer.update(k_opad);
    outer.update(inner_hash);
    let mut result = [0u8; 32];
    result.copy_from_slice(&outer.finalize());
    result
}

/// Signiert einen Download-Token mit 5 Minuten Gueltigkeit inkl. file_id und user_id.
pub fn generate_download_token(
    key: &[u8; 32],
    file_id: Uuid,
    user_id: Option<Uuid>,
    storage_path: &str,
    filename: &str,
) -> (String, i64) {
    let expires_at = chrono::Utc::now().timestamp() + 300; // 5 Minuten TTL
    let user_str = user_id.map(|u| u.to_string()).unwrap_or_default();
    let user_b64 = BASE64_URL_SAFE_NO_PAD.encode(user_str);
    let path_b64 = BASE64_URL_SAFE_NO_PAD.encode(storage_path);
    let name_b64 = BASE64_URL_SAFE_NO_PAD.encode(filename);

    let payload = format!("{file_id}.{user_b64}.{path_b64}.{name_b64}.{expires_at}");
    let hmac = hmac_sha256(key, payload.as_bytes());
    let sig_hex = hex::encode(hmac);

    (format!("{payload}.{sig_hex}"), expires_at)
}

/// Verifiziert den Download-Token und liefert Pfad und Dateinamen zurueck.
pub fn verify_download_token(
    key: &[u8; 32],
    expected_file_id: Uuid,
    token: &str,
) -> Result<(String, String), AppError> {
    let parts: Vec<&str> = token.split('.').collect();

    // Neuer Standard: 6 Teile (file_id, user_id_b64, path_b64, name_b64, expires_at, sig_hex)
    if parts.len() == 6 {
        let (file_id_str, user_b64, path_b64, name_b64, expires_at_str, sig_hex) =
            (parts[0], parts[1], parts[2], parts[3], parts[4], parts[5]);

        let token_file_id = Uuid::parse_str(file_id_str)
            .map_err(|_| AppError::BadRequest("Ungueltige Datei-ID im Token".to_string()))?;

        if token_file_id != expected_file_id {
            return Err(AppError::Forbidden("Token gehoert nicht zu dieser Datei".to_string()));
        }

        let expires_at = expires_at_str
            .parse::<i64>()
            .map_err(|_| AppError::BadRequest("Ungueltiger Token-Zeitstempel".to_string()))?;

        if chrono::Utc::now().timestamp() > expires_at {
            return Err(AppError::Forbidden("Download-Token ist abgelaufen".to_string()));
        }

        let payload = format!("{file_id_str}.{user_b64}.{path_b64}.{name_b64}.{expires_at_str}");
        let expected_hmac = hmac_sha256(key, payload.as_bytes());
        let expected_sig = hex::encode(expected_hmac);

        if sig_hex.as_bytes().ct_eq(expected_sig.as_bytes()).unwrap_u8() != 1 {
            return Err(AppError::Forbidden("Ungueltige Token-Signatur".to_string()));
        }

        let path_bytes = BASE64_URL_SAFE_NO_PAD
            .decode(path_b64)
            .map_err(|_| AppError::BadRequest("Ungueltiger Pfad im Token".to_string()))?;
        let name_bytes = BASE64_URL_SAFE_NO_PAD
            .decode(name_b64)
            .map_err(|_| AppError::BadRequest("Ungueltiger Dateiname im Token".to_string()))?;

        let storage_path = String::from_utf8(path_bytes)
            .map_err(|_| AppError::BadRequest("Pfad ist kein gueltiges UTF-8".to_string()))?;
        let filename = String::from_utf8(name_bytes)
            .map_err(|_| AppError::BadRequest("Dateiname ist kein gueltiges UTF-8".to_string()))?;

        if storage_path.contains("..") || storage_path.starts_with('/') {
            return Err(AppError::BadRequest("Ungueltiger Speicherpfad".to_string()));
        }

        return Ok((storage_path, filename));
    }

    // Abwaertskompatibilitaet: 4 Teile (path_b64, name_b64, expires_at, sig_hex)
    if parts.len() == 4 {
        let (path_b64, name_b64, expires_at_str, sig_hex) = (parts[0], parts[1], parts[2], parts[3]);

        let expires_at = expires_at_str
            .parse::<i64>()
            .map_err(|_| AppError::BadRequest("Ungueltiger Token-Zeitstempel".to_string()))?;

        if chrono::Utc::now().timestamp() > expires_at {
            return Err(AppError::Forbidden("Download-Token ist abgelaufen".to_string()));
        }

        let payload = format!("{path_b64}.{name_b64}.{expires_at_str}");
        let expected_hmac = hmac_sha256(key, payload.as_bytes());
        let expected_sig = hex::encode(expected_hmac);

        if sig_hex.as_bytes().ct_eq(expected_sig.as_bytes()).unwrap_u8() != 1 {
            return Err(AppError::Forbidden("Ungueltige Token-Signatur".to_string()));
        }

        let path_bytes = BASE64_URL_SAFE_NO_PAD
            .decode(path_b64)
            .map_err(|_| AppError::BadRequest("Ungueltiger Pfad im Token".to_string()))?;
        let name_bytes = BASE64_URL_SAFE_NO_PAD
            .decode(name_b64)
            .map_err(|_| AppError::BadRequest("Ungueltiger Dateiname im Token".to_string()))?;

        let storage_path = String::from_utf8(path_bytes)
            .map_err(|_| AppError::BadRequest("Pfad ist kein gueltiges UTF-8".to_string()))?;
        let filename = String::from_utf8(name_bytes)
            .map_err(|_| AppError::BadRequest("Dateiname ist kein gueltiges UTF-8".to_string()))?;

        if storage_path.contains("..") || storage_path.starts_with('/') {
            return Err(AppError::BadRequest("Ungueltiger Speicherpfad".to_string()));
        }

        return Ok((storage_path, filename));
    }

    Err(AppError::BadRequest("Ungueltiges Token-Format".to_string()))
}

/// GET /internal/files/{id}/download: Erstellt ein signiertes Download-Token (nur fuer Go-App erreichbar).
pub async fn create_download_url(
    State(state): State<Arc<AppState>>,
    Path(id): Path<Uuid>,
    Query(params): Query<DownloadTokenRequest>,
) -> Result<Json<DownloadTokenResponse>, AppError> {
    let (token, expires_at) = generate_download_token(
        &state.config.storage_key,
        id,
        params.user_id,
        &params.storage_path,
        &params.filename,
    );

    let download_url = format!("/api/v1/uploads/tus/{id}/download?token={token}");

    Ok(Json(DownloadTokenResponse {
        download_url,
        token,
        expires_at,
    }))
}

/// Dispatcher fuer GET /internal/files/{*path}
pub async fn internal_files_dispatcher(
    State(state): State<Arc<AppState>>,
    Path(path): Path<String>,
    Query(params): Query<DownloadTokenRequest>,
) -> Result<Json<DownloadTokenResponse>, AppError> {
    let parts: Vec<&str> = path.split('/').collect();
    if parts.len() == 2 && parts[1] == "download" {
        if let Ok(id) = Uuid::parse_str(parts[0]) {
            return create_download_url(State(state), Path(id), Query(params)).await;
        }
    }
    Err(AppError::NotFound("Endpunkt nicht gefunden".to_string()))
}

/// GET /api/v1/uploads/tus/{id}/download: Entschluesselt on-the-fly und streamt Chunks ohne Speicheroverhead.
pub async fn download_file(
    State(state): State<Arc<AppState>>,
    Path(id): Path<Uuid>,
    Query(params): Query<DownloadQueryParams>,
) -> Result<Response, AppError> {
    let (storage_path, filename) =
        verify_download_token(&state.config.storage_key, id, &params.token)?;

    let full_path = state.config.storage_dir.join(&storage_path);
    if !full_path.exists() {
        return Err(AppError::NotFound("Datei auf dem Storage nicht gefunden".to_string()));
    }

    // Chunk-weises Streaming via async ReaderStream (keine Pufferung im RAM)
    let stream = storage::stream_decrypt_file(full_path, state.config.storage_key);

    let mut headers = HeaderMap::new();
    headers.insert(
        header::CONTENT_TYPE,
        HeaderValue::from_static("application/octet-stream"),
    );

    // Dateiname fuer Content-Disposition absichern
    let sanitized_name = filename.replace('"', "\\\"");
    let disposition = format!("attachment; filename=\"{sanitized_name}\"");
    if let Ok(val) = HeaderValue::from_str(&disposition) {
        headers.insert(header::CONTENT_DISPOSITION, val);
    }

    Ok((StatusCode::OK, headers, Body::from_stream(stream)).into_response())
}
