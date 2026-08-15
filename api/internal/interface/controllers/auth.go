package controllers

import (
	"errors"
	"net/http"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
	"Threadly/internal/interface/dto"
	"Threadly/internal/middleware"
	"Threadly/internal/usecase/services"

	"github.com/gin-gonic/gin"
)

type AuthController struct {
	service *services.AuthService
}

func NewAuthController(service *services.AuthService) *AuthController {
	return &AuthController{service: service}
}

// RegisterHandler godoc
// @Summary Register a user
// @Description Create a user with a username and password, then set an authenticated session cookie.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.CredentialsRequest true "Registration credentials"
// @Success 201 {object} dto.AuthResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/auth/register [post]
func (ac *AuthController) RegisterHandler(c *gin.Context) {
	var req dto.CredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid request body"})
		return
	}

	user, token, err := ac.service.Register(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	http.SetCookie(c.Writer, middleware.NewSessionCookie(token))
	c.JSON(http.StatusCreated, dto.AuthResponseFromModel(user))
}

// LoginHandler godoc
// @Summary Login
// @Description Authenticate with a username and password, then set an authenticated session cookie.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.CredentialsRequest true "Login credentials"
// @Success 200 {object} dto.AuthResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/auth/login [post]
func (ac *AuthController) LoginHandler(c *gin.Context) {
	var req dto.CredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid request body"})
		return
	}

	user, token, err := ac.service.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	http.SetCookie(c.Writer, middleware.NewSessionCookie(token))
	c.JSON(http.StatusOK, dto.AuthResponseFromModel(user))
}

// LogoutHandler godoc
// @Summary Logout
// @Description Clear the authenticated session cookie.
// @Tags auth
// @Success 204
// @Router /api/auth/logout [post]
func (ac *AuthController) LogoutHandler(c *gin.Context) {
	http.SetCookie(c.Writer, middleware.NewExpiredSessionCookie())
	c.Status(http.StatusNoContent)
}

// MeHandler godoc
// @Summary Get current user
// @Description Return the user represented by the authenticated session cookie.
// @Tags auth
// @Produce json
// @Security SessionCookie
// @Success 200 {object} dto.UserResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/me [get]
func (ac *AuthController) MeHandler(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "unauthorized"})
		return
	}

	user, err := ac.service.GetMe(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
		return
	}
	c.JSON(http.StatusOK, dto.UserResponseFromModel(user))
}

func writeAuthError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, models.ErrInvalidUsername),
		errors.Is(err, models.ErrInvalidPassword):
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid credentials format"})
	case errors.Is(err, services.ErrInvalidCredentials):
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "invalid credentials"})
	case errors.Is(err, services.ErrUsernameAlreadyExists):
		c.JSON(http.StatusConflict, dto.ErrorResponse{Error: "username already exists"})
	default:
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
	}
}
