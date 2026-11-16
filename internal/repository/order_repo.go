package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kira/order-management/internal/models"
)

type OrderRepository struct {
	pool         *pgxpool.Pool
	inventoryRepo *InventoryRepository
}

func NewOrderRepository(pool *pgxpool.Pool, inventoryRepo *InventoryRepository) *OrderRepository {
	return &OrderRepository{
		pool:         pool,
		inventoryRepo: inventoryRepo,
	}
}

// validStatusTransitions defines allowed order status transitions.
var validStatusTransitions = map[models.OrderStatus][]models.OrderStatus{
	models.OrderStatusPending:    {models.OrderStatusConfirmed, models.OrderStatusCancelled},
	models.OrderStatusConfirmed:  {models.OrderStatusProcessing, models.OrderStatusCancelled},
	models.OrderStatusProcessing: {models.OrderStatusShipped, models.OrderStatusCancelled},
	models.OrderStatusShipped:    {models.OrderStatusDelivered},
	models.OrderStatusDelivered:  {models.OrderStatusRefunded},
	models.OrderStatusCancelled:  {},
	models.OrderStatusRefunded:   {},
}

// Create inserts a new order with all its items in a single serializable transaction.
// For each item it calls CheckAndReserveStock (SELECT ... FOR UPDATE) to lock
// inventory rows and prevent overselling under concurrent purchases.
func (r *OrderRepository) Create(ctx context.Context, order *models.Order) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.Serializable,
	})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Phase 1: Reserve stock for each item and determine prices
	var totalAmount float64
	for i := range order.Items {
		item := &order.Items[i]
		price, err := r.inventoryRepo.CheckAndReserveStock(ctx, tx, item.ProductID, item.Quantity)
		if err != nil {
			return fmt.Errorf("failed to reserve stock for product %d: %w", item.ProductID, err)
		}
		item.UnitPrice = price
		item.TotalPrice = price * float64(item.Quantity)
		totalAmount += item.TotalPrice
	}
	order.TotalAmount = totalAmount

	// Phase 2: Insert the order
	orderQuery := `
		INSERT INTO orders (user_id, status, total_amount, shipping_address)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at`

	err = tx.QueryRow(ctx, orderQuery,
		order.UserID,
		models.OrderStatusPending,
		order.TotalAmount,
		order.ShippingAddress,
	).Scan(&order.ID, &order.CreatedAt, &order.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to insert order: %w", err)
	}
	order.Status = models.OrderStatusPending

	// Phase 3: Insert all order items
	itemQuery := `
		INSERT INTO order_items (order_id, product_id, quantity, unit_price, total_price)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`

	for i := range order.Items {
		item := &order.Items[i]
		item.OrderID = order.ID
		err = tx.QueryRow(ctx, itemQuery,
			item.OrderID,
			item.ProductID,
			item.Quantity,
			item.UnitPrice,
			item.TotalPrice,
		).Scan(&item.ID)

		if err != nil {
			return fmt.Errorf("failed to insert order item: %w", err)
		}
	}

	// Phase 4: Commit
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit order transaction: %w", err)
	}

	return nil
}

func (r *OrderRepository) GetByID(ctx context.Context, id int) (*models.Order, error) {
	orderQuery := `
		SELECT id, user_id, status, total_amount, shipping_address, created_at, updated_at
		FROM orders
		WHERE id = $1`

	order := &models.Order{}
	err := r.pool.QueryRow(ctx, orderQuery, id).Scan(
		&order.ID,
		&order.UserID,
		&order.Status,
		&order.TotalAmount,
		&order.ShippingAddress,
		&order.CreatedAt,
		&order.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	// Fetch order items
	itemsQuery := `
		SELECT id, order_id, product_id, quantity, unit_price, total_price
		FROM order_items
		WHERE order_id = $1`

	rows, err := r.pool.Query(ctx, itemsQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get order items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item models.OrderItem
		err := rows.Scan(
			&item.ID,
			&item.OrderID,
			&item.ProductID,
			&item.Quantity,
			&item.UnitPrice,
			&item.TotalPrice,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order item: %w", err)
		}
		order.Items = append(order.Items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating order items: %w", err)
	}

	return order, nil
}

func (r *OrderRepository) ListByUserID(ctx context.Context, userID int, page, pageSize int) ([]models.Order, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	// Count total
	var total int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM orders WHERE user_id = $1", userID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count user orders: %w", err)
	}

	offset := (page - 1) * pageSize

	query := `
		SELECT id, user_id, status, total_amount, shipping_address, created_at, updated_at
		FROM orders
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query, userID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list user orders: %w", err)
	}
	defer rows.Close()

	orders := []models.Order{}
	for rows.Next() {
		var o models.Order
		err := rows.Scan(
			&o.ID,
			&o.UserID,
			&o.Status,
			&o.TotalAmount,
			&o.ShippingAddress,
			&o.CreatedAt,
			&o.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, o)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating orders: %w", err)
	}

	return orders, total, nil
}

func (r *OrderRepository) ListAll(ctx context.Context, page, pageSize int) ([]models.Order, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	var total int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM orders").Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count orders: %w", err)
	}

	offset := (page - 1) * pageSize

	query := `
		SELECT id, user_id, status, total_amount, shipping_address, created_at, updated_at
		FROM orders
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := r.pool.Query(ctx, query, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list orders: %w", err)
	}
	defer rows.Close()

	orders := []models.Order{}
	for rows.Next() {
		var o models.Order
		err := rows.Scan(
			&o.ID,
			&o.UserID,
			&o.Status,
			&o.TotalAmount,
			&o.ShippingAddress,
			&o.CreatedAt,
			&o.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, o)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating orders: %w", err)
	}

	return orders, total, nil
}

func (r *OrderRepository) UpdateStatus(ctx context.Context, id int, status models.OrderStatus) error {
	// Get current status
	var currentStatus models.OrderStatus
	err := r.pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", id).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ErrNotFound
		}
		return fmt.Errorf("failed to get order status: %w", err)
	}

	// Validate transition
	allowedTransitions, exists := validStatusTransitions[currentStatus]
	if !exists {
		return models.ErrInvalidOrderStatus
	}

	valid := false
	for _, allowed := range allowedTransitions {
		if allowed == status {
			valid = true
			break
		}
	}
	if !valid {
		return models.ErrInvalidOrderStatus
	}

	updateQuery := `
		UPDATE orders
		SET status = $1, updated_at = NOW()
		WHERE id = $2`

	result, err := r.pool.Exec(ctx, updateQuery, status, id)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return models.ErrNotFound
	}

	return nil
}

// CancelOrder cancels an order and releases all reserved stock in a transaction.
func (r *OrderRepository) CancelOrder(ctx context.Context, id int, userID int) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get order and verify ownership
	var currentStatus models.OrderStatus
	var orderUserID int
	err = tx.QueryRow(ctx,
		"SELECT status, user_id FROM orders WHERE id = $1 FOR UPDATE", id,
	).Scan(&currentStatus, &orderUserID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ErrNotFound
		}
		return fmt.Errorf("failed to get order: %w", err)
	}

	if orderUserID != userID {
		return models.ErrForbidden
	}

	// Validate transition
	allowedTransitions := validStatusTransitions[currentStatus]
	valid := false
	for _, allowed := range allowedTransitions {
		if allowed == models.OrderStatusCancelled {
			valid = true
			break
		}
	}
	if !valid {
		return models.ErrInvalidOrderStatus
	}

	// Get order items to release stock
	itemsQuery := `SELECT product_id, quantity FROM order_items WHERE order_id = $1`
	rows, err := tx.Query(ctx, itemsQuery, id)
	if err != nil {
		return fmt.Errorf("failed to get order items: %w", err)
	}
	defer rows.Close()

	type itemInfo struct {
		productID int
		quantity  int
	}
	var items []itemInfo
	for rows.Next() {
		var info itemInfo
		if err := rows.Scan(&info.productID, &info.quantity); err != nil {
			return fmt.Errorf("failed to scan order item: %w", err)
		}
		items = append(items, info)
	}
	rows.Close()

	// Release reserved stock for each item
	for _, item := range items {
		if err := r.inventoryRepo.ReleaseReservedStock(ctx, tx, item.productID, item.quantity); err != nil {
			return fmt.Errorf("failed to release stock for product %d: %w", item.productID, err)
		}
	}

	// Update order status
	_, err = tx.Exec(ctx,
		"UPDATE orders SET status = $1, updated_at = NOW() WHERE id = $2",
		models.OrderStatusCancelled, id,
	)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit cancel order: %w", err)
	}

	return nil
}

// RefundOrder refunds a delivered order: updates status to refunded, returns
// stock to inventory, and creates return transaction records.
func (r *OrderRepository) RefundOrder(ctx context.Context, id int) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get current status
	var currentStatus models.OrderStatus
	err = tx.QueryRow(ctx,
		"SELECT status FROM orders WHERE id = $1 FOR UPDATE", id,
	).Scan(&currentStatus)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ErrNotFound
		}
		return fmt.Errorf("failed to get order: %w", err)
	}

	// Only delivered orders can be refunded
	if currentStatus != models.OrderStatusDelivered {
		return models.ErrInvalidOrderStatus
	}

	// Get order items
	itemsQuery := `SELECT product_id, quantity FROM order_items WHERE order_id = $1`
	rows, err := tx.Query(ctx, itemsQuery, id)
	if err != nil {
		return fmt.Errorf("failed to get order items: %w", err)
	}
	defer rows.Close()

	type itemInfo struct {
		productID int
		quantity  int
	}
	var items []itemInfo
	for rows.Next() {
		var info itemInfo
		if err := rows.Scan(&info.productID, &info.quantity); err != nil {
			return fmt.Errorf("failed to scan order item: %w", err)
		}
		items = append(items, info)
	}
	rows.Close()

	// Return stock and create return transaction for each item
	for _, item := range items {
		// Add quantity back to inventory
		_, err = tx.Exec(ctx,
			"UPDATE inventory SET quantity = quantity + $1 WHERE product_id = $2",
			item.quantity, item.productID,
		)
		if err != nil {
			return fmt.Errorf("failed to return stock for product %d: %w", item.productID, err)
		}

		// Create return transaction record
		_, err = tx.Exec(ctx,
			`INSERT INTO inventory_transactions (product_id, type, quantity, reference)
			 VALUES ($1, $2, $3, $4)`,
			item.productID, models.TransactionReturn, item.quantity,
			fmt.Sprintf("refund for order #%d", id),
		)
		if err != nil {
			return fmt.Errorf("failed to create return transaction: %w", err)
		}
	}

	// Update order status
	_, err = tx.Exec(ctx,
		"UPDATE orders SET status = $1, updated_at = NOW() WHERE id = $2",
		models.OrderStatusRefunded, id,
	)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit refund order: %w", err)
	}

	return nil
}
