package models

import "time"

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Item struct {
	ID          int64      `json:"id"`
	UserID      int64      `json:"user_id"`
	CategoryID  *int64     `json:"category_id,omitempty"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	ItemType    string     `json:"item_type"` // "lost" or "found"
	Status      string     `json:"status"`    // "active", "claimed", "returned"
	Location    string     `json:"location,omitempty"`
	ImageURL    string     `json:"image_url,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Category struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type Claim struct {
	ID          int64     `json:"id"`
	ItemID      int64     `json:"item_id"`
	RequesterID int64     `json:"requester_id"`
	Status      string    `json:"status"`
	Message     string    `json:"message,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type CreateItemRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	ItemType    string `json:"item_type"`
	Location    string `json:"location,omitempty"`
	CategoryID  *int64 `json:"category_id,omitempty"`
	ImageURL    string `json:"image_url,omitempty"`
}

type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}
