use std::{net::SocketAddr, sync::Arc};
use tokio::signal;
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};
use upload_service::{create_router, db, AppState, Config};

#[tokio::main]
async fn main() {
    // Strukturiertes JSON-Logging initialisieren
    tracing_subscriber::registry()
        .with(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| "upload_service=info,tower_http=info".into()),
        )
        .with(tracing_subscriber::fmt::layer().json())
        .init();

    tracing::info!("4labscloud Upload-Service startet");

    // Konfiguration aus Umgebungsvariablen laden
    let config = match Config::from_env() {
        Ok(cfg) => cfg,
        Err(err) => {
            tracing::error!(error = %err, "start abgebrochen: fehlerhafte konfiguration");
            std::process::exit(1);
        }
    };

    // Speicherverzeichnisse sicherstellen
    if let Err(err) = tokio::fs::create_dir_all(&config.storage_dir).await {
        tracing::error!(
            error = %err,
            dir = ?config.storage_dir,
            "speicherverzeichnis konnte nicht erstellt werden"
        );
        std::process::exit(1);
    }
    if let Err(err) = tokio::fs::create_dir_all(&config.exports_dir).await {
        tracing::error!(
            error = %err,
            dir = ?config.exports_dir,
            "export-verzeichnis konnte nicht erstellt werden"
        );
        std::process::exit(1);
    }

    // Datenbankpool initialisieren
    let db_pool = match db::init_pool(&config.database_url).await {
        Ok(pool) => {
            tracing::info!("datenbankverbindung erfolgreich hergestellt");
            pool
        }
        Err(err) => {
            tracing::error!(error = %err, "datenbankverbindung fehlgeschlagen");
            std::process::exit(1);
        }
    };

    let port = config.upload_port;
    let active_sessions = tokio::sync::Mutex::new(std::collections::HashMap::new());
    let state = Arc::new(AppState {
        config,
        db_pool,
        active_sessions,
    });
    let app = create_router(state);

    let addr = SocketAddr::from(([0, 0, 0, 0], port));
    tracing::info!(port = port, "upload-service lauscht");

    let listener = match tokio::net::TcpListener::bind(addr).await {
        Ok(l) => l,
        Err(err) => {
            tracing::error!(error = %err, addr = %addr, "listener konnte nicht gebunden werden");
            std::process::exit(1);
        }
    };

    if let Err(err) = axum::serve(listener, app)
        .with_graceful_shutdown(shutdown_signal())
        .await
    {
        tracing::error!(error = %err, "server unerwartet beendet");
    }

    tracing::info!("4labscloud Upload-Service beendet");
}

/// Wartet auf Beendigungssignale (Ctrl+C oder SIGTERM).
async fn shutdown_signal() {
    let ctrl_c = async {
        signal::ctrl_c()
            .await
            .expect("Fehler beim Abfangen von Ctrl+C");
    };

    #[cfg(unix)]
    let terminate = async {
        signal::unix::signal(signal::unix::SignalKind::terminate())
            .expect("Fehler beim Abfangen von SIGTERM")
            .recv()
            .await;
    };

    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();

    tokio::select! {
        _ = ctrl_c => {},
        _ = terminate => {},
    }

    tracing::info!("beendigungssignal empfangen, fahre service herunter");
}
