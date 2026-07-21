package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/kalia/store/internal/model"
	"github.com/kalia/store/internal/repository"
	"github.com/kalia/store/pkg/apperr"
)

// AccountService holds inventory/account business logic.
type AccountService struct {
	accounts *repository.AccountRepository
	products *repository.ProductRepository
}

// NewAccountService builds an account service.
func NewAccountService(accounts *repository.AccountRepository, products *repository.ProductRepository) *AccountService {
	return &AccountService{accounts: accounts, products: products}
}

// validateCredentials ensures required schema fields are present and non-empty.
func validateCredentials(schema model.CredentialSchema, creds model.Credentials) error {
	for _, key := range schema.RequiredKeys() {
		v, ok := creds[key]
		if !ok {
			return apperr.BadRequest(fmt.Sprintf("credentials: missing required field '%s'", key))
		}
		if s, isStr := v.(string); isStr && s == "" {
			return apperr.BadRequest(fmt.Sprintf("credentials: field '%s' must not be empty", key))
		}
		if v == nil {
			return apperr.BadRequest(fmt.Sprintf("credentials: field '%s' must not be null", key))
		}
	}
	return nil
}

// CreateAccounts adds one or more accounts to a product, validating each
// credential set against the product's schema. All-or-nothing within a caller
// transaction is not required here (independent rows), but validation is done
// up front so a bad batch is rejected before any insert.
func (s *AccountService) CreateAccounts(ctx context.Context, productID int64, credsList []model.Credentials) ([]model.Account, error) {
	product, err := s.getProduct(ctx, productID)
	if err != nil {
		return nil, err
	}
	if len(credsList) == 0 {
		return nil, apperr.BadRequest("at least one account is required")
	}
	for _, creds := range credsList {
		if err := validateCredentials(product.CredentialSchema, creds); err != nil {
			return nil, err
		}
	}

	created := make([]model.Account, 0, len(credsList))
	for _, creds := range credsList {
		a, err := s.accounts.Create(ctx, &model.Account{
			ProductID:   productID,
			Credentials: creds,
			Status:      model.AccountAvailable,
		})
		if err != nil {
			return nil, apperr.Internal("could not create account").Wrap(err)
		}
		created = append(created, *a)
	}
	return created, nil
}

// Get returns an account by id.
func (s *AccountService) Get(ctx context.Context, id int64) (*model.Account, error) {
	a, err := s.accounts.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("account not found")
		}
		return nil, apperr.Internal("lookup failed").Wrap(err)
	}
	return a, nil
}

// ListByProduct lists accounts of a product with optional status filter.
func (s *AccountService) ListByProduct(ctx context.Context, productID int64, status *model.AccountStatus, limit, offset int) ([]model.Account, int64, error) {
	if _, err := s.getProduct(ctx, productID); err != nil {
		return nil, 0, err
	}
	items, total, err := s.accounts.ListByProduct(ctx, repository.AccountListParams{
		ProductID: productID,
		Status:    status,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		return nil, 0, apperr.Internal("could not list accounts").Wrap(err)
	}
	return items, total, nil
}

// Update replaces an account's credentials and status.
func (s *AccountService) Update(ctx context.Context, id int64, creds model.Credentials, status model.AccountStatus) (*model.Account, error) {
	existing, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	product, err := s.getProduct(ctx, existing.ProductID)
	if err != nil {
		return nil, err
	}
	if creds == nil {
		creds = existing.Credentials
	}
	if err := validateCredentials(product.CredentialSchema, creds); err != nil {
		return nil, err
	}
	if status == "" {
		status = existing.Status
	}
	if !isValidAccountStatus(status) {
		return nil, apperr.BadRequest("invalid account status")
	}
	a, err := s.accounts.Update(ctx, id, creds, status)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("account not found")
		}
		return nil, apperr.Internal("could not update account").Wrap(err)
	}
	return a, nil
}

// Delete removes an account.
func (s *AccountService) Delete(ctx context.Context, id int64) error {
	err := s.accounts.Delete(ctx, id)
	if err == nil {
		return nil
	}
	if errors.Is(err, repository.ErrNotFound) {
		return apperr.NotFound("account not found")
	}
	if repository.IsForeignKeyViolation(err) {
		return apperr.Conflict("account has a delivery/order record and can't be deleted; it may already be sold")
	}
	return apperr.Internal("could not delete account").Wrap(err)
}

// Summary returns inventory counts for a product.
func (s *AccountService) Summary(ctx context.Context, productID int64) (*model.InventorySummary, error) {
	if _, err := s.getProduct(ctx, productID); err != nil {
		return nil, err
	}
	sum, err := s.accounts.Summary(ctx, productID)
	if err != nil {
		return nil, apperr.Internal("could not compute inventory summary").Wrap(err)
	}
	return sum, nil
}

// ListCatalogForBot returns available accounts grouped by product + type for
// the bot catalog (e.g. "Account - premium - Rp 50.000").
func (s *AccountService) ListCatalogForBot(ctx context.Context) ([]model.BotCatalogItem, error) {
	items, err := s.accounts.ListAvailableByType(ctx)
	if err != nil {
		return nil, apperr.Internal("could not list catalog").Wrap(err)
	}
	return items, nil
}

// ListAvailableForBot returns available accounts across active products as a
// safe public listing (label only — never secret credentials). Used by the bot
// to show selectable accounts by username.
func (s *AccountService) ListAvailableForBot(ctx context.Context, limit int) ([]model.BotAccountListing, error) {
	rows, err := s.accounts.ListAvailableWithProduct(ctx, limit)
	if err != nil {
		return nil, apperr.Internal("could not list available accounts").Wrap(err)
	}
	out := make([]model.BotAccountListing, 0, len(rows))
	for _, r := range rows {
		out = append(out, model.BotAccountListing{
			AccountID:   r.AccountID,
			ProductID:   r.ProductID,
			ProductName: r.ProductName,
			Price:       r.BasePrice,
			Label:       pickPublicLabel(r.Credentials, r.AccountID),
		})
	}
	return out, nil
}

// pickPublicLabel derives a non-sensitive display label for an account. It
// prefers a handle-like field and NEVER exposes email/password.
func pickPublicLabel(creds model.Credentials, accountID int64) string {
	for _, key := range []string{"username", "handle", "name", "user", "title"} {
		if v, ok := creds[key]; ok {
			if str, isStr := v.(string); isStr && str != "" {
				return str
			}
		}
	}
	return fmt.Sprintf("Account #%d", accountID)
}

// getProduct looks up a product for internal use.
func (s *AccountService) getProduct(ctx context.Context, productID int64) (*model.Product, error) {
	p, err := s.products.GetByID(ctx, productID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("product not found")
		}
		return nil, apperr.Internal("product lookup failed").Wrap(err)
	}
	return p, nil
}

func isValidAccountStatus(s model.AccountStatus) bool {
	switch s {
	case model.AccountAvailable, model.AccountReserved, model.AccountSold:
		return true
	}
	return false
}
