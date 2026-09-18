use axum::{
    body::Body,
    extract::{Path, State},
    http::{header, HeaderMap, HeaderValue, StatusCode},
    response::{IntoResponse, Response},
    Json,
};
use chrono::Utc;
use serde::{Deserialize, Serialize};
use std::{io::Cursor, path::PathBuf, sync::Arc};
use uuid::Uuid;

use crate::{error::AppError, storage, AppState};

#[derive(Debug, Deserialize)]
pub struct GenerateThumbnailRequest {
    pub storage_path: String,
    pub extract_exif: Option<bool>,
}

#[derive(Debug, Serialize)]
pub struct GenerateThumbnailResponse {
    pub thumbnail_path: String,
    pub width: u32,
    pub height: u32,
    pub taken_at: Option<chrono::DateTime<Utc>>,
    pub exif_json: Option<serde_json::Value>,
}

/// Ermittelt den relativen Speicherpfad fuer Thumbnails mit Sharding.
pub fn get_relative_thumbnail_path(file_id: Uuid) -> String {
    let id_str = file_id.to_string();
    let shard = &id_str[..2];
    format!("{shard}/{id_str}.enc")
}

/// Ermittelt den absoluten Speicherpfad fuer Thumbnails.
pub fn get_thumbnail_path(base_dir: &std::path::Path, file_id: Uuid) -> PathBuf {
    let id_str = file_id.to_string();
    let shard = &id_str[..2];
    base_dir.join(shard).join(format!("{id_str}.enc"))
}

/// POST /internal/thumbs/{id}: Generiert ein Thumbnail (256x256 max, JPEG Quality 80)
/// und speichert es verschluesselt ab. EXIF GPS-Daten werden IMMER gestrippt.
pub async fn generate_thumbnail(
    State(state): State<Arc<AppState>>,
    Path(id): Path<Uuid>,
    Json(payload): Json<GenerateThumbnailRequest>,
) -> Result<Json<GenerateThumbnailResponse>, AppError> {
    let full_orig_path = state.config.storage_dir.join(&payload.storage_path);
    if !full_orig_path.exists() {
        return Err(AppError::NotFound("Originaldatei nicht gefunden".to_string()));
    }

    // Originaldatei entschluesseln
    let plain_bytes = storage::read_and_decrypt_all(&full_orig_path, &state.config.storage_key).await?;
    if plain_bytes.is_empty() {
        return Err(AppError::BadRequest("Datei ist leer".to_string()));
    }

    // Bild dekodieren
    let img = image::load_from_memory(&plain_bytes)
        .map_err(|e| AppError::BadRequest(format!("Bild konnte nicht geladen werden: {e}")))?;

    let orig_width = img.width();
    let orig_height = img.height();

    // 256x256 max, Aspect-Ratio erhalten
    let thumb = img.thumbnail(256, 256);

    // JPEG Qualitaet 80 encodieren
    let mut jpeg_bytes = Vec::new();
    let mut encoder = image::codecs::jpeg::JpegEncoder::new_with_quality(&mut jpeg_bytes, 80);
    encoder
        .encode_image(&thumb)
        .map_err(|e| AppError::Internal(format!("Thumbnail-Kodierung fehlgeschlagen: {e}")))?;

    // EXIF-Metadaten verarbeiten gemaess DSGVO Art. 5 (Datensparsamkeit)
    let mut taken_at: Option<chrono::DateTime<Utc>> = None;
    let mut exif_json: Option<serde_json::Value> = None;

    if payload.extract_exif == Some(true) {
        let mut cursor = Cursor::new(&plain_bytes);
        if let Ok(exif) = exif::Reader::new().read_from_container(&mut cursor) {
            let mut tags_map = serde_json::Map::new();

            for field in exif.fields() {
                let tag_name = format!("{}", field.tag);

                // DSGVO Schutz: GPS-Daten IMMER strippen, auch bei aktiviertem EXIF
                if tag_name.contains("GPS") || tag_name.starts_with("Gps") || tag_name.contains("Serial") {
                    continue;
                }

                // Aufnahmedatum extrahieren
                if (field.tag == exif::Tag::DateTimeOriginal || field.tag == exif::Tag::DateTime)
                    && taken_at.is_none()
                {
                    let val_str = field.display_value().to_string();
                    // EXIF-Standardformat: "YYYY:MM:DD HH:MM:SS"
                    if let Ok(naive) =
                        chrono::NaiveDateTime::parse_from_str(val_str.trim_matches('"'), "%Y:%m:%d %H:%M:%S")
                    {
                        taken_at = Some(chrono::DateTime::<Utc>::from_naive_utc_and_offset(naive, Utc));
                    }
                }

                let val_str = field.display_value().to_string();
                let clean_val = val_str.trim_matches('"').to_string();
                tags_map.insert(tag_name, serde_json::Value::String(clean_val));
            }

            if !tags_map.is_empty() {
                exif_json = Some(serde_json::Value::Object(tags_map));
            }
        }
    }

    // Thumbnail verschluesselt ablegen: /data/thumbs/{shard}/{id}.enc
    let relative_path = get_relative_thumbnail_path(id);
    let full_thumb_path = state.config.thumbs_dir.join(&relative_path);

    // Falls Verzeichnis nicht existiert, erstellen und speichern
    storage::append_chunk(&full_thumb_path, &state.config.storage_key, &jpeg_bytes).await?;

    tracing::info!(
        file_id = %id,
        thumbnail_path = %relative_path,
        orig_w = orig_width,
        orig_h = orig_height,
        "thumbnail erfolgreich generiert und verschluesselt"
    );

    Ok(Json(GenerateThumbnailResponse {
        thumbnail_path: relative_path,
        width: orig_width,
        height: orig_height,
        taken_at,
        exif_json,
    }))
}

#[derive(Debug, Deserialize)]
pub struct GetThumbnailQuery {
    pub thumbnail_path: Option<String>,
}

/// GET /internal/thumbs/{id}: Entschluesselt das Thumbnail und liefert JPEG an den Go-Server.
pub async fn get_thumbnail(
    State(state): State<Arc<AppState>>,
    Path(id): Path<Uuid>,
    axum::extract::Query(query): axum::extract::Query<GetThumbnailQuery>,
) -> Result<Response, AppError> {
    let mut full_path = if let Some(ref p) = query.thumbnail_path {
        if !p.is_empty() && !p.contains("..") {
            state.config.thumbs_dir.join(p)
        } else {
            state.config.thumbs_dir.join(get_relative_thumbnail_path(id))
        }
    } else {
        state.config.thumbs_dir.join(get_relative_thumbnail_path(id))
    };

    if !full_path.exists() {
        let fallback = state.config.thumbs_dir.join(get_relative_thumbnail_path(id));
        if fallback.exists() {
            full_path = fallback;
        } else {
            return Err(AppError::NotFound("Thumbnail nicht gefunden".to_string()));
        }
    }

    let decrypted = storage::read_and_decrypt_all(&full_path, &state.config.storage_key).await?;

    let mut headers = HeaderMap::new();
    headers.insert(header::CONTENT_TYPE, HeaderValue::from_static("image/jpeg"));
    headers.insert(
        header::CACHE_CONTROL,
        HeaderValue::from_static("private, max-age=86400"),
    );
    headers.insert(
        header::CONTENT_LENGTH,
        HeaderValue::from_str(&decrypted.len().to_string())
            .unwrap_or_else(|_| HeaderValue::from_static("0")),
    );

    Ok((StatusCode::OK, headers, Body::from(decrypted)).into_response())
}
