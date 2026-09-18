# Compliance-Dokumentation: DSGVO & BSI TR-02102-2

4LabCloud ist von Grund auf als datenschutzfreundliche („Privacy by Design“ & „Privacy by Default“) Plattform konzipiert. Dieses Dokument beschreibt die konkreten technischen und organisatorischen Maßnahmen (TOM) zur Einhaltung der Datenschutz-Grundverordnung (DSGVO) sowie der Vorgaben des Bundesamts für Sicherheit in der Informationstechnik (BSI).

---

## 1. DSGVO-Artikel-Mapping

| DSGVO-Artikel | Gesetzliche Anforderung | Umsetzung in 4LabCloud |
|---|---|---|
| **Art. 5 Abs. 1 lit. c**<br>*(Datenminimierung)* | Erhebung auf das notwendige Maß beschränken. | - Nginx loggt keine IP-Adressen (`log_format minimal '$request_method $request_uri $status'`).<br>- Keine externen Analytics, Cookies oder Tracking-Tools.<br>- Keine Inhaltsanalyse oder Metadaten-Scraping von Benutzerdateien. |
| **Art. 5 Abs. 1 lit. e**<br>*(Speicherbegrenzung)* | Daten nur so lange speichern wie nötig. | - Automatische Bereinigung von Audit-Logs nach 90 Tagen (`AUDIT_RETENTION_DAYS`).<br>- Automatisches Verfallsdatum für öffentliche Freigaben (Share-Links).<br>- Temporäre Chunk-Dateien werden nach 24h unvollständigem Upload bereinigt. |
| **Art. 17**<br>*(Recht auf Löschung)* | Betroffene können Löschung aller personenbezogenen Daten verlangen. | - Endpunkt `DELETE /api/v1/users/me` löscht Benutzerkonto, alle Dateien und Freigaben sofort physikalisch.<br>- Sofortiges Überschreiben und Unlinken verschlüsselter Blobs auf dem Dateisystem (kein bloßes Soft-Delete).<br>- Löschvorgang wird im Audit-Log vermerkt (ohne Namensnennung). |
| **Art. 20**<br>*(Recht auf Datenübertragbarkeit)* | Export aller eigenen Daten in gängigem, maschinenlesbarem Format. | - Endpunkt `POST /api/v1/users/me/export` generiert ein asynchrones ZIP-Archiv.<br>- Enthält alle Originaldateien, Metadaten im JSON-Format und eine `HOW_TO_DECRYPT.txt`.<br>- Archiv wird mit dem Benutzerpasswort verschlüsselt zum Download bereitgestellt. |
| **Art. 32**<br>*(Sicherheit der Verarbeitung / TOM)* | Gewährleistung eines angemessenen Schutzniveaus durch Verschlüsselung und Pseudonymisierung. | - Authentifizierte AES-256-GCM Verschlüsselung at-rest.<br>- Strikte Mandantentrennung via PostgreSQL Row Level Security (RLS).<br>- HMAC-SHA256 Einweg-Pseudonymisierung von User-IDs in Audit-Logs. |

---

## 2. BSI TR-02102-2 Konformität (Kryptografische Verfahren)

Die BSI Technische Richtlinie TR-02102-2 definiert Vorgaben für die Verwendung von Transport Layer Security (TLS). 4LabCloud erfüllt diese Anforderungen vollständig:

1. **Ausschließliche Nutzung von TLS 1.3**: Ältere Protokolle (TLS 1.0, 1.1, 1.2) sind in Nginx deaktiviert.
2. **Cipher Suites**: Verwendung von modernen AEAD-Ciphers (`TLS_AES_256_GCM_SHA384`, `TLS_CHACHA20_POLY1305_SHA256`).
3. **PFS (Perfect Forward Secrecy)**: Gewährleistet, dass frühere Sitzungen auch bei Kompromittierung privater Schlüssel nicht entschlüsselt werden können.
4. **HSTS (HTTP Strict Transport Security)**: Nginx sendet `Strict-Transport-Security: max-age=31536000; includeSubDomains` mit jedem Response Header.

---

## 3. Audit-Logging & Pseudonymisierungs-Konzept

Um unbefugte Datenzugriffe nachzuweisen, protokolliert der Go-App-Server relevante Sicherheitsereignisse in der Tabelle `audit_log`:

- **Ereignisse**: `login`, `logout`, `upload`, `download`, `share_create`, `share_delete`, `file_delete`, `user_delete`, `admin_action`.
- **HMAC-Pseudonymisierung**: Benutzer-IDs werden vor der Protokollierung mit `HMAC-SHA256(user_id, AUDIT_HMAC_KEY)` maskiert. Selbst Administratoren mit direktem Datenbank-Lesezugriff können Audit-Logs nicht ohne den geheimen Schlüssel einem Klarnamen zuordnen.
- **Aufbewahrungsfrist**: Ein täglicher Hintergrundjob löscht Einträge, die älter als `AUDIT_RETENTION_DAYS` (Standard: 90 Tage) sind.
- **Zugriffsschutz**: Der Zugriff auf Audit-Logs ist ausschließlich Administratoren über `/api/v1/admin/audit` vorbehalten.

---

## 4. Unabhängigkeit von externen Diensten (Zero-SaaS)

- **Keine externen CDNs**: Sämtliche JavaScript-Bundles, CSS-Dateien und Icons (@tabler/icons-svelte) werden lokal vom Nginx-Server ausgeliefert.
- **Lokale Schriftarten**: Alle Schriftarten (Inter, Great Vibes, Playfair Display) sind über NPM (`@fontsource/*`) gebündelt und erfordern keinen Zugriff auf Google Fonts.
- **Kein Tracking**: Keine Einbindung von Google Analytics, Matomo, Werbenetzwerken oder externen Gravatar-Servern.
- **EU/EWR Hosting**: Sämtliche Server und Speichermedien können in eigener Hoheit auf On-Premises-Hardware betrieben werden. Bei Dienstleisterbetrieb ist ein Standard-Vertrag zur Auftragsverarbeitung (AVV nach Art. 28 DSGVO) abzuschließen.

### 4.1 OpenStreetMap-Kartenkacheln & Geodaten (Art. 6 Abs. 1 lit. f DSGVO)
- **Kartenanzeige**: Für die interaktive Kartenansicht (`/photos/map`) und das Dashboard-Vorschau-Widget werden Kartenkacheln von OpenStreetMap (`tile.openstreetmap.org`) abgerufen.
- **IP-Übermittlung**: Beim Abruf der Kacheln wird aus technischen Gründen die IP-Adresse des Browsers an die OpenStreetMap Foundation übertragen.
- **Transparenz & Attribution**: Die Benutzeroberfläche blendet auf jeder Karte ein klares Hinweis-Banner ein (*"Kartenkacheln von OpenStreetMap"*) und führt die zwingend vorgeschriebene Attribution *„© OpenStreetMap contributors“* gut sichtbar auf.
- **Datensparsamkeit & GPS-Opt-Out**: Benutzer können das Speichern und Auswerten von GPS-Koordinaten in den Benutzereinstellungen (`store_gps: false`) jederzeit mit sofortiger Wirkung deaktivieren (Privacy by Default konfigurierbar).

