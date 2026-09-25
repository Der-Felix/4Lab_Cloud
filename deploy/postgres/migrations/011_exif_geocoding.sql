-- Migration 011: EXIF-Vollauslesung, Reverse-Geocoding und GPS-Datenschutz

-- 1. EXIF- und Standort-Felder fuer Dateien
ALTER TABLE files
    ADD COLUMN IF NOT EXISTS exif_json JSONB,
    ADD COLUMN IF NOT EXISTS gps_lat DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS gps_lon DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS location_name TEXT,
    ADD COLUMN IF NOT EXISTS location_address JSONB;

-- Index fuer raeumliche/Karten-Abfragen (nur wo GPS vorhanden)
CREATE INDEX IF NOT EXISTS idx_files_gps ON files(gps_lat, gps_lon) WHERE gps_lat IS NOT NULL;

-- 2. Datenschutz-Opt-out fuer Benutzer (Default: true / erlaubt)
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS store_gps BOOLEAN NOT NULL DEFAULT true;

-- 3. Cache-Tabelle fuer Reverse-Geocoding-Ergebnisse (Nominatim)
CREATE TABLE IF NOT EXISTS geocoding_cache (
    id SERIAL PRIMARY KEY,
    lat_rounded NUMERIC(9,4) NOT NULL,
    lon_rounded NUMERIC(9,4) NOT NULL,
    display_name TEXT NOT NULL,
    address_json JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (lat_rounded, lon_rounded)
);

-- Bestehende Tabelle nachtraeglich korrigieren: die alte Praezision reichte fuer
-- Breitengrad (max. +-90), aber Laengengrad geht bis +-180 und braucht eine
-- fuenfte Vorkommastelle, sonst schlaegt der Insert mit numeric-overflow fehl.
ALTER TABLE geocoding_cache ALTER COLUMN lat_rounded TYPE NUMERIC(9,4);
ALTER TABLE geocoding_cache ALTER COLUMN lon_rounded TYPE NUMERIC(9,4);

CREATE INDEX IF NOT EXISTS idx_geocoding_cache_coords ON geocoding_cache(lat_rounded, lon_rounded);
