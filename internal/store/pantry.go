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
