package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kira/order-management/internal/models"
)

type InventoryRepository struct {
	pool *pgxpool.Pool
}

func NewInventoryRepository(pool *pgxpool.Pool) *InventoryRepository {
	return &InventoryRepository{pool: pool}
}

func (r *InventoryRepository) GetByProductID(ctx context.Context, productID int) (*models.InventoryItem, error) {
	query := `
		SELECT id, product_id, quantity, reserved_quantity, min_stock_level, last_restocked_at
		FROM inventory
		WHERE product_id = $1`

	item := &models.InventoryItem{}
	err := r.pool.QueryRow(ctx, query, productID).Scan(
		&item.ID,
		&item.ProductID,
		&item.Quantity,
		&item.ReservedQuantity,
		&item.MinStockLevel,
		&item.LastRestockedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get inventory for product %d: %w", productID, err)
	}

	return item, nil
}

func (r *InventoryRepository) UpdateStock(ctx context.Context, productID int, req *models.UpdateInventoryRequest) (*models.InventoryItem, error) {
	setClauses := []string{}
	args := []interface{}{}
	argIndex := 1

	if req.Quantity != nil {
		setClauses = append(setClauses, fmt.Sprintf("quantity = $%d", argIndex))
		args = append(args, *req.Quantity)
		argIndex++
	}
	if req.MinStockLevel != nil {
		setClauses = append(setClauses, fmt.Sprintf("min_stock_level = $%d", argIndex))
		args = append(args, *req.MinStockLevel)
		argIndex++
	}

	if len(setClauses) == 0 {
		return r.GetByProductID(ctx, productID)
	}

	args = append(args, productID)

	query := fmt.Sprintf(`
		UPDATE inventory
		SET %s
		WHERE product_id = $%d
		RETURNING id, product_id, quantity, reserved_quantity, min_stock_level, last_restocked_at`,
		joinStrings(setClauses, ", "), argIndex)

	item := &models.InventoryItem{}
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&item.ID,
		&item.ProductID,
		&item.Quantity,
		&item.ReservedQuantity,
		&item.MinStockLevel,
		&item.LastRestockedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to update inventory: %w", err)
	}

	return item, nil
}

func (r *InventoryRepository) Restock(ctx context.Context, productID int, req *models.RestockRequest) (*models.InventoryItem, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now()

	updateQuery := `
		UPDATE inventory
		SET quantity = quantity + $1, last_restocked_at = $2
		WHERE product_id = $3
		RETURNING id, product_id, quantity, reserved_quantity, min_stock_level, last_restocked_at`

	item := &models.InventoryItem{}
	err = tx.QueryRow(ctx, updateQuery, req.Quantity, now, productID).Scan(
		&item.ID,
		&item.ProductID,
		&item.Quantity,
		&item.ReservedQuantity,
		&item.MinStockLevel,
		&item.LastRestockedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to restock inventory: %w", err)
	}

	// Create transaction record
	txQuery := `
		INSERT INTO inventory_transactions (product_id, type, quantity, reference)
		VALUES ($1, $2, $3, $4)`

	_, err = tx.Exec(ctx, txQuery, productID, models.TransactionRestock, req.Quantity, req.Reference)
	if err != nil {
		return nil, fmt.Errorf("failed to create restock transaction: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit restock: %w", err)
	}

	return item, nil
}

func (r *InventoryRepository) GetTransactions(ctx context.Context, productID int) ([]models.InventoryTransaction, error) {
	query := `
		SELECT id, product_id, type, quantity, reference, created_at
		FROM inventory_transactions
		WHERE product_id = $1
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, productID)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory transactions: %w", err)
	}
	defer rows.Close()

	transactions := []models.InventoryTransaction{}
	for rows.Next() {
		var t models.InventoryTransaction
		err := rows.Scan(&t.ID, &t.ProductID, &t.Type, &t.Quantity, &t.Reference, &t.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		transactions = append(transactions, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating transactions: %w", err)
	}

	return transactions, nil
}

func (r *InventoryRepository) ListAll(ctx context.Context) ([]models.InventoryItem, error) {
	query := `
		SELECT i.id, i.product_id, i.quantity, i.reserved_quantity, i.min_stock_level, i.last_restocked_at
		FROM inventory i
		JOIN products p ON p.id = i.product_id
		ORDER BY p.name`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list inventory: %w", err)
	}
	defer rows.Close()

	items := []models.InventoryItem{}
	for rows.Next() {
		var item models.InventoryItem
		err := rows.Scan(
			&item.ID,
			&item.ProductID,
			&item.Quantity,
			&item.ReservedQuantity,
			&item.MinStockLevel,
			&item.LastRestockedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan inventory item: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating inventory: %w", err)
	}

	return items, nil
}

// CheckAndReserveStock locks the inventory row using SELECT ... FOR UPDATE,
// verifies sufficient stock, reserves the quantity, records a sale transaction,
// and returns the product price. Must be called within a transaction.
func (r *InventoryRepository) CheckAndReserveStock(ctx context.Context, tx pgx.Tx, productID int, quantity int) (float64, error) {
	// Lock the inventory row and get current stock + product price
	lockQuery := `
		SELECT i.quantity, i.reserved_quantity, p.price
		FROM inventory i
		JOIN products p ON p.id = i.product_id
		WHERE i.product_id = $1
		FOR UPDATE OF i`

	var currentQty, reservedQty int
	var price float64
	err := tx.QueryRow(ctx, lockQuery, productID).Scan(&currentQty, &reservedQty, &price)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, models.ErrNotFound
		}
		return 0, fmt.Errorf("failed to lock inventory row: %w", err)
	}

	// Check available stock (total - already reserved)
	available := currentQty - reservedQty
	if available < quantity {
		return 0, models.ErrInsufficientStock
	}

	// Reserve the stock
	updateQuery := `
		UPDATE inventory
		SET reserved_quantity = reserved_quantity + $1
		WHERE product_id = $2`

	_, err = tx.Exec(ctx, updateQuery, quantity, productID)
	if err != nil {
		return 0, fmt.Errorf("failed to reserve stock: %w", err)
	}

	// Record sale transaction
	txQuery := `
		INSERT INTO inventory_transactions (product_id, type, quantity, reference)
		VALUES ($1, $2, $3, $4)`

	_, err = tx.Exec(ctx, txQuery, productID, models.TransactionSale, quantity, fmt.Sprintf("reserved %d units", quantity))
	if err != nil {
		return 0, fmt.Errorf("failed to create sale transaction: %w", err)
	}

	return price, nil
}

// ReleaseReservedStock reverses a stock reservation (e.g., on order cancellation).
// Must be called within a transaction.
func (r *InventoryRepository) ReleaseReservedStock(ctx context.Context, tx pgx.Tx, productID int, quantity int) error {
	query := `
		UPDATE inventory
		SET reserved_quantity = reserved_quantity - $1
		WHERE product_id = $2 AND reserved_quantity >= $1`

	result, err := tx.Exec(ctx, query, quantity, productID)
	if err != nil {
		return fmt.Errorf("failed to release reserved stock: %w", err)
	}

	if result.RowsAffected() == 0 {
		return models.ErrNotFound
	}

	return nil
}

// ConfirmReservedStock moves reserved stock to sold: decreases both quantity
// and reserved_quantity. Must be called within a transaction.
func (r *InventoryRepository) ConfirmReservedStock(ctx context.Context, tx pgx.Tx, productID int, quantity int) error {
	query := `
		UPDATE inventory
		SET quantity = quantity - $1, reserved_quantity = reserved_quantity - $1
		WHERE product_id = $2 AND reserved_quantity >= $1 AND quantity >= $1`

	result, err := tx.Exec(ctx, query, quantity, productID)
	if err != nil {
		return fmt.Errorf("failed to confirm reserved stock: %w", err)
	}

	if result.RowsAffected() == 0 {
		return models.ErrNotFound
	}

	return nil
}

// joinStrings joins a slice of strings with the given separator.
func joinStrings(parts []string, sep string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += sep
		}
		result += part
	}
	return result
}
