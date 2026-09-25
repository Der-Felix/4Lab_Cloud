# 4LabCloud

4LabCloud ist eine schlanke, souveräne Open-Source-Cloud-Plattform für verschlüsselten Dateiupload, Freigaben und Medienverwaltung. Die Plattform wurde ohne SaaS-Abhängigkeiten oder externe Tracking-Dienste speziell für maximale Performance, intuitive Bedienung und strikte europäische Datenschutzstandards konzipiert.

[![Dokumentation](https://img.shields.io/badge/docs-GitHub%20Pages-blue)](https://der-felix.github.io/4Lab_Cloud/)
[![Lizenz](https://img.shields.io/badge/Lizenz-Apache%202.0-green.svg)](LICENSE)

![4LabCloud Dashboard](docs/screenshots/dashboard-dark.png)

---

## Highlights & Benutzeroberfläche

- **Performantes Dashboard & Dateimanager**: Moderne Oberfläche mit SvelteKit, Dark-/Light-Modus und schnellem Chunk-Upload via Uppy und Tus.
- **Medienverwaltung mit Timeline & EXIF-Sidebar**: Automatische Extraktion technischer Kameradaten (Modell, Objektiv, Blende, Belichtung, ISO, Brennweite) und Gruppierung nach Monaten und Tagen.
- **Integrierte Fotokarte**: Lokales Reverse-Geocoding via Nominatim und Visualisierung aller GPS-Fotos mit interaktiven Leaflet-/OpenStreetMap-Karten.
- **DSGVO & Compliance Out-of-the-Box**: Selbstlöschung nach Art. 17 DSGVO, verschlüsselter ZIP-Export nach Art. 20 DSGVO, Audit-Logs mit HMAC-Pseudonymisierung und striktes TLS 1.3 nach BSI TR-02102-2.

| Chronologische Foto-Timeline | EXIF-Metadaten & Kartenlink |
|---|---|
| ![Foto-Timeline](docs/screenshots/timeline-month.png) | ![EXIF-Sidebar](docs/screenshots/lightbox-exif.png) |

| Dashboard Minikarten-Vorschau | DSGVO-Self-Service & Export |
|---|---|
| ![Fotokarte-Widget](docs/screenshots/dashboard-map-preview.png) | ![DSGVO Self-Service](docs/screenshots/settings-dsgvo.png) |

---

## Quick Start (Entwicklung)

```bash
cp .env.example .env
podman machine start
cd deploy/podman && podman-compose --env-file ../../.env up -d
# Frontend aufrufen: https://localhost:8443 (Zertifikatswarnung akzeptieren)
#
# Hinweis: Der Nominatim-Container startet bewusst NICHT mit. Er liegt hinter dem
# Compose-Profil "geocoding", weil er beim ersten Start einen OSM-Datenauszug
# importiert. Ohne ihn funktioniert alles ausser der Ortsnamen-Aufloesung.
```

---

## Architektur-Übersicht

```mermaid
flowchart TD
    Client["Client / Web-Browser"]
    Nginx["Nginx Reverse Proxy & TLS 1.3 Gateway\n(Port 80 / 443)"]
    Frontend["Frontend Service\n(SvelteKit / Node.js)"]
    AppServer["App-Server\n(Go / Gin)"]
    UploadService["Upload- / Storage-Service\n(Rust / Axum)"]
    DB[("PostgreSQL 17\n(RLS & Audit-Log)")]
    Cache[("Redis 7\n(Sessions & Quota)")]
    Storage[("Verschlüsselter Storage\n(AES-256-GCM Volume)")]

    Client -->|HTTPS TLS 1.3| Nginx
    Nginx -->|UI Requests| Frontend
    Nginx -->|API Requests| AppServer
    Nginx -->|Presigned Tus Chunks| UploadService
    AppServer -->|REST + Bearer Token| UploadService
    AppServer -->|SQL + RLS| DB
    AppServer -->|Cache / Sessions| Cache
    UploadService --> Storage
    UploadService -.->|Status & Metadaten| DB
```

---

## Produktion-Setup (Schritt für Schritt)

### 1. DNS & Server-Voraussetzungen
- Einen Linux-Server mit installiertem `podman` und `podman-compose` bereitstellen.
- DNS-A/AAAA-Record für Ihre Domain konfigurieren (z. B. `cloud.example.com`).
- Ports `80` und `443` in der Firewall freigeben.

### 2. TLS-Zertifikate via Certbot beziehen
```bash
sudo certbot certonly --standalone -d cloud.example.com
# Zertifikate liegen nun unter /etc/letsencrypt/live/cloud.example.com/
```

### 3. Produktions-Umgebung konfigurieren
```bash
cp .env.prod.example .env
# WICHTIG: Alle Passwörter und Schlüssel mit openssl generieren!
nano .env
```
In `deploy/podman/prod/nginx-prod.conf` den Domain-Namen im Zertifikatspfad (`ssl_certificate`) anpassen.

### 4. Container im Produktionsmodus starten
```bash
cd deploy/podman/prod
podman-compose --env-file ../../../.env -f docker-compose.prod.yml up -d
```

### 5. Status überprüfen
```bash
podman ps
podman logs -f 4labs-prod-nginx
```

---

## Backup & Restore Workflow

4LabCloud enthält vollautomatisierte Skripte zur Datensicherung und Wiederherstellung:

### Automatisiertes Backup
```bash
./scripts/backup.sh
```
- Erstellt einen konsistenten Live-Dump der PostgreSQL-Datenbank (`database.sql.gz`).
- Archiviert das verschlüsselte Storage-Volume als `tar.gz`.
- Rotiert Backups nach dem Grandfather-Father-Son-Prinzip (7 Daily, 4 Weekly, 12 Monthly).
- Protokolliert alle Aktionen in `backup.log`.

### Notfall-Wiederherstellung (Disaster Recovery)
```bash
./scripts/restore.sh ./backups/2026-09-18_120000
```
- Fragt vor Überschreiben nach Bestätigung (oder `--force` für CI/Automation).
- Stoppt alle Services, stellt PostgreSQL wieder her, restauriert den Datei-Storage und startet den App-Server zuletzt für saubere Schema-Migrationen.
- Verifiziert den Datenstand durch Zählung der aktiven Benutzerkonten.

---

## Umgebungsvariablen

| Variable | Standardwert | Beschreibung |
|---|---|---|
| `DOMAIN` | `cloud.example.com` | Öffentliche Domain für TLS-Zertifikate |
| `POSTGRES_USER` | `4labs` | Datenbank-Benutzername |
| `POSTGRES_PASSWORD` | *(Zufall)* | Sicheres Kennwort für PostgreSQL |
| `POSTGRES_DB` | `4labscloud` | Name der Produktions-Datenbank |
| `POSTGRES_HOST` | `postgres` | Hostname des PostgreSQL-Containers |
| `POSTGRES_PORT` | `5432` | Interner Port der PostgreSQL-Instanz |
| `REDIS_PASSWORD` | *(Zufall)* | Authentifizierungs-Passwort für Redis |
| `REDIS_HOST` | `redis` | Hostname des Redis-Containers |
| `REDIS_PORT` | `6379` | Interner Port der Redis-Instanz |
| `SERVICE_TOKEN` | *(32-byte hex)* | Interner Shared Secret zwischen Go und Rust |
| `STORAGE_KEY` | *(32-byte base64)*| Master-Key für AES-256-GCM Datei-Verschlüsselung |
| `MFA_KEY` | *(32-byte base64)*| AES-256-GCM Schlüssel für gespeicherte TOTP-Secrets |
| `JWT_SECRET` | *(32-byte hex)* | Signaturschlüssel für Web-Session-Tokens |
| `JWT_TTL_MINUTES` | `15` | Lebensdauer des Access-Tokens (Standard: 15 Min) |
| `AUDIT_HMAC_KEY` | *(32-byte base64)*| HMAC-Key für DSGVO-Pseudonymisierung in Audit-Logs |
| `AUDIT_RETENTION_DAYS` | `90` | Aufbewahrungsfrist für Audit-Logs in Tagen |
| `DEFAULT_QUOTA_GB` | `50` | Standard-Speicherplatzkontingent pro Benutzer in GB |
| `APP_PORT` | `8080` | Interner Port des Go-App-Servers |
| `UPLOAD_PORT` | `8081` | Interner Port des Rust-Storage-Services |
| `GEOCODING_ENABLED` | `false` | Reverse-Geocoding via Nominatim aktivieren. Standardmäßig aus – ohne laufenden Nominatim-Dienst bleibt `location_name` leer. |
| `NOMINATIM_URL` | `http://nominatim:8080` | Adresse der self-hosted Nominatim-Instanz (nur bei aktiviertem Geocoding) |
| `NOMINATIM_PBF_URL` | *(Monaco)* | OSM-Datenauszug, den der Nominatim-Container importiert. Nur in `.env.prod.example`. **Achtung:** Der Import läuft je nach Region Stunden bis Tage. |

---

## Security & Compliance

- **DSGVO-Konformität**: 
  - Datenminimierung nach Art. 5 (keine IP-Adressen im Web-Log).
  - Recht auf Löschung nach Art. 17 (echtes physikalisches Shreddern von Dateien).
  - Recht auf Datenübertragbarkeit nach Art. 20 (passwortgeschützter ZIP-Export).
  - Technische und organisatorische Maßnahmen (TOM) nach Art. 32.
- **BSI TR-02102-2**: 
  - Ausschließlich TLS 1.3 mit Forward Secrecy an allen externen Schnittstellen.
  - Zero-Trust internes Netzwerk: Der Rust-Storage-Service ist von außen unerreichbar.
- **Verschlüsselung**: 
  - AES-256-GCM mit eindeutiger Nonce pro Datei at-rest.
  - Authentifizierte Service-Kommunikation via Bearer-Token.
  - PostgreSQL Row Level Security (RLS) zur strikten Mandantentrennung auf Datenbankebene.
- **MFA**: TOTP-Zwei-Faktor-Authentifizierung (optional aktivierbar, kein Zwang).
- **Keine Cloud-Abhängigkeiten**: 100% self-hosted, keine CDNs, kein Tracking, keine externen Schriftarten.

---

## Fehlerbehebung

| Symptom | Ursache | Lösung |
|---|---|---|
| Browser warnt „Nicht sicher" | Im Entwicklungsmodus liegt ein selbstsigniertes Zertifikat unter `deploy/podman/certs/`. | Warnung einmalig bestätigen. Für Produktion Zertifikate via Certbot beziehen (siehe oben). |
| Code-Änderung wirkt nicht | `podman-compose up -d` verwendet vorhandene Images weiter und baut **nicht** neu. | `podman-compose --env-file ../../.env build <dienst>` – und danach `down` + `up -d`. Ein `--build` allein baut zwar das Image, ersetzt aber den **laufenden Container nicht**. Prüfen mit: `podman inspect 4labs-frontend --format '{{.Image}}'` gegen `podman images`. |
| Login antwortet mit `429` | Rate-Limiter: 5 Versuche pro Minute und IP, 10 pro Stunde und E-Mail-Adresse. | Warten, oder im Entwicklungsmodus die Zähler leeren: `podman exec -i 4labs-redis redis-cli -a "$REDIS_PASSWORD" --no-auth-warning KEYS "rl:login:*"` und die Schlüssel löschen. |
| Neue Spalten fehlen nach einem Update | Migrationen werden **nicht** automatisch beim Start angewendet. `init.sql` läuft nur bei einer leeren Datenbank. | Zuerst die Konfiguration in die Shell laden (`--env-file` von podman-compose tut das nicht): `set -a && . ./.env && set +a`, dann `podman exec -i 4labs-postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" < deploy/postgres/migrations/<datei>.sql` (produktiv: `4labs-prod-postgres`) |
| Fotos haben keinen Ortsnamen | `GEOCODING_ENABLED=false` (Standard) oder Nominatim läuft nicht. | Variable setzen und den Dienst mit dem Profil starten: `podman-compose --profile geocoding up -d`. Rechnen Sie mit langer Importdauer. |
| Karte bleibt leer | Es existieren keine Fotos mit GPS-Koordinaten, oder der Nutzer hat die GPS-Speicherung deaktiviert. | GPS-Einstellung unter *Einstellungen → Datenschutz* prüfen. Bereits ohne GPS gespeicherte Fotos lassen sich nicht nachträglich verorten. |

---

## Weiterführende Dokumentation

Die vollständige Online-Dokumentation ist über **[GitHub Pages](https://der-felix.github.io/4Lab_Cloud/)** verfügbar:

- [docs/index.md](docs/index.md) – Zentrale Doku-Übersicht & Benutzeroberfläche
- [docs/INSTALL.md](docs/INSTALL.md) – Installationsanleitung für den Eigenbetrieb (TLS, Geheimnisse, Updates)
- [docs/DESIGN.md](docs/DESIGN.md) – Verbindliches Design-System: Farbtoken, Kontraste, Typo-Skala, Layoutregeln
- [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) – Entwicklungshandbuch: Aufbau, Tests, Container-Rebuild, Konventionen
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) – Detaillierte Architektur, Datenflüsse und Komponenten
- [docs/COMPLIANCE.md](docs/COMPLIANCE.md) – DSGVO-Artikel-Mapping, BSI-Konformität und Audit-Konzept
- [docs/BACKUP.md](docs/BACKUP.md) – 3-2-1 Backup-Strategie und Disaster Recovery Handbuch

---

## Lizenz

Dieses Projekt ist lizenziert unter der [Apache-2.0-Lizenz](LICENSE).
Copyright 2026 4LabCloud Contributors.
