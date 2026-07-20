package repository

import (
	"context"
	"encoding/json"

	"github.com/kalia/store/internal/model"
)

// ProductRepository provides access to the products table.
type ProductRepository struct{ db DBTX }

// NewProductRepository builds a product repository over db.
func NewProductRepository(db DBTX) *ProductRepository { return &ProductRepository{db: db} }

// ProductListParams filters and paginates a product listing.
type ProductListParams struct {
	IsActive *bool
	Limit    int
	Offset   int
}

const productColumns = `id, name, description, base_price, is_active, credential_schema, created_at, updated_at`

func scanProduct(row interface {
	Scan(dest ...any) error
}) (*model.Product, error) {
	var p model.Product
	var schemaBytes []byte
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &p.BasePrice, &p.IsActive, &schemaBytes, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	if len(schemaBytes) > 0 {
		if err := json.Unmarshal(schemaBytes, &p.CredentialSchema); err != nil {
			return nil, err
		}
	}
	return &p, nil
}

// Create inserts a product.
func (r *ProductRepository) Create(ctx context.Context, p *model.Product) (*model.Product, error) {
	schema, err := p.CredentialSchema.MarshalJSONB()
	if err != nil {
		return nil, err
	}
	const q = `
		INSERT INTO products (name, description, base_price, is_active, credential_schema)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING ` + productColumns
	return scanProduct(r.db.QueryRow(ctx, q, p.Name, p.Description, p.BasePrice, p.IsActive, schema))
}

// GetByID fetches a product by id.
func (r *ProductRepository) GetByID(ctx context.Context, id int64) (*model.Product, error) {
	const q = `SELECT ` + productColumns + ` FROM products WHERE id = $1`
	p, err := scanProduct(r.db.QueryRow(ctx, q, id))
	if IsNotFound(err) {
		return nil, ErrNotFound
	}
	return p, err
}

// List returns products matching the params plus the total count.
func (r *ProductRepository) List(ctx context.Context, params ProductListParams) ([]model.Product, int64, error) {
	if params.Limit <= 0 || params.Limit > 200 {
		params.Limit = 50
	}

	// Total count with the same filter.
	var total int64
	countQ := `SELECT count(*) FROM products WHERE ($1::boolean IS NULL OR is_active = $1)`
	if err := r.db.QueryRow(ctx, countQ, params.IsActive).Scan(&total); err != nil {
		return nil, 0, err
	}

	const q = `
		SELECT ` + productColumns + `
		FROM products
		WHERE ($1::boolean IS NULL OR is_active = $1)
		ORDER BY id DESC
		LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, q, params.IsActive, params.Limit, params.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *p)
	}
	return items, total, rows.Err()
}

// Update modifies mutable product fields (name, description, price, schema).
func (r *ProductRepository) Update(ctx context.Context, p *model.Product) (*model.Product, error) {
	schema, err := p.CredentialSchema.MarshalJSONB()
	if err != nil {
		return nil, err
	}
	const q = `
		UPDATE products
		SET name = $2, description = $3, base_price = $4, credential_schema = $5, updated_at = now()
		WHERE id = $1
		RETURNING ` + productColumns
	res, err := scanProduct(r.db.QueryRow(ctx, q, p.ID, p.Name, p.Description, p.BasePrice, schema))
	if IsNotFound(err) {
		return nil, ErrNotFound
	}
	return res, err
}

// SetActive enables or disables a product.
func (r *ProductRepository) SetActive(ctx context.Context, id int64, active bool) (*model.Product, error) {
	const q = `
		UPDATE products SET is_active = $2, updated_at = now()
		WHERE id = $1
		RETURNING ` + productColumns
	res, err := scanProduct(r.db.QueryRow(ctx, q, id, active))
	if IsNotFound(err) {
		return nil, ErrNotFound
	}
	return res, err
}

// Delete removes a product by id. Returns ErrNotFound if it did not exist.
func (r *ProductRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM products WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
