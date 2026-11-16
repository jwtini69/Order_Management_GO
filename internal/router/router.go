package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kira/order-management/internal/handlers"
	"github.com/kira/order-management/internal/middleware"
)

type RouterConfig struct {
	AuthHandler      *handlers.AuthHandler
	ProductHandler   *handlers.ProductHandler
	InventoryHandler *handlers.InventoryHandler
	OrderHandler     *handlers.OrderHandler
	JWTSecret        string
}

func SetupRouter(cfg RouterConfig) *gin.Engine {
	r := gin.New()

	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "inventory-order-management-api",
		})
	})

	api := r.Group("/api/v1")
	{
		// Authentication routes
		auth := api.Group("/auth")
		{
			auth.POST("/register", cfg.AuthHandler.Register)
			auth.POST("/login", cfg.AuthHandler.Login)
			auth.GET("/profile", middleware.AuthMiddleware(cfg.JWTSecret), cfg.AuthHandler.GetProfile)
		}

		// Categories routes
		categories := api.Group("/categories")
		{
			categories.GET("", cfg.ProductHandler.ListCategories)
			categories.POST("", middleware.AuthMiddleware(cfg.JWTSecret), middleware.AdminOnly(), cfg.ProductHandler.CreateCategory)
		}

		// Products routes
		products := api.Group("/products")
		{
			products.GET("", cfg.ProductHandler.ListProducts)
			products.GET("/:id", cfg.ProductHandler.GetProduct)
			products.POST("", middleware.AuthMiddleware(cfg.JWTSecret), middleware.AdminOnly(), cfg.ProductHandler.CreateProduct)
			products.PUT("/:id", middleware.AuthMiddleware(cfg.JWTSecret), middleware.AdminOnly(), cfg.ProductHandler.UpdateProduct)
			products.DELETE("/:id", middleware.AuthMiddleware(cfg.JWTSecret), middleware.AdminOnly(), cfg.ProductHandler.DeleteProduct)
		}

		// Inventory routes (Admin only)
		inventory := api.Group("/inventory")
		inventory.Use(middleware.AuthMiddleware(cfg.JWTSecret), middleware.AdminOnly())
		{
			inventory.GET("", cfg.InventoryHandler.ListAllInventory)
			inventory.GET("/:product_id", cfg.InventoryHandler.GetInventory)
			inventory.PUT("/:product_id", cfg.InventoryHandler.UpdateInventory)
			inventory.POST("/:product_id/restock", cfg.InventoryHandler.Restock)
			inventory.GET("/:product_id/transactions", cfg.InventoryHandler.GetTransactions)
		}

		// Orders routes
		orders := api.Group("/orders")
		orders.Use(middleware.AuthMiddleware(cfg.JWTSecret))
		{
			orders.POST("", cfg.OrderHandler.PlaceOrder)
			orders.GET("", cfg.OrderHandler.ListOrders)
			orders.GET("/:id", cfg.OrderHandler.GetOrder)
			orders.PUT("/:id/cancel", cfg.OrderHandler.CancelOrder)
			orders.PUT("/:id/status", middleware.AdminOnly(), cfg.OrderHandler.UpdateStatus)
			orders.POST("/:id/refund", middleware.AdminOnly(), cfg.OrderHandler.RefundOrder)
		}
	}

	return r
}
