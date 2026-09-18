-- Migration 009: Tags und Tag-Zuordnungen fuer Alben (Etappe F3)
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

-- Row Level Security aktivieren (DSGVO Mandantentrennung)
ALTER TABLE tags ENABLE ROW LEVEL SECURITY;
ALTER TABLE file_tags ENABLE ROW LEVEL SECURITY;

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
