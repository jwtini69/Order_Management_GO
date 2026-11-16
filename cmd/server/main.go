package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kira/order-management/internal/config"
	"github.com/kira/order-management/internal/database"
	"github.com/kira/order-management/internal/handlers"
	"github.com/kira/order-management/internal/repository"
	"github.com/kira/order-management/internal/router"
	"github.com/kira/order-management/internal/services"
)

func main() {
	// Setup structured JSON logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting Inventory & Order Management System...")

	// 1. Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}
	slog.Info("Configuration loaded successfully",
		"port", cfg.ServerPort,
		"db_host", cfg.DBHost,
		"db_name", cfg.DBName,
	)

	// 2. Connect to database with retry loop
	slog.Info("Connecting to PostgreSQL database...")
	dbPool, err := connectWithRetry(cfg, 5, 3*time.Second)
	if err != nil {
		slog.Error("Database connection failed after retries", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()
	slog.Info("PostgreSQL connection pool established")

	// 3. Run database migrations (apply schema)
	if err := runMigrations(dbPool); err != nil {
		slog.Error("Migration failed", "error", err)
		os.Exit(1)
	}
	slog.Info("Database schema verified and up-to-date")

	// 4. Initialize Repositories
	userRepo := repository.NewUserRepository(dbPool)
	productRepo := repository.NewProductRepository(dbPool)
	inventoryRepo := repository.NewInventoryRepository(dbPool)
	orderRepo := repository.NewOrderRepository(dbPool, inventoryRepo)

	// 5. Initialize Services
	authService := services.NewAuthService(userRepo, cfg.JWTSecret)
	productService := services.NewProductService(productRepo)
	inventoryService := services.NewInventoryService(inventoryRepo)
	orderService := services.NewOrderService(orderRepo)

	// 6. Initialize Handlers
	authHandler := handlers.NewAuthHandler(authService)
	productHandler := handlers.NewProductHandler(productService)
	inventoryHandler := handlers.NewInventoryHandler(inventoryService)
	orderHandler := handlers.NewOrderHandler(orderService)

	// 7. Setup Router
	r := router.SetupRouter(router.RouterConfig{
		AuthHandler:      authHandler,
		ProductHandler:   productHandler,
		InventoryHandler: inventoryHandler,
		OrderHandler:     orderHandler,
		JWTSecret:        cfg.JWTSecret,
	})

	// 8. Start HTTP Server with Graceful Shutdown
	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("HTTP server listening", "address", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	// Listen for shutdown signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("Received shutdown signal", "signal", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("Server stopped cleanly. Goodbye!")
}

// connectWithRetry attempts to establish a connection pool with retries.
func connectWithRetry(cfg *config.Config, maxRetries int, delay time.Duration) (*pgxpool.Pool, error) {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		pool, err := database.NewPostgresPool(cfg)
		if err == nil {
			return pool, nil
		}
		slog.Warn("Failed to connect to database, retrying...",
			"attempt", attempt,
			"max_retries", maxRetries,
			"error", err,
		)
		time.Sleep(delay)
	}
	return nil, fmt.Errorf("could not connect to database after %d attempts", maxRetries)
}

// runMigrations executes the SQL migrations schema
func runMigrations(pool *pgxpool.Pool) error {
	migrationPath := "migrations/000001_init_schema.up.sql"
	sqlBytes, err := os.ReadFile(migrationPath)
	if err != nil {
		// If running from different directory, check relative
		return fmt.Errorf("failed to read migration file %s: %w", migrationPath, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = pool.Exec(ctx, string(sqlBytes))
	if err != nil {
		return fmt.Errorf("failed to execute schema migration: %w", err)
	}

	return nil
}
