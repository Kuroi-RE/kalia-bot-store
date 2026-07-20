// Package inventory holds concurrency-sensitive allocation/reservation logic.
package inventory

import (
	"context"
	"time"

	"github.com/kalia/store/internal/model"
	"github.com/kalia/store/internal/repository"
)

// Allocator reserves inventory accounts safely under concurrency.
type Allocator struct {
	tx *repository.TxManager
}

// NewAllocator builds an allocator over a transaction manager.
func NewAllocator(tx *repository.TxManager) *Allocator {
	return &Allocator{tx: tx}
}

// Reserve atomically reserves one AVAILABLE account for the product in its own
// transaction, returning repository.ErrNoStock when none is available. This is
// the standalone primitive; the order flow performs the same reservation inside
// its larger transaction via ReserveInTx.
func (a *Allocator) Reserve(ctx context.Context, productID int64, orderID *int64, reservedUntil time.Time) (*model.Account, error) {
	var reserved *model.Account
	err := a.tx.WithTx(ctx, func(db repository.DBTX) error {
		acc, err := repository.NewAccountRepository(db).ReserveOneAvailable(ctx, productID, orderID, reservedUntil)
		if err != nil {
			return err
		}
		reserved = acc
		return nil
	})
	if err != nil {
		return nil, err
	}
	return reserved, nil
}

// ReserveInTx reserves one AVAILABLE account using the provided transaction,
// so callers can compose reservation with other writes (e.g. order insertion)
// atomically.
func ReserveInTx(ctx context.Context, db repository.DBTX, productID int64, orderID *int64, reservedUntil time.Time) (*model.Account, error) {
	return repository.NewAccountRepository(db).ReserveOneAvailable(ctx, productID, orderID, reservedUntil)
}
