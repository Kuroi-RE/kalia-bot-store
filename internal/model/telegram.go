package model

import "time"

// TelegramMenu is a dynamic command menu (may trigger the order flow).
type TelegramMenu struct {
	ID        int64     `json:"id"`
	Command   string    `json:"command"`
	Title     string    `json:"title"`
	ReplyText string    `json:"reply_text"`
	IsEnabled bool      `json:"is_enabled"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TelegramResponse is a static command reply (text-only).
type TelegramResponse struct {
	ID        int64     `json:"id"`
	Command   string    `json:"command"`
	ReplyText string    `json:"reply_text"`
	IsEnabled bool      `json:"is_enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
