use axum::{routing::get, Json, Router};
use std::net::SocketAddr;
use tokio::net::TcpListener;
use upload_service::exif::{dms_to_decimal, extract_exif, strip_gps, ExifData, ExifGps};
use upload_service::geocoding::GeocodingClient;

#[test]
fn test_gps_parsing() {
    // Zuerich: 47 Grad, 22.614 Minuten N -> ~47.3769
    let lat = dms_to_decimal(47.0, 22.0, 36.84, "N");
    assert!((lat - 47.3769).abs() < 0.001);

    // Zuerich: 8 Grad, 32.502 Minuten E -> ~8.5417
    let lon = dms_to_decimal(8.0, 32.0, 30.12, "E");
    assert!((lon - 8.5417).abs() < 0.001);

    // Sydney: 33 Grad, 51.0 Minuten S -> negative Breite
    let s_lat = dms_to_decimal(33.0, 51.0, 0.0, "S");
    assert!(s_lat < 0.0);
    assert!((s_lat - (-33.85)).abs() < 0.01);

    // San Francisco: 122 Grad, 25.0 Minuten W -> negative Laenge
    let w_lon = dms_to_decimal(122.0, 25.0, 0.0, "W");
    assert!(w_lon < 0.0);
    assert!((w_lon - (-122.4167)).abs() < 0.01);
}

#[test]
fn test_exif_extraction_and_privacy_strip() {
    // 1. Minimales JPEG ohne EXIF
    let empty_jpeg = [
        0xFF, 0xD8, // SOI
        0xFF, 0xD9, // EOI
    ];
    let result = extract_exif(&empty_jpeg);
    // Ohne EXIF-Header sollte None geliefert werden
    assert!(result.is_none());

    // 2. Datenschutz: strip_gps entfernt Koordinaten verlaesslich
    let mut data = ExifData {
        make: Some("Apple".to_string()),
        model: Some("iPhone 15 Pro".to_string()),
        lens_model: Some("iPhone 15 Pro back triple camera 6.78mm f/1.78".to_string()),
        exposure_time: Some("1/250".to_string()),
        f_number: Some("f/1.8".to_string()),
        iso: Some(100),
        focal_length: Some("24mm".to_string()),
        datetime_original: Some("2026-09-18T15:30:00Z".to_string()),
        gps: Some(ExifGps {
            lat: 47.3769,
            lon: 8.5417,
            altitude: Some(408.5),
        }),
        orientation: Some(1),
    };

    assert!(data.gps.is_some());
    strip_gps(&mut data);
    assert!(data.gps.is_none());
    assert_eq!(data.make.as_deref(), Some("Apple"));
    assert_eq!(data.model.as_deref(), Some("iPhone 15 Pro"));
}

#[tokio::test]
async fn test_geocoding_disabled_mode() {
    // Wenn Geocoding deaktiviert ist, darf kein HTTP-Aufruf stattfinden
    let client = GeocodingClient::new("http://invalid-host:9999".to_string(), false, None);
    assert!(!client.is_enabled());

    let result = client.reverse_geocode(47.3769, 8.5417).await.unwrap();
    assert!(result.is_none());
}

#[tokio::test]
async fn test_geocoding_with_mock_nominatim() {
    // Mock-Nominatim Server starten
    let mock_app = Router::new().route(
        "/reverse",
        get(|| async {
            Json(serde_json::json!({
                "display_name": "Zuerich, Bezirk Zuerich, Zuerich, Schweiz",
                "address": {
                    "city": "Zuerich",
                    "state": "Zuerich",
                    "country": "Schweiz",
                    "country_code": "ch"
                }
            }))
        }),
    );

    let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let addr: SocketAddr = listener.local_addr().unwrap();
    tokio::spawn(async move {
        axum::serve(listener, mock_app).await.unwrap();
    });

    let mock_url = format!("http://{}", addr);
    let client = GeocodingClient::new(mock_url, true, None);
    assert!(client.is_enabled());

    let result = client
        .reverse_geocode(47.3769, 8.5417)
        .await
        .unwrap()
        .expect("Ergebnis erwartet");

    assert_eq!(result.display_name, "Zuerich, Bezirk Zuerich, Zuerich, Schweiz");
    assert_eq!(
        result.address.get("city").and_then(|v| v.as_str()),
        Some("Zuerich")
    );
}
