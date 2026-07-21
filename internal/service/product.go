package service

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kalia/store/internal/model"
	"github.com/kalia/store/internal/repository"
	"github.com/kalia/store/pkg/apperr"
)

// ProductService holds product catalog business logic.
type ProductService struct {
	products *repository.ProductRepository
	tx       *repository.TxManager
}

// NewProductService builds a product service.
func NewProductService(products *repository.ProductRepository, tx *repository.TxManager) *ProductService {
	return &ProductService{products: products, tx: tx}
}

// CreateInput carries fields for creating a product.
type CreateProductInput struct {
	Name             string
	Description      string
	BasePrice        int64
	IsActive         bool
	CredentialSchema model.CredentialSchema
}

// UpdateProductInput carries fields for updating a product.
type UpdateProductInput struct {
	Name             string
	Description      string
	BasePrice        int64
	CredentialSchema model.CredentialSchema
}

// validateSchema ensures credential field keys are present and unique.
func validateSchema(schema model.CredentialSchema) error {
	seen := make(map[string]struct{}, len(schema))
	for i := range schema {
		key := strings.TrimSpace(schema[i].Key)
		if key == "" {
			return apperr.BadRequest("credential_schema: each field requires a non-empty key")
		}
		if _, dup := seen[key]; dup {
			return apperr.BadRequest("credential_schema: duplicate field key '" + key + "'")
		}
		seen[key] = struct{}{}
		schema[i].Key = key
		if schema[i].Type == "" {
			schema[i].Type = "string"
		}
		if schema[i].Label == "" {
			schema[i].Label = key
		}
	}
	return nil
}

// ensureTypeField guarantees the schema contains a required "type" field, used
// to group inventory in the bot catalog. If missing it is prepended; if present
// it is forced to required so it is always captured when adding accounts.
func ensureTypeField(schema model.CredentialSchema) model.CredentialSchema {
	for i := range schema {
		if strings.EqualFold(strings.TrimSpace(schema[i].Key), "type") {
			schema[i].Key = "type"
			schema[i].Required = true
			if schema[i].Label == "" {
				schema[i].Label = "Type"
			}
			if schema[i].Type == "" {
				schema[i].Type = "string"
			}
			return schema
		}
	}
	typeField := model.CredentialField{Key: "type", Label: "Type", Type: "string", Required: true}
	return append(model.CredentialSchema{typeField}, schema...)
}

// Create validates and inserts a new product.
func (s *ProductService) Create(ctx context.Context, in CreateProductInput) (*model.Product, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, apperr.BadRequest("name is required")
	}
	if in.BasePrice < 0 {
		return nil, apperr.BadRequest("base_price must be >= 0")
	}
	in.CredentialSchema = ensureTypeField(in.CredentialSchema)
	if err := validateSchema(in.CredentialSchema); err != nil {
		return nil, err
	}
	p := &model.Product{
		Name:             strings.TrimSpace(in.Name),
		Description:      in.Description,
		BasePrice:        in.BasePrice,
		IsActive:         in.IsActive,
		CredentialSchema: in.CredentialSchema,
	}
	created, err := s.products.Create(ctx, p)
	if err != nil {
		return nil, apperr.Internal("could not create product").Wrap(err)
	}
	return created, nil
}

// ListForBot returns active, in-stock products as a public catalog listing
// (name + description + price + available count). Out-of-stock products are
// omitted so the bot never offers something that can't be bought.
func (s *ProductService) ListForBot(ctx context.Context) ([]model.BotProductListing, error) {
	rows, err := s.products.ListActiveWithAvailability(ctx)
	if err != nil {
		return nil, apperr.Internal("could not list products").Wrap(err)
	}
	out := make([]model.BotProductListing, 0, len(rows))
	for _, r := range rows {
		if r.Available <= 0 {
			continue
		}
		out = append(out, model.BotProductListing{
			ProductID:   r.ID,
			Name:        r.Name,
			Description: r.Description,
			Price:       r.BasePrice,
			Available:   r.Available,
		})
	}
	return out, nil
}

// Get returns a product by id.
func (s *ProductService) Get(ctx context.Context, id int64) (*model.Product, error) {
	p, err := s.products.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("product not found")
		}
		return nil, apperr.Internal("lookup failed").Wrap(err)
	}
	return p, nil
}

// List returns products with optional active filter and pagination.
func (s *ProductService) List(ctx context.Context, isActive *bool, limit, offset int) ([]model.Product, int64, error) {
	items, total, err := s.products.List(ctx, repository.ProductListParams{
		IsActive: isActive,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return nil, 0, apperr.Internal("could not list products").Wrap(err)
	}
	return items, total, nil
}

// Update modifies mutable fields of an existing product.
func (s *ProductService) Update(ctx context.Context, id int64, in UpdateProductInput) (*model.Product, error) {
	existing, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, apperr.BadRequest("name is required")
	}
	if in.BasePrice < 0 {
		return nil, apperr.BadRequest("base_price must be >= 0")
	}
	in.CredentialSchema = ensureTypeField(in.CredentialSchema)
	if err := validateSchema(in.CredentialSchema); err != nil {
		return nil, err
	}
	existing.Name = strings.TrimSpace(in.Name)
	existing.Description = in.Description
	existing.BasePrice = in.BasePrice
	existing.CredentialSchema = in.CredentialSchema

	updated, err := s.products.Update(ctx, existing)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("product not found")
		}
		return nil, apperr.Internal("could not update product").Wrap(err)
	}
	return updated, nil
}

// SetActive enables or disables a product.
func (s *ProductService) SetActive(ctx context.Context, id int64, active bool) (*model.Product, error) {
	p, err := s.products.SetActive(ctx, id, active)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("product not found")
		}
		return nil, apperr.Internal("could not update product status").Wrap(err)
	}
	return p, nil
}

// Delete removes a product. Without force it fails (409) when any inventory
// references it. With force it deletes AVAILABLE/RESERVED accounts first, but
// still refuses when SOLD accounts exist (those are order/delivery history —
// disable the product instead).
func (s *ProductService) Delete(ctx context.Context, id int64, force bool) error {
	if force {
		return s.forceDelete(ctx, id)
	}
	err := s.products.Delete(ctx, id)
	if err == nil {
		return nil
	}
	if errors.Is(err, repository.ErrNotFound) {
		return apperr.NotFound("product not found")
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return apperr.Conflict("product has associated accounts; delete them first (or use force)")
	}
	return apperr.Internal("could not delete product").Wrap(err)
}

// forceDelete removes a product and everything tied to it — its orders (which
// cascade to their payments and deliveries) and its accounts — in one
// transaction. This is destructive of order history and is only reached via an
// explicit "force" request; prefer disabling a product to preserve history.
func (s *ProductService) forceDelete(ctx context.Context, id int64) error {
	return s.tx.WithTx(ctx, func(db repository.DBTX) error {
		// 1. Delete orders first; FK cascades remove their payments & deliveries
		//    and null out accounts.reserved_order_id references.
		if _, err := repository.NewOrderRepository(db).DeleteByProduct(ctx, id); err != nil {
			return apperr.Internal("could not delete product orders").Wrap(err)
		}
		// 2. Accounts are now free of delivery references and can be removed.
		if _, err := repository.NewAccountRepository(db).DeleteByProduct(ctx, id); err != nil {
			return apperr.Internal("could not delete product accounts").Wrap(err)
		}
		// 3. Finally the product itself.
		if err := repository.NewProductRepository(db).Delete(ctx, id); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return apperr.NotFound("product not found")
			}
			return apperr.Internal("could not delete product").Wrap(err)
		}
		return nil
	})
}
