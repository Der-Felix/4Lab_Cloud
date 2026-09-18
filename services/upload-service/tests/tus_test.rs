use axum::{
    body::Body,
    http::{header, Request, StatusCode},
};
use http_body_util::BodyExt;
use serde_json::Value;
use std::{path::PathBuf, sync::Arc};
use tower::ServiceExt;
use upload_service::{create_router, crypto, db, storage, AppState, Config};
use uuid::Uuid;

async fn setup_test_app() -> Option<(axum::Router, PathBuf, sqlx::PgPool)> {
    let storage_dir = std::env::temp_dir().join(format!("4labs_tus_test_{}", Uuid::new_v4()));
    let _ = std::fs::create_dir_all(&storage_dir);

    let db_url = std::env::var("DATABASE_URL").unwrap_or_else(|_| {
        "postgres://4labs:ZW2rz93oet3JM4TH8Va71Y6U2XRsCMQ5@127.0.0.1:5432/4labscloud?sslmode=disable"
            .to_string()
    });

    let db_pool = match db::init_pool(&db_url).await {
        Ok(pool) => pool,
        Err(_) => return None,
    };
    let storage_key = [0x77u8; 32];

    let config = Config {
        upload_port: 8081,
        service_token: "test-token".to_string(),
        storage_key,
        storage_dir: storage_dir.clone(),
        thumbs_dir: storage_dir.join("thumbs"),
        exports_dir: storage_dir.join("exports"),
        database_url: db_url,
        max_upload_size: 5368709120,
    };

    let active_sessions = tokio::sync::Mutex::new(std::collections::HashMap::new());
    let state = Arc::new(AppState {
        config,
        db_pool: db_pool.clone(),
        active_sessions,
    });
    Some((create_router(state), storage_dir, db_pool))
}

#[tokio::test]
async fn tus_roundtrip_test() {
    let (app, _storage_dir, _pool) = match setup_test_app().await {
        Some(t) => t,
        None => return,
    };

    // 1. OPTIONS /api/v1/uploads/tus
    let req = Request::builder()
        .uri("/api/v1/uploads/tus")
        .method("OPTIONS")
        .body(Body::empty())
        .unwrap();
    let resp = app.clone().oneshot(req).await.unwrap();
    assert_eq!(resp.status(), StatusCode::NO_CONTENT);
    assert_eq!(resp.headers().get("Tus-Resumable").unwrap(), "1.0.0");
    assert_eq!(resp.headers().get("Tus-Version").unwrap(), "1.0.0");
    assert!(resp.headers().get("Tus-Extension").is_some());

    // 2. POST /api/v1/uploads/tus (Creation mit 10 Bytes)
    let req = Request::builder()
        .uri("/api/v1/uploads/tus")
        .method("POST")
        .header("Upload-Length", "10")
        .body(Body::empty())
        .unwrap();
    let resp = app.clone().oneshot(req).await.unwrap();
    assert_eq!(resp.status(), StatusCode::CREATED);
    assert_eq!(resp.headers().get("Tus-Resumable").unwrap(), "1.0.0");

    let location = resp.headers().get(header::LOCATION).unwrap().to_str().unwrap().to_string();
    assert!(location.starts_with("/api/v1/uploads/tus/"));

    // 3. HEAD /api/v1/uploads/tus/{id} (Anfang: Offset 0)
    let req = Request::builder()
        .uri(&location)
        .method("HEAD")
        .body(Body::empty())
        .unwrap();
    let resp = app.clone().oneshot(req).await.unwrap();
    assert_eq!(resp.status(), StatusCode::OK);
    assert_eq!(resp.headers().get("Upload-Offset").unwrap(), "0");
    assert_eq!(resp.headers().get("Upload-Length").unwrap(), "10");

    // 4. PATCH Chunk 1 (6 Bytes: "123456")
    let req = Request::builder()
        .uri(&location)
        .method("PATCH")
        .header("Upload-Offset", "0")
        .header("Content-Type", "application/offset+octet-stream")
        .body(Body::from("123456"))
        .unwrap();
    let resp = app.clone().oneshot(req).await.unwrap();
    assert_eq!(resp.status(), StatusCode::NO_CONTENT);
    assert_eq!(resp.headers().get("Upload-Offset").unwrap(), "6");

    // Rate-Limit Pause
    tokio::time::sleep(std::time::Duration::from_millis(1100)).await;

    // 5. PATCH Chunk 2 (4 Bytes: "7890")
    let req = Request::builder()
        .uri(&location)
        .method("PATCH")
        .header("Upload-Offset", "6")
        .header("Content-Type", "application/offset+octet-stream")
        .body(Body::from("7890"))
        .unwrap();
    let resp = app.clone().oneshot(req).await.unwrap();
    assert_eq!(resp.status(), StatusCode::NO_CONTENT);
    assert_eq!(resp.headers().get("Upload-Offset").unwrap(), "10");

    // 6. HEAD Pruefung nach Abschluss
    let req = Request::builder()
        .uri(&location)
        .method("HEAD")
        .body(Body::empty())
        .unwrap();
    let resp = app.clone().oneshot(req).await.unwrap();
    assert_eq!(resp.status(), StatusCode::FORBIDDEN); // status ist nun 'completed' -> 403 Forbidden!

    // 7. DELETE Abbruch/Loeschen
    let req = Request::builder()
        .uri(&location)
        .method("DELETE")
        .body(Body::empty())
        .unwrap();
    let resp = app.clone().oneshot(req).await.unwrap();
    assert_eq!(resp.status(), StatusCode::NO_CONTENT);

    // 8. HEAD nach DELETE -> 404
    let req = Request::builder()
        .uri(&location)
        .method("HEAD")
        .body(Body::empty())
        .unwrap();
    let resp = app.clone().oneshot(req).await.unwrap();
    assert_eq!(resp.status(), StatusCode::NOT_FOUND);
}

#[tokio::test]
async fn offset_conflict_test() {
    let (app, _storage_dir, _pool) = match setup_test_app().await {
        Some(t) => t,
        None => return,
    };

    let req = Request::builder()
        .uri("/api/v1/uploads/tus")
        .method("POST")
        .header("Upload-Length", "100")
        .body(Body::empty())
        .unwrap();
    let resp = app.clone().oneshot(req).await.unwrap();
    assert_eq!(resp.status(), StatusCode::CREATED);
    let location = resp.headers().get(header::LOCATION).unwrap().to_str().unwrap().to_string();

    let req = Request::builder()
        .uri(&location)
        .method("PATCH")
        .header("Upload-Offset", "50")
        .header("Content-Type", "application/offset+octet-stream")
        .body(Body::from("test-daten"))
        .unwrap();
    let resp = app.clone().oneshot(req).await.unwrap();
    assert_eq!(resp.status(), StatusCode::CONFLICT);
    assert_eq!(resp.headers().get("Upload-Offset").unwrap(), "0");
    assert_eq!(resp.headers().get("Tus-Resumable").unwrap(), "1.0.0");
}

#[tokio::test]
async fn encrypted_file_not_plaintext_test() {
    let (app, storage_dir, _pool) = match setup_test_app().await {
        Some(t) => t,
        None => return,
    };

    let secret_payload = "STRENG_GEHEIME_4LABS_DATEN_IN_PLAINTEXT_12345";
    let len = secret_payload.len().to_string();

    let req = Request::builder()
        .uri("/api/v1/uploads/tus")
        .method("POST")
        .header("Upload-Length", &len)
        .body(Body::empty())
        .unwrap();
    let resp = app.clone().oneshot(req).await.unwrap();
    let location = resp.headers().get(header::LOCATION).unwrap().to_str().unwrap().to_string();
    let upload_id_str = location.trim_start_matches("/api/v1/uploads/tus/");
    let upload_id = Uuid::parse_str(upload_id_str).unwrap();

    let req = Request::builder()
        .uri(&location)
        .method("PATCH")
        .header("Upload-Offset", "0")
        .header("Content-Type", "application/offset+octet-stream")
        .body(Body::from(secret_payload))
        .unwrap();
    let resp = app.clone().oneshot(req).await.unwrap();
    assert_eq!(resp.status(), StatusCode::NO_CONTENT);

    let path = storage::get_storage_path(&storage_dir, upload_id);
    assert!(path.exists(), "Verschluesselte Datei existiert nicht: {:?}", path);

    let raw_file_bytes = std::fs::read(&path).expect("Konnte Datei nicht lesen");
    assert!(!raw_file_bytes.is_empty());

    let contains_plaintext = raw_file_bytes
        .windows(secret_payload.len())
        .any(|window| window == secret_payload.as_bytes());

    assert!(
        !contains_plaintext,
        "Sicherheitsverletzung: Klartext-Payload wurde unverschluesselt auf der Festplatte gefunden!"
    );
}

#[tokio::test]
async fn checksum_test() {
    let (app, _storage_dir, _pool) = match setup_test_app().await {
        Some(t) => t,
        None => return,
    };

    let part1 = "HALLO_";
    let part2 = "WELT_4LABS";
    let full_content = format!("{part1}{part2}");
    let expected_checksum = crypto::sha256_hex(full_content.as_bytes());

    // 1. POST
    let req = Request::builder()
        .uri("/api/v1/uploads/tus")
        .method("POST")
        .header("Upload-Length", full_content.len().to_string())
        .body(Body::empty())
        .unwrap();
    let resp = app.clone().oneshot(req).await.unwrap();
    let location = resp.headers().get(header::LOCATION).unwrap().to_str().unwrap().to_string();
    let upload_id = location.trim_start_matches("/api/v1/uploads/tus/");

    // 2. PATCH Chunk 1
    let req = Request::builder()
        .uri(&location)
        .method("PATCH")
        .header("Upload-Offset", "0")
        .header("Content-Type", "application/offset+octet-stream")
        .body(Body::from(part1))
        .unwrap();
    let resp = app.clone().oneshot(req).await.unwrap();
    assert_eq!(resp.status(), StatusCode::NO_CONTENT);

    tokio::time::sleep(std::time::Duration::from_millis(1050)).await;

    // 3. PATCH Chunk 2 (Finalisiert Upload)
    let req = Request::builder()
        .uri(&location)
        .method("PATCH")
        .header("Upload-Offset", part1.len().to_string())
        .header("Content-Type", "application/offset+octet-stream")
        .body(Body::from(part2))
        .unwrap();
    let resp = app.clone().oneshot(req).await.unwrap();
    assert_eq!(resp.status(), StatusCode::NO_CONTENT);

    // 4. Status per interner API abfragen
    let req = Request::builder()
        .uri(format!("/internal/uploads/{upload_id}/status"))
        .method("GET")
        .header("Authorization", "Bearer test-token")
        .body(Body::empty())
        .unwrap();
    let resp = app.clone().oneshot(req).await.unwrap();
    assert_eq!(resp.status(), StatusCode::OK);

    let body = resp.into_body().collect().await.unwrap().to_bytes();
    let json: Value = serde_json::from_slice(&body).unwrap();

    assert_eq!(json["status"], "completed");
    assert_eq!(json["checksum"], expected_checksum);
}

#[tokio::test]
async fn expiry_test() {
    let (app, _storage_dir, pool) = match setup_test_app().await {
        Some(t) => t,
        None => return,
    };

    let upload_id = Uuid::new_v4();
    let user_id = Uuid::new_v4();

    // Session mit abgelaufenem Timestamp in der DB anlegen
    sqlx::query(
        r#"
        INSERT INTO upload_sessions (id, user_id, filename, size_bytes, upload_offset, status, expires_at)
        VALUES ($1, $2, 'abgelaufen.txt', 100, 0, 'uploading', now() - interval '1 hour')
        "#,
    )
    .bind(upload_id)
    .bind(user_id)
    .execute(&pool)
    .await
    .unwrap();

    // HEAD -> 410 Gone
    let req = Request::builder()
        .uri(format!("/api/v1/uploads/tus/{upload_id}"))
        .method("HEAD")
        .body(Body::empty())
        .unwrap();
    let resp = app.clone().oneshot(req).await.unwrap();
    assert_eq!(resp.status(), StatusCode::GONE);
    assert_eq!(resp.headers().get("Tus-Resumable").unwrap(), "1.0.0");

    // PATCH -> 410 Gone
    let req = Request::builder()
        .uri(format!("/api/v1/uploads/tus/{upload_id}"))
        .method("PATCH")
        .header("Upload-Offset", "0")
        .header("Content-Type", "application/offset+octet-stream")
        .body(Body::from("data"))
        .unwrap();
    let resp = app.clone().oneshot(req).await.unwrap();
    assert_eq!(resp.status(), StatusCode::GONE);
    assert_eq!(resp.headers().get("Tus-Resumable").unwrap(), "1.0.0");
}

#[tokio::test]
async fn status_test() {
    let (app, _storage_dir, pool) = match setup_test_app().await {
        Some(t) => t,
        None => return,
    };

    let upload_id = Uuid::new_v4();
    let user_id = Uuid::new_v4();

    // Session mit Status != 'uploading' in der DB anlegen
    sqlx::query(
        r#"
        INSERT INTO upload_sessions (id, user_id, filename, size_bytes, upload_offset, status)
        VALUES ($1, $2, 'fertig.txt', 100, 100, 'completed')
        "#,
    )
    .bind(upload_id)
    .bind(user_id)
    .execute(&pool)
    .await
    .unwrap();

    // HEAD -> 403 Forbidden
    let req = Request::builder()
        .uri(format!("/api/v1/uploads/tus/{upload_id}"))
        .method("HEAD")
        .body(Body::empty())
        .unwrap();
    let resp = app.clone().oneshot(req).await.unwrap();
    assert_eq!(resp.status(), StatusCode::FORBIDDEN);
    assert_eq!(resp.headers().get("Tus-Resumable").unwrap(), "1.0.0");

    // PATCH -> 403 Forbidden
    let req = Request::builder()
        .uri(format!("/api/v1/uploads/tus/{upload_id}"))
        .method("PATCH")
        .header("Upload-Offset", "100")
        .header("Content-Type", "application/offset+octet-stream")
        .body(Body::from("data"))
        .unwrap();
    let resp = app.clone().oneshot(req).await.unwrap();
    assert_eq!(resp.status(), StatusCode::FORBIDDEN);
    assert_eq!(resp.headers().get("Tus-Resumable").unwrap(), "1.0.0");
}

#[tokio::test]
async fn rate_limit_test() {
    let (app, _storage_dir, _pool) = match setup_test_app().await {
        Some(t) => t,
        None => return,
    };

    // POST
    let req = Request::builder()
        .uri("/api/v1/uploads/tus")
        .method("POST")
        .header("Upload-Length", "100")
        .body(Body::empty())
        .unwrap();
    let resp = app.clone().oneshot(req).await.unwrap();
    let location = resp.headers().get(header::LOCATION).unwrap().to_str().unwrap().to_string();

    // PATCH 1 -> 204
    let req = Request::builder()
        .uri(&location)
        .method("PATCH")
        .header("Upload-Offset", "0")
        .header("Content-Type", "application/offset+octet-stream")
        .body(Body::from("chunk1"))
        .unwrap();
    let resp = app.clone().oneshot(req).await.unwrap();
    assert_eq!(resp.status(), StatusCode::NO_CONTENT);

    // PATCH 2 direkt danach (< 1s) -> 429 Too Many Requests
    let req = Request::builder()
        .uri(&location)
        .method("PATCH")
        .header("Upload-Offset", "6")
        .header("Content-Type", "application/offset+octet-stream")
        .body(Body::from("chunk2"))
        .unwrap();
    let resp = app.clone().oneshot(req).await.unwrap();
    assert_eq!(resp.status(), StatusCode::TOO_MANY_REQUESTS);
    assert_eq!(resp.headers().get("Tus-Resumable").unwrap(), "1.0.0");
    assert_eq!(resp.headers().get("Retry-After").unwrap(), "1");
}

#[tokio::test]
async fn tus_complete_status_test() {
    let (app, _storage_dir, _pool) = match setup_test_app().await {
        Some(t) => t,
        None => return,
    };

    let user_id = Uuid::new_v4();
    let payload = "4LABS_TUS_COMPLETE_TEST_CONTENT_12345";
    let len = payload.len();
    let expected_checksum = crypto::sha256_hex(payload.as_bytes());

    // 1. Initialisierung via POST /internal/uploads/init (wie Go es aufruft)
    let init_req = Request::builder()
        .uri("/internal/uploads/init")
        .method("POST")
        .header("Authorization", "Bearer test-token")
        .header("Content-Type", "application/json")
        .body(Body::from(serde_json::to_string(&serde_json::json!({
            "user_id": user_id,
            "filename": "testfile.txt",
            "size_bytes": len
        })).unwrap()))
        .unwrap();
    let resp = app.clone().oneshot(init_req).await.unwrap();
    assert_eq!(resp.status(), StatusCode::OK);
    let body = resp.into_body().collect().await.unwrap().to_bytes();
    let init_json: Value = serde_json::from_slice(&body).unwrap();
    let upload_id = init_json["upload_id"].as_str().unwrap();

    // 2. Vollstaendiger Tus-Upload via PATCH /api/v1/uploads/tus/{id}
    let patch_req = Request::builder()
        .uri(format!("/api/v1/uploads/tus/{upload_id}"))
        .method("PATCH")
        .header("Upload-Offset", "0")
        .header("Content-Type", "application/offset+octet-stream")
        .body(Body::from(payload))
        .unwrap();
    let resp = app.clone().oneshot(patch_req).await.unwrap();
    assert_eq!(resp.status(), StatusCode::NO_CONTENT);

    // 3. Statusabfrage via GET /internal/uploads/{id}/status -> MUSS status="completed" liefern
    let status_req = Request::builder()
        .uri(format!("/internal/uploads/{upload_id}/status"))
        .method("GET")
        .header("Authorization", "Bearer test-token")
        .body(Body::empty())
        .unwrap();
    let resp = app.clone().oneshot(status_req).await.unwrap();
    assert_eq!(resp.status(), StatusCode::OK);

    let body = resp.into_body().collect().await.unwrap().to_bytes();
    let status_json: Value = serde_json::from_slice(&body).unwrap();

    assert_eq!(status_json["status"], "completed");
    assert_eq!(status_json["checksum"], expected_checksum);
    assert_eq!(status_json["size_bytes"], len);
    assert_eq!(status_json["user_id"], user_id.to_string());
}
