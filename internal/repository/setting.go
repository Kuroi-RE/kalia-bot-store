package repository

import (
	"context"

	"github.com/kalia/store/internal/model"
)

// SettingRepository provides access to the settings table.
type SettingRepository struct{ db DBTX }

// NewSettingRepository builds a setting repository over db.
func NewSettingRepository(db DBTX) *SettingRepository { return &SettingRepository{db: db} }

// Get returns a setting value. Returns ErrNotFound when absent.
func (r *SettingRepository) Get(ctx context.Context, key string) (*model.Setting, error) {
	var s model.Setting
	err := r.db.QueryRow(ctx, `SELECT key, value, updated_at FROM settings WHERE key = $1`, key).
		Scan(&s.Key, &s.Value, &s.UpdatedAt)
	if IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// List returns all settings ordered by key.
func (r *SettingRepository) List(ctx context.Context) ([]model.Setting, error) {
	rows, err := r.db.Query(ctx, `SELECT key, value, updated_at FROM settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []model.Setting
	for rows.Next() {
		var s model.Setting
		if err := rows.Scan(&s.Key, &s.Value, &s.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

// Upsert sets a setting value.
func (r *SettingRepository) Upsert(ctx context.Context, key, value string) (*model.Setting, error) {
	const q = `
		INSERT INTO settings (key, value, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()
		RETURNING key, value, updated_at`
	var s model.Setting
	if err := r.db.QueryRow(ctx, q, key, value).Scan(&s.Key, &s.Value, &s.UpdatedAt); err != nil {
		return nil, err
	}
	return &s, nil
}
