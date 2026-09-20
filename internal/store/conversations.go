package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"time"

	_ "modernc.org/sqlite"
)

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
