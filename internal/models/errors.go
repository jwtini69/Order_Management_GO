package models

import "errors"

var (
	ErrNotFound           = errors.New("resource not found")
	ErrDuplicateEmail     = errors.New("email already exists")
	ErrDuplicateSKU       = errors.New("SKU already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInsufficientStock  = errors.New("insufficient stock")
	ErrInvalidOrderStatus = errors.New("invalid order status transition")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
)
