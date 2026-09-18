use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

/// Antwort fuer den internen Healthcheck-Endpunkt.
#[derive(Debug, Serialize, Deserialize)]
pub struct HealthResponse {
    pub status: String,
    pub storage: String,
}

/// Eingabedaten fuer die Initialisierung eines Uploads.
#[derive(Debug, Deserialize, Serialize)]
pub struct UploadInitRequest {
    pub user_id: Uuid,
    pub filename: String,
    pub size_bytes: i64,
}

/// Rueckgabewerte nach erfolgreicher Upload-Initialisierung.
#[derive(Debug, Serialize, Deserialize)]
pub struct UploadInitResponse {
    pub upload_id: Uuid,
    pub presigned_url: String,
    pub expires_at: DateTime<Utc>,
}

/// Statusinformationen einer Upload-Session fuer den Go-Server.
#[derive(Debug, Serialize, Deserialize)]
pub struct UploadStatusResponse {
    pub upload_id: Uuid,
    pub user_id: Uuid,
    pub filename: String,
    pub size_bytes: i64,
    pub checksum: Option<String>,
    pub storage_path: String,
    pub status: String,
}
