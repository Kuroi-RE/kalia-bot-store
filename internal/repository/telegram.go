package repository

import (
	"context"

	"github.com/kalia/store/internal/model"
)

// ---- Menus ----

// MenuRepository provides access to telegram_menus.
type MenuRepository struct{ db DBTX }

// NewMenuRepository builds a menu repository over db.
func NewMenuRepository(db DBTX) *MenuRepository { return &MenuRepository{db: db} }

const menuColumns = `id, command, title, reply_text, is_enabled, sort_order, created_at, updated_at`

func scanMenu(row interface{ Scan(dest ...any) error }) (*model.TelegramMenu, error) {
	var m model.TelegramMenu
	if err := row.Scan(&m.ID, &m.Command, &m.Title, &m.ReplyText, &m.IsEnabled, &m.SortOrder, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return nil, err
	}
	return &m, nil
}

// Create inserts a menu.
func (r *MenuRepository) Create(ctx context.Context, m *model.TelegramMenu) (*model.TelegramMenu, error) {
	const q = `
		INSERT INTO telegram_menus (command, title, reply_text, is_enabled, sort_order)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING ` + menuColumns
	return scanMenu(r.db.QueryRow(ctx, q, m.Command, m.Title, m.ReplyText, m.IsEnabled, m.SortOrder))
}

// GetByID fetches a menu by id.
func (r *MenuRepository) GetByID(ctx context.Context, id int64) (*model.TelegramMenu, error) {
	m, err := scanMenu(r.db.QueryRow(ctx, `SELECT `+menuColumns+` FROM telegram_menus WHERE id = $1`, id))
	if IsNotFound(err) {
		return nil, ErrNotFound
	}
	return m, err
}

// List returns menus. When enabledOnly, only enabled menus are returned,
// ordered by sort_order (bot rendering order).
func (r *MenuRepository) List(ctx context.Context, enabledOnly bool) ([]model.TelegramMenu, error) {
	q := `SELECT ` + menuColumns + ` FROM telegram_menus`
	if enabledOnly {
		q += ` WHERE is_enabled = TRUE`
	}
	q += ` ORDER BY sort_order, id`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []model.TelegramMenu
	for rows.Next() {
		m, err := scanMenu(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *m)
	}
	return items, rows.Err()
}

// Update modifies mutable menu fields.
func (r *MenuRepository) Update(ctx context.Context, m *model.TelegramMenu) (*model.TelegramMenu, error) {
	const q = `
		UPDATE telegram_menus
		SET command = $2, title = $3, reply_text = $4, sort_order = $5, updated_at = now()
		WHERE id = $1
		RETURNING ` + menuColumns
	res, err := scanMenu(r.db.QueryRow(ctx, q, m.ID, m.Command, m.Title, m.ReplyText, m.SortOrder))
	if IsNotFound(err) {
		return nil, ErrNotFound
	}
	return res, err
}

// SetEnabled toggles a menu's enabled state.
func (r *MenuRepository) SetEnabled(ctx context.Context, id int64, enabled bool) (*model.TelegramMenu, error) {
	const q = `UPDATE telegram_menus SET is_enabled = $2, updated_at = now() WHERE id = $1 RETURNING ` + menuColumns
	res, err := scanMenu(r.db.QueryRow(ctx, q, id, enabled))
	if IsNotFound(err) {
		return nil, ErrNotFound
	}
	return res, err
}

// Delete removes a menu.
func (r *MenuRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM telegram_menus WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- Responses ----

// ResponseRepository provides access to telegram_responses.
type ResponseRepository struct{ db DBTX }

// NewResponseRepository builds a response repository over db.
func NewResponseRepository(db DBTX) *ResponseRepository { return &ResponseRepository{db: db} }

const responseColumns = `id, command, reply_text, is_enabled, created_at, updated_at`

func scanResponse(row interface{ Scan(dest ...any) error }) (*model.TelegramResponse, error) {
	var r model.TelegramResponse
	if err := row.Scan(&r.ID, &r.Command, &r.ReplyText, &r.IsEnabled, &r.CreatedAt, &r.UpdatedAt); err != nil {
		return nil, err
	}
	return &r, nil
}

// Create inserts a response.
func (r *ResponseRepository) Create(ctx context.Context, resp *model.TelegramResponse) (*model.TelegramResponse, error) {
	const q = `
		INSERT INTO telegram_responses (command, reply_text, is_enabled)
		VALUES ($1, $2, $3)
		RETURNING ` + responseColumns
	return scanResponse(r.db.QueryRow(ctx, q, resp.Command, resp.ReplyText, resp.IsEnabled))
}

// GetByID fetches a response by id.
func (r *ResponseRepository) GetByID(ctx context.Context, id int64) (*model.TelegramResponse, error) {
	resp, err := scanResponse(r.db.QueryRow(ctx, `SELECT `+responseColumns+` FROM telegram_responses WHERE id = $1`, id))
	if IsNotFound(err) {
		return nil, ErrNotFound
	}
	return resp, err
}

// GetByCommand fetches an enabled response for a command (bot resolution).
func (r *ResponseRepository) GetByCommand(ctx context.Context, command string) (*model.TelegramResponse, error) {
	resp, err := scanResponse(r.db.QueryRow(ctx, `SELECT `+responseColumns+` FROM telegram_responses WHERE command = $1 AND is_enabled = TRUE`, command))
	if IsNotFound(err) {
		return nil, ErrNotFound
	}
	return resp, err
}

// List returns responses, optionally only enabled ones.
func (r *ResponseRepository) List(ctx context.Context, enabledOnly bool) ([]model.TelegramResponse, error) {
	q := `SELECT ` + responseColumns + ` FROM telegram_responses`
	if enabledOnly {
		q += ` WHERE is_enabled = TRUE`
	}
	q += ` ORDER BY command`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []model.TelegramResponse
	for rows.Next() {
		resp, err := scanResponse(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *resp)
	}
	return items, rows.Err()
}

// Update modifies a response's reply text and command.
func (r *ResponseRepository) Update(ctx context.Context, resp *model.TelegramResponse) (*model.TelegramResponse, error) {
	const q = `
		UPDATE telegram_responses
		SET command = $2, reply_text = $3, updated_at = now()
		WHERE id = $1
		RETURNING ` + responseColumns
	res, err := scanResponse(r.db.QueryRow(ctx, q, resp.ID, resp.Command, resp.ReplyText))
	if IsNotFound(err) {
		return nil, ErrNotFound
	}
	return res, err
}

// SetEnabled toggles a response's enabled state.
func (r *ResponseRepository) SetEnabled(ctx context.Context, id int64, enabled bool) (*model.TelegramResponse, error) {
	const q = `UPDATE telegram_responses SET is_enabled = $2, updated_at = now() WHERE id = $1 RETURNING ` + responseColumns
	res, err := scanResponse(r.db.QueryRow(ctx, q, id, enabled))
	if IsNotFound(err) {
		return nil, ErrNotFound
	}
	return res, err
}

// Delete removes a response.
func (r *ResponseRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM telegram_responses WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
