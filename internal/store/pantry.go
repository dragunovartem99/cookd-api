package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Ingredient is something the owner buys now and then. It stays in the list
// when it runs out, so restocking is one switch rather than typing it again.
type Ingredient struct {
	ID      int64
	Name    string
	InStock bool
}

// AddIngredients puts names in the pantry, in stock. Names are expected
// normalised (lowercase) by the caller: that is what makes the uniqueness
// check case-insensitive for every alphabet, not just ASCII. A name already
// in the list is switched back on — typing "milk" again means you bought milk.
func (s *Store) AddIngredients(ctx context.Context, owner string, names []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().Unix()
	for _, name := range names {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO ingredients (owner, name, in_stock, created_at) VALUES (?, ?, 1, ?)
			 ON CONFLICT (owner, name) DO UPDATE SET in_stock = 1`,
			owner, name, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Ingredients lists the pantry alphabetically.
func (s *Store) Ingredients(ctx context.Context, owner string) ([]Ingredient, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, in_stock FROM ingredients WHERE owner = ? ORDER BY name`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []Ingredient{}
	for rows.Next() {
		var i Ingredient
		if err := rows.Scan(&i.ID, &i.Name, &i.InStock); err != nil {
			return nil, err
		}
		list = append(list, i)
	}
	return list, rows.Err()
}

func (s *Store) SetInStock(ctx context.Context, owner string, id int64, inStock bool) (Ingredient, error) {
	i := Ingredient{ID: id}
	err := s.db.QueryRowContext(ctx,
		`UPDATE ingredients SET in_stock = ? WHERE id = ? AND owner = ? RETURNING name, in_stock`,
		inStock, id, owner).Scan(&i.Name, &i.InStock)
	if errors.Is(err, sql.ErrNoRows) {
		return Ingredient{}, ErrNotFound
	}
	return i, err
}

func (s *Store) DeleteIngredient(ctx context.Context, owner string, id int64) error {
	return s.deleteOwned(ctx, `DELETE FROM ingredients WHERE id = ? AND owner = ?`, id, owner)
}

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

func (s *Store) deleteOwned(ctx context.Context, query string, id int64, owner string) error {
	res, err := s.db.ExecContext(ctx, query, id, owner)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
