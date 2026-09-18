use aes_gcm::{
    aead::{Aead, AeadCore, KeyInit, OsRng},
    Aes256Gcm, Key, Nonce,
};
use sha2::{Digest, Sha256};
use std::fmt::Write;

use crate::error::AppError;

/// Laenge der Nonce fuer AES-256-GCM in Bytes (96 Bit).
pub const NONCE_LEN: usize = 12;

/// Verschluesselt Nutzdaten mit AES-256-GCM.
/// Das Ausgabeformat ist: `nonce (12 Bytes) || ciphertext`.
pub fn encrypt(key: &[u8; 32], plaintext: &[u8]) -> Result<Vec<u8>, AppError> {
    let cipher = Aes256Gcm::new(Key::<Aes256Gcm>::from_slice(key));
    let nonce = Aes256Gcm::generate_nonce(&mut OsRng);

    let ciphertext = cipher
        .encrypt(&nonce, plaintext)
        .map_err(|e| AppError::Crypto(format!("Verschluesselung fehlgeschlagen: {e}")))?;

    let mut result = Vec::with_capacity(NONCE_LEN + ciphertext.len());
    result.extend_from_slice(&nonce);
    result.extend_from_slice(&ciphertext);

    Ok(result)
}

/// Entschluesselt Daten im Format `nonce (12 Bytes) || ciphertext` mit AES-256-GCM.
pub fn decrypt(key: &[u8; 32], nonce_and_ciphertext: &[u8]) -> Result<Vec<u8>, AppError> {
    if nonce_and_ciphertext.len() < NONCE_LEN {
        return Err(AppError::Crypto(
            "Verschluesselte Daten sind kuerzer als die zulaessige Nonce-Laenge".to_string(),
        ));
    }

    let (nonce_bytes, ciphertext) = nonce_and_ciphertext.split_at(NONCE_LEN);
    let cipher = Aes256Gcm::new(Key::<Aes256Gcm>::from_slice(key));
    let nonce = Nonce::from_slice(nonce_bytes);

    let plaintext = cipher
        .decrypt(nonce, ciphertext)
        .map_err(|e| AppError::Crypto(format!("Entschluesselung fehlgeschlagen: {e}")))?;

    Ok(plaintext)
}

/// Berechnet den SHA-256-Hash eines Byte-Puffers als hexadezimalen Kleinbuchstaben-String.
pub fn sha256_hex(data: &[u8]) -> String {
    let mut hasher = Sha256::new();
    hasher.update(data);
    let hash = hasher.finalize();

    let mut hex = String::with_capacity(64);
    for byte in hash {
        let _ = write!(hex, "{:02x}", byte);
    }
    hex
}

/// Leitet einen 32-Byte AES-Schluessel aus einem Passwort und Salt via Argon2id ab.
/// Parameter gemaess Vorgabe: Memory=64 MiB (65536 KiB), Iterations=3, Parallelism=4, Output=32 Bytes.
pub fn derive_argon2id_key(password: &str, salt: &[u8]) -> Result<[u8; 32], AppError> {
    use argon2::{Algorithm, Argon2, Params, Version};

    let params = Params::new(64 * 1024, 3, 4, Some(32))
        .map_err(|e| AppError::Crypto(format!("Ungueltige Argon2-Parameter: {e}")))?;
    let argon2 = Argon2::new(Algorithm::Argon2id, Version::V0x13, params);

    let mut key = [0u8; 32];
    argon2
        .hash_password_into(password.as_bytes(), salt, &mut key)
        .map_err(|e| AppError::Crypto(format!("Argon2id-Schluesselableitung fehlgeschlagen: {e}")))?;

    Ok(key)
}

/// Ueberschreibt einen Byte-Puffer im Speicher sicher mit Nullen (Shredding / Zeroization).
pub fn shred_bytes(buf: &mut [u8]) {
    for b in buf.iter_mut() {
        *b = 0;
    }
    std::sync::atomic::compiler_fence(std::sync::atomic::Ordering::SeqCst);
}

/// Loescht eine Datei sicher von der Festplatte durch vorheriges Ueberschreiben (Shredding).
pub fn shred_file(path: &std::path::Path) -> Result<(), AppError> {
    use std::io::Write;

    if !path.exists() {
        return Ok(());
    }

    let len = match std::fs::metadata(path) {
        Ok(m) => m.len(),
        Err(_) => 0,
    };

    if len > 0 {
        if let Ok(mut f) = std::fs::OpenOptions::new().write(true).open(path) {
            let zero_buf = [0u8; 4096];
            let mut remaining = len;
            while remaining > 0 {
                let to_write = std::cmp::min(remaining, zero_buf.len() as u64) as usize;
                let _ = f.write_all(&zero_buf[..to_write]);
                remaining -= to_write as u64;
            }
            let _ = f.sync_all();
        }
    }

    std::fs::remove_file(path)
        .map_err(|e| AppError::Storage(format!("Fehler beim Entfernen der Datei nach Shredding: {e}")))?;

    Ok(())
}


#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_encrypt_decrypt_roundtrip() {
        let key = [0x42u8; 32];
        let original_data = b"Streng vertrauliche Testdaten fuer 4labscloud";

        let encrypted = encrypt(&key, original_data).expect("Verschluesselung fehlgeschlagen");
        assert_ne!(encrypted, original_data);
        assert!(encrypted.len() >= NONCE_LEN);

        let decrypted = decrypt(&key, &encrypted).expect("Entschluesselung fehlgeschlagen");
        assert_eq!(decrypted, original_data);
    }

    #[test]
    fn test_decrypt_with_wrong_key_fails() {
        let key1 = [0x11u8; 32];
        let key2 = [0x22u8; 32];
        let original_data = b"Geheime Datei";

        let encrypted = encrypt(&key1, original_data).unwrap();
        let result = decrypt(&key2, &encrypted);
        assert!(result.is_err());
    }

    #[test]
    fn test_sha256_hex() {
        let data = b"4labscloud";
        let expected = "41481b951f7d1c290efee2cf302d68f087b6cffca6689cc4c4751cbc6382eca1";
        let hash = sha256_hex(data);
        assert_eq!(hash, expected);
    }
}
