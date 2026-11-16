package models

import "time"

type Product struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	CategoryID  *int      `json:"category_id,omitempty"`
	SKU         string    `json:"sku"`
	ImageURL    string    `json:"image_url,omitempty"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateProductRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description string  `json:"description"`
	Price       float64 `json:"price" binding:"required,gt=0"`
	CategoryID  *int    `json:"category_id"`
	SKU         string  `json:"sku" binding:"required"`
	ImageURL    string  `json:"image_url"`
}

type UpdateProductRequest struct {
	Name        *string  `json:"name"`
	Description *string  `json:"description"`
	Price       *float64 `json:"price"`
	CategoryID  *int     `json:"category_id"`
	SKU         *string  `json:"sku"`
	ImageURL    *string  `json:"image_url"`
	IsActive    *bool    `json:"is_active"`
}

type ProductFilter struct {
	CategoryID *int
	Search     string
	MinPrice   *float64
	MaxPrice   *float64
	IsActive   *bool
	Page       int
	PageSize   int
	SortBy     string
	SortOrder  string
}

type Category struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateCategoryRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}
