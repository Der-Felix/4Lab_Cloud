---
name: podman-compose-macos
description: Podman-Spezifika auf macOS Apple Silicon (M5). Bei jedem Container-, Compose- oder Netzwerk-Thema laden.
---

# Podman auf macOS M5

## Grundregeln
- podman-compose statt docker compose
- podman machine laeuft in einer VM (Apple Silicon = arm64)
- KEIN Docker Desktop, kein docker-Befehl verwenden

## Setup (einmalig)
podman machine init --cpus 4 --memory 8192 --disk-size 60
podman machine start
podman machine ssh "sudo sysctl -w net.ipv4.ip_unprivileged_port_start=80"

## Compose-Datei Besonderheiten
- Kein version:-Feld (podman-compose ignoriert es)
- Volumes: benannte Volumes bevorzugen, keine Bind-Mounts fuer DB
- Netzwerke explizit definieren:
    frontend:    # Nginx <-> Go
    backend:     # Go <-> Rust <-> PostgreSQL
- depends_on funktioniert, aber Healthchecks sind besser
- user: "1000:1000" fuer Bind-Mounts (sonst Root-Files auf Mac)

## Bekannte Fallstricke auf M5
- Nginx-Port 80/443 brauchen sudo in der VM (siehe Setup)
- host.docker.internal heisst in Podman: host.containers.internal
- Buildx gibt es nicht - multi-arch Builds via podman build --platform linux/arm64
- podman-compose down -v loescht Volumes - Vorsicht bei DB

## Starten / Stoppen
podman-compose up -d
podman-compose logs -f app-server
podman-compose down

## Rust-Build im Container
- Rust-Image: rust:1.85-slim-bookworm (arm64 nativ)
- Go-Image: golang:1.24-alpine
