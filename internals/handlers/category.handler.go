package handlers

import (
	"errors"
	"fmt"
	"inventory/internals/helpers"
	"inventory/internals/models"
	"inventory/internals/repository"
	"inventory/internals/services"
	"net/http"

	"github.com/google/uuid"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
)

type CategoryHandler struct {
	categoryService services.CategoryService
}

func NewCategoryHandler(categoryService services.CategoryService) *CategoryHandler {
	return &CategoryHandler{categoryService: categoryService}
}

// handleCategoryError maps repository/database errors to HTTP responses.
func handleCategoryError(c *gin.Context, err error) {
	var pgErr *pgconn.PgError

	switch {
	case errors.Is(err, repository.ErrCategoryNotFound):
		helpers.Error(c, http.StatusNotFound, "category not found")
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		helpers.Error(c, http.StatusConflict, "a category with this slug already exists")
	case errors.As(err, &pgErr) && pgErr.Code == "23503":
		helpers.Error(c, http.StatusConflict, "category is in use and cannot be deleted")
	case errors.As(err, &pgErr) && pgErr.Code == "22001":
		helpers.Error(c, http.StatusBadRequest, "name or slug is too long")
	default:
		helpers.Error(c, http.StatusInternalServerError, err.Error())
	}
}

func (h *CategoryHandler) GetAllCategories(c *gin.Context) {
	categories, err := h.categoryService.GetAllCategories(c.Request.Context())
	if err != nil {
		helpers.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	helpers.Success(c, http.StatusOK, "categories fetched successfully", categories)
}

func (h *CategoryHandler) CreateCategory(c *gin.Context) {
	var createCategoryData models.CreateCategory
	if err := c.ShouldBindJSON(&createCategoryData); err != nil {
		fmt.Println("Error binding JSON in create category:", err)
		helpers.Error(c, http.StatusBadRequest, "invalid payload")
		return
	}

	if err := h.categoryService.CreateCategory(c.Request.Context(), &createCategoryData); err != nil {
		handleCategoryError(c, err)
		return
	}

	helpers.Success(c, http.StatusCreated, "category created successfully", createCategoryData)
}

func (h *CategoryHandler) UpdateCategory(c *gin.Context) {
	var updateCategoryData models.UpdateCategory
	if err := c.ShouldBindJSON(&updateCategoryData); err != nil {
		fmt.Println("Error binding JSON in update category:", err)
		helpers.Error(c, http.StatusBadRequest, "invalid payload")
		return
	}

	if err := h.categoryService.UpdateCategory(c.Request.Context(), &updateCategoryData); err != nil {
		handleCategoryError(c, err)
		return
	}

	helpers.Success(c, http.StatusOK, "category updated successfully", updateCategoryData)
}

func (h *CategoryHandler) DeleteCategory(c *gin.Context) {
	var deleteCategoriesIds models.DeleteCategories

	if err := c.ShouldBindJSON(&deleteCategoriesIds); err != nil {
		helpers.Error(c, http.StatusBadRequest, "invalid payload")
		return
	}

	var deleteCategoriesIdsUUID []uuid.UUID

	for _, id := range deleteCategoriesIds.IDs {
		parsedUUID, err := helpers.StringToUUID(id)
		if err != nil {
			helpers.Error(c, http.StatusBadRequest, "invalid category ID")
			return
		}

		deleteCategoriesIdsUUID = append(deleteCategoriesIdsUUID, parsedUUID)
	}

	if err := h.categoryService.DeleteCategory(c.Request.Context(), deleteCategoriesIdsUUID); err != nil {
		handleCategoryError(c, err)
		return
	}

	helpers.Success(c, http.StatusOK, "category deleted successfully", nil)
}
