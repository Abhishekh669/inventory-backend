package repository

import (
	"context"
	"errors"
	"fmt"
	"inventory/internals/database"
	"inventory/internals/models"
	"log"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SettingRepo interface {
	GetSetting(ctx context.Context) (*models.Setting, error)
	UpdateSetting(ctx context.Context, setting *models.UpdateSettingRequest) error

	CreateNewUnit(ctx context.Context, units []models.CreateUnit) error
	UpdateNewUnit(ctx context.Context, id uuid.UUID, name string, allowDecimal bool) error
	DeleteUnits(ctx context.Context, ids []uuid.UUID) error
	GetAllUnits(ctx context.Context) (*[]models.Unit, error)
}

type settingRepo struct {
	pool *pgxpool.Pool
}

const settingColumns = `id, name, address, phone, pan_no, vat_no, default_vat_rate, low_stock_default, bill_footer, created_at`

const unitColumns = `id, name, allow_decimal`

// Postgres error codes
const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
)

var (
	ErrUnitNotFound = errors.New("unit not found")
	ErrUnitExists   = errors.New("unit already exists")
	ErrUnitInUse    = errors.New("unit is in use")
)

func pgErrCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

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

func (s *settingRepo) CreateNewUnit(ctx context.Context, units []models.CreateUnit) error {
	if len(units) == 0 {
		return fmt.Errorf("at least one unit is required")
	}

	names := make([]string, 0, len(units))
	decimals := make([]bool, 0, len(units))
	seen := make(map[string]struct{}, len(units))

	for _, u := range units {
		name := strings.TrimSpace(u.Name)
		if name == "" {
			return fmt.Errorf("unit name is required")
		}
		if utf8.RuneCountInString(name) > 30 {
			return fmt.Errorf("unit name %q is too long (max 30 characters)", name)
		}

		// The same name twice in one request would fail the UNIQUE constraint anyway
		key := strings.ToLower(name)
		if _, dup := seen[key]; dup {
			return fmt.Errorf("unit %q is listed more than once", name)
		}
		seen[key] = struct{}{}

		names = append(names, name)
		decimals = append(decimals, u.AllowDecimal)
	}

	query := `
		INSERT INTO units (name, allow_decimal)
		SELECT * FROM unnest($1::text[], $2::boolean[])
	`

	if _, err := s.pool.Exec(ctx, query, names, decimals); err != nil {
		if pgErrCode(err) == pgUniqueViolation {
			return ErrUnitExists
		}
		return fmt.Errorf("create units: %w", err)
	}
	return nil
}
func (s *settingRepo) UpdateNewUnit(ctx context.Context, id uuid.UUID, name string, allowDecimal bool) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("unit name is required")
	}

	query := `UPDATE units SET name = $2, allow_decimal = $3 WHERE id = $1`

	tag, err := s.pool.Exec(ctx, query, id, name, allowDecimal)
	if err != nil {
		if pgErrCode(err) == pgUniqueViolation {
			return fmt.Errorf("unit %q already exists", name)
		}
		return fmt.Errorf("update unit: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("unit not found")
	}
	return nil
}

func (s *settingRepo) DeleteUnits(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return fmt.Errorf("no unit ids provided")
	}

	query := `DELETE FROM units WHERE id = ANY($1)`

	tag, err := s.pool.Exec(ctx, query, ids)
	if err != nil {
		if pgErrCode(err) == pgForeignKeyViolation {
			return fmt.Errorf("one or more units are in use and cannot be deleted")
		}
		return fmt.Errorf("delete units: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("unit not found")
	}
	return nil
}

func (s *settingRepo) GetAllUnits(ctx context.Context) (*[]models.Unit, error) {
	query := `SELECT ` + unitColumns + ` FROM units ORDER BY name ASC`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get all units: %w", err)
	}
	defer rows.Close()

	units := make([]models.Unit, 0) // empty slice (not nil) so JSON returns [] instead of null
	for rows.Next() {
		var u models.Unit
		if err := rows.Scan(&u.ID, &u.Name, &u.AllowDecimal); err != nil {
			return nil, fmt.Errorf("scan unit: %w", err)
		}
		units = append(units, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate units: %w", err)
	}

	return &units, nil
}

func NewSettingRepository() SettingRepo {
	pool, err := database.GetPostgresPool()
	if err != nil {
		log.Println("Failed to initialize pool pool:", err)
		return nil
	}

	return &settingRepo{pool: pool}
}
