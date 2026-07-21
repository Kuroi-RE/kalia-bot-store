package model

import (
	"encoding/json"
	"time"
)

// Credentials holds a product account's credential values (JSONB).
type Credentials map[string]any

// Account is an inventory unit belonging to a product.
type Account struct {
	ID              int64         `json:"id"`
	ProductID       int64         `json:"product_id"`
	Credentials     Credentials   `json:"credentials"`
	Status          AccountStatus `json:"status"`
	ReservedOrderID *int64        `json:"reserved_order_id,omitempty"`
	ReservedUntil   *time.Time    `json:"reserved_until,omitempty"`
	SoldAt          *time.Time    `json:"sold_at,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

// MarshalJSONB serializes credentials for a JSONB column.
func (c Credentials) MarshalJSONB() ([]byte, error) {
	if c == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(c)
}

// InventorySummary reports account counts by status for a product.
type InventorySummary struct {
	ProductID int64 `json:"product_id"`
	Available int64 `json:"available"`
	Reserved  int64 `json:"reserved"`
	Sold      int64 `json:"sold"`
	Total     int64 `json:"total"`
}

// BotCatalogItem is a catalog entry grouped by account "type" (e.g. premium),
// shown to the bot as "ProductName - Type - Price".
type BotCatalogItem struct {
	ProductID   int64  `json:"product_id"`
	ProductName string `json:"product_name"`
	Type        string `json:"type"`
	Price       int64  `json:"price"`
	Available   int64  `json:"available"`
}

// BotProductListing is a public product entry for the bot catalog: name +
// description + price, with how many accounts are in stock.
type BotProductListing struct {
	ProductID   int64  `json:"product_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Price       int64  `json:"price"`
	Available   int64  `json:"available"`
}

// BotAccountListing is a public, safe view of an available account for the bot
// catalog. It intentionally excludes secret credentials (email/password) — only
// a display label (e.g. the Twitter username) is exposed before purchase.
type BotAccountListing struct {
	AccountID   int64  `json:"account_id"`
	ProductID   int64  `json:"product_id"`
	ProductName string `json:"product_name"`
	Price       int64  `json:"price"`
	Label       string `json:"label"`
}
