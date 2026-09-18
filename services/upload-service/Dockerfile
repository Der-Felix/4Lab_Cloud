# Stage 1: Build der statischen Binary mit musl
FROM rust:slim-bookworm AS builder

# Musl-Werkzeuge fuer statisches Linking auf Alpine
RUN apt-get update && apt-get install -y musl-tools musl-dev && rm -rf /var/lib/apt/lists/*

# Passendes musl-Target fuer die Architektur registrieren
RUN ARCH=$(uname -m) && rustup target add ${ARCH}-unknown-linux-musl

WORKDIR /app

# Abhaengigkeiten vorab kompilieren fuer schnellen Cache
COPY Cargo.toml Cargo.lock* ./
RUN mkdir src && echo "fn main() {}" > src/main.rs && \
    ARCH=$(uname -m) && cargo build --release --target ${ARCH}-unknown-linux-musl && \
    rm -rf src

# Quellcode kopieren und finale Binary bauen
COPY src ./src
RUN touch src/main.rs && \
    ARCH=$(uname -m) && \
    cargo build --release --target ${ARCH}-unknown-linux-musl && \
    cp target/${ARCH}-unknown-linux-musl/release/upload-service /app/upload-service

# Stage 2: Minimales Alpine Runtime-Image
FROM alpine:3.21

# Zertifikate, Zeitzonen und wget fuer Healthcheck
RUN apk --no-cache add ca-certificates tzdata wget && \
    addgroup -g 1000 appuser && \
    adduser -u 1000 -G appuser -s /bin/sh -D appuser && \
    mkdir -p /data/storage /data/thumbs /data/exports && \
    chown -R 1000:1000 /data/storage /data/thumbs /data/exports

WORKDIR /app
COPY --from=builder /app/upload-service /app/upload-service

USER 1000:1000

ENV UPLOAD_PORT=8081 \
    STORAGE_DIR=/data/storage \
    THUMBS_DIR=/data/thumbs \
    EXPORTS_DIR=/data/exports

EXPOSE 8081

ENTRYPOINT ["/app/upload-service"]
