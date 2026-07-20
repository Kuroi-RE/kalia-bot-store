package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kalia/store/internal/inventory"
	"github.com/kalia/store/internal/model"
	"github.com/kalia/store/internal/repository"
)

// TestConcurrentAllocationNoDoubleSell spins up many concurrent reservers
// against a limited pool of accounts and asserts that each account is reserved
// at most once and that over-demand yields ErrNoStock.
func TestConcurrentAllocationNoDoubleSell(t *testing.T) {
	c := newTestContainer(t)
	ctx := context.Background()

	tm := repository.NewTxManager(c.Pool)
	productRepo := repository.NewProductRepository(c.Pool)
	accountRepo := repository.NewAccountRepository(c.Pool)

	// Fresh product with no credential schema (no validation needed here).
	product, err := productRepo.Create(ctx, &model.Product{
		Name:      "concurrency-test",
		BasePrice: 1000,
		IsActive:  true,
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}

	const stock = 8
	const workers = 40
	for i := 0; i < stock; i++ {
		if _, err := accountRepo.Create(ctx, &model.Account{
			ProductID:   product.ID,
			Credentials: model.Credentials{"email": "x"},
			Status:      model.AccountAvailable,
		}); err != nil {
			t.Fatalf("seed account: %v", err)
		}
	}

	allocator := inventory.NewAllocator(tm)
	reservedUntil := time.Now().Add(10 * time.Minute)

	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		reservedIDs = map[int64]int{}
		noStock     int
		otherErr    error
	)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// nil orderID: this test exercises the allocation primitive itself,
			// not order linkage (reserved_order_id stays NULL, satisfying the FK).
			acc, err := allocator.Reserve(ctx, product.ID, nil, reservedUntil)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				reservedIDs[acc.ID]++
			case errors.Is(err, repository.ErrNoStock):
				noStock++
			default:
				otherErr = err
			}
		}()
	}
	wg.Wait()

	if otherErr != nil {
		t.Fatalf("unexpected error during allocation: %v", otherErr)
	}

	// Exactly `stock` successful reservations.
	if len(reservedIDs) != stock {
		t.Fatalf("expected %d distinct reserved accounts, got %d", stock, len(reservedIDs))
	}
	// No account reserved more than once (the core no-double-sell guarantee).
	for id, count := range reservedIDs {
		if count != 1 {
			t.Fatalf("account %d reserved %d times (double-sell!)", id, count)
		}
	}
	// The rest are out of stock.
	if noStock != workers-stock {
		t.Fatalf("expected %d out-of-stock results, got %d", workers-stock, noStock)
	}

	// All reserved accounts are actually RESERVED in the DB.
	summary, err := accountRepo.Summary(ctx, product.ID)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.Reserved != stock || summary.Available != 0 {
		t.Fatalf("expected %d reserved / 0 available, got reserved=%d available=%d", stock, summary.Reserved, summary.Available)
	}
}
