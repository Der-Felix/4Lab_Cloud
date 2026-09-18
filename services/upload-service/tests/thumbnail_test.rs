use axum::{
    body::Body,
    http::{header, Request, StatusCode},
};
use http_body_util::BodyExt;
use std::sync::Arc;
use tower::ServiceExt;
use uuid::Uuid;

use upload_service::{create_router, storage, AppState, Config};

#[tokio::test]
async fn test_thumbnail_generation_and_retrieval() {
    let base_temp = std::env::temp_dir().join(format!("4labs_thumb_test_{}", Uuid::new_v4()));
    let storage_dir = base_temp.join("storage");
    let thumbs_dir = base_temp.join("thumbs");
    let exports_dir = base_temp.join("exports");
    tokio::fs::create_dir_all(&storage_dir).await.unwrap();
    tokio::fs::create_dir_all(&thumbs_dir).await.unwrap();
    tokio::fs::create_dir_all(&exports_dir).await.unwrap();

    let storage_key = [0x42u8; 32];
    let service_token = "valid-service-token-secret".to_string();

    let db_url = "postgres://fake:fake@127.0.0.1:5432/fakedb";
    let db_pool = sqlx::Pool::<sqlx::Postgres>::connect_lazy(db_url).unwrap();

    let config = Config {
        upload_port: 8081,
        service_token: service_token.clone(),
        storage_key,
        storage_dir: storage_dir.clone(),
        thumbs_dir: thumbs_dir.clone(),
        exports_dir,
        database_url: db_url.to_string(),
        max_upload_size: 5368709120,
        ..Default::default()
    };

    let active_sessions = tokio::sync::Mutex::new(std::collections::HashMap::new());
    let state = Arc::new(AppState::new_for_test(
        config,
        db_pool,
        active_sessions,
    ));
    let app = create_router(state);

    // 1. Ein 800x600 Testbild im PNG-Format im Speicher erzeugen
    let img = image::RgbImage::new(800, 600);
    let mut orig_png_bytes = Vec::new();
    let mut cursor = std::io::Cursor::new(&mut orig_png_bytes);
    img.write_to(&mut cursor, image::ImageFormat::Png).unwrap();

    // 2. Verschluesselt im Storage ablegen (wie Tus-Upload)
    let file_id = Uuid::new_v4();
    let shard = &file_id.to_string()[..2];
    let rel_storage_path = format!("{shard}/{file_id}.enc");
    let full_storage_path = storage_dir.join(&rel_storage_path);
    storage::append_chunk(&full_storage_path, &storage_key, &orig_png_bytes).await.unwrap();

    // 3. POST /internal/thumbs/{file_id} aufrufen zur Thumbnail-Generierung
    let generate_req_body = serde_json::json!({
        "storage_path": rel_storage_path,
        "extract_exif": true
    });

    let req = Request::builder()
        .method("POST")
        .uri(format!("/internal/thumbs/{file_id}"))
        .header(header::AUTHORIZATION, format!("Bearer {service_token}"))
        .header(header::CONTENT_TYPE, "application/json")
        .body(Body::from(serde_json::to_vec(&generate_req_body).unwrap()))
        .unwrap();

    let response = app.clone().oneshot(req).await.unwrap();
    assert_eq!(response.status(), StatusCode::OK);

    let body_bytes = response.into_body().collect().await.unwrap().to_bytes();
    let gen_res: serde_json::Value = serde_json::from_slice(&body_bytes).unwrap();
    assert_eq!(gen_res["width"], 800);
    assert_eq!(gen_res["height"], 600);
    assert!(gen_res["thumbnail_path"].as_str().is_some());

    // 4. GET /internal/thumbs/{file_id} ohne Token -> 401 Unauthorized
    let unauth_req = Request::builder()
        .method("GET")
        .uri(format!("/internal/thumbs/{file_id}"))
        .body(Body::empty())
        .unwrap();
    let unauth_res = app.clone().oneshot(unauth_req).await.unwrap();
    assert_eq!(unauth_res.status(), StatusCode::UNAUTHORIZED);

    // 5. GET /internal/thumbs/{file_id} mit Token -> 200 OK + JPEG dekodierbar und max 256x256
    let auth_req = Request::builder()
        .method("GET")
        .uri(format!("/internal/thumbs/{file_id}"))
        .header(header::AUTHORIZATION, format!("Bearer {service_token}"))
        .body(Body::empty())
        .unwrap();
    let auth_res = app.clone().oneshot(auth_req).await.unwrap();
    assert_eq!(auth_res.status(), StatusCode::OK);
    assert_eq!(
        auth_res.headers().get(header::CONTENT_TYPE).unwrap(),
        "image/jpeg"
    );

    let thumb_bytes = auth_res.into_body().collect().await.unwrap().to_bytes();
    let thumb_img = image::load_from_memory(&thumb_bytes).expect("Thumbnail ist gueltiges Bild");
    assert!(thumb_img.width() <= 256);
    assert!(thumb_img.height() <= 256);
    assert_eq!(thumb_img.width(), 256);
    assert_eq!(thumb_img.height(), 192); // 800x600 skaliert auf 256x192
}

#[tokio::test]
async fn test_thumbnail_no_exif_by_default() {
    let base_temp = std::env::temp_dir().join(format!("4labs_thumb_test_{}", Uuid::new_v4()));
    let storage_dir = base_temp.join("storage");
    let thumbs_dir = base_temp.join("thumbs");
    let exports_dir = base_temp.join("exports");
    tokio::fs::create_dir_all(&storage_dir).await.unwrap();
    tokio::fs::create_dir_all(&thumbs_dir).await.unwrap();
    tokio::fs::create_dir_all(&exports_dir).await.unwrap();

    let storage_key = [0x42u8; 32];
    let service_token = "valid-service-token-secret".to_string();

    let db_url = "postgres://fake:fake@127.0.0.1:5432/fakedb";
    let db_pool = sqlx::Pool::<sqlx::Postgres>::connect_lazy(db_url).unwrap();

    let config = Config {
        upload_port: 8081,
        service_token: service_token.clone(),
        storage_key,
        storage_dir: storage_dir.clone(),
        thumbs_dir: thumbs_dir.clone(),
        exports_dir,
        database_url: db_url.to_string(),
        max_upload_size: 5368709120,
        ..Default::default()
    };

    let active_sessions = tokio::sync::Mutex::new(std::collections::HashMap::new());
    let state = Arc::new(AppState::new_for_test(
        config,
        db_pool,
        active_sessions,
    ));
    let app = create_router(state);

    let img = image::RgbImage::new(400, 400);
    let mut orig_png_bytes = Vec::new();
    let mut cursor = std::io::Cursor::new(&mut orig_png_bytes);
    img.write_to(&mut cursor, image::ImageFormat::Png).unwrap();

    let file_id = Uuid::new_v4();
    let shard = &file_id.to_string()[..2];
    let rel_storage_path = format!("{shard}/{file_id}.enc");
    let full_storage_path = storage_dir.join(&rel_storage_path);
    storage::append_chunk(&full_storage_path, &storage_key, &orig_png_bytes).await.unwrap();

    // Standardmaessig extract_exif nicht gesetzt (Art. 5 Datensparsamkeit)
    let generate_req_body = serde_json::json!({
        "storage_path": rel_storage_path
    });

    let req = Request::builder()
        .method("POST")
        .uri(format!("/internal/thumbs/{file_id}"))
        .header(header::AUTHORIZATION, format!("Bearer {service_token}"))
        .header(header::CONTENT_TYPE, "application/json")
        .body(Body::from(serde_json::to_vec(&generate_req_body).unwrap()))
        .unwrap();

    let response = app.clone().oneshot(req).await.unwrap();
    assert_eq!(response.status(), StatusCode::OK);

    let body_bytes = response.into_body().collect().await.unwrap().to_bytes();
    let gen_res: serde_json::Value = serde_json::from_slice(&body_bytes).unwrap();
    assert_eq!(gen_res["width"], 400);
    assert_eq!(gen_res["height"], 400);
    assert!(gen_res["taken_at"].is_null());
    assert!(gen_res["exif_json"].is_null());
}

#[tokio::test]
async fn thumbnail_permission_test() {
    let base_temp = std::env::temp_dir().join(format!("4labs_thumb_perm_{}", Uuid::new_v4()));
    let thumbs_dir = base_temp.join("thumbs");
    tokio::fs::create_dir_all(&thumbs_dir).await.unwrap();

    let storage_key = [0x42u8; 32];
    let file_id = Uuid::new_v4();
    let rel_path = upload_service::routes::thumbnails::get_relative_thumbnail_path(file_id);
    let full_path = thumbs_dir.join(&rel_path);

    // Verifiziert, dass Verzeichnishierarchie (/data/thumbs/xx/...) dynamisch erstellt und geschrieben werden kann
    upload_service::storage::append_chunk(&full_path, &storage_key, b"test-permission-jpeg-data")
        .await
        .expect("Thumbs-Verzeichnis muss beschreibbar sein");

    let read_back = upload_service::storage::read_and_decrypt_all(&full_path, &storage_key)
        .await
        .expect("Thumbnail muss lesbar und entschluesselbar sein");
    assert_eq!(read_back, b"test-permission-jpeg-data");

    let _ = tokio::fs::remove_dir_all(&base_temp).await;
}
