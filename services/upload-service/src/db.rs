use chrono::{DateTime, Utc};
use sqlx::{postgres::PgPoolOptions, PgPool};
use uuid::Uuid;

use crate::error::AppError;

/// Repraesentiert eine Upload-Sitzung aus der Tabelle upload_sessions.
#[derive(Debug, Clone, sqlx::FromRow)]
pub struct UploadSession {
    pub id: Uuid,
    pub user_id: Uuid,
    pub filename: String,
    pub size_bytes: i64,
    pub upload_offset: i64,
    pub checksum: Option<String>,
    pub status: String,
    pub created_at: DateTime<Utc>,
    pub expires_at: DateTime<Utc>,
}

/// Initialisiert den PostgreSQL-Verbindungspool.
pub async fn init_pool(database_url: &str) -> Result<PgPool, AppError> {
    PgPoolOptions::new()
        .max_connections(20)
        .min_connections(2)
        .acquire_timeout(std::time::Duration::from_secs(5))
        .connect(database_url)
        .await
        .map_err(|e| AppError::Internal(format!("Fehler beim Verbinden mit der Datenbank: {e}")))
}

/// Erstellt eine neue Upload-Sitzung in der Datenbank.
pub async fn create_session(
    pool: &PgPool,
    id: Uuid,
    user_id: Uuid,
    filename: &str,
    size_bytes: i64,
) -> Result<UploadSession, AppError> {
    let session = sqlx::query_as::<_, UploadSession>(
        r#"
        INSERT INTO upload_sessions (id, user_id, filename, size_bytes, upload_offset, status)
        VALUES ($1, $2, $3, $4, 0, 'uploading')
        RETURNING id, user_id, filename, size_bytes, upload_offset, checksum, status, created_at, expires_at
        "#,
    )
    .bind(id)
    .bind(user_id)
    .bind(filename)
    .bind(size_bytes)
    .fetch_one(pool)
    .await
    .map_err(|e| AppError::Internal(format!("Fehler beim Anlegen der Upload-Session: {e}")))?;

    Ok(session)
}

/// Ruft eine Upload-Sitzung anhand ihrer ID ab.
pub async fn get_session(pool: &PgPool, id: Uuid) -> Result<Option<UploadSession>, AppError> {
    let session = sqlx::query_as::<_, UploadSession>(
        r#"
        SELECT id, user_id, filename, size_bytes, upload_offset, checksum, status, created_at, expires_at
        FROM upload_sessions
        WHERE id = $1
        "#,
    )
    .bind(id)
    .fetch_optional(pool)
    .await
    .map_err(|e| AppError::Internal(format!("Fehler beim Abrufen der Upload-Session: {e}")))?;

    Ok(session)
}

/// Aktualisiert den Byte-Offset einer Upload-Sitzung atomar (nur wenn Session noch aktiv ist).
pub async fn update_offset(pool: &PgPool, id: Uuid, new_offset: i64) -> Result<(), AppError> {
    sqlx::query(
        r#"
        UPDATE upload_sessions
        SET upload_offset = $1
        WHERE id = $2 AND status = 'uploading'
        "#,
    )
    .bind(new_offset)
    .bind(id)
    .execute(pool)
    .await
    .map_err(|e| AppError::Internal(format!("Fehler beim Aktualisieren des Upload-Offsets: {e}")))?;

    Ok(())
}

/// Loescht eine Upload-Sitzung.
pub async fn delete_session(pool: &PgPool, id: Uuid) -> Result<bool, AppError> {
    let result = sqlx::query(
        r#"
        DELETE FROM upload_sessions
        WHERE id = $1
        "#,
    )
    .bind(id)
    .execute(pool)
    .await
    .map_err(|e| AppError::Internal(format!("Fehler beim Loeschen der Upload-Session: {e}")))?;

    Ok(result.rows_affected() > 0)
}

/// Markiert eine Upload-Session atomar als abgeschlossen und speichert Pruefsumme sowie finalen Offset.
pub async fn complete_session(
    pool: &PgPool,
    id: Uuid,
    checksum: &str,
    final_offset: i64,
) -> Result<(), AppError> {
    sqlx::query(
        r#"
        UPDATE upload_sessions
        SET status = 'completed', checksum = $1, upload_offset = $2
        WHERE id = $3
        "#,
    )
    .bind(checksum)
    .bind(final_offset)
    .bind(id)
    .execute(pool)
    .await
    .map_err(|e| AppError::Internal(format!("Fehler beim Abschliessen der Upload-Session: {e}")))?;

    Ok(())
}
