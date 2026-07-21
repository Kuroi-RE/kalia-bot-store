package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/kalia/store/internal/model"
)

// AccountRepository provides access to the accounts table.
type AccountRepository struct{ db DBTX }

// NewAccountRepository builds an account repository over db.
func NewAccountRepository(db DBTX) *AccountRepository { return &AccountRepository{db: db} }

const accountColumns = `id, product_id, credentials, status, reserved_order_id, reserved_until, sold_at, created_at, updated_at`

func scanAccount(row interface {
	Scan(dest ...any) error
}) (*model.Account, error) {
	var a model.Account
	var credBytes []byte
	if err := row.Scan(&a.ID, &a.ProductID, &credBytes, &a.Status, &a.ReservedOrderID, &a.ReservedUntil, &a.SoldAt, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return nil, err
	}
	if len(credBytes) > 0 {
		if err := json.Unmarshal(credBytes, &a.Credentials); err != nil {
			return nil, err
		}
	}
	return &a, nil
}

// AccountListParams filters accounts of a product.
type AccountListParams struct {
	ProductID int64
	Status    *model.AccountStatus
	Limit     int
	Offset    int
}

// AvailableAccount is an AVAILABLE account joined with its product, used to
// build the bot-facing selectable list.
type AvailableAccount struct {
	AccountID   int64
	ProductID   int64
	ProductName string
	BasePrice   int64
	Credentials model.Credentials
}

// ListAvailableWithProduct returns AVAILABLE accounts of ACTIVE products,
// joined with product name and price, ordered for stable display.
func (r *AccountRepository) ListAvailableWithProduct(ctx context.Context, limit int) ([]AvailableAccount, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	const q = `
		SELECT a.id, a.product_id, p.name, p.base_price, a.credentials
		FROM accounts a
		JOIN products p ON p.id = a.product_id
		WHERE a.status = 'AVAILABLE' AND p.is_active = TRUE
		ORDER BY p.name, a.id
		LIMIT $1`
	rows, err := r.db.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AvailableAccount
	for rows.Next() {
		var a AvailableAccount
		var credBytes []byte
		if err := rows.Scan(&a.AccountID, &a.ProductID, &a.ProductName, &a.BasePrice, &credBytes); err != nil {
			return nil, err
		}
		if len(credBytes) > 0 {
			_ = json.Unmarshal(credBytes, &a.Credentials)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ListAvailableByType groups AVAILABLE accounts of ACTIVE products by their
// credential "type" field, returning one row per (product, type) with a count.
func (r *AccountRepository) ListAvailableByType(ctx context.Context) ([]model.BotCatalogItem, error) {
	const q = `
		SELECT a.product_id, p.name, COALESCE(a.credentials->>'type', ''), p.base_price, count(*)
		FROM accounts a
		JOIN products p ON p.id = a.product_id
		WHERE a.status = 'AVAILABLE' AND p.is_active = TRUE
		GROUP BY a.product_id, p.name, COALESCE(a.credentials->>'type', ''), p.base_price
		ORDER BY p.name, 3`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.BotCatalogItem
	for rows.Next() {
		var it model.BotCatalogItem
		if err := rows.Scan(&it.ProductID, &it.ProductName, &it.Type, &it.Price, &it.Available); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// ReserveOneAvailableOfType reserves one AVAILABLE account of the given product
// AND credential type, using FOR UPDATE SKIP LOCKED. An empty accType matches
// accounts with no "type" field. Returns ErrNoStock when none is available.
// Must run in a transaction.
func (r *AccountRepository) ReserveOneAvailableOfType(ctx context.Context, productID int64, accType string, orderID *int64, reservedUntil time.Time) (*model.Account, error) {
	const sel = `
		SELECT id FROM accounts
		WHERE product_id = $1 AND status = 'AVAILABLE' AND COALESCE(credentials->>'type', '') = $2
		ORDER BY created_at, id
		LIMIT 1
		FOR UPDATE SKIP LOCKED`
	var id int64
	err := r.db.QueryRow(ctx, sel, productID, accType).Scan(&id)
	if IsNotFound(err) {
		return nil, ErrNoStock
	}
	if err != nil {
		return nil, err
	}
	const upd = `
		UPDATE accounts
		SET status = 'RESERVED', reserved_order_id = $2, reserved_until = $3, updated_at = now()
		WHERE id = $1
		RETURNING ` + accountColumns
	return scanAccount(r.db.QueryRow(ctx, upd, id, orderID, reservedUntil))
}

// ReserveSpecificAvailable reserves a specific account by id if it is still
// AVAILABLE, using FOR UPDATE so concurrent buyers of the same account can't
// both win. Returns ErrNoStock when it is no longer available. Must run in a tx.
func (r *AccountRepository) ReserveSpecificAvailable(ctx context.Context, accountID int64, orderID *int64, reservedUntil time.Time) (*model.Account, error) {
	const sel = `SELECT id FROM accounts WHERE id = $1 AND status = 'AVAILABLE' FOR UPDATE`
	var id int64
	err := r.db.QueryRow(ctx, sel, accountID).Scan(&id)
	if IsNotFound(err) {
		return nil, ErrNoStock
	}
	if err != nil {
		return nil, err
	}
	const upd = `
		UPDATE accounts
		SET status = 'RESERVED', reserved_order_id = $2, reserved_until = $3, updated_at = now()
		WHERE id = $1
		RETURNING ` + accountColumns
	return scanAccount(r.db.QueryRow(ctx, upd, id, orderID, reservedUntil))
}

// Create inserts an account.
func (r *AccountRepository) Create(ctx context.Context, a *model.Account) (*model.Account, error) {
	creds, err := a.Credentials.MarshalJSONB()
	if err != nil {
		return nil, err
	}
	status := a.Status
	if status == "" {
		status = model.AccountAvailable
	}
	const q = `
		INSERT INTO accounts (product_id, credentials, status)
		VALUES ($1, $2, $3)
		RETURNING ` + accountColumns
	return scanAccount(r.db.QueryRow(ctx, q, a.ProductID, creds, status))
}

// GetByID fetches an account by id.
func (r *AccountRepository) GetByID(ctx context.Context, id int64) (*model.Account, error) {
	const q = `SELECT ` + accountColumns + ` FROM accounts WHERE id = $1`
	a, err := scanAccount(r.db.QueryRow(ctx, q, id))
	if IsNotFound(err) {
		return nil, ErrNotFound
	}
	return a, err
}

// ListByProduct returns accounts of a product with optional status filter.
func (r *AccountRepository) ListByProduct(ctx context.Context, params AccountListParams) ([]model.Account, int64, error) {
	if params.Limit <= 0 || params.Limit > 200 {
		params.Limit = 50
	}

	var total int64
	countQ := `SELECT count(*) FROM accounts WHERE product_id = $1 AND ($2::account_status IS NULL OR status = $2)`
	if err := r.db.QueryRow(ctx, countQ, params.ProductID, params.Status).Scan(&total); err != nil {
		return nil, 0, err
	}

	const q = `
		SELECT ` + accountColumns + `
		FROM accounts
		WHERE product_id = $1 AND ($2::account_status IS NULL OR status = $2)
		ORDER BY id DESC
		LIMIT $3 OFFSET $4`
	rows, err := r.db.Query(ctx, q, params.ProductID, params.Status, params.Limit, params.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.Account
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *a)
	}
	return items, total, rows.Err()
}

// Update sets credentials and status of an account.
func (r *AccountRepository) Update(ctx context.Context, id int64, creds model.Credentials, status model.AccountStatus) (*model.Account, error) {
	credBytes, err := creds.MarshalJSONB()
	if err != nil {
		return nil, err
	}
	const q = `
		UPDATE accounts
		SET credentials = $2, status = $3, updated_at = now()
		WHERE id = $1
		RETURNING ` + accountColumns
	a, err := scanAccount(r.db.QueryRow(ctx, q, id, credBytes, status))
	if IsNotFound(err) {
		return nil, ErrNotFound
	}
	return a, err
}

// Delete removes an account by id.
func (r *AccountRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM accounts WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteByProduct removes all accounts for a product, returning the count.
// Callers must ensure no SOLD accounts remain (deliveries FK would block them).
func (r *AccountRepository) DeleteByProduct(ctx context.Context, productID int64) (int64, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM accounts WHERE product_id = $1`, productID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ReserveOneAvailable atomically reserves a single AVAILABLE account for the
// product using SELECT ... FOR UPDATE SKIP LOCKED, so concurrent callers grab
// different rows (or none) instead of blocking or double-allocating. It must be
// called inside a transaction (r.db is a pgx.Tx). Returns ErrNoStock when no
// AVAILABLE account exists.
func (r *AccountRepository) ReserveOneAvailable(ctx context.Context, productID int64, orderID *int64, reservedUntil time.Time) (*model.Account, error) {
	const selectQ = `
		SELECT id
		FROM accounts
		WHERE product_id = $1 AND status = 'AVAILABLE'
		ORDER BY created_at, id
		LIMIT 1
		FOR UPDATE SKIP LOCKED`
	var id int64
	err := r.db.QueryRow(ctx, selectQ, productID).Scan(&id)
	if IsNotFound(err) {
		return nil, ErrNoStock
	}
	if err != nil {
		return nil, err
	}

	const updateQ = `
		UPDATE accounts
		SET status = 'RESERVED', reserved_order_id = $2, reserved_until = $3, updated_at = now()
		WHERE id = $1
		RETURNING ` + accountColumns
	return scanAccount(r.db.QueryRow(ctx, updateQ, id, orderID, reservedUntil))
}

// ReleaseReservation returns a RESERVED account to AVAILABLE, clearing its
// reservation fields. Used by cleanup jobs and order cancellation.
func (r *AccountRepository) ReleaseReservation(ctx context.Context, id int64) error {
	const q = `
		UPDATE accounts
		SET status = 'AVAILABLE', reserved_order_id = NULL, reserved_until = NULL, updated_at = now()
		WHERE id = $1 AND status = 'RESERVED'`
	_, err := r.db.Exec(ctx, q, id)
	return err
}

// MarkSold flips a reserved account to SOLD. Used on delivery.
func (r *AccountRepository) MarkSold(ctx context.Context, id int64) error {
	const q = `
		UPDATE accounts
		SET status = 'SOLD', sold_at = now(), updated_at = now()
		WHERE id = $1`
	_, err := r.db.Exec(ctx, q, id)
	return err
}

// Summary returns per-status counts for a product's inventory.
func (r *AccountRepository) Summary(ctx context.Context, productID int64) (*model.InventorySummary, error) {
	const q = `
		SELECT
			count(*) FILTER (WHERE status = 'AVAILABLE') AS available,
			count(*) FILTER (WHERE status = 'RESERVED')  AS reserved,
			count(*) FILTER (WHERE status = 'SOLD')      AS sold,
			count(*)                                     AS total
		FROM accounts WHERE product_id = $1`
	s := &model.InventorySummary{ProductID: productID}
	if err := r.db.QueryRow(ctx, q, productID).Scan(&s.Available, &s.Reserved, &s.Sold, &s.Total); err != nil {
		return nil, err
	}
	return s, nil
}
