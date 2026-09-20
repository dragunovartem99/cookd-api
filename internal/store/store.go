// Package store keeps conversations, messages and their photos in SQLite.
//
// Every query is scoped to an owner (the Google email), so the API can grow
// past one user without a conversation ever leaking across accounts.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// ErrNotFound is returned when a row does not exist or belongs to someone else.
var ErrNotFound = errors.New("not found")

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

type Conversation struct {
	ID        string
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Message struct {
	ID        int64
	Role      string
	Text      string
	Images    []Image
	CreatedAt time.Time
}

// Image is a photo attached to a user message. Data is only filled in when the
// caller asked for it — listings need the id and type, not the bytes.
type Image struct {
	ID        int64
	MediaType string
	Data      []byte
}

type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the database at path and applies the schema.
func Open(path string) (*Store, error) {
	// WAL lets reads proceed during a write; foreign_keys is per connection, so
	// it has to ride in the DSN rather than in a one-off statement.
	dsn := "file:" + path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) CreateConversation(ctx context.Context, owner string) (Conversation, error) {
	now := time.Now().Truncate(time.Second)
	c := Conversation{ID: rand.Text(), CreatedAt: now, UpdatedAt: now}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO conversations (id, owner, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		c.ID, owner, now.Unix(), now.Unix())
	return c, err
}

func (s *Store) ListConversations(ctx context.Context, owner string) ([]Conversation, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title, created_at, updated_at FROM conversations WHERE owner = ? ORDER BY updated_at DESC, rowid DESC`,
		owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []Conversation{}
	for rows.Next() {
		c, err := scanConversation(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (s *Store) Conversation(ctx context.Context, owner, id string) (Conversation, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, title, created_at, updated_at FROM conversations WHERE id = ? AND owner = ?`, id, owner)
	c, err := scanConversation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Conversation{}, ErrNotFound
	}
	return c, err
}

func (s *Store) DeleteConversation(ctx context.Context, owner, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM conversations WHERE id = ? AND owner = ?`, id, owner)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Messages returns a conversation's messages oldest first. withData controls
// whether photo bytes are loaded; the model needs them, a listing does not.
func (s *Store) Messages(ctx context.Context, owner, conversationID string, withData bool) ([]Message, error) {
	if _, err := s.Conversation(ctx, owner, conversationID); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, role, text, created_at FROM messages WHERE conversation_id = ? ORDER BY id`, conversationID)
	if err != nil {
		return nil, err
	}
	messages := []Message{}
	index := map[int64]int{}
	for rows.Next() {
		var m Message
		var created int64
		if err := rows.Scan(&m.ID, &m.Role, &m.Text, &created); err != nil {
			rows.Close()
			return nil, err
		}
		m.CreatedAt = time.Unix(created, 0)
		index[m.ID] = len(messages)
		messages = append(messages, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	imgs, err := s.db.QueryContext(ctx,
		`SELECT i.id, i.message_id, i.media_type, CASE WHEN ? THEN i.data END
		   FROM images i JOIN messages m ON m.id = i.message_id
		  WHERE m.conversation_id = ? ORDER BY i.id`, withData, conversationID)
	if err != nil {
		return nil, err
	}
	defer imgs.Close()
	for imgs.Next() {
		var img Image
		var messageID int64
		if err := imgs.Scan(&img.ID, &messageID, &img.MediaType, &img.Data); err != nil {
			return nil, err
		}
		i := index[messageID]
		messages[i].Images = append(messages[i].Images, img)
	}
	return messages, imgs.Err()
}

// AppendExchange stores a user message and the assistant's answer to it in one
// transaction, so a conversation never ends on an unanswered turn. The first
// exchange also names the conversation.
func (s *Store) AppendExchange(ctx context.Context, owner, conversationID string, user Message, answer string) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	now := time.Now().Unix()
	res, err := tx.ExecContext(ctx,
		`UPDATE conversations
		    SET title = CASE WHEN title = '' THEN ? ELSE title END, updated_at = ?
		  WHERE id = ? AND owner = ?`,
		titleFrom(user), now, conversationID, owner)
	if err != nil {
		return 0, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return 0, ErrNotFound
	}

	res, err = tx.ExecContext(ctx,
		`INSERT INTO messages (conversation_id, role, text, created_at) VALUES (?, 'user', ?, ?)`,
		conversationID, user.Text, now)
	if err != nil {
		return 0, err
	}
	userID, _ := res.LastInsertId()
	for _, img := range user.Images {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO images (message_id, media_type, data) VALUES (?, ?, ?)`,
			userID, img.MediaType, img.Data); err != nil {
			return 0, err
		}
	}

	res, err = tx.ExecContext(ctx,
		`INSERT INTO messages (conversation_id, role, text, created_at) VALUES (?, 'assistant', ?, ?)`,
		conversationID, answer, now)
	if err != nil {
		return 0, err
	}
	assistantID, _ := res.LastInsertId()

	return assistantID, tx.Commit()
}

// Image returns one photo, provided it belongs to the owner.
func (s *Store) Image(ctx context.Context, owner string, id int64) (Image, error) {
	img := Image{ID: id}
	err := s.db.QueryRowContext(ctx,
		`SELECT i.media_type, i.data
		   FROM images i
		   JOIN messages m ON m.id = i.message_id
		   JOIN conversations c ON c.id = m.conversation_id
		  WHERE i.id = ? AND c.owner = ?`, id, owner).Scan(&img.MediaType, &img.Data)
	if errors.Is(err, sql.ErrNoRows) {
		return Image{}, ErrNotFound
	}
	return img, err
}

type scanner interface{ Scan(dest ...any) error }

func scanConversation(row scanner) (Conversation, error) {
	var c Conversation
	var created, updated int64
	if err := row.Scan(&c.ID, &c.Title, &created, &updated); err != nil {
		return Conversation{}, err
	}
	c.CreatedAt, c.UpdatedAt = time.Unix(created, 0), time.Unix(updated, 0)
	return c, nil
}

const titleRunes = 60

// titleFrom names a conversation after its opening message.
func titleFrom(m Message) string {
	runes := []rune(m.Text)
	if len(runes) == 0 {
		return "Photo"
	}
	if len(runes) > titleRunes {
		return string(runes[:titleRunes]) + "…"
	}
	return string(runes)
}
