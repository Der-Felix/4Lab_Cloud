# 4labscloud - Agent Instructions

Du arbeitest am Open-Source-Projekt **4labscloud** - einer schlanken Cloud-Plattform
(File-Upload, Shares, User, Foto-Integration). Kein Nextcloud-Klon.

## Pflicht: Skills laden

Lade bei Bedarf die folgenden Skills (liegen in `.agents/skills/`):

| Skill | Wann laden |
|---|---|
| `4labs-architecture` | IMMER - bei jeder Code-Aenderung |
| `dsgvo-bsi-checklist` | Bei neuen Endpoints, Tabellen, Features |
| `podman-compose-macos` | Bei Containern, Compose, Netzwerk |
| `rust-axum-patterns` | Bei jedem Rust-Code |
| `go-gin-conventions` | Bei jedem Go-Code |
| `sveltekit-uppy` | Bei jedem Frontend-Code |

## Tech-Stack (verbindlich)

- **Nginx** - einziger oeffentlicher Endpunkt (TLS 1.3)
- **Go** (Gin) - App-Server: User, Shares, Auth, Metadaten
- **Rust** (Axum) - Upload/Storage-Service: Dateien, AES-256-GCM
- **PostgreSQL** - Metadaten, Row Level Security
- **Redis** - Sessions, Cache, Queue
- **SvelteKit + Uppy** - Frontend
- **Podman** (NICHT Docker)

## Kommunikation

- Alle Services reden per **REST/JSON** (kein gRPC, kein Protobuf)
- Rust ist **niemals** oeffentlich erreichbar
- Upload laeuft ueber Presigned URLs direkt zum Rust-Service
- Go <-> Rust mit Service-Token (Authorization: Bearer)

## Projektstruktur

4Lab_Cloud/
  services/app-server/       (Go)
  services/upload-service/   (Rust)
  frontend/                  (SvelteKit)
  deploy/podman/             (podman-compose.yml, Nginx-Config)
  deploy/postgres/           (Init-SQL, RLS-Policies)
  docs/                      (Architektur, Compliance)
  .agents/skills/            (Skills)

## Arbeitsumgebung

- Hardware: MacBook Pro M5 (Apple Silicon, arm64)
- Container: Podman, kein Docker Desktop
- Workspace: /Users/felix/Documents/4Lab_Cloud

## Compliance (nicht verhandelbar)

- DSGVO: Art. 5 (Datensparsamkeit), Art. 17 (Loeschung), Art. 32 (TOM)
- BSI: TLS 1.3 nach TR-02102-2, AES-256-GCM at rest
- MFA ist OPTIONAL. Kein Zwang, nie. 4labscloud ist Self-Hosted.
- Audit-Logs fuer alle Aktionen
- Keine externen CDNs, kein Tracking, kein SaaS
- EU/EWR-Hosting only

## Regeln fuer den Agenten

1. **Erst Skill laden, dann Code schreiben.** Kein Blindflug.
2. **Keine Platzhalter.** Code muss lauffaehig sein.
3. **Kommentare auf Deutsch** - kurze, klare Saetze.
4. **Kein Code ohne Auth-Pruefung** an oeffentlichen Endpoints.
5. **Kein Code ohne Audit-Log** bei schreibenden Aktionen.
6. **Bei Unsicherheit fragen** - nicht raten.
7. **Podman statt Docker** in allen Befehlen und Configs.
8. **MFA ist OPTIONAL. Kein Zwang, nie. 4labscloud ist Self-Hosted.**
9. **localStorage verboten ausser UI-Preferences.** Keine Tokens oder sensiblen Daten im localStorage.

## Lizenz

Apache 2.0

## Ausgabe-Regeln (verbindlich)

1. Kein Code im Chat ausgeben – Dateien direkt ins Repo schreiben.
2. Antwort-Format:
   - Plan (max. 10 Bulletpoints)
   - Liste der erstellten/geaenderten Dateien (Pfade)
   - Kurze Erklaerung pro Datei (1 Zeile)
   - Test-Befehle
   - Offene Punkte
3. Max. 50 Zeilen Antwort.
4. Nur auf explizite Aufforderung Code zeigen.

## Etappen-Ablauf

Jede Etappe folgt exakt diesem Muster:

1. Plan vorschlagen (max. 10 Punkte)
2. Warten auf Freigabe
3. Dateien ins Repo schreiben
4. Kurzbericht: Was wurde gemacht, welche Tests
5. Tests ausfuehren (cargo test / go test / podman-compose)
6. Warten auf Freigabe fuer naechste Etappe

NIE: Code im Chat ausgeben.
IMMER: Dateien direkt schreiben.

## Test-Umgebung (verbindlich)

- Test-User: felix@4labs.local
- Passwort: (siehe lokale .env oder vom Betreiber)
- Nach jedem Test-Run: DB NICHT truncaten, User bleibt bestehen
- E2E-Tests MUESSEN ihre Test-User selbst aufraeumen (defer DELETE)
- felix@4labs.local ist der Admin-Account, NIEMALS loeschen
