package repository

import (
	"context"

	"github.com/kalia/store/internal/model"
)

// AdminRepository provides access to the admins table.
type AdminRepository struct{ db DBTX }

// NewAdminRepository builds an admin repository over db.
func NewAdminRepository(db DBTX) *AdminRepository { return &AdminRepository{db: db} }

// Count returns the number of admin accounts.
func (r *AdminRepository) Count(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM admins`).Scan(&n)
	return n, err
}

// Create inserts a new admin and returns it.
func (r *AdminRepository) Create(ctx context.Context, username, passwordHash string) (*model.Admin, error) {
	const q = `
		INSERT INTO admins (username, password_hash)
		VALUES ($1, $2)
		RETURNING id, username, password_hash, created_at, updated_at`
	var a model.Admin
	err := r.db.QueryRow(ctx, q, username, passwordHash).
		Scan(&a.ID, &a.Username, &a.PasswordHash, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// GetByUsername fetches an admin by username. Returns ErrNotFound if missing.
func (r *AdminRepository) GetByUsername(ctx context.Context, username string) (*model.Admin, error) {
	const q = `
		SELECT id, username, password_hash, created_at, updated_at
		FROM admins WHERE username = $1`
	var a model.Admin
	err := r.db.QueryRow(ctx, q, username).
		Scan(&a.ID, &a.Username, &a.PasswordHash, &a.CreatedAt, &a.UpdatedAt)
	if IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// GetByID fetches an admin by id.
func (r *AdminRepository) GetByID(ctx context.Context, id int64) (*model.Admin, error) {
	const q = `
		SELECT id, username, password_hash, created_at, updated_at
		FROM admins WHERE id = $1`
	var a model.Admin
	err := r.db.QueryRow(ctx, q, id).
		Scan(&a.ID, &a.Username, &a.PasswordHash, &a.CreatedAt, &a.UpdatedAt)
	if IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}
