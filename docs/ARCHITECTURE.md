# Architektur-Dokumentation: 4LabCloud

4LabCloud ist als moderne, micro-service-orientierte On-Premises-Cloud-Plattform konzipiert. Ziel ist es, höchste Datensicherheit, vollständige Souveränität und maximale Upload-Geschwindigkeit zu vereinen.

---

## 1. Komponenten-Übersicht

```mermaid
graph TD
    subgraph Oeffentlichkeit
        Browser["Web-Browser (Client)"]
    end

    subgraph Podman_Netzwerk_Frontend
        Nginx["Nginx Reverse Proxy\n(Port 80/443, TLS 1.3)"]
        Frontend["Frontend Service\n(SvelteKit + Node.js)"]
    end

    subgraph Podman_Netzwerk_Backend_Isoliert
        AppServer["App-Server\n(Go / Gin REST API)"]
        UploadService["Upload-Service\n(Rust / Axum / AES-256-GCM)"]
        Postgres[("PostgreSQL 17\n(Row Level Security)")]
        Redis[("Redis 7\n(Sessions / Cache)")]
        StorageVolume[("Dateisystem-Volume\n(AES-256-GCM Chunks)")]
    end

    Browser -->|TLS 1.3 HTTPS| Nginx
    Nginx -->|Statische Assets & SSR| Frontend
    Nginx -->|/api/*| AppServer
    Nginx -->|/api/v1/uploads/tus/*| UploadService

    AppServer -->|Auth: Bearer SERVICE_TOKEN| UploadService
    AppServer -->|SQL mit app.user_id| Postgres
    AppServer -->|Session Cache / Locks| Redis

    UploadService -->|Verschluesselte Blobs| StorageVolume
    UploadService -.->|Metadaten & Thumbnail Status| Postgres
```

### Die Rollen der Services:
1. **Nginx**: Einziger öffentlich exponierter Endpunkt. Erzwingt TLS 1.3 nach BSI TR-02102-2, Security-Header und striktes Rate-Limiting.
2. **Go (App-Server)**: Verwaltet Benutzer, Sessions, Authentifizierung (inkl. optionaler TOTP-MFA), Metadaten, Freigaben, Quotas und Audit-Logs.
3. **Rust (Upload-Service)**: Reiner High-Performance Storage- und Chunking-Dienst. Führt transparente AES-256-GCM Verschlüsselung durch. Liest niemals Datei-Inhalte aus.
4. **PostgreSQL 17**: Persistiert Benutzer- und Datei-Metadaten. Nutzt strikte Row Level Security (RLS) zur Isolation zwischen Mandanten.
5. **Redis 7**: Schneller Key-Value-Store für aktive Sitzungen, Rate-Limits und temporäre Upload-Token.
6. **SvelteKit (Frontend)**: Performantes, barrierefreies UI mit TailwindCSS und Uppy für robuste Chunk-Uploads.

---

## 2. Datenflüsse & Sequenzdiagramme

### A. Upload-Ablauf (Presigned Tus Chunking)
Der Upload läuft direkt vom Browser über Nginx an den Rust-Upload-Service, ohne den Go-App-Server mit Streaming-Daten zu belasten:

```mermaid
sequenceDiagram
    autonumber
    actor User as Benutzer / Browser
    participant Nginx
    participant Go as Go App-Server
    participant Rust as Rust Upload-Service
    participant DB as PostgreSQL
    participant Storage as Festplatte (Storage)

    User->>Nginx: POST /api/v1/uploads/init (Dateiname, Groesse)
    Nginx->>Go: Weiterleitung /api/v1/uploads/init
    Go->>Go: Pruefe Benutzer-Authentifizierung & Quota
    Go->>Rust: POST /internal/uploads/init (Auth: Bearer SERVICE_TOKEN)
    Rust->>Rust: Session anlegen & temporaere Upload-ID generieren
    Rust-->>Go: 201 Created (Upload-ID)
    Go-->>User: 200 OK (Upload-ID, Presigned Tus URL)

    loop Chunk-Upload (Tus Protocol)
        User->>Nginx: PATCH /api/v1/uploads/tus/{upload_id} (Chunk Bytes)
        Nginx->>Rust: Direkter Stream an Rust
        Rust->>Rust: AES-256-GCM verschluesseln
        Rust->>Storage: Verschluesselte Chunks schreiben
        Rust-->>User: 204 No Content (Upload-Offset)
    end

    User->>Nginx: POST /api/v1/uploads/complete
    Nginx->>Go: Weiterleitung /api/v1/uploads/complete
    Go->>Rust: POST /internal/uploads/complete (Upload-ID)
    Rust->>Storage: Datei finalisieren (Nonce + Ciphertext)
    Rust-->>Go: 200 OK (Dateipfad, Groesse, Checksumme)
    Go->>DB: INSERT INTO files (user_id, name, path, ...)
    Go->>DB: INSERT INTO audit_log (action = 'upload')
    Go-->>User: 201 Created (File-Metadaten)
```

### B. Download-Ablauf
```mermaid
sequenceDiagram
    autonumber
    actor User as Benutzer / Browser
    participant Nginx
    participant Go as Go App-Server
    participant Rust as Rust Upload-Service
    participant Storage as Festplatte (Storage)

    User->>Nginx: GET /api/v1/files/{id}/download
    Nginx->>Go: GET /api/v1/files/{id}/download
    Go->>Go: Pruefe RLS / Berechtigung & Rate-Limit
    Go->>Rust: GET /internal/files/{id}/stream (Bearer Token)
    Rust->>Storage: Verschluesselte Blobs einlesen
    Rust->>Rust: AES-256-GCM Entschluesselung On-the-Fly
    Rust-->>Go: Entschluesselter Byte-Stream
    Go-->>User: 200 OK (Content-Disposition: attachment)
    Go->>Go: Audit-Log: action = 'download'
```

### C. Share-Ablauf (Öffentliche Freigaben)
```mermaid
sequenceDiagram
    autonumber
    actor Recipient as Empfaenger (Extern)
    participant Nginx
    participant Go as Go App-Server
    participant Rust as Rust Upload-Service

    Recipient->>Nginx: GET /share/{token}
    Nginx->>Go: GET /api/v1/shares/{token}
    Go->>Go: Pruefe Token-Gueltigkeit & Ablaufdatum
    alt Passwortgeschuetzt
        Go-->>Recipient: 401 Unauthorized (Passwort erforderlich)
        Recipient->>Go: POST /api/v1/shares/{token}/auth (Passwort)
        Go->>Go: Verifiziere bcrypt Hash
    end
    Go-->>Recipient: 200 OK (Datei-Vorschau / Download-URL)
    Recipient->>Go: GET /api/v1/shares/{token}/download
    Go->>Rust: Stream Anfrage
    Rust-->>Recipient: Datei-Stream (Decrypted)
```

---

## 3. Sicherheits- und Isolationskonzept

### Row Level Security (RLS) in PostgreSQL
Jede schreibende oder lesende Abfrage in der Datenbank erfolgt unter Mandantenisolierung.
```sql
ALTER TABLE files ENABLE ROW LEVEL SECURITY;

CREATE POLICY files_isolation_policy ON files
    FOR ALL
    TO cloud_app_role
    USING (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid);
```
Der Go-App-Server setzt vor jeder Transaktion `SET LOCAL app.user_id = '...'`. Ein Zugriff über Benutzergrenzen hinweg ist auf Datenbankebene physikalisch unmöglich.

### Verschlüsselung (At Rest & In Transit)
- **In Transit**: Ausschließlich TLS 1.3 mit PFS (Forward Secrecy) an Nginx. Internes Docker/Podman-Backend-Netzwerk ist nicht nach außen geroutet.
- **At Rest**: AES-256-GCM. Jede Datei erhält eine zufällige 12-Byte-Nonce. Das Datei-Format auf der Festplatte folgt strikt dem Standard:
  `[12 Bytes Nonce][Verschluesselte Nutzdaten (Ciphertext)][16 Bytes GCM Auth-Tag]`.

### Service-to-Service Token
Der Rust-Upload-Service weist alle Anfragen ohne gültigen `Authorization: Bearer <SERVICE_TOKEN>` Header im isolierten Backend-Netzwerk mit `401 Unauthorized` ab.
