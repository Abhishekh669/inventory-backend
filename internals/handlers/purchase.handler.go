package handlers

import (
	"errors"
	"fmt"
	"inventory/internals/helpers"
	"inventory/internals/models"
	"inventory/internals/services"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
)

type PurchaseHandler struct {
	purchaseService services.PurchaseService
}

func NewPurchaseHandler(purchaseService services.PurchaseService) *PurchaseHandler {
	return &PurchaseHandler{
		purchaseService: purchaseService,
	}
}

func (h *PurchaseHandler) GetAllPurchases(c *gin.Context) {
	limit := 0
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			helpers.Error(c, http.StatusBadRequest, "limit must be a positive number")
			return
		}
		limit = n
	}

	resp, err := h.purchaseService.GetAllPurchases(
		c.Request.Context(),
		models.PurchaseListQuery{
			PurchaseOrderStatus: c.Query("purchase_order_status"),
			PurchaseOrderNumber: c.Query("po_number"),
			PurchaseStatus:      c.Query("purchase_status"),
			PaymentStatus:       c.Query("payment_status"),
			Search:              c.Query("search"),
			FromDate:            c.Query("from_date"),
			ToDate:              c.Query("to_date"),
			Cursor:              c.Query("cursor"),
			Limit:               limit,
		},
	)
	if err != nil {
		handlePurchaseError(c, err)
		return
	}

	helpers.Success(c, http.StatusOK, "purchases fetched successfully", resp)
}

func handlePurchaseError(c *gin.Context, err error) {
	var pgErr *pgconn.PgError

	switch {
	case errors.Is(err, services.ErrInvalidInput):
		helpers.Error(c, http.StatusBadRequest, err.Error())

	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		helpers.Error(c, http.StatusConflict, "purchase already exists")

	case errors.As(err, &pgErr) && pgErr.Code == "23503":
		helpers.Error(c, http.StatusConflict, "related record does not exist or is in use")

	case errors.As(err, &pgErr) && pgErr.Code == "22001":
		helpers.Error(c, http.StatusBadRequest, "provided value is too long")

	default:
		fmt.Println("Purchase handler error:", err)
		helpers.Error(c, http.StatusInternalServerError, "internal server error")
	}
}

func (h *PurchaseHandler) CreatePurchase(c *gin.Context) {
	user, ok := helpers.GetUserContext(c) // adjust to your helper's actual return
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "unauthorized user")
		return
	}

	var req models.CreatePurchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helpers.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	purchaseID, err := h.purchaseService.CreatePurchase(c.Request.Context(), user.UserID, req)
	if err != nil {
		var ve *models.ValidationError
		if errors.As(err, &ve) {
			helpers.Error(c, http.StatusBadRequest, ve.Message)
			return
		}
		log.Println("create purchase:", err)
		helpers.Error(c, http.StatusInternalServerError, "could not create the purchase")
		return
	}

	helpers.Success(c, http.StatusCreated, "purchase created", gin.H{
		"purchase_id": purchaseID,
	})
}
