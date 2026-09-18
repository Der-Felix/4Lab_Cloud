use axum::{
    extract::{Path, State},
    http::{header, HeaderMap, StatusCode},
    response::IntoResponse,
    Json,
};
use serde::{Deserialize, Serialize};
use std::{io::Write, path::PathBuf, sync::Arc};
use tokio::fs;
use uuid::Uuid;
use zip::{write::SimpleFileOptions, ZipWriter};

use crate::{crypto, error::AppError, AppState};

#[derive(Debug, Deserialize, Serialize)]
pub struct ExportFileItem {
    pub filename: String,
    pub storage_path: String,
}

#[derive(Debug, Deserialize)]
pub struct CreateExportRequest {
    pub user_id: Uuid,
    pub job_id: Uuid,
    pub password: Option<String>,
    pub key_salt: Option<String>,
    pub profile: Option<serde_json::Value>,
    pub shares: Option<serde_json::Value>,
    pub files: Option<Vec<ExportFileItem>>,
}

#[derive(Debug, Serialize)]
pub struct CreateExportResponse {
    pub path: String,
    pub size_bytes: u64,
}

/// Liest und entschluesselt eine gespeicherte Datei synchron (im Blocking-Thread).
fn read_and_decrypt_file_sync(path: &std::path::Path, key: &[u8; 32]) -> Result<Vec<u8>, AppError> {
    use std::io::Read;

    if !path.exists() {
        return Ok(Vec::new());
    }

    let mut file = std::fs::File::open(path)
        .map_err(|e| AppError::Internal(format!("Datei konnte nicht geoeffnet werden: {e}")))?;

    let mut plaintext_all = Vec::new();

    loop {
        let mut nonce = [0u8; crypto::NONCE_LEN];
        match file.read_exact(&mut nonce) {
            Ok(_) => {}
            Err(e) if e.kind() == std::io::ErrorKind::UnexpectedEof => break,
            Err(e) => return Err(AppError::Internal(format!("Fehler beim Lesen der Nonce: {e}"))),
        }

        let mut len_bytes = [0u8; 4];
        file.read_exact(&mut len_bytes)
            .map_err(|e| AppError::Internal(format!("Fehler beim Lesen der Chunk-Laenge: {e}")))?;
        let cipher_len = u32::from_be_bytes(len_bytes) as usize;

        let mut ciphertext = vec![0u8; cipher_len];
        file.read_exact(&mut ciphertext)
            .map_err(|e| AppError::Internal(format!("Fehler beim Lesen des Ciphertexts: {e}")))?;

        let mut combined = Vec::with_capacity(crypto::NONCE_LEN + cipher_len);
        combined.extend_from_slice(&nonce);
        combined.extend_from_slice(&ciphertext);

        let chunk_plain = crypto::decrypt(key, &combined)?;
        plaintext_all.extend_from_slice(&chunk_plain);
    }

    Ok(plaintext_all)
}

/// Erstellt eine passwortverschluesselte ZIP-Datei gemaess DSGVO Art. 20.
/// 1. Entschluesselt Chunks aus .enc
/// 2. Baut inneres Klartext-ZIP (profil, shares, files)
/// 3. Verschluesselt ZIP via AES-256-GCM mit Argon2id-Key (m=64MiB, t=3, p=4) -> .zip.age
/// 4. Shreddet temporaere Klartext-Dateien und Puffer
/// 5. Baut Download-Paket mit export.zip.age und HOW_TO_DECRYPT.txt
pub async fn create_export(
    State(state): State<Arc<AppState>>,
    Json(payload): Json<CreateExportRequest>,
) -> Result<(StatusCode, Json<CreateExportResponse>), AppError> {
    let user_export_dir = state.config.exports_dir.join(payload.user_id.to_string());
    fs::create_dir_all(&user_export_dir).await.map_err(|e| {
        AppError::Storage(format!("Export-Verzeichnis konnte nicht erstellt werden: {e}"))
    })?;

    let final_zip_path = user_export_dir.join(format!("{}.zip", payload.job_id));
    let temp_inner_path = user_export_dir.join(format!("temp_inner_{}.zip", payload.job_id));
    let age_path = user_export_dir.join(format!("{}.zip.age", payload.job_id));

    let storage_dir = state.config.storage_dir.clone();
    let storage_key = state.config.storage_key;
    let final_zip_clone = final_zip_path.clone();

    // Intensive CPU- und Disk-Operationen im Blocking-Pool ausfuehren
    let size_bytes = tokio::task::spawn_blocking(move || -> Result<u64, AppError> {
        // 1. Inneres Klartext-ZIP erstellen
        {
            let file = std::fs::File::create(&temp_inner_path).map_err(|e| {
                AppError::Storage(format!("Temporaere ZIP-Datei konnte nicht angelegt werden: {e}"))
            })?;
            let mut zip = ZipWriter::new(file);
            let options = SimpleFileOptions::default()
                .compression_method(zip::CompressionMethod::Deflated);

            // 1.1 profil.json
            if let Some(profile) = &payload.profile {
                zip.start_file("profil.json", options).map_err(|e| {
                    AppError::Storage(format!("Fehler beim Anlegen von profil.json im ZIP: {e}"))
                })?;
                let json_bytes = serde_json::to_vec_pretty(profile).map_err(|e| {
                    AppError::Storage(format!("Fehler beim Serialisieren von profil.json: {e}"))
                })?;
                zip.write_all(&json_bytes).map_err(|e| {
                    AppError::Storage(format!("Fehler beim Schreiben von profil.json: {e}"))
                })?;
            }

            // 1.2 shares.json
            if let Some(shares) = &payload.shares {
                zip.start_file("shares.json", options).map_err(|e| {
                    AppError::Storage(format!("Fehler beim Anlegen von shares.json im ZIP: {e}"))
                })?;
                let json_bytes = serde_json::to_vec_pretty(shares).map_err(|e| {
                    AppError::Storage(format!("Fehler beim Serialisieren von shares.json: {e}"))
                })?;
                zip.write_all(&json_bytes).map_err(|e| {
                    AppError::Storage(format!("Fehler beim Schreiben von shares.json: {e}"))
                })?;
            }

            // 1.3 files/ (Klartext-Dateien entschluesselt)
            if let Some(files) = &payload.files {
                let mut seen_names = std::collections::HashMap::<String, usize>::new();
                for item in files {
                    let file_path = storage_dir.join(&item.storage_path);
                    if file_path.exists() {
                        let count = seen_names.entry(item.filename.clone()).or_insert(0);
                        let zip_entry_name = if *count == 0 {
                            format!("files/{}", item.filename)
                        } else {
                            format!("files/{}_{}", item.filename, count)
                        };
                        *count += 1;

                        // Chunks aus .enc entschluesseln
                        let mut plain_bytes = read_and_decrypt_file_sync(&file_path, &storage_key)?;

                        zip.start_file(zip_entry_name, options).map_err(|e| {
                            AppError::Storage(format!("Fehler beim Anlegen von Datei im ZIP: {e}"))
                        })?;
                        zip.write_all(&plain_bytes).map_err(|e| {
                            AppError::Storage(format!("Fehler beim Schreiben in das ZIP: {e}"))
                        })?;

                        // Klartext-Puffer sofort im RAM shredden
                        crypto::shred_bytes(&mut plain_bytes);
                    }
                }
            }

            zip.finish().map_err(|e| {
                AppError::Storage(format!("Inneres ZIP konnte nicht abgeschlossen werden: {e}"))
            })?;
        }

        // 2. Inneres Klartext-ZIP lesen und verschluesseln
        let mut inner_zip_data = std::fs::read(&temp_inner_path).map_err(|e| {
            AppError::Storage(format!("Inneres ZIP konnte nicht gelesen werden: {e}"))
        })?;

        // 3. Temporaere Klartext-Datei auf Platte sicher shredden
        crypto::shred_file(&temp_inner_path)?;

        // 4. Verschluesselung vorbereiten (Argon2id + AES-256-GCM)
        let password = payload.password.unwrap_or_default();
        let salt_hex = payload.key_salt.unwrap_or_default();
        let salt_bytes = hex::decode(&salt_hex).unwrap_or_else(|_| salt_hex.as_bytes().to_vec());

        let derived_key = crypto::derive_argon2id_key(&password, &salt_bytes)?;
        let encrypted_age = crypto::encrypt(&derived_key, &inner_zip_data)?;

        // Klartext-Daten im RAM sofort nullen
        crypto::shred_bytes(&mut inner_zip_data);

        // .zip.age auf Disk speichern
        std::fs::write(&age_path, &encrypted_age).map_err(|e| {
            AppError::Storage(format!("Verschluesselte .zip.age konnte nicht geschrieben werden: {e}"))
        })?;

        // 5. Download-Paket schnueren: export-{job_id}.zip mit export.zip.age und HOW_TO_DECRYPT.txt
        {
            let download_file = std::fs::File::create(&final_zip_clone).map_err(|e| {
                AppError::Storage(format!("Download-ZIP konnte nicht erstellt werden: {e}"))
            })?;
            let mut outer_zip = ZipWriter::new(download_file);
            let options = SimpleFileOptions::default()
                .compression_method(zip::CompressionMethod::Stored); // age ist bereits verschluesselt

            // 5.1 export.zip.age hinzufuegen
            outer_zip.start_file("export.zip.age", options).map_err(|e| {
                AppError::Storage(format!("Fehler beim Hinzufuegen von export.zip.age: {e}"))
            })?;
            outer_zip.write_all(&encrypted_age).map_err(|e| {
                AppError::Storage(format!("Fehler beim Schreiben von export.zip.age: {e}"))
            })?;

            // 5.2 HOW_TO_DECRYPT.txt hinzufuegen
            let decrypt_doc = format!(
                "================================================================================\n\
                4labscloud - Sicherer Datenexport (DSGVO Art. 20)\n\
                ================================================================================\n\n\
                Ihr Datenexport wurde gemaess Art. 20 DSGVO und BSI-Sicherheitsstandards mit\n\
                Ihrem Passwort und AES-256-GCM verschluesselt.\n\n\
                DATEISTRUKTUR:\n\
                - export.zip.age      : Das verschluesselte ZIP-Archiv mit Ihren persoenlichen Daten.\n\
                - HOW_TO_DECRYPT.txt  : Diese Anleitung.\n\n\
                KRYPTOGRAFISCHE PARAMETER:\n\
                - Chiffre             : AES-256-GCM\n\
                - Key Derivation      : Argon2id\n\
                - Memory              : 64 MiB (65536 KiB)\n\
                - Iterationen         : 3\n\
                - Parallelismus       : 4\n\
                - Schluessellaenge    : 32 Bytes (256 Bit)\n\
                - Salt (hex)          : {salt_hex}\n\
                - Nonce-Laenge        : 12 Bytes (am Anfang von export.zip.age)\n\n\
                ENTSCHLUESSELUNG VIA PYTHON 3:\n\
                Fuehren Sie folgenden Befehl oder folgendes Skript im selben Verzeichnis aus:\n\n\
                pip install argon2-cffi cryptography\n\n\
                --------------------------------------------------------------------------------\n\
                from cryptography.hazmat.primitives.ciphers.aead import AESGCM\n\
                from argon2.low_level import hash_secret_raw, Type\n\n\
                password = input(\"Ihr 4labscloud-Passwort: \").encode()\n\
                salt = bytes.fromhex(\"{salt_hex}\")\n\n\
                key = hash_secret_raw(\n\
                    secret=password,\n\
                    salt=salt,\n\
                    time_cost=3,\n\
                    memory_cost=64 * 1024,\n\
                    parallelism=4,\n\
                    hash_len=32,\n\
                    type=Type.ID\n\
                )\n\n\
                with open(\"export.zip.age\", \"rb\") as f:\n\
                    data = f.read()\n\n\
                nonce = data[:12]\n\
                ciphertext = data[12:]\n\n\
                aesgcm = AESGCM(key)\n\
                plaintext = aesgcm.decrypt(nonce, ciphertext, None)\n\n\
                with open(\"export_decrypted.zip\", \"wb\") as f:\n\
                    f.write(plaintext)\n\n\
                print(\"Erfolgreich entschluesselt nach export_decrypted.zip!\")\n\
                --------------------------------------------------------------------------------\n"
            );

            outer_zip.start_file("HOW_TO_DECRYPT.txt", SimpleFileOptions::default().compression_method(zip::CompressionMethod::Deflated)).map_err(|e| {
                AppError::Storage(format!("Fehler beim Hinzufuegen von HOW_TO_DECRYPT.txt: {e}"))
            })?;
            outer_zip.write_all(decrypt_doc.as_bytes()).map_err(|e| {
                AppError::Storage(format!("Fehler beim Schreiben von HOW_TO_DECRYPT.txt: {e}"))
            })?;

            outer_zip.finish().map_err(|e| {
                AppError::Storage(format!("Download-ZIP konnte nicht abgeschlossen werden: {e}"))
            })?;
        }

        let metadata = std::fs::metadata(&final_zip_clone).map_err(|e| {
            AppError::Storage(format!("Metadaten fuer ZIP konnten nicht ermittelt werden: {e}"))
        })?;

        Ok(metadata.len())
    })
    .await
    .map_err(|e| AppError::Storage(format!("Join-Fehler im ZIP-Worker: {e}")))??;

    Ok((
        StatusCode::OK,
        Json(CreateExportResponse {
            path: final_zip_path.to_string_lossy().to_string(),
            size_bytes,
        }),
    ))
}

/// Loescht eine erstellte Export-ZIP-Datei und ihre verschluesselten Artefakte von der Festplatte.
pub async fn delete_export(
    State(state): State<Arc<AppState>>,
    Path(job_id): Path<Uuid>,
) -> Result<StatusCode, AppError> {
    let mut entries = fs::read_dir(&state.config.exports_dir).await.map_err(|e| {
        AppError::Storage(format!("Exports-Verzeichnis konnte nicht gelesen werden: {e}"))
    })?;

    let filename_zip = format!("{job_id}.zip");
    let filename_age = format!("{job_id}.zip.age");
    let filename_temp = format!("temp_inner_{job_id}.zip");

    while let Some(entry) = entries.next_entry().await.map_err(|e| {
        AppError::Storage(format!("Verzeichniseintrag konnte nicht gelesen werden: {e}"))
    })? {
        if entry.file_type().await.map(|t| t.is_dir()).unwrap_or(false) {
            let p_zip = entry.path().join(&filename_zip);
            if p_zip.exists() {
                let _ = fs::remove_file(p_zip).await;
            }
            let p_age = entry.path().join(&filename_age);
            if p_age.exists() {
                let _ = fs::remove_file(p_age).await;
            }
            let p_temp = entry.path().join(&filename_temp);
            if p_temp.exists() {
                let _ = fs::remove_file(p_temp).await;
            }
        }
    }

    Ok(StatusCode::NO_CONTENT)
}

/// Liefert das fertige ZIP-Archiv zum Download aus.
pub async fn download_export(
    State(state): State<Arc<AppState>>,
    Path(job_id): Path<Uuid>,
) -> Result<impl IntoResponse, AppError> {
    let mut entries = fs::read_dir(&state.config.exports_dir).await.map_err(|e| {
        AppError::Storage(format!("Exports-Verzeichnis konnte nicht gelesen werden: {e}"))
    })?;

    let filename = format!("{job_id}.zip");
    let mut found_path: Option<PathBuf> = None;

    while let Some(entry) = entries.next_entry().await.map_err(|e| {
        AppError::Storage(format!("Verzeichniseintrag konnte nicht gelesen werden: {e}"))
    })? {
        if entry.file_type().await.map(|t| t.is_dir()).unwrap_or(false) {
            let candidate = entry.path().join(&filename);
            if candidate.exists() {
                found_path = Some(candidate);
                break;
            }
        }
    }

    let path = found_path.ok_or_else(|| AppError::NotFound("Export-Datei nicht gefunden".to_string()))?;
    let file = fs::File::open(&path).await.map_err(|e| {
        AppError::Storage(format!("Export-Datei konnte nicht geoeffnet werden: {e}"))
    })?;

    let stream = tokio_util::io::ReaderStream::new(file);
    let body = axum::body::Body::from_stream(stream);

    let mut headers = HeaderMap::new();
    headers.insert(header::CONTENT_TYPE, "application/zip".parse().unwrap());
    headers.insert(
        header::CONTENT_DISPOSITION,
        format!("attachment; filename=\"export-{job_id}.zip\"")
            .parse()
            .unwrap(),
    );

    Ok((headers, body))
}

/// Loescht eine verschluesselte Speicherdatei physisch von der Festplatte.
pub async fn delete_storage_file(
    State(state): State<Arc<AppState>>,
    Path(storage_path): Path<String>,
) -> Result<StatusCode, AppError> {
    let safe_path = std::path::Path::new(&storage_path);
    if safe_path.components().any(|c| matches!(c, std::path::Component::ParentDir)) {
        return Err(AppError::Forbidden("Ungueltiger Pfad".to_string()));
    }

    let full_path = state.config.storage_dir.join(&storage_path);
    if full_path.exists() {
        fs::remove_file(&full_path).await.map_err(|e| {
            AppError::Storage(format!("Datei konnte nicht geloescht werden: {e}"))
        })?;
    }

    Ok(StatusCode::NO_CONTENT)
}
