package services

import (
	"context"
	"errors"
	"inventory/internals/models"
	"inventory/internals/repository"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type SupplierPaymentService interface {
	CreateAdvancePaymentForSupplierService(
		ctx context.Context,
		data *models.CreateAdvancePaymentType,
		userId uuid.UUID,
	) error
}

type supplierPaymentService struct {
	repo repository.SupplierPaymentRepository
}

func NewSupplierPaymentService(repo repository.SupplierPaymentRepository) SupplierPaymentService {
	return &supplierPaymentService{repo: repo}
}

func (s *supplierPaymentService) CreateAdvancePaymentForSupplierService(
	ctx context.Context,
	data *models.CreateAdvancePaymentType,
	userId uuid.UUID,
) error {

	if data.SupplierID == uuid.Nil {
		return errors.New("supplier is required")
	}

	if data.Amount.LessThanOrEqual(decimal.Zero) {
		return errors.New("payment amount must be greater than zero")
	}

	switch data.PaymentMethod {
	case models.PaymentMethodCash,
		models.PaymentMethodBank,
		models.PaymentMethodCheque,
		models.PaymentMethodOnline:
	default:
		return errors.New("invalid payment method")
	}

	paymentData := &models.SupplierPaymentCreationType{
		SupplierID:    data.SupplierID,
		Amount:        data.Amount,
		PaymentMethod: data.PaymentMethod,
		PaymentType:   models.PaymentTypeAdvance,
		Note:          data.Note,
	}

	_, err := s.repo.CreateAdvancePaymentForSupplier(ctx, paymentData, userId)
	return err
}
