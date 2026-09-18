-- 4labscloud - Grundschema
-- Wird beim ersten Start automatisch ausgefuehrt.

-- Erweiterungen
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Benutzer
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    totp_secret TEXT,
    is_admin BOOLEAN NOT NULL DEFAULT false,
    mfa_enabled BOOLEAN NOT NULL DEFAULT false,
    mfa_secret_encrypted BYTEA,
    quota_bytes BIGINT,
    store_gps BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Refresh-Tokens (Opaque, nur Hash gespeichert)
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL,
    user_agent TEXT,
    ip_address TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_token_hash ON refresh_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);

-- Einladungen (Registrierung nur via Einladung)
CREATE TABLE IF NOT EXISTS invitations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_invitations_token_hash ON invitations(token_hash);

-- MFA Recovery-Codes (Argon2id-Hashes, Einmalnutzung)
CREATE TABLE IF NOT EXISTS mfa_recovery_codes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    used_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_mfa_recovery_codes_user_id ON mfa_recovery_codes(user_id);

-- Dateien (Metadaten)
CREATE TABLE IF NOT EXISTS files (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    filename TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    mime_type TEXT,
    checksum_sha256 TEXT NOT NULL,
    storage_path TEXT NOT NULL,
    upload_id UUID UNIQUE,
    thumbnail_path TEXT,
    width INTEGER,
    height INTEGER,
    taken_at TIMESTAMPTZ,
    exif_json JSONB,
    gps_lat DOUBLE PRECISION,
    gps_lon DOUBLE PRECISION,
    location_name TEXT,
    location_address JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_files_user ON files(user_id);
CREATE INDEX IF NOT EXISTS idx_files_gps ON files(gps_lat, gps_lon) WHERE gps_lat IS NOT NULL;

-- Reverse-Geocoding Cache (Nominatim)
CREATE TABLE IF NOT EXISTS geocoding_cache (
    id SERIAL PRIMARY KEY,
    lat_rounded NUMERIC(6,4) NOT NULL,
    lon_rounded NUMERIC(6,4) NOT NULL,
    display_name TEXT NOT NULL,
    address_json JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (lat_rounded, lon_rounded)
);
CREATE INDEX IF NOT EXISTS idx_geocoding_cache_coords ON geocoding_cache(lat_rounded, lon_rounded);
CREATE INDEX IF NOT EXISTS idx_files_upload_id ON files(upload_id);
CREATE INDEX IF NOT EXISTS idx_files_taken_at ON files(taken_at);

-- Shares
CREATE TABLE IF NOT EXISTS shares (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token TEXT UNIQUE NOT NULL,
    password_hash TEXT,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_shares_token ON shares(token);

-- Tags und Alben (Etappe F3)
CREATE TABLE IF NOT EXISTS tags (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    color TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_tags_user_name UNIQUE (user_id, name)
);
CREATE INDEX IF NOT EXISTS idx_tags_user ON tags(user_id);

CREATE TABLE IF NOT EXISTS file_tags (
    file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (file_id, tag_id)
);
CREATE INDEX IF NOT EXISTS idx_file_tags_file ON file_tags(file_id);
CREATE INDEX IF NOT EXISTS idx_file_tags_tag ON file_tags(tag_id);

-- Audit-Log (DSGVO Art. 32, BSI OPS.1.1.7)
CREATE TABLE IF NOT EXISTS audit_log (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID,
    pseudonym_hash TEXT,
    action TEXT NOT NULL,
    target_id UUID,
    ip_address INET,
    result TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_log(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_pseudonym ON audit_log(pseudonym_hash);

-- Export-Jobs (DSGVO Datenexport)
CREATE TABLE IF NOT EXISTS export_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_salt BYTEA,
    status TEXT NOT NULL DEFAULT 'pending',
    storage_path TEXT,
    size_bytes BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_export_jobs_user ON export_jobs(user_id);
CREATE INDEX IF NOT EXISTS idx_export_jobs_status ON export_jobs(status);
CREATE INDEX IF NOT EXISTS idx_export_jobs_expires ON export_jobs(expires_at);

-- Row Level Security aktivieren
ALTER TABLE files ENABLE ROW LEVEL SECURITY;
ALTER TABLE shares ENABLE ROW LEVEL SECURITY;
ALTER TABLE export_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE tags ENABLE ROW LEVEL SECURITY;
ALTER TABLE file_tags ENABLE ROW LEVEL SECURITY;

-- RLS-Policies (User sieht nur eigene Daten)
DROP POLICY IF EXISTS files_user_isolation ON files;
CREATE POLICY files_user_isolation ON files
    USING (user_id = current_setting('app.user_id', true)::uuid);

DROP POLICY IF EXISTS shares_user_isolation ON shares;
CREATE POLICY shares_user_isolation ON shares
    USING (owner_id = current_setting('app.user_id', true)::uuid);

DROP POLICY IF EXISTS export_jobs_user_isolation ON export_jobs;
CREATE POLICY export_jobs_user_isolation ON export_jobs
    USING (user_id = current_setting('app.user_id', true)::uuid);

DROP POLICY IF EXISTS tags_user_isolation ON tags;
CREATE POLICY tags_user_isolation ON tags
    USING (user_id = current_setting('app.user_id', true)::uuid);

DROP POLICY IF EXISTS file_tags_user_isolation ON file_tags;
CREATE POLICY file_tags_user_isolation ON file_tags
    USING (
        EXISTS (
            SELECT 1 FROM files f
            WHERE f.id = file_tags.file_id
            AND f.user_id = current_setting('app.user_id', true)::uuid
        )
    );

-- Upload Sessions fuer Tus-Uploads (ohne RLS, rein technische Metadaten)
CREATE TABLE IF NOT EXISTS upload_sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    filename TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    upload_offset BIGINT NOT NULL DEFAULT 0,
    checksum TEXT,
    status TEXT NOT NULL DEFAULT 'uploading',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT (now() + interval '24 hours')
);
CREATE INDEX IF NOT EXISTS idx_upload_sessions_expires ON upload_sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_upload_sessions_status ON upload_sessions(status);


