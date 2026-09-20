package store

import (
	"context"
	"time"

	_ "modernc.org/sqlite"
)

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
