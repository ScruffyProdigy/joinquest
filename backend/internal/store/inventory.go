package store

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

func (s *Store) ListUserInventory(ctx context.Context, userID uuid.UUID, gameID *uuid.UUID) ([]InventoryItem, error) {
	var (
		rows  *sql.Rows
		err   error
		query string
		args  []any
	)

	baseQuery := `
		SELECT g.id, g.name, g.description, g.category, g.game_id, g.created_at,
		       i.quantity, i.acquired_at
		FROM user_inventory i
		JOIN digital_goods g ON g.id = i.good_id
		WHERE i.user_id = $1
	`
	args = append(args, userID)

	if gameID != nil {
		query = baseQuery + ` AND g.game_id = $2 ORDER BY i.acquired_at DESC`
		args = append(args, *gameID)
	} else {
		query = baseQuery + ` ORDER BY i.acquired_at DESC`
	}

	rows, err = s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []InventoryItem
	for rows.Next() {
		var item InventoryItem
		var description sql.NullString
		var category sql.NullString
		var rowGameID sql.NullString
		if err := rows.Scan(
			&item.Good.ID,
			&item.Good.Name,
			&description,
			&category,
			&rowGameID,
			&item.Good.CreatedAt,
			&item.Quantity,
			&item.AcquiredAt,
		); err != nil {
			return nil, err
		}
		if description.Valid {
			item.Good.Description = &description.String
		}
		if category.Valid {
			item.Good.Category = &category.String
		}
		if rowGameID.Valid {
			id, err := uuid.Parse(rowGameID.String)
			if err != nil {
				return nil, err
			}
			item.Good.GameID = &id
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GrantInventoryItem(ctx context.Context, userID, goodID uuid.UUID, quantity int) error {
	if quantity <= 0 {
		quantity = 1
	}

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO user_inventory (user_id, good_id, quantity)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, good_id) DO UPDATE
		SET quantity = user_inventory.quantity + EXCLUDED.quantity,
		    acquired_at = NOW()
	`, userID, goodID, quantity)
	if err != nil {
		return err
	}
	if _, err := result.RowsAffected(); err != nil {
		return err
	}
	return nil
}

func (s *Store) RevokeInventoryItem(ctx context.Context, userID, goodID uuid.UUID, quantity int) error {
	if quantity <= 0 {
		quantity = 1
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE user_inventory
		SET quantity = quantity - $3
		WHERE user_id = $1 AND good_id = $2 AND quantity >= $3
	`, userID, goodID, quantity)
	if err != nil {
		return err
	}
	if err := ensureRowsAffected(result, ErrNotFound); err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		DELETE FROM user_inventory
		WHERE user_id = $1 AND good_id = $2 AND quantity <= 0
	`, userID, goodID)
	return err
}

func (s *Store) EnsureDigitalGoodExists(ctx context.Context, goodID uuid.UUID) error {
	_, err := s.GetDigitalGoodByID(ctx, goodID)
	return err
}

func (s *Store) EnsureUserExists(ctx context.Context, userID uuid.UUID) error {
	_, err := s.GetUserByID(ctx, userID)
	return err
}
