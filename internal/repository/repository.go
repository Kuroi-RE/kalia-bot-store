package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX abstracts *pgxpool.Pool and pgx.Tx so repositories work inside or
// outside an explicit transaction.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// TxManager owns the pool and provides transactional execution.
type TxManager struct {
	pool *pgxpool.Pool
}

// NewTxManager builds a transaction manager.
func NewTxManager(pool *pgxpool.Pool) *TxManager { return &TxManager{pool: pool} }

// DB returns the pool as a DBTX for non-transactional queries.
func (m *TxManager) DB() DBTX { return m.pool }

// Pool exposes the raw pool.
func (m *TxManager) Pool() *pgxpool.Pool { return m.pool }

// WithTx runs fn inside a transaction, committing on success and rolling back
// on error or panic. The callback receives the transaction as a DBTX.
func (m *TxManager) WithTx(ctx context.Context, fn func(tx DBTX) error) error {
	pgtx, err := m.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			_ = pgtx.Rollback(ctx)
			panic(p)
		}
	}()

	if err := fn(pgtx); err != nil {
		_ = pgtx.Rollback(ctx)
		return err
	}
	return pgtx.Commit(ctx)
}
