use base64::prelude::*;
use std::path::PathBuf;

use crate::error::AppError;

/// Konfiguration fuer den Upload-Service.
#[derive(Clone, Debug)]
pub struct Config {
    pub upload_port: u16,
    pub service_token: String,
    pub storage_key: [u8; 32],
    pub storage_dir: PathBuf,
    pub thumbs_dir: PathBuf,
    pub exports_dir: PathBuf,
    pub database_url: String,
    pub max_upload_size: i64,
}

impl Config {
    /// Laedt und validiert alle Umgebungsvariablen beim Start.
    pub fn from_env() -> Result<Self, AppError> {
        let upload_port = match std::env::var("UPLOAD_PORT") {
            Ok(v) if !v.trim().is_empty() => v
                .trim()
                .parse::<u16>()
                .map_err(|e| AppError::Config(format!("Ungueltiger UPLOAD_PORT: {e}")))?,
            _ => 8081,
        };

        let service_token = std::env::var("SERVICE_TOKEN")
            .map_err(|_| AppError::Config("SERVICE_TOKEN ist nicht gesetzt".to_string()))?;
        if service_token.trim().is_empty() {
            return Err(AppError::Config("SERVICE_TOKEN darf nicht leer sein".to_string()));
        }

        let storage_key_str = std::env::var("STORAGE_KEY")
            .map_err(|_| AppError::Config("STORAGE_KEY ist nicht gesetzt".to_string()))?;

        let decoded_key = BASE64_STANDARD
            .decode(storage_key_str.trim())
            .map_err(|e| AppError::Config(format!("STORAGE_KEY ist kein gueltiges Base64: {e}")))?;

        let storage_key: [u8; 32] = decoded_key
            .try_into()
            .map_err(|v: Vec<u8>| {
                AppError::Config(format!(
                    "STORAGE_KEY muss genau 32 Bytes lang sein, erhalten: {} Bytes",
                    v.len()
                ))
            })?;

        let storage_dir = match std::env::var("STORAGE_DIR") {
            Ok(v) if !v.trim().is_empty() => PathBuf::from(v),
            _ => PathBuf::from("/data/storage"),
        };

        let thumbs_dir = match std::env::var("THUMBS_DIR") {
            Ok(v) if !v.trim().is_empty() => PathBuf::from(v),
            _ => PathBuf::from("/data/thumbs"),
        };

        let exports_dir = match std::env::var("EXPORTS_DIR") {
            Ok(v) if !v.trim().is_empty() => PathBuf::from(v),
            _ => PathBuf::from("/data/exports"),
        };

        let max_upload_size = std::env::var("MAX_UPLOAD_SIZE")
            .unwrap_or_else(|_| "5368709120".to_string())
            .parse::<i64>()
            .unwrap_or(5368709120);

        let database_url = match std::env::var("DATABASE_URL") {
            Ok(url) => url,
            Err(_) => {
                let host = std::env::var("POSTGRES_HOST").unwrap_or_else(|_| "localhost".to_string());
                let port = std::env::var("POSTGRES_PORT").unwrap_or_else(|_| "5432".to_string());
                let user = std::env::var("POSTGRES_USER").unwrap_or_else(|_| "4labs".to_string());
                let pass = std::env::var("POSTGRES_PASSWORD").unwrap_or_default();
                let db_name = std::env::var("POSTGRES_DB").unwrap_or_else(|_| "4labscloud".to_string());
                format!("postgres://{user}:{pass}@{host}:{port}/{db_name}?sslmode=disable")
            }
        };

        Ok(Self {
            upload_port,
            service_token,
            storage_key,
            storage_dir,
            thumbs_dir,
            exports_dir,
            database_url,
            max_upload_size,
        })
    }
}
