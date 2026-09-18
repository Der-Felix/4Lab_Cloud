#!/usr/bin/env bash
# ==============================================================================
# 4labscloud - Backup-Skript (Produktion & Entwicklung)
# ==============================================================================
# Ablauf:
# 1. pg_dump (live, ohne Stop der Datenbank) -> database.sql.gz
# 2. Storage-Volume sichern -> storage.tar.gz
# 3. GFS-Retention anwenden: 7 daily / 4 weekly / 12 monthly
# 4. Protokollierung in backup.log
# ==============================================================================

set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Umgebungsvariablen laden (.env aus Root oder lokal)
if [ -f "$ROOT_DIR/.env" ]; then
    # shellcheck disable=SC1091
    source "$ROOT_DIR/.env"
fi

POSTGRES_USER="${POSTGRES_USER:-4labs}"
POSTGRES_DB="${POSTGRES_DB:-4labscloud}"

# Zielverzeichnis fuer Backups
if [ -z "$BACKUP_DIR" ]; then
    if [ -w "/var/backups" ]; then
        BACKUP_DIR="/var/backups/4labscloud"
    else
        BACKUP_DIR="$ROOT_DIR/backups"
    fi
fi

TIMESTAMP="$(date +%Y-%m-%d_%H%M%S)"
TARGET_DIR="$BACKUP_DIR/$TIMESTAMP"
LOG_FILE="$BACKUP_DIR/backup.log"

mkdir -p "$TARGET_DIR"
mkdir -p "$BACKUP_DIR"

log() {
    local MSG="[$(date '+%Y-%m-%d %H:%M:%S')] $1"
    echo "$MSG"
    echo "$MSG" >> "$LOG_FILE"
}

cleanup_on_error() {
    log "FEHLER: Backup fehlgeschlagen! Bereinige unvollstaendiges Verzeichnis: $TARGET_DIR"
    rm -rf "$TARGET_DIR"
    exit 1
}

trap cleanup_on_error ERR

log "=== Starte 4labscloud Backup in $TARGET_DIR ==="

# 1. Postgres-Container ermitteln
POSTGRES_CONTAINER="$(podman ps --format '{{.Names}}' | grep -E '^(4labs-prod-postgres|4labs-postgres)$' | head -n 1 || true)"

if [ -z "$POSTGRES_CONTAINER" ]; then
    log "FEHLER: Kein laufender PostgreSQL-Container gefunden (weder 4labs-prod-postgres noch 4labs-postgres)."
    exit 1
fi

log "Gefundener PostgreSQL-Container: $POSTGRES_CONTAINER"

# 2. Schritt 1: pg_dump (live, ohne Stop)
log "Erstelle komprimierten Datenbank-Dump (pg_dump)..."
podman exec "$POSTGRES_CONTAINER" pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists | gzip > "$TARGET_DIR/database.sql.gz"

if [ ! -s "$TARGET_DIR/database.sql.gz" ]; then
    log "FEHLER: database.sql.gz ist leer oder wurde nicht erstellt!"
    exit 1
fi
log "Datenbank-Dump erfolgreich erstellt ($(du -h "$TARGET_DIR/database.sql.gz" | cut -f1))."

# 3. Schritt 2: Storage-Volume tar.gz
STORAGE_VOLUME="$(podman volume ls --format '{{.Name}}' | grep -E '^(prod_storage_data|podman_storage_data|storage_data)$' | head -n 1 || true)"
if [ -z "$STORAGE_VOLUME" ]; then
    STORAGE_VOLUME="$(podman volume ls --format '{{.Name}}' | grep 'storage_data' | head -n 1 || true)"
fi

if [ -z "$STORAGE_VOLUME" ]; then
    log "FEHLER: Kein Storage-Volume fuer Verschluesselungsdaten gefunden!"
    exit 1
fi

log "Sichere Storage-Volume: $STORAGE_VOLUME ..."
podman run --rm \
    -v "$STORAGE_VOLUME":/data/storage:ro \
    -v "$TARGET_DIR":/backup:z \
    docker.io/library/alpine:3.20 \
    tar -czf /backup/storage.tar.gz -C /data/storage .

if [ ! -s "$TARGET_DIR/storage.tar.gz" ]; then
    log "FEHLER: storage.tar.gz ist leer oder wurde nicht erstellt!"
    exit 1
fi
log "Storage-Volume erfolgreich archiviert ($(du -h "$TARGET_DIR/storage.tar.gz" | cut -f1))."

# Metadaten-Manifest speichern
cat <<EOF > "$TARGET_DIR/manifest.json"
{
  "timestamp": "$TIMESTAMP",
  "database": "$POSTGRES_DB",
  "postgres_container": "$POSTGRES_CONTAINER",
  "storage_volume": "$STORAGE_VOLUME",
  "db_size": "$(wc -c < "$TARGET_DIR/database.sql.gz" | tr -d ' ')",
  "storage_size": "$(wc -c < "$TARGET_DIR/storage.tar.gz" | tr -d ' ')"
}
EOF

# 4. Schritt 3: Retention (7 daily / 4 weekly / 12 monthly)
log "Pruefe Retention-Regeln (7 Daily / 4 Weekly / 12 Monthly)..."
NOW_SEC="$(date +%s)"

# Alle Backup-Ordner listen (Format: YYYY-MM-DD_HHMMSS)
for bdir in "$BACKUP_DIR"/*; do
    if [ ! -d "$bdir" ]; then
        continue
    fi
    bname="$(basename "$bdir")"
    # Nur Datums-Ordner pruefen
    if [[ ! "$bname" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}_[0-9]{6}$ ]]; then
        continue
    fi

    # Ordner-Datum extrahieren
    folder_date="${bname:0:10}"
    # Berechne Alter in Tagen
    if date -v -1d >/dev/null 2>&1; then
        # BSD/macOS date
        folder_sec="$(date -j -f "%Y-%m-%d" "$folder_date" "+%s" 2>/dev/null || echo "$NOW_SEC")"
    else
        # GNU date
        folder_sec="$(date -d "$folder_date" "+%s" 2>/dev/null || echo "$NOW_SEC")"
    fi

    age_days=$(( (NOW_SEC - folder_sec) / 86400 ))

    # 1. Juenger als 7 Tage: Behalten (Daily)
    if [ "$age_days" -le 7 ]; then
        continue
    fi

    # 2. Tag der Woche (Sonntag = wochentliches Backup)
    if date -v -1d >/dev/null 2>&1; then
        dow="$(date -j -f "%Y-%m-%d" "$folder_date" "+%u" 2>/dev/null || echo 1)"
        dom="$(date -j -f "%Y-%m-%d" "$folder_date" "+%d" 2>/dev/null || echo 01)"
    else
        dow="$(date -d "$folder_date" "+%u" 2>/dev/null || echo 1)"
        dom="$(date -d "$folder_date" "+%d" 2>/dev/null || echo 01)"
    fi

    # Zwischen 8 und 28 Tagen: Nur sonntaegliche Backups behalten (4 Weekly)
    if [ "$age_days" -gt 7 ] && [ "$age_days" -le 28 ]; then
        if [ "$dow" -eq 7 ]; then
            continue
        fi
    fi

    # Zwischen 29 und 365 Tagen: Nur Monats-Erste behalten (12 Monthly)
    if [ "$age_days" -gt 28 ] && [ "$age_days" -le 365 ]; then
        if [ "$dom" -eq "01" ]; then
            continue
        fi
    fi

    # Aelter als 365 Tage oder nicht in Retention: Loeschen
    log "Retention: Entferne altes Backup: $bname (Alter: $age_days Tage)"
    rm -rf "$bdir"
done

log "=== 4labscloud Backup erfolgreich abgeschlossen: $TARGET_DIR ==="
exit 0
