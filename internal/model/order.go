package model

import "time"

// TelegramUser is a bot customer recorded by the backend.
type TelegramUser struct {
	ID         int64     `json:"id"`
	TelegramID int64     `json:"telegram_id"`
	Username   string    `json:"username"`
	FirstName  string    `json:"first_name"`
	CreatedAt  time.Time `json:"created_at"`
}

// Order coordinates a customer, product, reserved account, and payment.
type Order struct {
	ID             int64       `json:"id"`
	OrderRef       string      `json:"order_ref"`
	TelegramUserID int64       `json:"telegram_user_id"`
	ProductID      int64       `json:"product_id"`
	AccountID      *int64      `json:"account_id,omitempty"`
	Amount         int64       `json:"amount"`
	Status         OrderStatus `json:"status"`
	ExpiresAt      *time.Time  `json:"expires_at,omitempty"`
	PaidAt         *time.Time  `json:"paid_at,omitempty"`
	DeliveredAt    *time.Time  `json:"delivered_at,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}
