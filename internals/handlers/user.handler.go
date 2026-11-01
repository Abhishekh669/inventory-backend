package handlers

import (
	"fmt"
	"inventory/internals/helpers"
	"inventory/internals/models"
	"inventory/internals/services"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UserHandler struct {
	userService services.UserService
}

// get
func (h *UserHandler) GetUserFromTokenHandler(c *gin.Context) {
	errMessage := "unauthorized user"
	token, err := helpers.ExtractTokenFromHeader(c)
	if err != nil || token == "" {
		helpers.Error(c, http.StatusUnauthorized, errMessage)
		return
	}
	claims, err := helpers.ParseJwtToken(token)
	if err != nil {
		helpers.Error(c, http.StatusUnauthorized, errMessage)
		return
	}

	userUUID, err := helpers.StringToUUID(claims.UserId)
	if uuid.Nil == userUUID || err != nil {
		helpers.Error(c, http.StatusUnauthorized, errMessage)
		return
	}
	user, err := h.userService.GetUserById(c.Request.Context(), userUUID)
	if user == nil || err != nil {
		helpers.Error(c, http.StatusUnauthorized, errMessage)
		return
	}

	if user.LastPasswordResetAt != claims.LastPasswordResetAt {
		helpers.Error(c, http.StatusUnauthorized, errMessage)
		return
	}

	safeUser := helpers.ConvertToSafeUser(user)
	if safeUser == nil {
		helpers.Error(c, http.StatusInternalServerError, errMessage)
		return
	}
	helpers.Success(c, http.StatusOK, "validated user", gin.H{
		"user": safeUser,
	})
}

// post
func (h *UserHandler) LoginUserHandler(c *gin.Context) {
	errMessage := "incorrect credentails"
	var loginData models.UserLogin

	if err := c.ShouldBindJSON(&loginData); err != nil {
		fmt.Println("Error binding JSON in registration :", err)
		helpers.Error(c, http.StatusBadRequest, errMessage)
		return
	}

	token, err := h.userService.LoginUserService(loginData.Email, loginData.Password, c.Request.Context())
	if token == nil || err != nil {
		helpers.Error(c, http.StatusUnauthorized, errMessage)
		return
	}

	helpers.Success(c, http.StatusOK, "logged in successfully", gin.H{
		"token": token,
	})
}

func NewUserHandler(userService services.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}
