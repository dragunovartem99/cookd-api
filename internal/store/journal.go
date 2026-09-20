package store

import (
	"context"
	"database/sql"
	"time"
)

// JournalEntry is one dish the owner cooked and how it went.
type JournalEntry struct {
	ID int64
	// CookedOn is a calendar date, YYYY-MM-DD.
	CookedOn string
	Dish     string
	// Taste is 1 to 5.
	Taste int
	// Minutes is how long it took, start to plate; nil when not recorded.
	Minutes   *int
	Note      string
	CreatedAt time.Time
}

func (s *Store) AddJournalEntry(ctx context.Context, owner string, e JournalEntry) (JournalEntry, error) {
	e.CreatedAt = time.Now().Truncate(time.Second)
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO journal (owner, cooked_on, dish, taste, minutes, note, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		owner, e.CookedOn, e.Dish, e.Taste, e.Minutes, e.Note, e.CreatedAt.Unix())
	if err != nil {
		return JournalEntry{}, err
	}
	e.ID, _ = res.LastInsertId()
	return e, nil
}

// Journal returns up to limit entries, most recently cooked first.
func (s *Store) Journal(ctx context.Context, owner string, limit int) ([]JournalEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, cooked_on, dish, taste, minutes, note, created_at
		   FROM journal WHERE owner = ? ORDER BY cooked_on DESC, id DESC LIMIT ?`, owner, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []JournalEntry{}
	for rows.Next() {
		var e JournalEntry
		var minutes sql.NullInt64
		var created int64
		if err := rows.Scan(&e.ID, &e.CookedOn, &e.Dish, &e.Taste, &minutes, &e.Note, &created); err != nil {
			return nil, err
		}
		if minutes.Valid {
			m := int(minutes.Int64)
			e.Minutes = &m
		}
		e.CreatedAt = time.Unix(created, 0)
		list = append(list, e)
	}
	return list, rows.Err()
}

func (s *Store) DeleteJournalEntry(ctx context.Context, owner string, id int64) error {
	return s.deleteOwned(ctx, `DELETE FROM journal WHERE id = ? AND owner = ?`, id, owner)
}
