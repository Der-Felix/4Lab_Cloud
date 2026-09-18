#!/usr/bin/env bash
# ==============================================================================
# 4labscloud - Disaster Recovery / Restore-Skript
# ==============================================================================
# Reihenfolge gemaess Spezifikation:
# 1. Argument & Bestaetigung pruefen ("ACHTUNG: Ueberschreibt alle Daten!")
# 2. Alle Container stoppen
# 3. Postgres zuerst wiederherstellen
# 4. Storage danach wiederherstellen
# 5. Hilfsdienste starten & app-server ZULETZT starten (fuer Migrationen!)
# 6. Verifikation: SELECT count(*) FROM users
# ==============================================================================

set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Umgebungsvariablen laden
if [ -f "$ROOT_DIR/.env" ]; then
    # shellcheck disable=SC1091
    source "$ROOT_DIR/.env"
fi

POSTGRES_USER="${POSTGRES_USER:-4labs}"
POSTGRES_DB="${POSTGRES_DB:-4labscloud}"

LOG_FILE="$ROOT_DIR/restore.log"

log() {
    local MSG="[$(date '+%Y-%m-%d %H:%M:%S')] $1"
    echo "$MSG"
    echo "$MSG" >> "$LOG_FILE"
}

# 1. Parameter pruefen
BACKUP_PATH=""
FORCE=false

for arg in "$@"; do
    case "$arg" in
        -f|--force)
            FORCE=true
            ;;
        *)
            if [ -z "$BACKUP_PATH" ]; then
                BACKUP_PATH="$arg"
            fi
            ;;
    esac
done

if [ -z "$BACKUP_PATH" ] || [ ! -d "$BACKUP_PATH" ]; then
    echo "Verwendung: $0 <backup-verzeichnis> [--force]"
    echo "Beispiel:   $0 ./backups/2026-09-18_120000"
    exit 1
fi

if [ ! -f "$BACKUP_PATH/database.sql.gz" ]; then
    log "FEHLER: $BACKUP_PATH enthaelt keine database.sql.gz!"
    exit 1
fi

if [ ! -f "$BACKUP_PATH/storage.tar.gz" ]; then
    log "FEHLER: $BACKUP_PATH enthaelt keine storage.tar.gz!"
    exit 1
fi

log "=== Starte 4labscloud Wiederherstellung aus $BACKUP_PATH ==="

# Interaktive Sicherheitsabfrage
if [ "$FORCE" = false ]; then
    echo ""
    echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
    echo "ACHTUNG: Ueberschreibt alle Daten!"
    echo "Dieser Vorgang ersetzt die aktuelle Datenbank und das Storage-Volume unwiderruflich."
    echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
    read -rp "Moechtest du wirklich fortfahren? (ja/NEIN): " CONFIRM
    if [ "$CONFIRM" != "ja" ]; then
        log "Restore durch Benutzer abgebrochen."
        exit 1
    fi
fi

# Bestimme Compose-Datei und Container-Typ (Prod oder Dev)
COMPOSE_FILE=""
POSTGRES_CONTAINER=""
STORAGE_VOLUME=""

if [ -f "$ROOT_DIR/deploy/podman/prod/docker-compose.prod.yml" ] && podman ps -a --format '{{.Names}}' | grep -q "4labs-prod-"; then
    COMPOSE_FILE="$ROOT_DIR/deploy/podman/prod/docker-compose.prod.yml"
    POSTGRES_CONTAINER="4labs-prod-postgres"
    STORAGE_VOLUME="prod_storage_data"
else
    COMPOSE_FILE="$ROOT_DIR/deploy/podman/podman-compose.yml"
    POSTGRES_CONTAINER="4labs-postgres"
    STORAGE_VOLUME="podman_storage_data"
fi

# Falls Volume-Name abweicht, dynamisch ermitteln
if ! podman volume ls --format '{{.Name}}' | grep -wq "$STORAGE_VOLUME"; then
    STORAGE_VOLUME="$(podman volume ls --format '{{.Name}}' | grep 'storage_data' | head -n 1 || true)"
fi

log "Genutzte Konfiguration: Container=$POSTGRES_CONTAINER, Volume=$STORAGE_VOLUME"

# 2. Alle Container stoppen
log "1. Stoppe alle laufenden 4labscloud Container..."
CONTAINERS_TO_STOP=$(podman ps --format '{{.Names}}' | grep -E '^4labs-' || true)
if [ -n "$CONTAINERS_TO_STOP" ]; then
    # shellcheck disable=SC2086
    podman stop $CONTAINERS_TO_STOP
fi
log "Alle Container gestoppt."

# 3. Postgres zuerst wiederherstellen
log "2. Starte PostgreSQL fuer Datenbank-Wiederherstellung..."
podman start "$POSTGRES_CONTAINER"

log "Warte auf PostgreSQL Bereitschaft..."
for i in {1..30}; do
    if podman exec "$POSTGRES_CONTAINER" pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB" >/dev/null 2>&1; then
        break
    fi
    sleep 1
    if [ "$i" -eq 30 ]; then
        log "FEHLER: PostgreSQL ist nicht rechtzeitig hochgefahren."
        exit 1
    fi
done

log "Importiere Datenbank-Dump ($BACKUP_PATH/database.sql.gz)..."
gunzip -c "$BACKUP_PATH/database.sql.gz" | podman exec -i "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" > /dev/null
log "Datenbank erfolgreich wiederhergestellt."

# 4. Storage danach
log "3. Stelle Storage-Volume wieder her ($STORAGE_VOLUME)..."
podman run --rm \
    -v "$STORAGE_VOLUME":/data/storage:z \
    -v "$BACKUP_PATH":/backup:ro:z \
    docker.io/library/alpine:3.20 \
    sh -c "rm -rf /data/storage/* && tar -xzf /backup/storage.tar.gz -C /data/storage"
log "Storage-Volume erfolgreich wiederhergestellt."

# 5. Hilfsdienste starten & app-server ZULETZT starten (Migrationen!)
log "4. Starte Services: Redis, Upload-Service, Frontend..."
for s in "4labs-prod-redis" "4labs-redis" "4labs-prod-upload-service" "4labs-upload-service" "4labs-prod-frontend" "4labs-frontend"; do
    if podman ps -a --format '{{.Names}}' | grep -wq "$s"; then
        podman start "$s" || true
    fi
done

log "Warte kurz auf Hintergrunddienste..."
sleep 3

log "Starte app-server ZULETZT (fuehrt Pruefungen & evtl. Migrationen aus)..."
for a in "4labs-prod-app-server" "4labs-app-server"; do
    if podman ps -a --format '{{.Names}}' | grep -wq "$a"; then
        podman start "$a"
    fi
done

# Nginx starten
for n in "4labs-prod-nginx" "4labs-nginx"; do
    if podman ps -a --format '{{.Names}}' | grep -wq "$n"; then
        podman start "$n" || true
    fi
done

log "Services neu gestartet."

# 6. Verifikation: SELECT count(*) FROM users
log "5. Verifiziere wiederhergestellten Zustand..."
USER_COUNT="$(podman exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -t -A -c "SELECT count(*) FROM users;" 2>/dev/null || echo "ERROR")"

if [ "$USER_COUNT" = "ERROR" ]; then
    log "WARNUNG: Konnte Benutzeranzahl nicht verifizieren."
else
    log "ERFOLG: Datenbank verifiziert! Benutzer in der Datenbank: $USER_COUNT"
fi

log "=== 4labscloud Wiederherstellung erfolgreich beendet ==="
exit 0
