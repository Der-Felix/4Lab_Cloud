# Entwicklungshandbuch

Dieses Dokument richtet sich an Entwickler, die an 4LabCloud arbeiten. Für die reine
Installation als Anwender siehe [INSTALL.md](INSTALL.md).

---

## 1. Voraussetzungen

| Werkzeug | Version | Wofür |
|---|---|---|
| Podman + podman-compose | aktuell | Lokaler Stack (PostgreSQL, Redis, Services) |
| Node.js | 22 (laut `frontend/Dockerfile`) | Frontend, Tests |
| Go | 1.24 (laut `go.mod`) | App-Server |
| Rust | Edition 2021 | Upload-Service |

Auf macOS muss die Podman-VM laufen:

```bash
podman machine start
```

Go und Rust werden **nur** für lokale Tests und Linting gebraucht. Die Container bringen
ihre eigenen Toolchains mit.

---

## 2. Repository-Aufbau

```
frontend/              SvelteKit-Anwendung (Svelte 5, TypeScript, TailwindCSS v4)
  src/lib/             Komponenten, Stores, API-Clients
  src/routes/          Seiten; layout.css enthält die Design-Token
  tests/               Playwright-E2E-Tests
services/app-server/   Go (Gin): Auth, Metadaten, Freigaben, Quotas, Audit
services/upload-service/  Rust (Axum): Chunk-Upload, AES-256-GCM, Thumbnails, EXIF
deploy/podman/         Compose-Dateien, nginx-Konfiguration, TLS-Zertifikate
deploy/postgres/       init.sql (Neuinstallation) und migrations/ (Bestandsupdates)
scripts/               backup.sh, restore.sh
docs/                  Diese Dokumentation (zugleich Quelle für GitHub Pages)
```

---

## 3. Lokal starten

Alle Befehle in diesem Handbuch starten im **Wurzelverzeichnis des Repositorys**,
sofern nicht anders angegeben.

```bash
cp .env.example .env
podman-compose --env-file .env -f deploy/podman/podman-compose.yml up -d
```

Erreichbar unter `https://localhost:8443`. Das Entwicklungszertifikat ist selbstsigniert –
die Browserwarnung einmalig bestätigen.

Der Nominatim-Container startet bewusst **nicht** mit: Er liegt hinter dem Compose-Profil
`geocoding` und importiert beim ersten Start einen OSM-Datenauszug, was je nach Region
Stunden dauert. Ohne ihn funktioniert alles außer der Auflösung von Ortsnamen.

---

## 4. Nach Codeänderungen neu bauen

**Das ist die häufigste Stolperfalle.** `podman-compose up -d` verwendet vorhandene Images
weiter und baut nicht neu. Und `--build` baut zwar das Image, **ersetzt aber den laufenden
Container nicht** – man testet dann weiter die alte Version.

Verlässlicher Ablauf für einen einzelnen Dienst:

```bash
# aus dem Wurzelverzeichnis
podman-compose --env-file .env -f deploy/podman/podman-compose.yml build frontend
podman-compose --env-file .env -f deploy/podman/podman-compose.yml up -d --force-recreate frontend
```

Immer gegenprüfen, dass der Container auch wirklich das neue Image fährt:

```bash
podman inspect 4labs-frontend --format '{{.Image}}'
podman images --format "{{.ID}} {{.Repository}}" | grep podman_frontend
```

Stimmen die IDs nicht überein, läuft noch das alte Image.

---

## 5. Tests

### Frontend

```bash
cd frontend
npm ci
npm run check       # svelte-check (Typen)
npx vitest run      # Unit-Tests
npx playwright test # E2E
```

Die E2E-Tests brauchen **weder Backend noch Datenbank noch Zugangsdaten**: Sie mocken die
gesamte API (`page.route`) und starten über `playwright.config.ts` ihren eigenen Server
(`npm run build && npm run preview`, Port 4173). Den Podman-Stack dafür nicht starten.

Die E2E-Tests erzeugen außerdem die Screenshots unter `docs/screenshots/`. Ändert sich die
Oberfläche, ändern sich diese Dateien – beim Committen bewusst entscheiden, ob die neuen
Bilder gewollt sind.

### Go

```bash
cd services/app-server
go build ./... && go vet ./...
go test ./...
```

Datenbankabhängige Tests überspringen sich selbst, wenn kein PostgreSQL erreichbar ist
(`SKIP`, nicht `FAIL`). Um sie laufen zu lassen, auf eine Instanz zeigen:

```bash
POSTGRES_HOST=127.0.0.1 POSTGRES_PORT=5432 \
POSTGRES_USER=4labs POSTGRES_PASSWORD=... POSTGRES_DB=4labscloud \
go test ./internal/routes/
```

Laut `AGENTS.md` laufen Tests **niemals** gegen die Live-Entwicklungsdatenbank. Für
DB-Tests eine separate Datenbank oder einen Wegwerf-Container verwenden.

### Rust

```bash
cd services/upload-service
cargo check --all-targets
cargo test --lib                              # ohne Datenbank
DATABASE_URL="postgres://…" cargo test        # inkl. Integrationstests
```

---

## 6. Datenbankmigrationen

Migrationen laufen **nicht** automatisch beim Start – es gibt keinen Migrations-Runner.

- `deploy/postgres/init.sql` wird von PostgreSQL nur ausgeführt, wenn das Datenverzeichnis
  leer ist, also bei einer Neuinstallation.
- Bei einer bestehenden Datenbank muss die Migration von Hand eingespielt werden:

```bash
# .env zuerst in die Shell laden - `--env-file` von podman-compose tut das nicht
set -a && . ./.env && set +a

podman exec -i 4labs-postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  < deploy/postgres/migrations/011_exif_geocoding.sql
```

**Wichtig:** Jede Schemaänderung gehört in **beide** Dateien – `init.sql` für
Neuinstallationen und eine Migration für Bestandssysteme. Sonst unterscheiden sich frische
und aktualisierte Installationen. Migrationen idempotent schreiben
(`IF NOT EXISTS`, `ALTER … TYPE`), weil `CREATE TABLE IF NOT EXISTS` eine bereits
bestehende Tabelle nicht korrigiert.

---

## 7. Konventionen

- **Design**: Farben, Typo-Skala, Dichte- und Layoutregeln sind verbindlich in den
  Design-Regeln festgehalten. Keine Hex-Werte im Svelte-Code – ausschließlich Token-Klassen.
  Kontraste vor einer Farbänderung ausrechnen, nicht schätzen, und gegen **beide** möglichen
  Hintergründe prüfen (Karte und Seitenhintergrund).
- **Sprache**: Kommentare und Benutzertexte auf Deutsch. Quelltextkommentare ohne Umlaute
  (`fuer` statt `für`), Oberflächentexte mit.
- **Keine externen Ressourcen**: Schriften und Icons werden lokal gebündelt, keine CDNs,
  kein Tracking.
- **Fail closed**: Sicherheits- und Datenschutzentscheidungen im Zweifel restriktiv
  behandeln. Kann eine Berechtigung nicht ermittelt werden, gilt sie als nicht erteilt.

---

## 8. Häufige Stolpersteine

| Symptom | Ursache |
|---|---|
| Änderung wirkt nicht | Container läuft auf altem Image – siehe Abschnitt 4. |
| Login antwortet `429` | Rate-Limiter: 5 Versuche/Minute je IP, 10/Stunde je E-Mail. Zum Zurücksetzen die Schlüssel `rl:login:*` in Redis löschen. |
| E2E-Tests schlagen plötzlich fehl | Selektoren wie `text=dateiname.jpg` treffen mehrere Elemente (Strict Mode), wenn eine Komponente denselben Text mehrfach rendert. Selektor präzisieren, Assertion nicht abschwächen. |
| Spalte fehlt nach Update | Migration nicht eingespielt – siehe Abschnitt 6. |
| Layout bricht auf schmalen Fenstern | Flex-Kind ohne `min-w-0` schrumpft nicht unter seine Inhaltsbreite. |

---

## 9. Weiterführend

- [Architektur & Datenflüsse](ARCHITECTURE.html)
- [Compliance & Sicherheit](COMPLIANCE.html)
- [Backup & Wiederherstellung](BACKUP.html)
- [Installationsanleitung](INSTALL.html)
