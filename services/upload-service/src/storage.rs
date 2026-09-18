use std::path::{Path, PathBuf};
use bytes::Bytes;
use futures_core::Stream;
use tokio::{
    fs::{File, OpenOptions},
    io::{AsyncReadExt, AsyncWriteExt},
};
use uuid::Uuid;

use crate::{crypto, error::AppError};

/// Ermittelt den vollstaendigen Dateipfad mit Sharding (erste 2 Hex-Zeichen der UUID).
pub fn get_storage_path(base_dir: &Path, upload_id: Uuid) -> PathBuf {
    let id_str = upload_id.to_string();
    let shard = &id_str[..2];
    base_dir.join(shard).join(format!("{id_str}.enc"))
}

/// Ermittelt den relativen Speicherpfad mit Sharding (z.B. "ab/abcd...enc") fuer die DB.
pub fn get_relative_storage_path(upload_id: Uuid) -> String {
    let id_str = upload_id.to_string();
    let shard = &id_str[..2];
    format!("{shard}/{id_str}.enc")
}

/// Verschluesselt einen Daten-Chunk und haengt ihn atomar an die Datei an.
/// Format pro Chunk: `[nonce(12) | ciphertext_len(4) | ciphertext]`
///
/// Begruendung der 4-Byte Laenge trotz AES-GCM-Tag-Redundanz:
/// Die explizite Laengenangabe (Big-Endian u32) ermoeglicht das deterministische,
/// sequenzielle Streamen und Parsen von Chunks variabler Groesse aus einer einzigen
/// Datei ohne Parsing-Mehrdeutigkeiten oder separates Index-File.
pub async fn append_chunk(path: &Path, key: &[u8; 32], chunk: &[u8]) -> Result<(), AppError> {
    if let Some(parent) = path.parent() {
        tokio::fs::create_dir_all(parent)
            .await
            .map_err(|e| AppError::Internal(format!("Verzeichnis konnte nicht erstellt werden: {e}")))?;
    }

    // Verschluesselung via AES-256-GCM (erzeugt nonce (12) + ciphertext)
    let encrypted_data = crypto::encrypt(key, chunk)?;
    let (nonce, ciphertext) = encrypted_data.split_at(crypto::NONCE_LEN);
    let ciphertext_len = (ciphertext.len() as u32).to_be_bytes();

    let mut file = OpenOptions::new()
        .create(true)
        .append(true)
        .open(path)
        .await
        .map_err(|e| AppError::Internal(format!("Datei konnte nicht geoeffnet werden: {e}")))?;

    // Schreiben: 12 Bytes Nonce, 4 Bytes Laenge, dann Ciphertext
    file.write_all(nonce)
        .await
        .map_err(|e| AppError::Internal(format!("Fehler beim Schreiben der Nonce: {e}")))?;
    file.write_all(&ciphertext_len)
        .await
        .map_err(|e| AppError::Internal(format!("Fehler beim Schreiben der Chunk-Laenge: {e}")))?;
    file.write_all(ciphertext)
        .await
        .map_err(|e| AppError::Internal(format!("Fehler beim Schreiben des Ciphertexts: {e}")))?;
    file.flush()
        .await
        .map_err(|e| AppError::Internal(format!("Fehler beim Schreiben auf Platte: {e}")))?;

    Ok(())
}

/// Liest und entschluesselt alle bisher gespeicherten Chunks einer Datei.
/// Dient der Rekonstruktion des Klartext-Hashers nach einem Server-Neustart.
pub async fn read_and_decrypt_all(path: &Path, key: &[u8; 32]) -> Result<Vec<u8>, AppError> {
    if !path.exists() {
        return Ok(Vec::new());
    }

    let mut file = File::open(path)
        .await
        .map_err(|e| AppError::Internal(format!("Datei konnte nicht geoeffnet werden: {e}")))?;

    let mut plaintext_all = Vec::new();

    loop {
        let mut nonce = [0u8; crypto::NONCE_LEN];
        match file.read_exact(&mut nonce).await {
            Ok(_) => {}
            Err(e) if e.kind() == std::io::ErrorKind::UnexpectedEof => break,
            Err(e) => return Err(AppError::Internal(format!("Fehler beim Lesen der Nonce: {e}"))),
        }

        let mut len_bytes = [0u8; 4];
        file.read_exact(&mut len_bytes)
            .await
            .map_err(|e| AppError::Internal(format!("Fehler beim Lesen der Chunk-Laenge: {e}")))?;
        let cipher_len = u32::from_be_bytes(len_bytes) as usize;

        let mut ciphertext = vec![0u8; cipher_len];
        file.read_exact(&mut ciphertext)
            .await
            .map_err(|e| AppError::Internal(format!("Fehler beim Lesen des Ciphertexts: {e}")))?;

        let mut combined = Vec::with_capacity(crypto::NONCE_LEN + cipher_len);
        combined.extend_from_slice(&nonce);
        combined.extend_from_slice(&ciphertext);

        let chunk_plain = crypto::decrypt(key, &combined)?;
        plaintext_all.extend_from_slice(&chunk_plain);
    }

    Ok(plaintext_all)
}

/// Loescht die verschluesselte Datei von der Festplatte.
pub async fn delete_file(path: &Path) -> Result<(), AppError> {
    if path.exists() {
        tokio::fs::remove_file(path)
            .await
            .map_err(|e| AppError::Internal(format!("Fehler beim Loeschen der Datei: {e}")))?;
    }
    Ok(())
}

/// Streamt und entschluesselt Chunks on-the-fly ohne die gesamte Datei im Arbeitsspeicher zu halten.
pub fn stream_decrypt_file(
    path: PathBuf,
    key: [u8; 32],
) -> impl Stream<Item = Result<Bytes, std::io::Error>> {
    async_stream::try_stream! {
        let mut file = File::open(&path).await?;

        loop {
            let mut nonce = [0u8; crypto::NONCE_LEN];
            match file.read_exact(&mut nonce).await {
                Ok(_) => {},
                Err(e) if e.kind() == std::io::ErrorKind::UnexpectedEof => break,
                Err(e) => {
                    Err(e)?;
                    return;
                }
            }

            let mut len_bytes = [0u8; 4];
            file.read_exact(&mut len_bytes).await?;
            let cipher_len = u32::from_be_bytes(len_bytes) as usize;

            let mut ciphertext = vec![0u8; cipher_len];
            file.read_exact(&mut ciphertext).await?;

            let mut combined = Vec::with_capacity(crypto::NONCE_LEN + cipher_len);
            combined.extend_from_slice(&nonce);
            combined.extend_from_slice(&ciphertext);

            match crypto::decrypt(&key, &combined) {
                Ok(plain) => {
                    yield Bytes::from(plain);
                }
                Err(e) => {
                    Err(std::io::Error::new(std::io::ErrorKind::InvalidData, e.to_string()))?;
                    return;
                }
            }
        }
    }
}

