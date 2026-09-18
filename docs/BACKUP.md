# Backup- und Wiederherstellungs-Strategie: 4LabCloud

Dieses Dokument beschreibt die Datensicherungsarchitektur, die Einrichtung automatisierter Backups und das Vorgehen bei Notfall-Wiederherstellungen (Disaster Recovery).

---

## 1. Die 3-2-1 Backup-Strategie

Für den produktiven Betrieb von 4LabCloud wird die Einhaltung der anerkannten 3-2-1-Regel empfohlen:

1. **3 Kopien der Daten**:
   - Originaldaten auf dem Produktivsystem (PostgreSQL + Storage-Volume).
   - Lokales Backup auf dem Host-System (z. B. unter `/var/backups/4labscloud/`).
   - Externes Offsite-Backup (z. B. auf einem räumlich getrennten Backup-Server via Borg, Restic oder rsync).
2. **2 verschiedene Medienarten**:
   - Primärer NVMe/SSD-Speicher für den laufenden Cloud-Betrieb.
   - Sekundärer HDD/ZFS- oder NAS-Speicher für Backup-Archive.
3. **1 Offsite-Kopie**:
   - Mindestens eine Kopie muss sich an einem physisch getrennten Standort befinden (Schutz vor Brand, Diebstahl, Elementarschäden oder Ransomware).

---

## 2. Automatische Datensicherung per Cronjob

Das Skript `scripts/backup.sh` sichert sowohl die relationale Datenbank als auch alle verschlüsselten Dateiblöcke konsistent.

### Cronjob-Konfiguration (als Root oder Service-User)
Fügen Sie folgende Zeile via `crontab -e` hinzu, um täglich um 02:00 Uhr nachts ein Backup durchzuführen:

```cron
0 2 * * * cd /pfad/zu/4Lab_Cloud && ./scripts/backup.sh >> /var/log/4labs_backup_cron.log 2>&1
```

### Grandfather-Father-Son (GFS) Rotation
Das Skript verwaltet die Archivbereinigung automatisch:
- **7 tägliche Backups (Daily)**: Die letzten 7 Tage bleiben lückenlos erhalten.
- **4 wöchentliche Backups (Weekly)**: Jeweils die Sicherung des Sonntags wird für 4 Wochen aufbewahrt.
- **12 monatliche Backups (Monthly)**: Jeweils die Sicherung des 1. Tages eines Monats wird für ein volles Jahr archiviert.

---

## 3. Notfall-Wiederherstellung (Restore-Szenarien)

### Szenario A: Vollständiger System-Restore (Komplett-Restore)
Bei Ausfall der Datenbank oder Hardware-Wechsel:

```bash
# 1. Sicherstellen, dass das Backup-Verzeichnis entpackt und lesbar ist
ls -l ./backups/2026-09-18_020000/
# Enthaelt: database.sql.gz, storage.tar.gz, manifest.json

# 2. Wiederherstellung starten
./scripts/restore.sh ./backups/2026-09-18_020000
```

Das Skript führt folgende Schritte strikt sequenziell durch:
1. Stoppt alle abhängigen Container (`app-server`, `upload-service`, `frontend`, `nginx`).
2. Fährt die isolierte PostgreSQL-Instanz hoch und importiert den SQL-Dump via `psql`.
3. Leert das Storage-Volume und entpackt alle verschlüsselten Dateiblöcke aus `storage.tar.gz`.
4. Startet Redis, Upload-Service, Frontend und ZULETZT den `app-server`, um eventuelle Migrationen sauber aufzusetzen.
5. Führt eine Validierungs-Abfrage durch (`SELECT count(*) FROM users`) und bestätigt den Datenstand.

### Szenario B: Point-in-Time-Recovery (Datenbank-Korrektur)
Falls versehentlich Tabellen beschädigt wurden, die Storage-Dateien jedoch intakt sind:
```bash
# Nur Datenbank importieren:
gunzip -c ./backups/2026-09-18_020000/database.sql.gz | podman exec -i 4labs-postgres psql -U 4labs -d 4labscloud
```

---

## 4. Regelmäßige Backup-Prüfung (Test-Wiederherstellung)

Ein ungetestetes Backup ist kein Backup. Es wird empfohlen, vierteljährlich einen Probelauf auf einer Staging-Instanz durchzuführen:

1. Staging-Server aufsetzen oder temporären Test-Container starten.
2. Backup-Verzeichnis auf den Staging-Host kopieren.
3. `./scripts/restore.sh /pfad/zum/backup --force` ausführen.
4. Prüfen:
   - Benutzer-Login mit Test-Account (`felix@4labs.local`).
   - Download einer Testdatei (verifiziert AES-256-GCM Integrität).
   - Einsicht in das Audit-Log unter `/api/v1/admin/audit`.
5. Testergebnis im Betriebstagebuch dokumentieren.
