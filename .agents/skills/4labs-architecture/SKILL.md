---
name: 4labs-architecture
description: Verbindliche Architektur fuer 4labscloud. Bei JEDER Aenderung am Code laden. Definiert Tech-Stack, Datenfluss und Sicherheitsregeln.
---

# 4labscloud - Architektur

## Tech-Stack (NICHT verhandelbar)
- Nginx - einziger oeffentlicher Endpunkt (Port 443, TLS 1.3)
- Go - App-Server: User, Shares, Auth, Metadaten, REST-API
- Rust - Storage/Upload-Service: Dateien, AES-256-GCM, Chunks
- PostgreSQL - Metadaten, Row Level Security
- Redis - Sessions, Cache, Queue
- SvelteKit + Uppy - Frontend
- Podman - Container (NICHT Docker)

## Kommunikation
- Alle Services reden per REST/JSON (kein gRPC)
- Browser -> Nginx -> Go-App
- Go-App -> Rust (intern, Header: Authorization: Bearer <token>)
- Go-App -> PostgreSQL (intern)

## Datenfluss Upload
1. Browser fragt Go: POST /api/v1/uploads/init
2. Go prueft Auth + Quota, fragt Rust: POST /internal/uploads/init
3. Rust antwortet mit Presigned URL
4. Browser laedt DIREKT zu Rust hoch (nicht durch Go)
5. Browser meldet Go: POST /api/v1/uploads/complete
6. Go bestaetigt Rust: POST /internal/uploads/complete

## Harte Regeln
- Rust ist NIEMALS oeffentlich erreichbar - nur internes Podman-Netz.
- Rust liest NIE den Inhalt von Dateien (DSGVO Zweckbindung).
- Jede Tabelle in PostgreSQL hat RLS-Policies.
- Jede Aktion wird in Audit-Logs geschrieben.
- Kein externes SaaS - alles self-hosted.

## Ordnerstruktur
4Lab_Cloud/
  services/app-server/       (Go)
  services/upload-service/   (Rust)
  frontend/                  (SvelteKit)
  deploy/podman/             (podman-compose.yml, Nginx-Config)
  deploy/postgres/           (Init-SQL, RLS-Policies)
  docs/                      (Architektur, Compliance)
  .agents/skills/            (diese Skills)
