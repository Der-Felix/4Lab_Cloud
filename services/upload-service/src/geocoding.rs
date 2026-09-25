use reqwest::Client;
use serde::{Deserialize, Serialize};
use sqlx::{PgPool, Row};
use std::sync::Arc;
use std::time::{Duration, Instant};
use tokio::sync::Mutex;

/// Ergebnis einer Reverse-Geocoding-Abfrage.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ReverseGeocodingResult {
    pub display_name: String,
    pub address: serde_json::Value,
}

/// Antwort-Struktur von der Nominatim /reverse API.
#[derive(Debug, Deserialize)]
struct NominatimResponse {
    display_name: Option<String>,
    address: Option<serde_json::Value>,
}

/// Client fuer das self-hosted Nominatim Reverse-Geocoding mit Caching und Rate-Limiting.
#[derive(Clone)]
pub struct GeocodingClient {
    http_client: Client,
    nominatim_url: String,
    enabled: bool,
    db_pool: Option<PgPool>,
    last_request_time: Arc<Mutex<Option<Instant>>>,
}

impl GeocodingClient {
    /// Erstellt einen neuen Geocoding-Client.
    pub fn new(nominatim_url: String, enabled: bool, db_pool: Option<PgPool>) -> Self {
        let http_client = Client::builder()
            .timeout(Duration::from_secs(5))
            .build()
            .unwrap_or_default();

        Self {
            http_client,
            nominatim_url,
            enabled,
            db_pool,
            last_request_time: Arc::new(Mutex::new(None)),
        }
    }

    /// Prueft, ob Geocoding aktiv ist.
    pub fn is_enabled(&self) -> bool {
        self.enabled
    }

    /// Fuehrt das Reverse-Geocoding durch (mit Cache-Pruefung und 1s Rate-Limiting).
    pub async fn reverse_geocode(
        &self,
        lat: f64,
        lon: f64,
    ) -> Result<Option<ReverseGeocodingResult>, crate::error::AppError> {
        if !self.enabled {
            return Ok(None);
        }

        // Auf 4 Nachkommastellen runden (~11 Meter Aufloesung fuer optimales Caching)
        let lat_rounded = (lat * 10000.0).round() / 10000.0;
        let lon_rounded = (lon * 10000.0).round() / 10000.0;
        let lat_str = format!("{lat_rounded:.4}");
        let lon_str = format!("{lon_rounded:.4}");

        // 1. Cache-Pruefung in PostgreSQL
        if let Some(ref pool) = self.db_pool {
            let cached = sqlx::query(
                "SELECT display_name, address_json FROM geocoding_cache WHERE lat_rounded = $1::numeric AND lon_rounded = $2::numeric LIMIT 1"
            )
            .bind(&lat_str)
            .bind(&lon_str)
            .fetch_optional(pool)
            .await;

            if let Ok(Some(row)) = cached {
                tracing::debug!(lat = lat_rounded, lon = lon_rounded, "geocoding cache-treffer");
                let display_name: String = row.get("display_name");
                let address: Option<serde_json::Value> = row.get("address_json");
                return Ok(Some(ReverseGeocodingResult {
                    display_name,
                    address: address.unwrap_or(serde_json::Value::Null),
                }));
            }
        }

        // 2. Rate-Limiting einhalten (mindestens 1000ms Abstand fuer Nominatim)
        {
            let mut last = self.last_request_time.lock().await;
            if let Some(last_instant) = *last {
                let elapsed = last_instant.elapsed();
                if elapsed < Duration::from_millis(1000) {
                    let sleep_dur = Duration::from_millis(1000) - elapsed;
                    tokio::time::sleep(sleep_dur).await;
                }
            }
            *last = Some(Instant::now());
        }

        // 3. Nominatim HTTP-Aufruf
        let url = format!(
            "{}/reverse?format=json&lat={:.6}&lon={:.6}&zoom=18&addressdetails=1",
            self.nominatim_url.trim_end_matches('/'),
            lat,
            lon
        );

        let resp = match self
            .http_client
            .get(&url)
            .header(
                "User-Agent",
                concat!("4LabCloud/", env!("CARGO_PKG_VERSION"), " (Self-Hosted)"),
            )
            .send()
            .await
        {
            Ok(r) => r,
            Err(e) => {
                tracing::warn!(error = %e, url = %url, "nominatim http-anfrage fehlgeschlagen");
                return Err(crate::error::AppError::Internal(format!(
                    "Nominatim Anfrage fehlgeschlagen: {e}"
                )));
            }
        };

        if !resp.status().is_success() {
            let status = resp.status();
            tracing::warn!(status = %status, "nominatim meldet fehler-status");
            return Err(crate::error::AppError::Internal(format!(
                "Nominatim Statusfehler: {status}"
            )));
        }

        let nom_data: NominatimResponse = match resp.json().await {
            Ok(d) => d,
            Err(e) => {
                tracing::warn!(error = %e, "nominatim json-antwort ungueltig");
                return Err(crate::error::AppError::Internal(format!(
                    "Nominatim ungueltiges JSON: {e}"
                )));
            }
        };

        let display_name = nom_data.display_name.unwrap_or_else(|| {
            format!("{lat_rounded:.4}, {lon_rounded:.4}")
        });
        let address = nom_data.address.unwrap_or(serde_json::Value::Null);

        let result = ReverseGeocodingResult {
            display_name: display_name.clone(),
            address: address.clone(),
        };

        // 4. In Cache-Tabelle speichern
        if let Some(ref pool) = self.db_pool {
            let insert_result = sqlx::query(
                "INSERT INTO geocoding_cache (lat_rounded, lon_rounded, display_name, address_json)
                 VALUES ($1::numeric, $2::numeric, $3, $4)
                 ON CONFLICT (lat_rounded, lon_rounded) DO NOTHING"
            )
            .bind(&lat_str)
            .bind(&lon_str)
            .bind(&display_name)
            .bind(&address)
            .execute(pool)
            .await;

            // Ein fehlgeschlagenes Cache-Schreiben darf die Anfrage nicht scheitern
            // lassen (Geocoding war erfolgreich) - aber der Fehler muss sichtbar sein,
            // sonst bleibt ein zugrundeliegender Schema-Fehler unbemerkt.
            if let Err(e) = insert_result {
                tracing::warn!(
                    error = %e,
                    lat = lat_rounded,
                    lon = lon_rounded,
                    "geocoding cache-schreibvorgang fehlgeschlagen, geocoding aber erfolgreich"
                );
            }
        }

        Ok(Some(result))
    }
}
