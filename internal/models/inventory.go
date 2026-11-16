package models

import "time"

type InventoryItem struct {
	ID               int        `json:"id"`
	ProductID        int        `json:"product_id"`
	Quantity         int        `json:"quantity"`
	ReservedQuantity int        `json:"reserved_quantity"`
	MinStockLevel    int        `json:"min_stock_level"`
	LastRestockedAt  *time.Time `json:"last_restocked_at,omitempty"`
}

type TransactionType string

const (
	TransactionRestock    TransactionType = "restock"
	TransactionSale       TransactionType = "sale"
	TransactionAdjustment TransactionType = "adjustment"
	TransactionReturn     TransactionType = "return"
)

type InventoryTransaction struct {
	ID        int             `json:"id"`
	ProductID int             `json:"product_id"`
	Type      TransactionType `json:"type"`
	Quantity  int             `json:"quantity"`
	Reference string          `json:"reference"`
	CreatedAt time.Time       `json:"created_at"`
}

type UpdateInventoryRequest struct {
	Quantity      *int `json:"quantity"`
	MinStockLevel *int `json:"min_stock_level"`
}

type RestockRequest struct {
	Quantity  int    `json:"quantity" binding:"required,gt=0"`
	Reference string `json:"reference"`
}
