# Changelog

Alle nennenswerten Änderungen an diesem Projekt werden in dieser Datei dokumentiert.

Das Format basiert auf [Keep a Changelog](https://keepachangelog.com/de/1.1.0/),
und dieses Projekt hält sich an [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.4] - 2026-09-25

### Behoben
- **Geocoding-Cache Koordinatenbereich**: `geocoding_cache.lat_rounded` und `lon_rounded` in `deploy/postgres/init.sql` sowie Migration `011_exif_geocoding.sql` von `NUMERIC(6,4)` auf `NUMERIC(9,4)` erweitert, um Numeric-Overflows bei Längengraden bis ±180° zu beheben; inklusive idempotenter `ALTER COLUMN`-Statements für bereits migrierte Datenbanken.
- **Geocoding Fehler-Logging**: Zuvor verschluckte Fehler beim Einfügen in den Geocoding-Cache in `services/upload-service/src/geocoding.rs` werden nun explizit protokolliert.
- **EXIF Nenner-Null-Guard & Wertebereich**: Nenner-Null-Guard (Schutz vor `NaN`/`Infinity` durch `kamadak-exif`) sowie Koordinaten-Range-Validierung (Breitengrad [-90, 90], Längengrad [-180, 180]) in `services/upload-service/src/exif.rs` ergänzt; mit Unit-Tests abgesichert.
- **DSGVO GPS-Opt-Out Fail-Closed**: Fehler bei der Abfrage der `store_gps`-Präferenz verhalten sich nun restriktiv (Fail-Closed) statt Fail-Open; Fehler in `services/app-server/internal/routes/uploads.go` und `services/upload-service/src/routes/thumbnails.rs` werden geloggt und GPS-Metadaten im Zweifel verworfen.
- **Timeline-Tagesgruppierung**: Zeitzonen-Inkonsistenz in `frontend/src/lib/photos.ts` behoben (Tages-Schlüssel nutzte UTC, Labels lokale Zeit); Fotos werden nun konsistent anhand lokaler Datumsteile gruppiert; Regressionstest für `Europe/Berlin` hinzugefügt.
- **Lightbox Race Conditions**: Nebenläufige Ladekonflikte bei schnellem Wechseln von Fotos in `frontend/src/lib/components/Lightbox.svelte` (EXIF-Sidebar und Hauptbild) durch Request-Token-Guards behoben.
- **Fotofilter & Progressive Pagination**: Veralteter Jahr-Filter in `frontend/src/routes/photos/+page.svelte` leerte das Foto-Grid bei neueren Fotos nicht mehr still; progressives Nachladen aller Bibliotheksseiten implementiert, damit Filter die gesamte Medienbibliothek erfassen.
- **E2E-Kartenassertion**: Assertion in `frontend/tests/photos_exif_map.e2e.ts` prüft nun explizit auf gerenderte Leaflet-Marker (`toHaveCount(2)`) statt unzuverlässigem Timeout.

## [0.2.3] - 2026-09-19

### Behoben
- **Test-Isolation**: Robuste Isolation für Registrierungstests (`register_first_user_test.go`), um Flakiness bei parallelen Testläufen zu verhindern.
- **MapPreview-Widget**: Fehlerkorrekturen und optimiertes Re-Rendering der Leaflet-Minikarte auf dem Dashboard (`MapPreview.svelte`).
- **E2E-Teststabilität**: Lokale Test-Fixtures (`osm_tile.png`, `thumb.jpg`) für deterministisches Mocking im Playwright-Testlauf; Bereinigung von Port-Konflikten in `podman-compose.yml`.

## [0.2.2] - 2026-09-19

### Hinzugefügt
- **Lightbox EXIF-Seitenleiste**: Ausklappbare Seitenleiste (280px) in `Lightbox.svelte` mit detaillierten EXIF-Kameradaten (Modell, Objektiv, Blende, Verschlusszeit, ISO, Brennweite), Standortanzeige und Schnelllink zur Karte ("Auf Karte zeigen").
- **Timeline-Gruppierung**: Chronologische Gruppierung der Fotogalerie nach Monaten und Tagen mit Monats- und Tages-Headern sowie Filterleiste nach Jahr, Ort und GPS-Verfügbarkeit.
- **Karten-Integration (Leaflet)**: Interaktive Vollbild-Kartenansicht (`/photos/map`) mit OpenStreetMap-Kacheln, Leaflet-Markern und Popups für georeferenzierte Fotos; kompaktes Mini-Karten-Vorschau-Widget auf dem Dashboard (`MapPreview.svelte`).
- **Lokale Leaflet-Assets**: Marker-Icons und Schatten als SVGs lokal im Repository gebündelt ohne externe CDN-Aufrufe.

## [0.2.1] - 2026-09-19

### Hinzugefügt
- **EXIF-Metadaten-Extraktion**: Vollständiges Parsen von EXIF-Metadaten (Kameramodell, Objektiv, Blende, Verschlusszeit, ISO, Brennweite, Aufnahmedatum, GPS-Koordinaten) im Rust-Upload-Service (`kamadak-exif`).
- **Reverse-Geocoding (Nominatim)**: Asynchrones, nicht-blockierendes Reverse-Geocoding über self-hosted Nominatim (`mediagis/nominatim:4.5`) im isolierten Backend-Netzwerk mit automatischem Caching in `geocoding_cache` (gerundet auf 4 Nachkommastellen) und Rate-Limiting (1 Req/s).
- **DSGVO GPS-Opt-Out**: Benutzerindividuelle Einstellung zur Deaktivierung der GPS-Speicherung (`store_gps`-Präferenz); bei Deaktivierung werden GPS-Metadaten vor der Persistierung vollständig entfernt.
- **Datenbank & API**: Migration `011_exif_geocoding.sql` für Geodaten- und Cache-Tabellen, neue Felder in `files` und neue Go-API-Endpunkte für EXIF/Geodaten.

## [0.1.1] - 2026-09-18

### Geändert
- **Markenname**: Projektname im UI und der Dokumentation einheitlich auf **4LabCloud** korrigiert.
- **Logo & Favicon**: Vektorisiertes Netzwerk-Cloud SVG-Icon (`logo.svg`, `logo-icon.svg`, `favicon.svg`) mit Farbverlauf (#1E6FD9 -> #2DD4BF) und gestochen scharfer HTML-Wortmarke integriert; veraltete PNG-Dateien bereinigt.
- **Light Mode Kontrast**: Hintergrundfarbe auf `#F4F4F5` und Sidebar auf `#EFEFF1` geschärft; alle Karten mit 1px Rahmen und dezentem Schatten akzentuiert.
- **Layout-Optimierung**: Dashboard-Breite auf bis zu 1600px erweitert, 5 Spalten bei `xl:` und vergrößerte rechte Seitenleiste (320px).

## [0.1.0] - 2026-09-18

### Hinzugefügt
- **Architektur & Microservices**:
  - Isolierter Rust-Upload-Service (Axum) für performante Chunk-Uploads und AES-256-GCM Verschlüsselung at-rest.
  - Go App-Server (Gin) für Mandantenverwaltung, Session-Handling, Quotas und Metadaten.
  - PostgreSQL 17 Datenbank mit strikter Row Level Security (RLS) zur vollständigen Mandantenisolierung.
  - Nginx Reverse Proxy als einziger öffentlicher Endpunkt mit erzwungenem TLS 1.3 nach BSI TR-02102-2.
- **Authentifizierung & Sicherheit**:
  - Sichere JWT- und Cookie-basierte Authentifizierung mit kurzer Token-Lebensdauer (15 min).
  - Optionale Zwei-Faktor-Authentifizierung (TOTP / RFC 6238) mit verschlüsselten Secrets und Einmal-Wiederherstellungscodes.
  - Interne Service-zu-Service-Authentifizierung mit kryptografischem Bearer-Token.
- **Dateiverwaltung & Uploads**:
  - Resumable Tus-Uploads mit direktem Stream vom Browser zum Rust-Service ohne Zwischenspeicherung im App-Server.
  - Generierung und verschlüsseltes Caching von Bild-Thumbnails für Fotos und Mediendateien.
  - Dateikategorisierung, Schlagwortverwaltung (Tags) und Volltextsuche über Dateinamen.
  - Quota-Management mit individuellen und globalen Limits (`DEFAULT_QUOTA_GB`).
- **Freigaben & Teilen**:
  - Token-basierte Share-Links für Dateien mit optionalem Passwortschutz (bcrypt) und Verfallsdatum.
  - Eigene öffentliche Download- und Vorschauseite für externe Empfänger.
- **DSGVO & Compliance**:
  - Vollständiges Selbstlöschungskonzept nach Art. 17 DSGVO mit physikalischer Datenbereinigung.
  - DSGVO-Datenexport nach Art. 20 als passwortgeschütztes ZIP-Archiv mit Entschlüsselungsanleitung.
  - Revisionssicheres Audit-Logging mit HMAC-SHA256 Einweg-Pseudonymisierung von Benutzerkennungen.
  - Strikte Zero-SaaS-Richtlinie: Keine externen CDNs, Schriftarten oder Tracking-Skripte.
- **Benutzeroberfläche (Frontend)**:
  - Modernes, barrierefreies Dashboard mit SvelteKit und TailwindCSS.
  - Vollständige Integration von `@tabler/icons-svelte` (Outline- und Filled-Zustände).
  - Dynamischer Dark-/Light-Modus mit nahtlosem Farbschema.
  - Einklappbare Sidebar mit kompakter Icon-Navigation.
- **Produktion & Betrieb**:
  - Gehärtete Produktions-Compose-Konfiguration (`docker-compose.prod.yml`) mit V1-Ressourcenlimits und automatischem Neustart.
  - Vollautomatisiertes Backup-Skript (`scripts/backup.sh`) mit GFS-Retention (7 Daily, 4 Weekly, 12 Monthly).
  - Disaster-Recovery-Skript (`scripts/restore.sh`) mit definierter Reihenfolge und Datenverifikation.
  - GitHub Actions CI-Pipeline (`.github/workflows/ci.yml`) für Go-, Rust- und SvelteKit-Prüfungen.
