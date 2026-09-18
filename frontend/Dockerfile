# 4labscloud - Frontend (SvelteKit + Node Adapter)
# Multi-Stage Build fuer minimalen Container-Footprint

# Stage 1: Build
FROM docker.io/library/node:22-alpine AS builder

WORKDIR /app

# Abhaengigkeiten installieren
COPY package*.json ./
RUN npm ci

# Quellcode kopieren und bauen
COPY . .
RUN npm run build
RUN npm prune --production

# Stage 2: Runtime
FROM docker.io/library/node:22-alpine AS runner

WORKDIR /app

ENV NODE_ENV=production
ENV PORT=3000
ENV HOST=0.0.0.0

# Nicht-privilegierter Benutzer (DSGVO / BSI Minimal Privilege)
USER node

# Nur gebaute Artefakte und Produktions-Abhaengigkeiten uebernehmen
COPY --chown=node:node --from=builder /app/package*.json ./
COPY --chown=node:node --from=builder /app/node_modules ./node_modules
COPY --chown=node:node --from=builder /app/build ./build

EXPOSE 3000

CMD ["node", "build/index.js"]
