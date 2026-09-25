# Installationsanleitung

Diese Anleitung führt Schritt für Schritt durch die Einrichtung einer eigenen
4LabCloud-Instanz. Sie richtet sich an Anwender und Administratoren, die die Plattform
selbst betreiben möchten. Für die Mitarbeit am Quelltext siehe
[DEVELOPMENT.md](DEVELOPMENT.html).

---

## 1. Voraussetzungen

**Server**

- Linux-Server mit installiertem `podman` und `podman-compose`
- Mindestens 2 CPU-Kerne und 4 GB RAM. Für Reverse-Geocoding mit eigener
  Nominatim-Instanz deutlich mehr – dazu Abschnitt 7.
- Speicherplatz nach Bedarf: Die Nutzdaten liegen in Podman-Volumes.

**Netzwerk**

- Eine Domain, deren A-/AAAA-Record auf den Server zeigt
- Die Ports **80** und **443** müssen von außen erreichbar sein

**Wichtig:** Alle Dienste außer dem Nginx-Gateway laufen in einem internen Netzwerk und
sind von außen nicht erreichbar. Die Datenbank wird bewusst **nicht** nach außen
veröffentlicht.

---

## 2. Quelltext beziehen

```bash
git clone https://github.com/Der-Felix/4Lab_Cloud.git
cd 4Lab_Cloud
```

---

## 3. TLS-Zertifikate beziehen

4LabCloud erzwingt TLS 1.3. Für den Produktivbetrieb werden echte Zertifikate benötigt:

```bash
sudo certbot certonly --standalone -d cloud.example.com
```

Die Zertifikate liegen anschließend unter
`/etc/letsencrypt/live/cloud.example.com/`. Die Produktions-Compose-Datei erwartet sie
dort; `DOMAIN` in der Konfiguration muss exakt diesem Verzeichnisnamen entsprechen.

> Im Entwicklungsmodus liegt stattdessen ein selbstsigniertes Zertifikat unter
> `deploy/podman/certs/`. Das ist für den Produktivbetrieb ungeeignet.

---

## 4. Konfiguration anlegen

```bash
cp .env.prod.example .env
```

**Alle Geheimnisse müssen ersetzt werden.** Die Beispielwerte sind öffentlich bekannt und
dürfen niemals produktiv verwendet werden. Passende Werte erzeugen:

```bash
openssl rand -hex 32     # SERVICE_TOKEN, JWT_SECRET
openssl rand -base64 32  # STORAGE_KEY, MFA_KEY, AUDIT_HMAC_KEY
openssl rand -base64 24  # POSTGRES_PASSWORD, REDIS_PASSWORD
```

Anschließend `DOMAIN` auf die eigene Domain setzen.

> **`STORAGE_KEY` sichern.** Damit werden alle Dateien verschlüsselt. Geht er verloren,
> sind die gespeicherten Daten unwiederbringlich verloren – es gibt keine Hintertür.
> Gleiches gilt für `MFA_KEY` bezüglich hinterlegter Zwei-Faktor-Geheimnisse.

Eine vollständige Beschreibung aller Variablen steht in der
[README](https://github.com/Der-Felix/4Lab_Cloud/blob/main/README.md#umgebungsvariablen).

---

## 5. Starten

```bash
cd ~/4Lab_Cloud/deploy/podman/prod        # Pfad ggf. anpassen
podman-compose --env-file ../../../.env -f docker-compose.prod.yml up -d
```

Status prüfen:

```bash
podman ps
```

Alle Container sollten `healthy` melden. Die Anwendung ist nun unter
`https://cloud.example.com` erreichbar.

---

## 6. Erster Start und Administrator

Der **erste registrierte Benutzer wird automatisch Administrator**. Danach ist die freie
Selbstregistrierung geschlossen; weitere Benutzer werden per Einladung angelegt.

Deshalb: Registrieren Sie sich unmittelbar nach dem ersten Start selbst, bevor die Instanz
öffentlich erreichbar ist.

Empfohlene erste Schritte:

1. Konto anlegen (wird Administrator)
2. Unter *Einstellungen → Sicherheit* die Zwei-Faktor-Authentifizierung aktivieren
3. Unter *Einstellungen → Datenschutz* entscheiden, ob GPS-Daten aus Fotos gespeichert
   werden sollen. Standard ist aktiviert; bei Deaktivierung werden Koordinaten vor dem
   Speichern entfernt und lassen sich später nicht rekonstruieren.

---

## 7. Optional: Reverse-Geocoding

Ohne diesen Schritt funktioniert alles, Fotos erhalten lediglich keine Ortsnamen.

4LabCloud nutzt eine **selbst gehostete** Nominatim-Instanz – es werden keine
Koordinaten an Dritte übertragen. Der Container liegt hinter dem Compose-Profil
`geocoding`:

```bash
podman-compose --env-file ../../../.env -f docker-compose.prod.yml \
  --profile geocoding up -d
```

In der Konfiguration `GEOCODING_ENABLED=true` setzen.

> **Planen Sie Zeit und Platz ein.** Der Container importiert beim ersten Start einen
> OSM-Datenauszug (`NOMINATIM_PBF_URL`). Der Standardwert in `.env.prod.example` ist
> **Deutschland** – dieser Import dauert je nach Hardware mehrere Stunden und belegt
> zweistellige Gigabyte. Zum Ausprobieren empfiehlt sich zunächst ein kleiner Auszug:
>
> ```bash
> NOMINATIM_PBF_URL=https://download.geofabrik.de/europe/monaco-latest.osm.pbf
> ```
>
> Ein Wechsel des Auszugs erfordert einen erneuten Import.

---

## 8. Backups einrichten

Ein Backup ist erst dann eines, wenn die Wiederherstellung getestet wurde.

Alle folgenden Befehle werden im **Wurzelverzeichnis des Repositorys** ausgeführt:

```bash
cd ~/4Lab_Cloud          # Pfad ggf. anpassen
./scripts/backup.sh
```

Für regelmäßige Sicherungen einen Cron-Eintrag anlegen (Beispiel: täglich 02:00 Uhr):

```bash
crontab -e
# 0 2 * * * /pfad/zu/4Lab_Cloud/scripts/backup.sh
```

Gesichert werden Datenbank und Storage-Volumes. Details, Rotation und die
Wiederherstellung über `scripts/restore.sh` stehen in
[BACKUP.html](BACKUP.html).

---

## 9. Aktualisieren

```bash
cd ~/4Lab_Cloud          # Pfad ggf. anpassen
git pull
cd deploy/podman/prod
podman-compose --env-file ../../../.env -f docker-compose.prod.yml build
podman-compose --env-file ../../../.env -f docker-compose.prod.yml up -d --force-recreate
```

**Vor dem Update ein Backup anlegen.**

**Datenbankmigrationen laufen nicht automatisch.** Sie werden weder beim Start noch beim
Update angewendet: `init.sql` greift ausschließlich bei einer leeren Datenbank. Prüfen Sie
nach einem Update, ob unter `deploy/postgres/migrations/` neue Dateien hinzugekommen sind,
und spielen Sie diese ein:

```bash
cd ~/4Lab_Cloud          # Pfad ggf. anpassen

# .env in die aktuelle Shell laden. podman-compose --env-file reicht die Werte nur
# an Compose weiter, nicht an Ihre Shell - ohne diesen Schritt sind die Variablen leer.
set -a && . ./.env && set +a

podman exec -i 4labs-prod-postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  < deploy/postgres/migrations/<datei>.sql
```

> Der Container heisst im Produktionsbetrieb `4labs-prod-postgres`. Im Entwicklungsmodus
> lautet der Name `4labs-postgres` – die Compose-Dateien vergeben unterschiedliche Namen.

Der Änderungsverlauf steht im
[Changelog](https://github.com/Der-Felix/4Lab_Cloud/blob/main/CHANGELOG.md).

---

## 10. Fehlerbehebung

| Symptom | Ursache und Lösung |
|---|---|
| Browser warnt vor dem Zertifikat | Im Entwicklungsmodus normal (selbstsigniert). Produktiv: Certbot-Zertifikate und korrekte `DOMAIN` prüfen. |
| Anmeldung antwortet `429` | Rate-Limiter: 5 Versuche pro Minute und IP, 10 pro Stunde und E-Mail. Kurz warten. |
| Container startet nicht | Logs ansehen: `podman logs 4labs-app-server`. Häufigste Ursache sind fehlende oder unvollständige Werte in der Konfiguration. |
| Fotos ohne Ortsnamen | Geocoding ist deaktiviert (Standard) oder Nominatim läuft nicht – siehe Abschnitt 7. |
| Karte bleibt leer | Keine Fotos mit GPS-Koordinaten vorhanden, oder GPS-Speicherung ist deaktiviert. Bereits ohne Koordinaten gespeicherte Fotos lassen sich nicht nachträglich verorten. |
| Spalten fehlen nach einem Update | Migration nicht eingespielt – siehe Abschnitt 9. |

Eine ausführlichere Tabelle steht in der
[README](https://github.com/Der-Felix/4Lab_Cloud/blob/main/README.md#fehlerbehebung).

---

## 11. Deinstallation

```bash
cd ~/4Lab_Cloud/deploy/podman/prod        # Pfad ggf. anpassen
podman-compose --env-file ../../../.env -f docker-compose.prod.yml down
```

Die Volumes bleiben dabei erhalten. **Nur wenn Sie sämtliche Daten unwiderruflich
löschen möchten**, zusätzlich `-v` anhängen:

```bash
podman-compose --env-file ../../../.env -f docker-compose.prod.yml down -v
```

Das entfernt Datenbank, hochgeladene Dateien und Thumbnails endgültig. Legen Sie vorher
ein Backup an, falls Sie die Daten noch benötigen könnten.
