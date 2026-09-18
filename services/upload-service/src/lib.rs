use axum::{
    middleware,
    routing::{delete, get, options, post},
    Router,
};
use sha2::Sha256;
use sqlx::PgPool;
use std::{collections::HashMap, sync::Arc, time::Instant};
use tokio::sync::Mutex;
use uuid::Uuid;

pub mod auth;
pub mod config;
pub mod crypto;
pub mod db;
pub mod error;
pub mod exif;
pub mod geocoding;
pub mod models;
pub mod routes;
pub mod storage;

pub use config::Config;

/// Haelt den aktiven Hasher und den Zeitstempel des letzten Chunks fuer Rate-Limiting.
pub struct SessionHasher {
    pub hasher: Sha256,
    pub last_patch: Instant,
}

/// Globaler Anwendungszustand.
pub struct AppState {
    pub config: Config,
    pub db_pool: PgPool,
    pub active_sessions: Mutex<HashMap<Uuid, SessionHasher>>,
    pub geocoding_client: Arc<geocoding::GeocodingClient>,
}

impl AppState {
    pub fn new(
        config: Config,
        db_pool: PgPool,
        active_sessions: Mutex<HashMap<Uuid, SessionHasher>>,
        geocoding_client: Arc<geocoding::GeocodingClient>,
    ) -> Self {
        Self {
            config,
            db_pool,
            active_sessions,
            geocoding_client,
        }
    }

    pub fn new_for_test(
        config: Config,
        db_pool: PgPool,
        active_sessions: Mutex<HashMap<Uuid, SessionHasher>>,
    ) -> Self {
        let geocoding_client = Arc::new(geocoding::GeocodingClient::new(
            config.nominatim_url.clone(),
            config.geocoding_enabled,
            Some(db_pool.clone()),
        ));
        Self {
            config,
            db_pool,
            active_sessions,
            geocoding_client,
        }
    }
}

/// Erstellt den Axum-Router fuer den Upload-Service.
pub fn create_router(state: Arc<AppState>) -> Router {
    Router::new()
        // Oeffentlicher Healthcheck (nur intern im Netz)
        .route("/health", get(routes::health::health_check))
        // Durch Service-Token geschuetzte interne Upload-Endpunkte
        .route(
            "/internal/uploads/init",
            post(routes::uploads::init_upload).route_layer(middleware::from_fn_with_state(
                state.clone(),
                auth::require_service_token,
            )),
        )
        .route(
            "/internal/uploads/{id}/status",
            get(routes::uploads::get_upload_status).route_layer(middleware::from_fn_with_state(
                state.clone(),
                auth::require_service_token,
            )),
        )
        .route(
            "/internal/uploads/{id}",
            delete(routes::uploads::delete_upload).route_layer(middleware::from_fn_with_state(
                state.clone(),
                auth::require_service_token,
            )),
        )
        // Interne Export- und Dateiloesch-Endpunkte
        .route(
            "/internal/exports",
            post(routes::exports::create_export).route_layer(middleware::from_fn_with_state(
                state.clone(),
                auth::require_service_token,
            )),
        )
        .route(
            "/internal/exports/{job_id}",
            delete(routes::exports::delete_export).route_layer(middleware::from_fn_with_state(
                state.clone(),
                auth::require_service_token,
            )),
        )
        .route(
            "/internal/exports/{job_id}/download",
            get(routes::exports::download_export).route_layer(middleware::from_fn_with_state(
                state.clone(),
                auth::require_service_token,
            )),
        )
        .route(
            "/internal/files/{*path}",
            get(routes::downloads::internal_files_dispatcher)
                .delete(routes::exports::delete_storage_file)
                .route_layer(middleware::from_fn_with_state(
                    state.clone(),
                    auth::require_service_token,
                )),
        )
        // Interne Thumbnail-Endpunkte (geschuetzt mit Service-Token)
        .route(
            "/internal/thumbs/{id}",
            get(routes::thumbnails::get_thumbnail)
                .post(routes::thumbnails::generate_thumbnail)
                .route_layer(middleware::from_fn_with_state(
                    state.clone(),
                    auth::require_service_token,
                )),
        )
        // Tus-Endpunkte fuer den Datei-Upload
        .route(
            "/api/v1/uploads/tus",
            options(routes::tus::tus_options).post(routes::tus::tus_create),
        )
        .route(
            "/api/v1/uploads/tus/{id}",
            get(routes::tus::tus_head)
                .head(routes::tus::tus_head)
                .patch(routes::tus::tus_patch)
                .delete(routes::tus::tus_delete),
        )
        .route(
            "/api/v1/uploads/tus/{id}/download",
            get(routes::downloads::download_file),
        )
        .with_state(state)
}
