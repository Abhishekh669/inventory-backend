package services

import (
	"context"
	"inventory/internals/models"
	"inventory/internals/repository"
)

type SettingService interface {
	GetSetting(ctx context.Context) (*models.Setting, error)
	UpdateSetting(ctx context.Context, setting *models.UpdateSettingRequest) error
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

func NewSettingService(settingRepo repository.SettingRepo) SettingService {
	return &settingService{settingRepo: settingRepo}
}
