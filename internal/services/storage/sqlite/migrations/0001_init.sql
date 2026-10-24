CREATE TABLE IF NOT EXISTS words (
    id INTEGER PRIMARY KEY,
    lang TEXT NOT NULL CHECK (lang IN ('source', 'con')),
    text TEXT NOT NULL,
    normalized TEXT NOT NULL,
    kind TEXT NOT NULL DEFAULT 'root' CHECK (kind IN ('root', 'prefix', 'suffix')),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(lang, normalized, kind)
);
CREATE TABLE IF NOT EXISTS translations (
    id INTEGER PRIMARY KEY,
    source_word_id INTEGER NOT NULL REFERENCES words(id) ON DELETE CASCADE,
    con_word_id INTEGER NOT NULL REFERENCES words(id) ON DELETE CASCADE,
    source TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'generated')),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(source_word_id, con_word_id)
);
CREATE INDEX IF NOT EXISTS idx_translations_source ON translations(source_word_id);
CREATE INDEX IF NOT EXISTS idx_translations_con ON translations(con_word_id);