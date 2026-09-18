-- 4labscloud - Upload Sessions fuer Tus-Uploads
-- Rust greift direkt auf diese Tabelle zu (ohne RLS, rein technische Metadaten)

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
