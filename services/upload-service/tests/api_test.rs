use axum::{
    body::Body,
    http::{header, Request, StatusCode},
};
use http_body_util::BodyExt;
use serde_json::Value;
use std::sync::Arc;
use tower::ServiceExt;
use upload_service::{create_router, db, AppState, Config};
use uuid::Uuid;

async fn setup_test_app() -> Option<axum::Router> {
    let storage_dir = std::env::temp_dir().join(format!("4labs_test_{}", Uuid::new_v4()));
    let _ = std::fs::create_dir_all(&storage_dir);

    let db_url = std::env::var("DATABASE_URL").unwrap_or_else(|_| {
        "postgres://4labs:ZW2rz93oet3JM4TH8Va71Y6U2XRsCMQ5@127.0.0.1:5432/4labscloud?sslmode=disable"
            .to_string()
    });

    let db_pool = match db::init_pool(&db_url).await {
        Ok(pool) => pool,
        Err(_) => return None,
    };

    let config = Config {
        upload_port: 8081,
        service_token: "geheimes-service-token-12345".to_string(),
        storage_key: [0x55u8; 32],
        storage_dir: storage_dir.clone(),
        thumbs_dir: storage_dir.join("thumbs"),
        exports_dir: storage_dir.join("exports"),
        database_url: db_url,
        max_upload_size: 5368709120,
    };

    let active_sessions = tokio::sync::Mutex::new(std::collections::HashMap::new());
    let state = Arc::new(AppState { config, db_pool, active_sessions });
    Some(create_router(state))
}

#[tokio::test]
async fn test_health_check() {
    let app = match setup_test_app().await {
        Some(app) => app,
        None => return,
    };

    let request = Request::builder()
        .uri("/health")
        .method("GET")
        .body(Body::empty())
        .unwrap();

    let response = app.oneshot(request).await.unwrap();
    assert_eq!(response.status(), StatusCode::OK);

    let body = response.into_body().collect().await.unwrap().to_bytes();
    let json: Value = serde_json::from_slice(&body).unwrap();

    assert_eq!(json["status"], "ok");
    assert_eq!(json["storage"], "ok");
}

#[tokio::test]
async fn test_upload_init_without_token() {
    let app = match setup_test_app().await {
        Some(app) => app,
        None => return,
    };

    let payload = serde_json::json!({
        "user_id": Uuid::new_v4(),
        "filename": "dokument.pdf",
        "size_bytes": 1024
    });

    let request = Request::builder()
        .uri("/internal/uploads/init")
        .method("POST")
        .header(header::CONTENT_TYPE, "application/json")
        .body(Body::from(payload.to_string()))
        .unwrap();

    let response = app.oneshot(request).await.unwrap();
    assert_eq!(response.status(), StatusCode::UNAUTHORIZED);
}

#[tokio::test]
async fn test_upload_init_with_invalid_token() {
    let app = match setup_test_app().await {
        Some(app) => app,
        None => return,
    };

    let payload = serde_json::json!({
        "user_id": Uuid::new_v4(),
        "filename": "dokument.pdf",
        "size_bytes": 1024
    });

    let request = Request::builder()
        .uri("/internal/uploads/init")
        .method("POST")
        .header(header::CONTENT_TYPE, "application/json")
        .header(header::AUTHORIZATION, "Bearer falsches-token")
        .body(Body::from(payload.to_string()))
        .unwrap();

    let response = app.oneshot(request).await.unwrap();
    assert_eq!(response.status(), StatusCode::UNAUTHORIZED);
}

#[tokio::test]
async fn test_upload_init_with_valid_token() {
    let app = match setup_test_app().await {
        Some(app) => app,
        None => return,
    };

    let user_id = Uuid::new_v4();
    let payload = serde_json::json!({
        "user_id": user_id,
        "filename": "urlaub.jpg",
        "size_bytes": 2048576
    });

    let request = Request::builder()
        .uri("/internal/uploads/init")
        .method("POST")
        .header(header::CONTENT_TYPE, "application/json")
        .header(header::AUTHORIZATION, "Bearer geheimes-service-token-12345")
        .body(Body::from(payload.to_string()))
        .unwrap();

    let response = app.oneshot(request).await.unwrap();
    assert_eq!(response.status(), StatusCode::OK);

    let body = response.into_body().collect().await.unwrap().to_bytes();
    let json: Value = serde_json::from_slice(&body).unwrap();

    let upload_id_str = json["upload_id"].as_str().expect("upload_id fehlt");
    let upload_id = Uuid::parse_str(upload_id_str).expect("upload_id ist keine gueltige UUID");
    assert_eq!(upload_id.get_version(), Some(uuid::Version::Random)); // UUID v4

    let presigned_url = json["presigned_url"].as_str().expect("presigned_url fehlt");
    assert!(presigned_url.contains(upload_id_str));

    assert!(json["expires_at"].as_str().is_some());
}

#[tokio::test]
async fn test_upload_init_invalid_body() {
    let app = match setup_test_app().await {
        Some(app) => app,
        None => return,
    };

    // Leerer Dateiname
    let payload = serde_json::json!({
        "user_id": Uuid::new_v4(),
        "filename": "   ",
        "size_bytes": 1024
    });

    let request = Request::builder()
        .uri("/internal/uploads/init")
        .method("POST")
        .header(header::CONTENT_TYPE, "application/json")
        .header(header::AUTHORIZATION, "Bearer geheimes-service-token-12345")
        .body(Body::from(payload.to_string()))
        .unwrap();

    let response = app.oneshot(request).await.unwrap();
    assert_eq!(response.status(), StatusCode::BAD_REQUEST);
}
