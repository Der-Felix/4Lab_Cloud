-- Migration 008: Thumbnail- und Bild-Metadaten (Etappe F3)
-- DSGVO-Hinweis: EXIF-Daten sind standardmaessig NULL (Art. 5 Datensparsamkeit).
ALTER TABLE files ADD COLUMN IF NOT EXISTS thumbnail_path TEXT;
ALTER TABLE files ADD COLUMN IF NOT EXISTS width INTEGER;
ALTER TABLE files ADD COLUMN IF NOT EXISTS height INTEGER;
ALTER TABLE files ADD COLUMN IF NOT EXISTS taken_at TIMESTAMPTZ;
ALTER TABLE files ADD COLUMN IF NOT EXISTS exif_json JSONB;

CREATE INDEX IF NOT EXISTS idx_files_taken_at ON files(taken_at);
