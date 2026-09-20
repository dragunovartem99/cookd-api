package store

import (
	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS conversations (
	id         TEXT PRIMARY KEY,
	owner      TEXT    NOT NULL,
	title      TEXT    NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS conversations_owner ON conversations (owner, updated_at DESC);

CREATE TABLE IF NOT EXISTS messages (
	id              INTEGER PRIMARY KEY AUTOINCREMENT,
	conversation_id TEXT    NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
	role            TEXT    NOT NULL CHECK (role IN ('user', 'assistant')),
	text            TEXT    NOT NULL,
	created_at      INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS messages_conversation ON messages (conversation_id, id);

CREATE TABLE IF NOT EXISTS images (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	message_id INTEGER NOT NULL REFERENCES messages (id) ON DELETE CASCADE,
	media_type TEXT    NOT NULL,
	data       BLOB    NOT NULL
);
CREATE INDEX IF NOT EXISTS images_message ON images (message_id);

CREATE TABLE IF NOT EXISTS ingredients (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	owner      TEXT    NOT NULL,
	name       TEXT    NOT NULL,
	in_stock   INTEGER NOT NULL DEFAULT 1,
	created_at INTEGER NOT NULL,
	UNIQUE (owner, name)
);

CREATE TABLE IF NOT EXISTS journal (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	owner      TEXT    NOT NULL,
	cooked_on  TEXT    NOT NULL,
	dish       TEXT    NOT NULL,
	taste      INTEGER NOT NULL CHECK (taste BETWEEN 1 AND 5),
	minutes    INTEGER,
	note       TEXT    NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS journal_owner ON journal (owner, cooked_on DESC, id DESC);
`
