package model

import (
	"encoding/json"
	"time"
)

// CredentialField describes one credential input for a product's accounts.
type CredentialField struct {
	Key      string `json:"key" validate:"required"`
	Label    string `json:"label"`
	Type     string `json:"type"`     // string | secret | url | text
	Required bool   `json:"required"`
}

// CredentialSchema is the ordered set of credential fields for a product.
type CredentialSchema []CredentialField

// Product is a sellable catalog item.
type Product struct {
	ID               int64            `json:"id"`
	Name             string           `json:"name"`
	Description      string           `json:"description"`
	BasePrice        int64            `json:"base_price"`
	IsActive         bool             `json:"is_active"`
	CredentialSchema CredentialSchema `json:"credential_schema"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
}

// Keys returns the credential field keys in declared order.
func (s CredentialSchema) Keys() []string {
	keys := make([]string, len(s))
	for i, f := range s {
		keys[i] = f.Key
	}
	return keys
}

// RequiredKeys returns the keys of required credential fields.
func (s CredentialSchema) RequiredKeys() []string {
	var keys []string
	for _, f := range s {
		if f.Required {
			keys = append(keys, f.Key)
		}
	}
	return keys
}

// MarshalJSONB serializes the schema for a JSONB column.
func (s CredentialSchema) MarshalJSONB() ([]byte, error) {
	if s == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(s)
}
