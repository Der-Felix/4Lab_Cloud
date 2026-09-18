use axum::{
    body::Body,
    http::{Request, StatusCode},
};
use http_body_util::BodyExt;
use serde_json::json;
use std::{io::Read, path::PathBuf, sync::Arc};
use tower::ServiceExt;
use upload_service::{create_router, crypto, db, storage, AppState, Config};
use uuid::Uuid;

async fn setup_app() -> (axum::Router, PathBuf, String, [u8; 32]) {
    let temp_dir = std::env::temp_dir().join(format!("4labs_export_test_{}", Uuid::new_v4()));
    let storage_dir = temp_dir.join("storage");
    let exports_dir = temp_dir.join("exports");
    tokio::fs::create_dir_all(&storage_dir).await.unwrap();
    tokio::fs::create_dir_all(&exports_dir).await.unwrap();

    let db_url = "postgres://4labs:password@localhost:5432/nonexistent".to_string();
    let db_pool = match db::init_pool(&db_url).await {
        Ok(pool) => pool,
        Err(_) => {
            sqlx::Pool::<sqlx::Postgres>::connect_lazy(&db_url).unwrap()
        }
    };

    let storage_key = [0x88u8; 32];
    let service_token = "export-test-token-12345".to_string();
    let config = Config {
        upload_port: 8081,
        service_token: service_token.clone(),
        storage_key,
        storage_dir: storage_dir.clone(),
        thumbs_dir: storage_dir.join("thumbs"),
        exports_dir,
        database_url: db_url,
        max_upload_size: 5368709120,
        ..Default::default()
    };

    let active_sessions = tokio::sync::Mutex::new(std::collections::HashMap::new());
    let state = Arc::new(AppState::new_for_test(
        config,
        db_pool,
        active_sessions,
    ));
    (create_router(state), temp_dir, service_token, storage_key)
}

#[tokio::test]
async fn test_export_lifecycle_and_delete() {
    let (app, temp_dir, token, storage_key) = setup_app().await;

    let user_id = Uuid::new_v4();
    let job_id = Uuid::new_v4();

    // 1. Eine Test-Datei via append_chunk im storage_dir anlegen
    let storage_dir = temp_dir.join("storage");
    let test_file_rel = "sub/testfile.enc";
    let test_file_full = storage_dir.join(test_file_rel);
    storage::append_chunk(&test_file_full, &storage_key, b"verschluesselte-daten-12345").await.unwrap();

    // 2. Export anfordern
    let payload = json!({
        "user_id": user_id,
        "job_id": job_id,
        "password": "TestPassword123!",
        "key_salt": "0102030405060708090a0b0c0d0e0f10",
        "profile": { "email": "test@4labs.internal", "is_admin": false },
        "shares": [],
        "files": [
            { "filename": "mein_foto.jpg", "storage_path": test_file_rel }
        ]
    });

    let req = Request::builder()
        .method("POST")
        .uri("/internal/exports")
        .header("Authorization", format!("Bearer {token}"))
        .header("Content-Type", "application/json")
        .body(Body::from(serde_json::to_vec(&payload).unwrap()))
        .unwrap();

    let resp = app.clone().oneshot(req).await.unwrap();
    assert_eq!(resp.status(), StatusCode::OK);

    let body_bytes = resp.into_body().collect().await.unwrap().to_bytes();
    let resp_json: serde_json::Value = serde_json::from_slice(&body_bytes).unwrap();
    assert!(resp_json["size_bytes"].as_u64().unwrap() > 0);

    // 3. Export herunterladen
    let req_dl = Request::builder()
        .method("GET")
        .uri(format!("/internal/exports/{job_id}/download"))
        .header("Authorization", format!("Bearer {token}"))
        .body(Body::empty())
        .unwrap();

    let resp_dl = app.clone().oneshot(req_dl).await.unwrap();
    assert_eq!(resp_dl.status(), StatusCode::OK);
    assert_eq!(resp_dl.headers().get("content-type").unwrap(), "application/zip");

    // 4. Export loeschen
    let req_del = Request::builder()
        .method("DELETE")
        .uri(format!("/internal/exports/{job_id}"))
        .header("Authorization", format!("Bearer {token}"))
        .body(Body::empty())
        .unwrap();

    let resp_del = app.clone().oneshot(req_del).await.unwrap();
    assert_eq!(resp_del.status(), StatusCode::NO_CONTENT);

    // 5. Speicherdatei physisch loeschen
    let req_del_file = Request::builder()
        .method("DELETE")
        .uri(format!("/internal/files/{test_file_rel}"))
        .header("Authorization", format!("Bearer {token}"))
        .body(Body::empty())
        .unwrap();

    let resp_del_file = app.oneshot(req_del_file).await.unwrap();
    assert_eq!(resp_del_file.status(), StatusCode::NO_CONTENT);
    assert!(!test_file_full.exists(), "datei muss physisch geloescht sein");
}

#[tokio::test]
async fn test_export_encryption_and_wrong_password_fails() {
    let (app, temp_dir, token, storage_key) = setup_app().await;

    let user_id = Uuid::new_v4();
    let job_id = Uuid::new_v4();

    // 1. Datei anlegen
    let storage_dir = temp_dir.join("storage");
    let test_file_rel = "docs/geheim.enc";
    let test_file_full = storage_dir.join(test_file_rel);
    storage::append_chunk(&test_file_full, &storage_key, b"Streng vertrauliche Kundendaten").await.unwrap();

    let password = "SuperSecretPassword2026!";
    let salt_hex = "a1b2c3d4e5f60718293a4b5c6d7e8f90";
    let salt_bytes = hex::decode(salt_hex).unwrap();

    // 2. Export erstellen
    let payload = json!({
        "user_id": user_id,
        "job_id": job_id,
        "password": password,
        "key_salt": salt_hex,
        "profile": { "email": "kunde@4labs.example", "is_admin": false },
        "shares": [],
        "files": [
            { "filename": "geheim.txt", "storage_path": test_file_rel }
        ]
    });

    let req = Request::builder()
        .method("POST")
        .uri("/internal/exports")
        .header("Authorization", format!("Bearer {token}"))
        .header("Content-Type", "application/json")
        .body(Body::from(serde_json::to_vec(&payload).unwrap()))
        .unwrap();

    let resp = app.clone().oneshot(req).await.unwrap();
    assert_eq!(resp.status(), StatusCode::OK);

    // 3. Download abrufen
    let req_dl = Request::builder()
        .method("GET")
        .uri(format!("/internal/exports/{job_id}/download"))
        .header("Authorization", format!("Bearer {token}"))
        .body(Body::empty())
        .unwrap();

    let resp_dl = app.oneshot(req_dl).await.unwrap();
    assert_eq!(resp_dl.status(), StatusCode::OK);

    let zip_bytes = resp_dl.into_body().collect().await.unwrap().to_bytes();

    // 4. Download-ZIP entpacken und pruefen: Enthaelt export.zip.age und HOW_TO_DECRYPT.txt
    let cursor = std::io::Cursor::new(zip_bytes);
    let mut zip_archive = zip::ZipArchive::new(cursor).unwrap();

    let mut found_age = false;
    let mut found_readme = false;
    let mut age_bytes = Vec::new();
    let mut readme_text = String::new();

    for i in 0..zip_archive.len() {
        let mut file = zip_archive.by_index(i).unwrap();
        if file.name() == "export.zip.age" {
            found_age = true;
            file.read_to_end(&mut age_bytes).unwrap();
        } else if file.name() == "HOW_TO_DECRYPT.txt" {
            found_readme = true;
            file.read_to_string(&mut readme_text).unwrap();
        }
    }

    assert!(found_age, "export.zip.age muss im Download-ZIP enthalten sein");
    assert!(found_readme, "HOW_TO_DECRYPT.txt muss im Download-ZIP enthalten sein");
    assert!(readme_text.contains(salt_hex), "Anleitung muss das Salt enthalten");
    assert!(readme_text.contains("Argon2id"), "Anleitung muss Argon2id erwaehnen");

    // 5. Entschluesselung mit KORREKTEM Passwort -> muss gelingen
    let correct_key = crypto::derive_argon2id_key(password, &salt_bytes).unwrap();
    let decrypted_zip_bytes = crypto::decrypt(&correct_key, &age_bytes)
        .expect("Entschluesselung mit korrektem Passwort muss gelingen");

    // Inneres ZIP inspizieren
    let inner_cursor = std::io::Cursor::new(decrypted_zip_bytes);
    let mut inner_zip = zip::ZipArchive::new(inner_cursor).unwrap();
    let mut found_profile = false;
    let mut found_file = false;

    for i in 0..inner_zip.len() {
        let mut file = inner_zip.by_index(i).unwrap();
        if file.name() == "profil.json" {
            found_profile = true;
        } else if file.name() == "files/geheim.txt" {
            found_file = true;
            let mut file_content = String::new();
            file.read_to_string(&mut file_content).unwrap();
            assert_eq!(file_content, "Streng vertrauliche Kundendaten");
        }
    }
    assert!(found_profile, "profil.json muss im entschluesselten ZIP vorhanden sein");
    assert!(found_file, "files/geheim.txt muss im entschluesselten ZIP vorhanden sein");

    // 6. Entschluesselung mit FALSCHEM Passwort -> MUSS fehlschlagen
    let wrong_key = crypto::derive_argon2id_key("FalschesPasswort123!", &salt_bytes).unwrap();
    let decrypt_res = crypto::decrypt(&wrong_key, &age_bytes);
    assert!(decrypt_res.is_err(), "Entschluesselung mit falschem Passwort muss scheitern");
}
