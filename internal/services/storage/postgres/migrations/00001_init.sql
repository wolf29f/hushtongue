-- +goose Up
CREATE TABLE users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    role TEXT NOT NULL CHECK (role IN ('gm', 'player')),
    -- Hash of the player's magic link token, none for the GM
    login_token_hash BYTEA UNIQUE CHECK (role = 'player' OR login_token_hash IS NULL),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- A single GM
CREATE UNIQUE INDEX users_single_gm ON users (role) WHERE role = 'gm';

CREATE TABLE sessions (
    token_hash BYTEA PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX sessions_user_id ON sessions (user_id);
CREATE INDEX sessions_expires_at ON sessions (expires_at);

CREATE TABLE words (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    lang TEXT NOT NULL CHECK (lang IN ('source', 'con')),
    text TEXT NOT NULL,
    normalized TEXT NOT NULL,
    kind TEXT NOT NULL DEFAULT 'root' CHECK (kind IN ('root', 'prefix', 'suffix')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, lang, normalized, kind),
    -- Target of the translations' composite foreign keys
    UNIQUE (user_id, id)
);

CREATE TABLE translations (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    source_word_id BIGINT NOT NULL,
    con_word_id BIGINT NOT NULL,
    origin TEXT NOT NULL DEFAULT 'manual' CHECK (origin IN ('manual', 'generated')),
    note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source_word_id, con_word_id),
    -- Both words belong to the translation's user: no link across dictionaries
    FOREIGN KEY (user_id, source_word_id) REFERENCES words (user_id, id) ON DELETE CASCADE,
    FOREIGN KEY (user_id, con_word_id) REFERENCES words (user_id, id) ON DELETE CASCADE
);

CREATE INDEX translations_con_word_id ON translations (con_word_id);

CREATE TABLE texts (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    lang TEXT NOT NULL CHECK (lang IN ('source', 'con')),
    content TEXT NOT NULL,
    note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX texts_user_id ON texts (user_id);
