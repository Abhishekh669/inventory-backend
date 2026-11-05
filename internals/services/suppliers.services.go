package services

import (
	"context"
	"errors"
	"fmt"
	"inventory/internals/models"
	"inventory/internals/repository"
	"strings"

	"github.com/google/uuid"
)

var ErrInvalidSupplier = errors.New("invalid supplier data")

type SupplierService interface {
	// interface
	CreateSuppliers(ctx context.Context, in []models.CreateSupplierInput) ([]models.Supplier, error)
	DeleteSuppliers(ctx context.Context, ids []uuid.UUID) (int64, error)
	UpdateSupplier(ctx context.Context, id uuid.UUID, in models.UpdateSupplierInput) (*models.Supplier, error)
	GetSupplierById(ctx context.Context, id uuid.UUID) (*models.Supplier, error)
	GetSuppliers(ctx context.Context) ([]models.Supplier, error)
}

type supplierService struct {
	repo repository.SupplierRepo
}

func NewSupplierService(repo repository.SupplierRepo) SupplierService {
	return &supplierService{repo: repo}
}

func trimPtr(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	return &t
}

func (s *supplierService) UpdateSupplier(ctx context.Context, id uuid.UUID, in models.UpdateSupplierInput) (*models.Supplier, error) {
	in.Name = trimPtr(in.Name)
	if in.Name != nil && *in.Name == "" {
		return nil, errors.New("supplier name cannot be empty")
	}
	in.ContactPerson = trimPtr(in.ContactPerson)
	in.Phone = trimPtr(in.Phone)
	in.Address = trimPtr(in.Address)
	in.PanNo = trimPtr(in.PanNo)

	return s.repo.UpdateSupplier(ctx, id, in)
}

func (s *supplierService) GetSupplierById(ctx context.Context, id uuid.UUID) (*models.Supplier, error) {
	return s.repo.GetSupplierById(ctx, id)
}

func (s *supplierService) GetSuppliers(ctx context.Context) ([]models.Supplier, error) {
	return s.repo.GetSuppliers(ctx)
}
func (s *supplierService) CreateSuppliers(ctx context.Context, inputs []models.CreateSupplierInput) ([]models.Supplier, error) {
	if len(inputs) == 0 {
		return nil, errors.New("at least one supplier is required")
	}
	if len(inputs) > 500 {
		return nil, errors.New("too many suppliers in one request (max 500)")
	}
	for i := range inputs {
		inputs[i].Name = strings.TrimSpace(inputs[i].Name)
		if inputs[i].Name == "" {
			return nil, fmt.Errorf("supplier #%d: name is required", i+1)
		}
		inputs[i].ContactPerson = trimPtr(inputs[i].ContactPerson)
		inputs[i].Phone = trimPtr(inputs[i].Phone)
		inputs[i].Address = trimPtr(inputs[i].Address)
		inputs[i].PanNo = trimPtr(inputs[i].PanNo)
	}
	return s.repo.CreateSuppliers(ctx, inputs)
}

func (s *supplierService) DeleteSuppliers(ctx context.Context, ids []uuid.UUID) (int64, error) {
	if len(ids) == 0 {
		return 0, errors.New("select at least one supplier")
	}
	return s.repo.DeleteSuppliers(ctx, ids)
}
