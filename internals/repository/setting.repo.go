package repository

import (
	"context"
	"errors"
	"fmt"
	"inventory/internals/database"
	"inventory/internals/models"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SettingRepo interface {
	GetSetting(ctx context.Context) (*models.Setting, error)
	UpdateSetting(ctx context.Context, setting *models.UpdateSettingRequest) error
}

type settingRepo struct {
	pool *pgxpool.Pool
}

const settingColumns = `id, name, address, phone, pan_no, vat_no, default_vat_rate, low_stock_default, bill_footer, created_at`

func (s *settingRepo) scanSetting(row pgx.Row) (*models.Setting, error) {
	var setting models.Setting
	err := row.Scan(
		&setting.ID,
		&setting.Name,
		&setting.Address,
		&setting.Phone,
		&setting.PanNo,
		&setting.VatNo,
		&setting.DefaultVatRate,
		&setting.LowStockDefault,
		&setting.BillFooter,
		&setting.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

func (s *settingRepo) GetSetting(ctx context.Context) (*models.Setting, error) {
	query := `SELECT ` + settingColumns + ` FROM setting LIMIT 1`

	setting, err := s.scanSetting(s.pool.QueryRow(ctx, query))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("setting not found")
		}
		return nil, fmt.Errorf("get setting: %w", err)
	}
	return setting, nil
}

func (s *settingRepo) UpdateSetting(ctx context.Context, setting *models.UpdateSettingRequest) error {
	query := `
		UPDATE setting
		SET name = $1,
			address = $2,
			phone = $3,
			pan_no = $4,
			vat_no = $5,
			default_vat_rate = $6,
			low_stock_default = $7,
			bill_footer = $8`

	tag, err := s.pool.Exec(ctx, query,
		setting.Name,
		setting.Address,
		setting.Phone,
		setting.PanNo,
		setting.VatNo,
		setting.DefaultVatRate,
		setting.LowStockDefault,
		setting.BillFooter,
	)
	if err != nil {
		return fmt.Errorf("update setting: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("setting not found")
	}
	return nil
}

func NewSettingRepository() SettingRepo {
	pool, err := database.GetPostgresPool()
	if err != nil {
		log.Println("Failed to initialize pool pool:", err)
		return nil
	}

	return &settingRepo{pool: pool}
}
