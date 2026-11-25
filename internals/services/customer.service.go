// NEW FILE: internals/services/customer.go
package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"inventory/internals/models"
	"inventory/internals/repository"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type CustomerService interface {
	GetAllCustomers(ctx context.Context, q models.CustomerListQuery) (*models.GetCustomersResponse, error)
	CreateCustomer(ctx context.Context, req models.CreateCustomerRequest) (*models.Customer, error)
	UpdateCustomer(ctx context.Context, customerID uuid.UUID, req models.UpdateCustomerRequest) (*models.Customer, error)
	DeleteCustomer(ctx context.Context, customerID uuid.UUID) error
}

type customerService struct {
	repo repository.CustomerRepository
}

func NewCustomerService(repo repository.CustomerRepository) CustomerService {
	return &customerService{repo: repo}
}

// the limits match the VARCHAR sizes of the customers table
const (
	defaultCustomerListLimit = 20
	maxCustomerListLimit     = 100
	maxCustomerSearchLen     = 100

	maxCustomerNameLen    = 150
	maxCustomerContactLen = 100
	maxCustomerPhoneLen   = 30
	maxCustomerEmailLen   = 150
	maxCustomerAddressLen = 250
	maxCustomerPanLen     = 30
)

var (
	customerEmailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	customerPhoneRe = regexp.MustCompile(`^[0-9+\-()\s]+$`)
)

// ---------- cursor ----------

func encodeCustomerCursor(c models.Customer) string {
	b, _ := json.Marshal(models.CustomerListCursor{CreatedAt: c.CreatedAt, ID: c.ID})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCustomerCursor(raw string) (*models.CustomerListCursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, models.NewValidationError("invalid cursor")
	}
	var c models.CustomerListCursor
	if err := json.Unmarshal(b, &c); err != nil || c.ID == uuid.Nil || c.CreatedAt.IsZero() {
		return nil, models.NewValidationError("invalid cursor")
	}
	return &c, nil
}

// ---------- list ----------

func (s *customerService) GetAllCustomers(ctx context.Context, q models.CustomerListQuery) (*models.GetCustomersResponse, error) {
	bad := models.NewValidationError

	limit := q.Limit
	if limit <= 0 {
		limit = defaultCustomerListLimit
	}
	if limit > maxCustomerListLimit {
		limit = maxCustomerListLimit
	}
	filter := models.CustomerListFilter{Limit: limit}

	if raw := strings.TrimSpace(q.Search); raw != "" {
		if utf8.RuneCountInString(raw) > maxCustomerSearchLen {
			return nil, bad(fmt.Sprintf("search must be at most %d characters", maxCustomerSearchLen))
		}
		filter.Search = &raw
	}

	// the balance filter needs both parts: "more than" / "less than" and an amount
	opRaw := strings.TrimSpace(q.BalanceOp)
	amountRaw := strings.TrimSpace(q.BalanceAmount)
	switch {
	case opRaw == "" && amountRaw == "":
		// no balance filter
	case opRaw == "" || amountRaw == "":
		return nil, bad("choose more than or less than, and enter an amount")
	default:
		op := models.BalanceOp(opRaw)
		switch op {
		case models.BalanceGT, models.BalanceGTE, models.BalanceLT, models.BalanceLTE, models.BalanceEQ:
		default:
			return nil, bad(fmt.Sprintf("unknown balance_op %q", opRaw))
		}
		amount, err := decimal.NewFromString(amountRaw)
		if err != nil || amount.IsNegative() {
			return nil, bad("balance_amount must be a number that is not negative")
		}
		amount = amount.Round(2)
		filter.BalanceOp = &op
		filter.BalanceAmount = &amount
	}

	cursor, err := decodeCustomerCursor(q.Cursor)
	if err != nil {
		return nil, err
	}
	filter.Cursor = cursor

	items, total, err := s.repo.GetAllCustomers(ctx, filter)
	if err != nil {
		return nil, err
	}

	resp := &models.GetCustomersResponse{Items: items, Total: total}
	if len(items) > limit {
		resp.Items = items[:limit]
		resp.HasMore = true
		next := encodeCustomerCursor(items[limit-1])
		resp.NextCursor = &next
	}
	return resp, nil
}

// ---------- validation shared by create and update ----------

// empty text becomes nil, and too long text is rejected
func cleanOptionalText(p *string, max int, label string) (*string, error) {
	if p == nil {
		return nil, nil
	}
	v := strings.TrimSpace(*p)
	if v == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(v) > max {
		return nil, models.NewValidationError(fmt.Sprintf("%s is too long (max %d characters)", label, max))
	}
	return &v, nil
}

type customerFields struct {
	Name          string
	ContactPerson *string
	Phone         *string
	Email         *string
	Address       *string
	PanNo         *string
}

func normalizeCustomerFields(name string, contact, phone, email, address, pan *string) (customerFields, error) {
	bad := models.NewValidationError
	var f customerFields
	var err error

	f.Name = strings.TrimSpace(name)
	if f.Name == "" || utf8.RuneCountInString(f.Name) > maxCustomerNameLen {
		return f, bad(fmt.Sprintf("customer name is required (max %d characters)", maxCustomerNameLen))
	}

	if f.ContactPerson, err = cleanOptionalText(contact, maxCustomerContactLen, "contact person"); err != nil {
		return f, err
	}
	if f.Address, err = cleanOptionalText(address, maxCustomerAddressLen, "address"); err != nil {
		return f, err
	}
	if f.PanNo, err = cleanOptionalText(pan, maxCustomerPanLen, "PAN number"); err != nil {
		return f, err
	}

	if f.Phone, err = cleanOptionalText(phone, maxCustomerPhoneLen, "phone"); err != nil {
		return f, err
	}
	if f.Phone != nil && (utf8.RuneCountInString(*f.Phone) < 5 || !customerPhoneRe.MatchString(*f.Phone)) {
		return f, bad("phone must be at least 5 characters and use only digits, spaces, + - ( )")
	}

	if f.Email, err = cleanOptionalText(email, maxCustomerEmailLen, "email"); err != nil {
		return f, err
	}
	if f.Email != nil {
		lower := strings.ToLower(*f.Email)
		if !customerEmailRe.MatchString(lower) {
			return f, bad("enter a valid email address")
		}
		f.Email = &lower
	}
	return f, nil
}

// ---------- writes ----------

func (s *customerService) CreateCustomer(ctx context.Context, req models.CreateCustomerRequest) (*models.Customer, error) {
	f, err := normalizeCustomerFields(req.Name, req.ContactPerson, req.Phone, req.Email, req.Address, req.PanNo)
	if err != nil {
		return nil, err
	}
	req.Name, req.ContactPerson, req.Phone, req.Email, req.Address, req.PanNo =
		f.Name, f.ContactPerson, f.Phone, f.Email, f.Address, f.PanNo

	req.OpeningBalance = req.OpeningBalance.Round(2)
	if req.OpeningBalance.IsNegative() {
		return nil, models.NewValidationError("opening balance cannot be negative")
	}
	return s.repo.CreateCustomer(ctx, &req)
}

func (s *customerService) UpdateCustomer(ctx context.Context, customerID uuid.UUID, req models.UpdateCustomerRequest) (*models.Customer, error) {
	if customerID == uuid.Nil {
		return nil, models.NewValidationError("invalid customer id")
	}
	f, err := normalizeCustomerFields(req.Name, req.ContactPerson, req.Phone, req.Email, req.Address, req.PanNo)
	if err != nil {
		return nil, err
	}
	req.Name, req.ContactPerson, req.Phone, req.Email, req.Address, req.PanNo =
		f.Name, f.ContactPerson, f.Phone, f.Email, f.Address, f.PanNo
	return s.repo.UpdateCustomer(ctx, customerID, &req)
}

func (s *customerService) DeleteCustomer(ctx context.Context, customerID uuid.UUID) error {
	if customerID == uuid.Nil {
		return models.NewValidationError("invalid customer id")
	}
	return s.repo.DeleteCustomer(ctx, customerID)
}