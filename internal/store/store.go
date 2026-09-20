// Package store keeps conversations, messages and their photos in SQLite.
//
// Every query is scoped to an owner (the Google email), so the API can grow
// past one user without a conversation ever leaking across accounts.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// ErrNotFound is returned when a row does not exist or belongs to someone else.
var ErrNotFound = errors.New("not found")

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

type scanner interface{ Scan(dest ...any) error }
