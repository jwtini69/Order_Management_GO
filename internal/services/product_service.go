package services

import (
	"context"
	"math"

	"github.com/kira/order-management/internal/models"
	"github.com/kira/order-management/internal/repository"
)

type ProductService struct {
	productRepo *repository.ProductRepository
}

func NewProductService(productRepo *repository.ProductRepository) *ProductService {
	return &ProductService{
		productRepo: productRepo,
	}
}

func (s *ProductService) CreateProduct(ctx context.Context, req models.CreateProductRequest) (*models.Product, error) {
	product := &models.Product{
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		CategoryID:  req.CategoryID,
		SKU:         req.SKU,
		ImageURL:    req.ImageURL,
		IsActive:    true,
	}

	if err := s.productRepo.Create(ctx, product); err != nil {
		return nil, err
	}

	return product, nil
}

func (s *ProductService) GetProduct(ctx context.Context, id int) (*models.Product, error) {
	return s.productRepo.GetByID(ctx, id)
}

func (s *ProductService) UpdateProduct(ctx context.Context, id int, req models.UpdateProductRequest) (*models.Product, error) {
	return s.productRepo.Update(ctx, id, &req)
}

func (s *ProductService) DeleteProduct(ctx context.Context, id int) error {
	return s.productRepo.Delete(ctx, id)
}

func (s *ProductService) ListProducts(ctx context.Context, filter models.ProductFilter) (*models.PaginatedResponse, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}

	products, total, err := s.productRepo.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	totalPages := int(math.Ceil(float64(total) / float64(filter.PageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	return &models.PaginatedResponse{
		Data:       products,
		Total:      total,
		Page:       filter.Page,
		PageSize:   filter.PageSize,
		TotalPages: totalPages,
	}, nil
}

func (s *ProductService) CreateCategory(ctx context.Context, req models.CreateCategoryRequest) (*models.Category, error) {
	category := &models.Category{
		Name:        req.Name,
		Description: req.Description,
	}

	if err := s.productRepo.CreateCategory(ctx, category); err != nil {
		return nil, err
	}

	return category, nil
}

func (s *ProductService) ListCategories(ctx context.Context) ([]models.Category, error) {
	return s.productRepo.ListCategories(ctx)
}
