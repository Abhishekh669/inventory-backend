package services

import (
	"context"
	"inventory/internals/models"
	"inventory/internals/repository"

	"github.com/google/uuid"
)

type SettingService interface {
	GetSetting(ctx context.Context) (*models.Setting, error)
	UpdateSetting(ctx context.Context, setting *models.UpdateSettingRequest) error

	GetAllUnits(ctx context.Context) ([]models.Unit, error)
	CreateUnit(ctx context.Context, units []models.CreateUnit) error
	UpdateUnit(ctx context.Context, unit *models.UpdateUnit) error
	DeleteUnits(ctx context.Context, ids []uuid.UUID) error
}

type settingService struct {
	settingRepo repository.SettingRepo
}

func (s *settingService) GetSetting(ctx context.Context) (*models.Setting, error) {
	return s.settingRepo.GetSetting(ctx)
}

func (s *settingService) UpdateSetting(ctx context.Context, setting *models.UpdateSettingRequest) error {
	return s.settingRepo.UpdateSetting(ctx, setting)
}

//create a units

func (s *settingService) GetAllUnits(ctx context.Context) ([]models.Unit, error) {
	units, err := s.settingRepo.GetAllUnits(ctx)
	if err != nil {
		return nil, err
	}
	if units == nil {
		return []models.Unit{}, nil
	}
	return *units, nil
}

func (s *settingService) CreateUnit(ctx context.Context, units []models.CreateUnit) error {
	return s.settingRepo.CreateNewUnit(ctx, units)
}

func (s *settingService) UpdateUnit(ctx context.Context, unit *models.UpdateUnit) error {
	return s.settingRepo.UpdateNewUnit(ctx, unit.ID, unit.Name, unit.AllowDecimal)
}

func (s *settingService) DeleteUnits(ctx context.Context, ids []uuid.UUID) error {
	return s.settingRepo.DeleteUnits(ctx, ids)
}
func NewSettingService(settingRepo repository.SettingRepo) SettingService {
	return &settingService{settingRepo: settingRepo}
}
