package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kira/order-management/internal/models"
)

type ProductRepository struct {
	pool *pgxpool.Pool
}

func NewProductRepository(pool *pgxpool.Pool) *ProductRepository {
	return &ProductRepository{pool: pool}
}

func (r *ProductRepository) Create(ctx context.Context, product *models.Product) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO products (name, description, price, category_id, sku, image_url)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, is_active, created_at, updated_at`

	err = tx.QueryRow(ctx, query,
		product.Name,
		product.Description,
		product.Price,
		product.CategoryID,
		product.SKU,
		product.ImageURL,
	).Scan(&product.ID, &product.IsActive, &product.CreatedAt, &product.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "sku") {
			return models.ErrDuplicateSKU
		}
		return fmt.Errorf("failed to create product: %w", err)
	}

	// Auto-create inventory entry with quantity 0
	inventoryQuery := `
		INSERT INTO inventory (product_id, quantity, reserved_quantity, min_stock_level)
		VALUES ($1, 0, 0, 0)`

	_, err = tx.Exec(ctx, inventoryQuery, product.ID)
	if err != nil {
		return fmt.Errorf("failed to create inventory entry: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *ProductRepository) GetByID(ctx context.Context, id int) (*models.Product, error) {
	query := `
		SELECT id, name, description, price, category_id, sku, image_url, is_active, created_at, updated_at
		FROM products
		WHERE id = $1`

	product := &models.Product{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&product.ID,
		&product.Name,
		&product.Description,
		&product.Price,
		&product.CategoryID,
		&product.SKU,
		&product.ImageURL,
		&product.IsActive,
		&product.CreatedAt,
		&product.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	return product, nil
}

func (r *ProductRepository) Update(ctx context.Context, id int, req *models.UpdateProductRequest) (*models.Product, error) {
	setClauses := []string{}
	args := []interface{}{}
	argIndex := 1

	if req.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argIndex))
		args = append(args, *req.Name)
		argIndex++
	}
	if req.Description != nil {
		setClauses = append(setClauses, fmt.Sprintf("description = $%d", argIndex))
		args = append(args, *req.Description)
		argIndex++
	}
	if req.Price != nil {
		setClauses = append(setClauses, fmt.Sprintf("price = $%d", argIndex))
		args = append(args, *req.Price)
		argIndex++
	}
	if req.CategoryID != nil {
		setClauses = append(setClauses, fmt.Sprintf("category_id = $%d", argIndex))
		args = append(args, *req.CategoryID)
		argIndex++
	}
	if req.SKU != nil {
		setClauses = append(setClauses, fmt.Sprintf("sku = $%d", argIndex))
		args = append(args, *req.SKU)
		argIndex++
	}
	if req.ImageURL != nil {
		setClauses = append(setClauses, fmt.Sprintf("image_url = $%d", argIndex))
		args = append(args, *req.ImageURL)
		argIndex++
	}
	if req.IsActive != nil {
		setClauses = append(setClauses, fmt.Sprintf("is_active = $%d", argIndex))
		args = append(args, *req.IsActive)
		argIndex++
	}

	if len(setClauses) == 0 {
		return r.GetByID(ctx, id)
	}

	setClauses = append(setClauses, "updated_at = NOW()")
	args = append(args, id)

	query := fmt.Sprintf(`
		UPDATE products
		SET %s
		WHERE id = $%d
		RETURNING id, name, description, price, category_id, sku, image_url, is_active, created_at, updated_at`,
		strings.Join(setClauses, ", "), argIndex)

	product := &models.Product{}
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&product.ID,
		&product.Name,
		&product.Description,
		&product.Price,
		&product.CategoryID,
		&product.SKU,
		&product.ImageURL,
		&product.IsActive,
		&product.CreatedAt,
		&product.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "sku") {
			return nil, models.ErrDuplicateSKU
		}
		return nil, fmt.Errorf("failed to update product: %w", err)
	}

	return product, nil
}

func (r *ProductRepository) Delete(ctx context.Context, id int) error {
	query := `
		UPDATE products
		SET is_active = false, updated_at = NOW()
		WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to soft delete product: %w", err)
	}

	if result.RowsAffected() == 0 {
		return models.ErrNotFound
	}

	return nil
}

func (r *ProductRepository) List(ctx context.Context, filter models.ProductFilter) ([]models.Product, int, error) {
	// Apply defaults
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}

	whereClauses := []string{}
	args := []interface{}{}
	argIndex := 1

	if filter.CategoryID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("p.category_id = $%d", argIndex))
		args = append(args, *filter.CategoryID)
		argIndex++
	}
	if filter.Search != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("p.name ILIKE $%d", argIndex))
		args = append(args, "%"+filter.Search+"%")
		argIndex++
	}
	if filter.MinPrice != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("p.price >= $%d", argIndex))
		args = append(args, *filter.MinPrice)
		argIndex++
	}
	if filter.MaxPrice != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("p.price <= $%d", argIndex))
		args = append(args, *filter.MaxPrice)
		argIndex++
	}
	if filter.IsActive != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("p.is_active = $%d", argIndex))
		args = append(args, *filter.IsActive)
		argIndex++
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM products p %s", whereSQL)
	var total int
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count products: %w", err)
	}

	// Determine sort
	sortColumn := "p.created_at"
	switch filter.SortBy {
	case "name":
		sortColumn = "p.name"
	case "price":
		sortColumn = "p.price"
	case "created_at":
		sortColumn = "p.created_at"
	}

	sortOrder := "ASC"
	if strings.EqualFold(filter.SortOrder, "desc") {
		sortOrder = "DESC"
	}

	offset := (filter.Page - 1) * filter.PageSize

	args = append(args, filter.PageSize)
	limitIdx := argIndex
	argIndex++
	args = append(args, offset)
	offsetIdx := argIndex

	query := fmt.Sprintf(`
		SELECT p.id, p.name, p.description, p.price, p.category_id, p.sku, p.image_url, p.is_active, p.created_at, p.updated_at
		FROM products p
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d`,
		whereSQL, sortColumn, sortOrder, limitIdx, offsetIdx)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list products: %w", err)
	}
	defer rows.Close()

	products := []models.Product{}
	for rows.Next() {
		var p models.Product
		err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Description,
			&p.Price,
			&p.CategoryID,
			&p.SKU,
			&p.ImageURL,
			&p.IsActive,
			&p.CreatedAt,
			&p.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan product: %w", err)
		}
		products = append(products, p)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating products: %w", err)
	}

	return products, total, nil
}

func (r *ProductRepository) CreateCategory(ctx context.Context, cat *models.Category) error {
	query := `
		INSERT INTO categories (name, description)
		VALUES ($1, $2)
		RETURNING id, created_at`

	err := r.pool.QueryRow(ctx, query, cat.Name, cat.Description).Scan(&cat.ID, &cat.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create category: %w", err)
	}

	return nil
}

func (r *ProductRepository) ListCategories(ctx context.Context) ([]models.Category, error) {
	query := `SELECT id, name, description, created_at FROM categories ORDER BY name`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list categories: %w", err)
	}
	defer rows.Close()

	categories := []models.Category{}
	for rows.Next() {
		var c models.Category
		err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}
		categories = append(categories, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating categories: %w", err)
	}

	return categories, nil
}
