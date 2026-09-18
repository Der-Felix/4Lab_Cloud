# Changelog

Alle nennenswerten Änderungen an diesem Projekt werden in dieser Datei dokumentiert.

Das Format basiert auf [Keep a Changelog](https://keepachangelog.com/de/1.1.0/),
und dieses Projekt hält sich an [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
