# Multi-Stage Dockerfile fuer den 4labscloud Go-App-Server
# Stage 1: Build der statischen Binary
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Abhaengigkeiten herunterladen und cachen
COPY go.mod go.sum* ./
RUN go mod download

# Quellcode kopieren und kompilieren
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/server ./cmd/server

# Stage 2: Minimales Runtime-Image
FROM alpine:3.21

# Sicherheits-Updates, Zertifikate und wget fuer den Healthcheck
RUN apk --no-cache add ca-certificates tzdata wget && \
    addgroup -g 1000 appuser && \
    adduser -u 1000 -G appuser -s /bin/sh -D appuser

WORKDIR /app

# Ausfuehrbare Datei aus Stage 1 kopieren
COPY --from=builder /app/server /app/server

# Berechtigungen setzen
USER 1000:1000

# Standard-Umgebungsvariablen
ENV GIN_MODE=release \
    APP_PORT=8080

EXPOSE 8080

ENTRYPOINT ["/app/server"]
