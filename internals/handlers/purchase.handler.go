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
	"github.com/google/uuid"
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

func (h *PurchaseHandler) GetPurchaseDataById(c *gin.Context) {
	purchaseID, err := helpers.StringToUUID(c.Param("purchase_id"))
	if err != nil || purchaseID == uuid.Nil {
		helpers.Error(c, http.StatusBadRequest, "invalid purchase id")
		return
	}

	data, err := h.purchaseService.GetPurchaseDataById(c.Request.Context(), purchaseID)
	if err != nil {
		var ve *models.ValidationError
		switch {
		case errors.Is(err, models.ErrNotFound):
			helpers.Error(c, http.StatusNotFound, "purchase not found")
		case errors.As(err, &ve):
			helpers.Error(c, http.StatusBadRequest, ve.Message)
		default:
			log.Println("get purchase by id:", err)
			helpers.Error(c, http.StatusInternalServerError, "could not load the purchase")
		}
		return
	}

	helpers.Success(c, http.StatusOK, "purchase fetched", gin.H{"purchase": data})
}

// purchaseActor returns the signed in user's id (adjust to your helpers.GetUserContext)
func purchaseActor(c *gin.Context) (uuid.UUID, bool) {
	user, ok := helpers.GetUserContext(c)
	if !ok || user == nil {
		helpers.Error(c, http.StatusUnauthorized, "unauthorized user")
		return uuid.Nil, false
	}
	return user.UserID, true
}

func purchaseIDParam(c *gin.Context) (uuid.UUID, bool) {
	id, err := helpers.StringToUUID(c.Param("purchase_id"))
	if err != nil || id == uuid.Nil {
		helpers.Error(c, http.StatusBadRequest, "invalid purchase id")
		return uuid.Nil, false
	}
	return id, true
}

func respondPurchaseError(c *gin.Context, err error, action string) {
	var ve *models.ValidationError
	switch {
	case errors.Is(err, models.ErrNotFound):
		helpers.Error(c, http.StatusNotFound, "purchase not found")
	case errors.As(err, &ve):
		helpers.Error(c, http.StatusBadRequest, ve.Message)
	default:
		log.Println(action+":", err)
		helpers.Error(c, http.StatusInternalServerError, "could not "+action)
	}
}

func (h *PurchaseHandler) UpdatePurchase(c *gin.Context) {
	userID, ok := purchaseActor(c)
	if !ok {
		return
	}
	purchaseID, ok := purchaseIDParam(c)
	if !ok {
		return
	}
	var req models.UpdatePurchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helpers.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.purchaseService.UpdatePurchase(c.Request.Context(), userID, purchaseID, req); err != nil {
		respondPurchaseError(c, err, "update the purchase")
		return
	}
	helpers.Success(c, http.StatusOK, "purchase updated", gin.H{"purchase_id": purchaseID})
}

func (h *PurchaseHandler) CreatePurchaseReturn(c *gin.Context) {
	userID, ok := purchaseActor(c)
	if !ok {
		return
	}
	purchaseID, ok := purchaseIDParam(c)
	if !ok {
		return
	}
	var req models.CreatePurchaseReturnRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helpers.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	returnID, err := h.purchaseService.CreatePurchaseReturn(c.Request.Context(), userID, purchaseID, req)
	if err != nil {
		respondPurchaseError(c, err, "create the return")
		return
	}
	helpers.Success(c, http.StatusCreated, "return created", gin.H{"return_id": returnID})
}

func (h *PurchaseHandler) CreatePurchasePayment(c *gin.Context) {
	userID, ok := purchaseActor(c)
	if !ok {
		return
	}
	purchaseID, ok := purchaseIDParam(c)
	if !ok {
		return
	}
	var req models.CreatePurchasePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helpers.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	paymentID, err := h.purchaseService.CreatePurchasePayment(c.Request.Context(), userID, purchaseID, req)
	if err != nil {
		respondPurchaseError(c, err, "record the payment")
		return
	}
	helpers.Success(c, http.StatusCreated, "payment recorded", gin.H{"payment_id": paymentID})
}

func (h *PurchaseHandler) DeletePurchase(c *gin.Context) {
	userID, ok := purchaseActor(c)
	if !ok {
		return
	}
	purchaseID, ok := purchaseIDParam(c)
	if !ok {
		return
	}
	if err := h.purchaseService.DeletePurchase(c.Request.Context(), userID, purchaseID); err != nil {
		respondPurchaseError(c, err, "delete the purchase")
		return
	}
	helpers.Success(c, http.StatusOK, "purchase deleted", gin.H{"purchase_id": purchaseID})
}

func (h *PurchaseHandler) UpdatePurchaseReturn(c *gin.Context) {
	userID, ok := purchaseActor(c)
	if !ok {
		return
	}
	purchaseID, ok := purchaseIDParam(c)
	if !ok {
		return
	}
	returnID, err := helpers.StringToUUID(c.Param("return_id"))
	if err != nil || returnID == uuid.Nil {
		helpers.Error(c, http.StatusBadRequest, "invalid return id")
		return
	}
	var req models.CreatePurchaseReturnRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helpers.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.purchaseService.UpdatePurchaseReturn(c.Request.Context(), userID, purchaseID, returnID, req); err != nil {
		respondPurchaseError(c, err, "update the return")
		return
	}
	helpers.Success(c, http.StatusOK, "return updated", gin.H{"return_id": returnID})
}

func purchaseSubIDParam(c *gin.Context, param, label string) (uuid.UUID, bool) {
	id, err := helpers.StringToUUID(c.Param(param))
	if err != nil || id == uuid.Nil {
		helpers.Error(c, http.StatusBadRequest, "invalid "+label+" id")
		return uuid.Nil, false
	}
	return id, true
}

func (h *PurchaseHandler) DeletePurchaseReturn(c *gin.Context) {
	userID, ok := purchaseActor(c)
	if !ok {
		return
	}
	purchaseID, ok := purchaseIDParam(c)
	if !ok {
		return
	}
	returnID, ok := purchaseSubIDParam(c, "return_id", "return")
	if !ok {
		return
	}
	if err := h.purchaseService.DeletePurchaseReturn(c.Request.Context(), userID, purchaseID, returnID); err != nil {
		respondPurchaseError(c, err, "delete the return")
		return
	}
	helpers.Success(c, http.StatusOK, "return deleted", gin.H{"return_id": returnID})
}

func (h *PurchaseHandler) UpdatePurchasePayment(c *gin.Context) {
	userID, ok := purchaseActor(c)
	if !ok {
		return
	}
	purchaseID, ok := purchaseIDParam(c)
	if !ok {
		return
	}
	paymentID, ok := purchaseSubIDParam(c, "payment_id", "payment")
	if !ok {
		return
	}
	var req models.UpdatePurchasePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helpers.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.purchaseService.UpdatePurchasePayment(c.Request.Context(), userID, purchaseID, paymentID, req); err != nil {
		respondPurchaseError(c, err, "update the payment")
		return
	}
	helpers.Success(c, http.StatusOK, "payment updated", gin.H{"payment_id": paymentID})
}

func (h *PurchaseHandler) DeletePurchasePayment(c *gin.Context) {
	userID, ok := purchaseActor(c)
	if !ok {
		return
	}
	purchaseID, ok := purchaseIDParam(c)
	if !ok {
		return
	}
	paymentID, ok := purchaseSubIDParam(c, "payment_id", "payment")
	if !ok {
		return
	}
	if err := h.purchaseService.DeletePurchasePayment(c.Request.Context(), userID, purchaseID, paymentID); err != nil {
		respondPurchaseError(c, err, "delete the payment")
		return
	}
	helpers.Success(c, http.StatusOK, "payment deleted", gin.H{"payment_id": paymentID})
}
