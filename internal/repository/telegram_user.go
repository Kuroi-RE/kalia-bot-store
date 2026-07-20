package repository

import (
	"context"

	"github.com/kalia/store/internal/model"
)

// TelegramUserRepository provides access to telegram_users.
type TelegramUserRepository struct{ db DBTX }

// NewTelegramUserRepository builds a telegram user repository over db.
func NewTelegramUserRepository(db DBTX) *TelegramUserRepository {
	return &TelegramUserRepository{db: db}
}

const telegramUserColumns = `id, telegram_id, username, first_name, created_at`

func scanTelegramUser(row interface{ Scan(dest ...any) error }) (*model.TelegramUser, error) {
	var u model.TelegramUser
	if err := row.Scan(&u.ID, &u.TelegramID, &u.Username, &u.FirstName, &u.CreatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

// Upsert inserts or updates a telegram user by telegram_id, returning the row.
func (r *TelegramUserRepository) Upsert(ctx context.Context, u *model.TelegramUser) (*model.TelegramUser, error) {
	const q = `
		INSERT INTO telegram_users (telegram_id, username, first_name)
		VALUES ($1, $2, $3)
		ON CONFLICT (telegram_id) DO UPDATE
			SET username = EXCLUDED.username, first_name = EXCLUDED.first_name
		RETURNING ` + telegramUserColumns
	return scanTelegramUser(r.db.QueryRow(ctx, q, u.TelegramID, u.Username, u.FirstName))
}

// GetByTelegramID fetches a user by their telegram id.
func (r *TelegramUserRepository) GetByTelegramID(ctx context.Context, telegramID int64) (*model.TelegramUser, error) {
	u, err := scanTelegramUser(r.db.QueryRow(ctx, `SELECT `+telegramUserColumns+` FROM telegram_users WHERE telegram_id = $1`, telegramID))
	if IsNotFound(err) {
		return nil, ErrNotFound
	}
	return u, err
}

// GetByID fetches a user by internal id.
func (r *TelegramUserRepository) GetByID(ctx context.Context, id int64) (*model.TelegramUser, error) {
	u, err := scanTelegramUser(r.db.QueryRow(ctx, `SELECT `+telegramUserColumns+` FROM telegram_users WHERE id = $1`, id))
	if IsNotFound(err) {
		return nil, ErrNotFound
	}
	return u, err
}
