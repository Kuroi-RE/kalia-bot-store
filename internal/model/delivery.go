package model

import "time"

// Delivery records a one-time credential dispatch for a paid order.
type Delivery struct {
	ID          int64          `json:"id"`
	OrderID     int64          `json:"order_id"`
	AccountID   int64          `json:"account_id"`
	Status      DeliveryStatus `json:"status"`
	Attempts    int            `json:"attempts"`
	LastError   string         `json:"last_error,omitempty"`
	DeliveredAt *time.Time     `json:"delivered_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}
