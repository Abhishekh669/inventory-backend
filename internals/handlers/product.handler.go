package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"inventory/internals/helpers"
	"inventory/internals/models"
	"inventory/internals/repository"
	"inventory/internals/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"
)

const (
	defaultPage   = 0
	defaultLimit  = 20
	maxLimit      = 30
	maxSKULength  = 50
	maxNameLength = 200
	maxBatchSize  = 100
)

// Nepal Time (UTC+5:45), used for date-only query params
var nepalTZ = time.FixedZone("NPT", 5*3600+45*60)

type ProductHandler struct {
	productService services.ProductService
}

func NewProductHandler(productService services.ProductService) *ProductHandler {
	return &ProductHandler{productService: productService}
}

// handleProductError maps repository/database errors to HTTP responses.
func handleProductError(c *gin.Context, err error) {
	var pgErr *pgconn.PgError

	switch {
	case errors.Is(err, repository.ErrProductNotFound):
		helpers.Error(c, http.StatusNotFound, "product not found")
	case errors.Is(err, repository.ErrProductSKUExists):
		helpers.Error(c, http.StatusConflict, err.Error())
	case errors.Is(err, repository.ErrInvalidCategoryOrUnit):
		helpers.Error(c, http.StatusBadRequest, "invalid category or unit")
	case errors.As(err, &pgErr) && pgErr.Code == "22003":
		helpers.Error(c, http.StatusBadRequest, "a numeric value is out of range")
	case errors.As(err, &pgErr) && pgErr.Code == "22001":
		helpers.Error(c, http.StatusBadRequest, "a text value is too long")
	default:
		fmt.Println("product error:", err)
		helpers.Error(c, http.StatusInternalServerError, "internal server error")
	}
}

// ---------- validation helpers ----------

func parseDateParam(value string, endOfDay bool) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	t, err := time.ParseInLocation("2006-01-02", value, nepalTZ)
	if err != nil {
		return time.Time{}, err
	}
	if endOfDay {
		t = t.Add(24*time.Hour - time.Nanosecond)
	}
	return t, nil
}

func validateProductFields(
	sku, name string,
	categoryID, unitID uuid.UUID,
	vatRate, costPrice, costWithVat, marginPercent, sellingPrice, reOrderLevel, currentStock decimal.Decimal,
) error {
	if sku == "" {
		return errors.New("sku is required")
	}
	if len(sku) > maxSKULength {
		return fmt.Errorf("sku must be at most %d characters", maxSKULength)
	}
	if name == "" {
		return errors.New("name is required")
	}
	if len(name) > maxNameLength {
		return fmt.Errorf("name must be at most %d characters", maxNameLength)
	}
	if categoryID == uuid.Nil {
		return errors.New("category_id is required")
	}
	if unitID == uuid.Nil {
		return errors.New("unit_id is required")
	}
	if vatRate.IsNegative() || vatRate.GreaterThan(decimal.NewFromInt(100)) {
		return errors.New("vat_rate must be between 0 and 100")
	}

	nonNegative := []struct {
		field string
		value decimal.Decimal
	}{
		{"cost_price", costPrice},
		{"cost_with_vat", costWithVat},
		{"margin_percent", marginPercent},
		{"selling_price", sellingPrice},
		{"re_order_level", reOrderLevel},
		{"current_stock", currentStock},
	}
	for _, f := range nonNegative {
		if f.value.IsNegative() {
			return fmt.Errorf("%s cannot be negative", f.field)
		}
	}
	return nil
}

// ---------- handlers ----------

type productListQuery struct {
	limit        int
	page         int
	fromDate     *time.Time
	toDate       *time.Time
	name         *string
	currentStock *int
	greaterThan  *bool
}

// parseProductListQuery validates the shared query params.
// On failure it writes the 400 response and returns false.
func parseProductListQuery(c *gin.Context) (productListQuery, bool) {
	q := productListQuery{page: defaultPage, limit: defaultLimit}

	if v := c.Query("page"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil || p < 0 {
			helpers.Error(c, http.StatusBadRequest, "page must be a non-negative integer")
			return q, false
		}
		q.page = p
	}

	if v := c.Query("limit"); v != "" {
		l, err := strconv.Atoi(v)
		if err != nil || l < 1 {
			helpers.Error(c, http.StatusBadRequest, "limit must be a positive integer")
			return q, false
		}
		q.limit = l
	}
	if q.limit > maxLimit {
		q.limit = maxLimit
	}

	if v := c.Query("from_date"); v != "" {
		t, err := parseDateParam(v, false)
		if err != nil {
			helpers.Error(c, http.StatusBadRequest, "from_date must be YYYY-MM-DD or RFC3339")
			return q, false
		}
		q.fromDate = &t
	}
	if v := c.Query("to_date"); v != "" {
		t, err := parseDateParam(v, true)
		if err != nil {
			helpers.Error(c, http.StatusBadRequest, "to_date must be YYYY-MM-DD or RFC3339")
			return q, false
		}
		q.toDate = &t
	}
	if q.fromDate != nil && q.toDate != nil && q.fromDate.After(*q.toDate) {
		helpers.Error(c, http.StatusBadRequest, "from_date cannot be after to_date")
		return q, false
	}

	if v := strings.TrimSpace(c.Query("name")); v != "" {
		if len(v) > maxNameLength {
			helpers.Error(c, http.StatusBadRequest, fmt.Sprintf("name must be at most %d characters", maxNameLength))
			return q, false
		}
		q.name = &v
	}

	if v := c.Query("current_stock"); v != "" {
		s, err := strconv.Atoi(v)
		if err != nil || s < 0 {
			helpers.Error(c, http.StatusBadRequest, "current_stock must be a non-negative integer")
			return q, false
		}
		q.currentStock = &s
	}

	if v := c.Query("greater_than"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			helpers.Error(c, http.StatusBadRequest, "greater_than must be true or false")
			return q, false
		}
		q.greaterThan = &b
	}

	return q, true
}

func (h *ProductHandler) GetProducts(c *gin.Context) {
	q, ok := parseProductListQuery(c)
	if !ok {
		return
	}

	result, err := h.productService.GetProducts(c.Request.Context(), q.limit, q.page, q.toDate, q.fromDate, q.name, q.currentStock, q.greaterThan)
	if err != nil {
		handleProductError(c, err)
		return
	}

	helpers.Success(c, http.StatusOK, "products fetched successfully", result)
}

func (h *ProductHandler) GetProductsByCategory(c *gin.Context) {
	slug := strings.ToLower(strings.TrimSpace(c.Param("slug")))
	if slug == "" {
		helpers.Error(c, http.StatusBadRequest, "category slug is required")
		return
	}
	if len(slug) > 20 {
		helpers.Error(c, http.StatusBadRequest, "invalid category slug")
		return
	}

	q, ok := parseProductListQuery(c)
	if !ok {
		return
	}

	result, err := h.productService.GetProductsByCategory(c.Request.Context(), slug, q.limit, q.page, q.toDate, q.fromDate, q.name, q.currentStock, q.greaterThan)
	if err != nil {
		handleProductError(c, err)
		return
	}

	helpers.Success(c, http.StatusOK, "products fetched successfully", result)
}

func (h *ProductHandler) CreateProduct(c *gin.Context) {
	var products []models.CreateProductRequestType
	if err := c.ShouldBindJSON(&products); err != nil {
		fmt.Println("Error binding JSON in create product:", err)
		helpers.Error(c, http.StatusBadRequest, "invalid payload")
		return
	}

	fmt.Println("this is hte proudcts in backend : ", products)

	if len(products) == 0 {
		helpers.Error(c, http.StatusBadRequest, "at least one product is required")
		return
	}
	if len(products) > maxBatchSize {
		helpers.Error(c, http.StatusBadRequest, fmt.Sprintf("at most %d products can be created at once", maxBatchSize))
		return
	}

	seenSKUs := make(map[string]int, len(products))
	for i := range products {
		p := &products[i]
		p.SKU = strings.TrimSpace(p.SKU)
		p.Name = strings.TrimSpace(p.Name)

		if err := validateProductFields(
			p.SKU, p.Name, p.CategoryID, p.UnitID,
			p.VatRate, p.CostPrice, p.CostWithVat, p.MarginPercent, p.SellingPrice, p.ReOrderLevel, p.CurrentStock,
		); err != nil {
			helpers.Error(c, http.StatusBadRequest, fmt.Sprintf("product at index %d: %s", i, err.Error()))
			return
		}

		key := strings.ToLower(p.SKU)
		if first, dup := seenSKUs[key]; dup {
			helpers.Error(c, http.StatusBadRequest, fmt.Sprintf("duplicate sku %q at index %d (first used at index %d)", p.SKU, i, first))
			return
		}
		seenSKUs[key] = i
	}

	user, status := helpers.GetUserContext(c)
	if !status {
		helpers.Error(c, http.StatusBadRequest, "unauthoirzed user")
		return
	}

	if err := h.productService.CreateNewProduct(c.Request.Context(), &products, user.UserID); err != nil {
		handleProductError(c, err)
		return
	}

	helpers.Success(c, http.StatusCreated, "products created successfully", products)
}

func (h *ProductHandler) UpdateProduct(c *gin.Context) {
	var product models.UpdateProductRequestType
	if err := c.ShouldBindJSON(&product); err != nil {
		fmt.Println("Error binding JSON in update product:", err)
		helpers.Error(c, http.StatusBadRequest, "invalid payload")
		return
	}

	if product.ID == uuid.Nil {
		helpers.Error(c, http.StatusBadRequest, "id is required")
		return
	}

	product.SKU = strings.TrimSpace(product.SKU)
	product.Name = strings.TrimSpace(product.Name)

	if err := validateProductFields(
		product.SKU, product.Name, product.CategoryID, product.UnitID,
		product.VatRate, product.CostPrice, product.CostWithVat, product.MarginPercent, product.SellingPrice, product.ReOrderLevel, product.CurrentStock,
	); err != nil {
		helpers.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.productService.UpdateNewProduct(c.Request.Context(), &product); err != nil {
		handleProductError(c, err)
		return
	}

	helpers.Success(c, http.StatusOK, "product updated successfully", product)
}

func (h *ProductHandler) DeleteProducts(c *gin.Context) {
	var req models.DeleteProductRequestType
	if err := c.ShouldBindJSON(&req); err != nil {
		helpers.Error(c, http.StatusBadRequest, "invalid payload")
		return
	}

	if len(req.IDs) == 0 {
		helpers.Error(c, http.StatusBadRequest, "at least one product id is required")
		return
	}
	if len(req.IDs) > maxBatchSize {
		helpers.Error(c, http.StatusBadRequest, fmt.Sprintf("at most %d products can be deleted at once", maxBatchSize))
		return
	}

	ids := make([]uuid.UUID, 0, len(req.IDs))
	for _, id := range req.IDs {
		parsed, err := helpers.StringToUUID(id)
		if err != nil {
			helpers.Error(c, http.StatusBadRequest, "invalid product ID")
			return
		}
		ids = append(ids, parsed)
	}

	if err := h.productService.DeleteNewProducts(c.Request.Context(), ids); err != nil {
		handleProductError(c, err)
		return
	}

	helpers.Success(c, http.StatusOK, "products deleted successfully", nil)
}
