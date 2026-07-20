package model

// AccountStatus mirrors the account_status enum.
type AccountStatus string

const (
	AccountAvailable AccountStatus = "AVAILABLE"
	AccountReserved  AccountStatus = "RESERVED"
	AccountSold      AccountStatus = "SOLD"
)

// OrderStatus mirrors the order_status enum.
type OrderStatus string

const (
	OrderPending   OrderStatus = "PENDING"
	OrderPaid      OrderStatus = "PAID"
	OrderDelivered OrderStatus = "DELIVERED"
	OrderExpired   OrderStatus = "EXPIRED"
	OrderCancelled OrderStatus = "CANCELLED"
	OrderFailed    OrderStatus = "FAILED"
)

// PaymentStatus mirrors the payment_status enum.
type PaymentStatus string

const (
	PaymentPending    PaymentStatus = "PENDING"
	PaymentSettlement PaymentStatus = "SETTLEMENT"
	PaymentExpired    PaymentStatus = "EXPIRED"
	PaymentDenied     PaymentStatus = "DENIED"
	PaymentCancelled  PaymentStatus = "CANCELLED"
)

// DeliveryStatus mirrors the delivery_status enum.
type DeliveryStatus string

const (
	DeliveryPending   DeliveryStatus = "PENDING"
	DeliveryDelivered DeliveryStatus = "DELIVERED"
	DeliveryFailed    DeliveryStatus = "FAILED"
)
