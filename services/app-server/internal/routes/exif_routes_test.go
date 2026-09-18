package routes_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/4labscloud/4labscloud/services/app-server/internal/auth"
	"github.com/4labscloud/4labscloud/services/app-server/internal/db"
	"github.com/4labscloud/4labscloud/services/app-server/internal/routes"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestExifAPI_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := routes.NewFilesHandler(nil, "http://127.0.0.1:9999", "secret", nil)
	r.GET("/api/v1/files/:id/exif", auth.Middleware("jwt-secret"), handler.GetFileExif)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/files/00000000-0000-0000-0000-000000000001/exif", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("erwartet HTTP 401 ohne JWT, erhalten %d", w.Code)
	}
}

func TestPhotosMap_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := routes.NewFilesHandler(nil, "http://127.0.0.1:9999", "secret", nil)
	r.GET("/api/v1/photos/map", auth.Middleware("jwt-secret"), handler.GetPhotosMap)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/photos/map", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("erwartet HTTP 401 ohne JWT, erhalten %d", w.Code)
	}
}

func TestExifAndMap_RLS_Lifecycle(t *testing.T) {
	pool := getRoutesTestDB(t)
	if pool == nil {
		t.Skip("Postgres nicht verfuegbar")
		return
	}
	defer pool.Close()

	ctx := context.Background()
	jwtSecret := "exif-map-test-secret-123"

	// 1. Zwei Test-Benutzer anlegen fuer RLS-Test
	userA := uuid.New()
	emailA := fmt.Sprintf("exif_a_%d@test.internal", time.Now().UnixNano())
	pwHash, _ := auth.HashPassword("SicheresPasswort123!")
	_, err := pool.Exec(ctx, "INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)", userA, emailA, pwHash)
	if err != nil {
		t.Fatalf("User A konnte nicht angelegt werden: %v", err)
	}
	defer pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userA)

	userB := uuid.New()
	emailB := fmt.Sprintf("exif_b_%d@test.internal", time.Now().UnixNano())
	_, err = pool.Exec(ctx, "INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)", userB, emailB, pwHash)
	if err != nil {
		t.Fatalf("User B konnte nicht angelegt werden: %v", err)
	}
	defer pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userB)

	// 2. Datei fuer User A mit EXIF und GPS Koordinaten anlegen
	fileID_A := uuid.New()
	exifData := `{"make":"Apple","model":"iPhone 15 Pro","f_number":"f/1.8","iso":100}`
	locationAddr := `{"city":"Zuerich","country":"Schweiz"}`
	latA := 47.3769
	lonA := 8.5417
	locNameA := "Zuerich, Schweiz"

	err = db.WithUserRLS(ctx, pool, userA, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO files (id, user_id, filename, size_bytes, mime_type, checksum_sha256, storage_path, exif_json, gps_lat, gps_lon, location_name, location_address)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
			fileID_A, userA, "photo_zurich.jpg", 1024, "image/jpeg", "sha256_mock_a", "storage/a.enc",
			exifData, latA, lonA, locNameA, locationAddr,
		)
		return err
	})
	if err != nil {
		t.Fatalf("Datei fuer User A konnte nicht angelegt werden: %v", err)
	}

	// 3. Datei fuer User B anlegen (mit anderem Standort)
	fileID_B := uuid.New()
	latB := 52.5200
	lonB := 13.4050
	locNameB := "Berlin, Deutschland"
	err = db.WithUserRLS(ctx, pool, userB, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO files (id, user_id, filename, size_bytes, mime_type, checksum_sha256, storage_path, gps_lat, gps_lon, location_name)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			fileID_B, userB, "photo_berlin.jpg", 2048, "image/jpeg", "sha256_mock_b", "storage/b.enc",
			latB, lonB, locNameB,
		)
		return err
	})
	if err != nil {
		t.Fatalf("Datei fuer User B konnte nicht angelegt werden: %v", err)
	}

	// 4. Gin-Router aufsetzen
	gin.SetMode(gin.TestMode)
	r := gin.New()
	filesHandler := routes.NewFilesHandler(pool, "http://127.0.0.1:9999", "secret", nil)
	userHandler := routes.NewUserHandler(pool, nil, nil, "http://127.0.0.1:9999", "secret", []byte("12345678901234567890123456789012"), []byte("12345678901234567890123456789012"), 50)

	v1 := r.Group("/api/v1")
	v1.Use(auth.Middleware(jwtSecret))
	{
		v1.GET("/files/:id/exif", filesHandler.GetFileExif)
		v1.GET("/photos/map", filesHandler.GetPhotosMap)
		v1.GET("/users/me/preferences", userHandler.GetPreferences)
		v1.PATCH("/users/me/preferences", userHandler.UpdatePreferences)
	}

	tokenA, _ := auth.GenerateToken(userA, jwtSecret, 15*time.Minute)
	tokenB, _ := auth.GenerateToken(userB, jwtSecret, 15*time.Minute)

	// Test 1: User A ruft eigene EXIF-Daten ab -> 200 OK
	{
		req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/files/%s/exif", fileID_A), nil)
		req.Header.Set("Authorization", "Bearer "+tokenA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("User A sollte eigene EXIF-Daten abrufen koennen: erwartet 200, erhalten %d", w.Code)
		}

		var exifResp routes.ExifResponse
		if err := json.Unmarshal(w.Body.Bytes(), &exifResp); err != nil {
			t.Fatalf("ungueltiges JSON bei EXIF-Antwort: %v", err)
		}
		if exifResp.LocationName == nil || *exifResp.LocationName != locNameA {
			t.Fatalf("erwartet LocationName %q, erhalten %v", locNameA, exifResp.LocationName)
		}
		if exifResp.GPSLat == nil || (*exifResp.GPSLat-latA) > 0.0001 {
			t.Fatalf("erwartet GPSLat %f, erhalten %v", latA, exifResp.GPSLat)
		}
	}

	// Test 2: RLS-Schutz: User B versucht EXIF von User A abzurufen -> 404 Not Found
	{
		req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/files/%s/exif", fileID_A), nil)
		req.Header.Set("Authorization", "Bearer "+tokenB)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("RLS-Verletzung: User B darf EXIF von User A nicht sehen: erwartet 404, erhalten %d, body: %s", w.Code, w.Body.String())
		}
	}

	// Test 3: Photos Map fuer User A -> enthaelt genau Zuerich, NICHT Berlin
	{
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/photos/map", nil)
		req.Header.Set("Authorization", "Bearer "+tokenA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("User A sollte Photos Map abrufen koennen: erwartet 200, erhalten %d", w.Code)
		}

		var mapItems []routes.PhotoMapItem
		if err := json.Unmarshal(w.Body.Bytes(), &mapItems); err != nil {
			t.Fatalf("ungueltiges JSON bei Photos Map Antwort: %v", err)
		}
		if len(mapItems) != 1 {
			t.Fatalf("erwartet 1 Karten-Foto fuer User A, erhalten %d", len(mapItems))
		}
		if mapItems[0].ID != fileID_A {
			t.Fatalf("erwartet Foto ID %s fuer User A, erhalten %s", fileID_A, mapItems[0].ID)
		}
		if mapItems[0].LocationName == nil || *mapItems[0].LocationName != locNameA {
			t.Fatalf("erwartet Standort %q, erhalten %v", locNameA, mapItems[0].LocationName)
		}
	}

	// Test 4: Preferences API fuer User A (Opt-out Toggle)
	{
		// Default auslesen -> true
		reqGet, _ := http.NewRequest(http.MethodGet, "/api/v1/users/me/preferences", nil)
		reqGet.Header.Set("Authorization", "Bearer "+tokenA)
		wGet := httptest.NewRecorder()
		r.ServeHTTP(wGet, reqGet)
		if wGet.Code != http.StatusOK {
			t.Fatalf("Preferences GET erwartet 200, erhalten %d", wGet.Code)
		}
		var prefResp routes.UserPreferencesResponse
		json.Unmarshal(wGet.Body.Bytes(), &prefResp)
		if !prefResp.StoreGPS {
			t.Fatalf("Default store_gps sollte true sein")
		}

		// Opt-out auf false setzen
		patchBody := []byte(`{"store_gps": false}`)
		reqPatch, _ := http.NewRequest(http.MethodPatch, "/api/v1/users/me/preferences", bytes.NewReader(patchBody))
		reqPatch.Header.Set("Authorization", "Bearer "+tokenA)
		reqPatch.Header.Set("Content-Type", "application/json")
		wPatch := httptest.NewRecorder()
		r.ServeHTTP(wPatch, reqPatch)
		if wPatch.Code != http.StatusOK {
			t.Fatalf("Preferences PATCH erwartet 200, erhalten %d: %s", wPatch.Code, wPatch.Body.String())
		}

		// DB direkt pruefen
		var dbStoreGPS bool
		err = pool.QueryRow(ctx, "SELECT store_gps FROM users WHERE id = $1", userA).Scan(&dbStoreGPS)
		if err != nil || dbStoreGPS != false {
			t.Fatalf("DB sollte store_gps=false enthalten, erhalten: %v, err: %v", dbStoreGPS, err)
		}
	}
}
