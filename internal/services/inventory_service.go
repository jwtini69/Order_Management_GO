package services

import (
	"context"

	"github.com/kira/order-management/internal/models"
	"github.com/kira/order-management/internal/repository"
)

type InventoryService struct {
	inventoryRepo *repository.InventoryRepository
}

func NewInventoryService(inventoryRepo *repository.InventoryRepository) *InventoryService {
	return &InventoryService{
		inventoryRepo: inventoryRepo,
	}
}

func (s *InventoryService) GetInventory(ctx context.Context, productID int) (*models.InventoryItem, error) {
	return s.inventoryRepo.GetByProductID(ctx, productID)
}

func (s *InventoryService) UpdateInventory(ctx context.Context, productID int, req models.UpdateInventoryRequest) (*models.InventoryItem, error) {
	return s.inventoryRepo.UpdateStock(ctx, productID, &req)
}

func (s *InventoryService) RestockProduct(ctx context.Context, productID int, req models.RestockRequest) (*models.InventoryItem, error) {
	return s.inventoryRepo.Restock(ctx, productID, &req)
}

func (s *InventoryService) GetTransactions(ctx context.Context, productID int) ([]models.InventoryTransaction, error) {
	return s.inventoryRepo.GetTransactions(ctx, productID)
}

func (s *InventoryService) ListAllInventory(ctx context.Context) ([]models.InventoryItem, error) {
	return s.inventoryRepo.ListAll(ctx)
}
