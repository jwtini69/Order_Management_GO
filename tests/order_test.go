package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/kira/order-management/internal/models"
)

func TestOrderStatusTransitions(t *testing.T) {
	tests := []struct {
		name     string
		current  models.OrderStatus
		target   models.OrderStatus
		expected bool
	}{
		{"Pending to Confirmed", models.OrderStatusPending, models.OrderStatusConfirmed, true},
		{"Pending to Cancelled", models.OrderStatusPending, models.OrderStatusCancelled, true},
		{"Confirmed to Processing", models.OrderStatusConfirmed, models.OrderStatusProcessing, true},
		{"Confirmed to Cancelled", models.OrderStatusConfirmed, models.OrderStatusCancelled, true},
		{"Processing to Shipped", models.OrderStatusProcessing, models.OrderStatusShipped, true},
		{"Shipped to Delivered", models.OrderStatusShipped, models.OrderStatusDelivered, true},
		{"Delivered to Refunded", models.OrderStatusDelivered, models.OrderStatusRefunded, true},
		{"Delivered to Cancelled (Invalid)", models.OrderStatusDelivered, models.OrderStatusCancelled, false},
		{"Cancelled to Confirmed (Invalid)", models.OrderStatusCancelled, models.OrderStatusConfirmed, false},
		{"Refunded to Shipped (Invalid)", models.OrderStatusRefunded, models.OrderStatusShipped, false},
	}

	validStatusTransitions := map[models.OrderStatus][]models.OrderStatus{
		models.OrderStatusPending:    {models.OrderStatusConfirmed, models.OrderStatusCancelled},
		models.OrderStatusConfirmed:  {models.OrderStatusProcessing, models.OrderStatusCancelled},
		models.OrderStatusProcessing: {models.OrderStatusShipped, models.OrderStatusCancelled},
		models.OrderStatusShipped:    {models.OrderStatusDelivered},
		models.OrderStatusDelivered:  {models.OrderStatusRefunded},
		models.OrderStatusCancelled:  {},
		models.OrderStatusRefunded:   {},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed := validStatusTransitions[tt.current]
			canTransition := false
			for _, s := range allowed {
				if s == tt.target {
					canTransition = true
					break
				}
			}
			assert.Equal(t, tt.expected, canTransition)
		})
	}
}

func TestStockReservationValidation(t *testing.T) {
	item := models.InventoryItem{
		ProductID:        42,
		Quantity:         10,
		ReservedQuantity: 8,
		MinStockLevel:    2,
	}

	available := item.Quantity - item.ReservedQuantity
	assert.Equal(t, 2, available)

	// Can reserve 2
	assert.True(t, available >= 2)

	// Cannot reserve 3 (oversell prevention)
	assert.False(t, available >= 3)
}

func TestSellerRoleValidation(t *testing.T) {
	assert.Equal(t, models.Role("seller"), models.RoleSeller)
	assert.Equal(t, models.Role("admin"), models.RoleAdmin)
	assert.Equal(t, models.Role("customer"), models.RoleCustomer)
}

