package services

import (
	"context"
	"math"

	"github.com/kira/order-management/internal/models"
	"github.com/kira/order-management/internal/repository"
)

type OrderService struct {
	orderRepo *repository.OrderRepository
}

func NewOrderService(orderRepo *repository.OrderRepository) *OrderService {
	return &OrderService{
		orderRepo: orderRepo,
	}
}

// PlaceOrder creates an order using PostgreSQL row-level locking (SELECT ... FOR UPDATE)
// within a Serializable transaction to ensure thread-safe stock reservations.
func (s *OrderService) PlaceOrder(ctx context.Context, userID int, req models.CreateOrderRequest) (*models.Order, error) {
	items := make([]models.OrderItem, len(req.Items))
	for i, itemReq := range req.Items {
		items[i] = models.OrderItem{
			ProductID: itemReq.ProductID,
			Quantity:  itemReq.Quantity,
		}
	}

	order := &models.Order{
		UserID:          userID,
		ShippingAddress: req.ShippingAddress,
		Items:           items,
	}

	if err := s.orderRepo.Create(ctx, order); err != nil {
		return nil, err
	}

	return order, nil
}

func (s *OrderService) GetOrder(ctx context.Context, orderID int, userID int, role models.Role) (*models.Order, error) {
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	// Customer can only view their own order
	if role != models.RoleAdmin && order.UserID != userID {
		return nil, models.ErrForbidden
	}

	return order, nil
}

func (s *OrderService) ListOrders(ctx context.Context, userID int, role models.Role, page, pageSize int) (*models.PaginatedResponse, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	var orders []models.Order
	var total int
	var err error

	if role == models.RoleAdmin {
		orders, total, err = s.orderRepo.ListAll(ctx, page, pageSize)
	} else {
		orders, total, err = s.orderRepo.ListByUserID(ctx, userID, page, pageSize)
	}

	if err != nil {
		return nil, err
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	return &models.PaginatedResponse{
		Data:       orders,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

func (s *OrderService) CancelOrder(ctx context.Context, orderID int, userID int) error {
	return s.orderRepo.CancelOrder(ctx, orderID, userID)
}

func (s *OrderService) UpdateOrderStatus(ctx context.Context, orderID int, status models.OrderStatus) error {
	return s.orderRepo.UpdateStatus(ctx, orderID, status)
}

func (s *OrderService) RefundOrder(ctx context.Context, orderID int) error {
	return s.orderRepo.RefundOrder(ctx, orderID)
}
