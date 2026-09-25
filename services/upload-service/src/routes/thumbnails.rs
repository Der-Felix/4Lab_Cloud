use axum::{
    body::Body,
    extract::{Path, State},
    http::{header, HeaderMap, HeaderValue, StatusCode},
    response::{IntoResponse, Response},
    Json,
};
use chrono::Utc;
use serde::{Deserialize, Serialize};
use sqlx::Row;
use std::{path::PathBuf, sync::Arc};
use uuid::Uuid;

use crate::{error::AppError, storage, AppState};

#[derive(Debug, Deserialize)]
pub struct GenerateThumbnailRequest {
    pub storage_path: String,
    pub extract_exif: Option<bool>,
    pub store_gps: Option<bool>,
    pub user_id: Option<Uuid>,
}

#[derive(Debug, Serialize)]
pub struct GenerateThumbnailResponse {
    pub thumbnail_path: String,
    pub width: u32,
    pub height: u32,
    pub taken_at: Option<chrono::DateTime<Utc>>,
    pub exif_json: Option<serde_json::Value>,
    pub gps_lat: Option<f64>,
    pub gps_lon: Option<f64>,
    pub location_name: Option<String>,
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

/// POST /internal/thumbs/{id}: Generiert ein Thumbnail (256x256 max, JPEG Quality 80),
/// extrahiert vollstaendige EXIF-Daten und startet optional asynchrones Reverse-Geocoding.
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

    // EXIF-Metadaten verarbeiten
    let mut taken_at: Option<chrono::DateTime<Utc>> = None;
    let mut exif_json: Option<serde_json::Value> = None;
    let mut gps_lat: Option<f64> = None;
    let mut gps_lon: Option<f64> = None;

    if payload.extract_exif == Some(true) {
        if let Some(mut exif_data) = crate::exif::extract_exif(&plain_bytes) {
            // Datenschutz: Pruefen, ob der Nutzer GPS-Speicherung erlaubt hat
            let store_gps_allowed = match payload.store_gps {
                Some(allowed) => allowed,
                None => {
                    // Falls nicht im Payload uebergeben, aus der Datenbank pruefen.
                    // DSGVO: Fail-closed - falls die Praeferenz nicht ermittelt werden
                    // kann (Fehler oder keine Zeile in beiden Lookups), wird GPS entfernt.
                    let mut allowed = false;
                    let mut determined = false;

                    let row = sqlx::query(
                        "SELECT u.store_gps FROM users u \
                         JOIN upload_sessions s ON s.user_id = u.id \
                         WHERE s.id = $1 LIMIT 1",
                    )
                    .bind(id)
                    .fetch_optional(&state.db_pool)
                    .await;

                    if let Ok(Some(r)) = row {
                        allowed = r.get("store_gps");
                        determined = true;
                    } else {
                        let row2 = sqlx::query(
                            "SELECT u.store_gps FROM users u \
                             JOIN files f ON f.user_id = u.id \
                             WHERE f.id = $1 OR f.upload_id = $1 LIMIT 1",
                        )
                        .bind(id)
                        .fetch_optional(&state.db_pool)
                        .await;
                        if let Ok(Some(r2)) = row2 {
                            allowed = r2.get("store_gps");
                            determined = true;
                        }
                    }

                    if !determined {
                        tracing::warn!(
                            file_id = %id,
                            "store_gps-Praeferenz konnte nicht ermittelt werden (Fehler oder keine Zeile in beiden Lookups); GPS wird sicherheitshalber entfernt"
                        );
                    }

                    allowed
                }
            };

            if !store_gps_allowed {
                crate::exif::strip_gps(&mut exif_data);
                tracing::info!(file_id = %id, "GPS-Speicherung durch Benutzer deaktiviert (store_gps = false)");
            } else if let Some(ref gps) = exif_data.gps {
                gps_lat = Some(gps.lat);
                gps_lon = Some(gps.lon);
                tracing::info!(file_id = %id, lat = gps.lat, lon = gps.lon, "exif extrahiert");

                // Asynchrones Reverse-Geocoding in tokio::spawn starten (blockiert Upload NICHT)
                if state.geocoding_client.is_enabled() {
                    let geocoding = state.geocoding_client.clone();
                    let pool = state.db_pool.clone();
                    let target_id = id;
                    let lat = gps.lat;
                    let lon = gps.lon;

                    tokio::spawn(async move {
                        tracing::info!(
                            upload_id = %target_id,
                            lat = lat,
                            lon = lon,
                            "geocoding gestartet (async)"
                        );
                        match geocoding.reverse_geocode(lat, lon).await {
                            Ok(Some(geo)) => {
                                // Wiederhole Update kurz, falls der Go-Server die files-Zeile zeitgleich einfuegt
                                for _ in 0..10 {
                                    let res = sqlx::query(
                                        "UPDATE files SET location_name = $1, location_address = $2 \
                                         WHERE upload_id = $3 OR id = $3",
                                    )
                                    .bind(&geo.display_name)
                                    .bind(&geo.address)
                                    .bind(target_id)
                                    .execute(&pool)
                                    .await;

                                    if let Ok(r) = res {
                                        if r.rows_affected() > 0 {
                                            tracing::info!(
                                                upload_id = %target_id,
                                                location = %geo.display_name,
                                                "geocoding abgeschlossen: {}",
                                                geo.display_name
                                            );
                                            break;
                                        }
                                    }
                                    tokio::time::sleep(std::time::Duration::from_millis(300)).await;
                                }
                            }
                            Ok(None) => {
                                tracing::debug!(upload_id = %target_id, "geocoding deaktiviert oder kein ergebnis");
                            }
                            Err(e) => {
                                tracing::warn!(
                                    upload_id = %target_id,
                                    error = %e,
                                    "geocoding fehlgeschlagen (upload unbeeinflusst)"
                                );
                            }
                        }
                    });
                }
            }

            // Aufnahmedatum extrahieren
            if let Some(ref dt_str) = exif_data.datetime_original {
                if let Ok(parsed) = chrono::DateTime::parse_from_rfc3339(dt_str) {
                    taken_at = Some(parsed.with_timezone(&Utc));
                }
            }

            exif_json = serde_json::to_value(&exif_data).ok();
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
        gps_lat,
        gps_lon,
        location_name: None,
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
