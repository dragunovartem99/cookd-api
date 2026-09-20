package store

import (
	"context"
	"database/sql"
	"errors"

	_ "modernc.org/sqlite"
)

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
