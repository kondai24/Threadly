package controllers

import (
	"errors"
	"net/http"
	"time"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
	"Threadly/internal/middleware"
	"Threadly/internal/usecase/services"

	"github.com/gin-gonic/gin"
)

type AuthController struct {
	service *services.AuthService
}

type credentialsRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type userResponse struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type authResponse struct {
	User userResponse `json:"user"`
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
// @Param request body credentialsRequest true "Registration credentials"
// @Success 201 {object} authResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/auth/register [post]
func (ac *AuthController) RegisterHandler(c *gin.Context) {
	var req credentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	user, token, err := ac.service.Register(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	http.SetCookie(c.Writer, middleware.NewSessionCookie(token))
	c.JSON(http.StatusCreated, authResponse{
		User: toUserResponse(user),
	})
}

// LoginHandler godoc
// @Summary Login
// @Description Authenticate with a username and password, then set an authenticated session cookie.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body credentialsRequest true "Login credentials"
// @Success 200 {object} authResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/auth/login [post]
func (ac *AuthController) LoginHandler(c *gin.Context) {
	var req credentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	user, token, err := ac.service.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	http.SetCookie(c.Writer, middleware.NewSessionCookie(token))
	c.JSON(http.StatusOK, authResponse{
		User: toUserResponse(user),
	})
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
// @Success 200 {object} userResponse
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/me [get]
func (ac *AuthController) MeHandler(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	user, err := ac.service.GetMe(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, toUserResponse(user))
}

func toUserResponse(user *models.User) userResponse {
	// 永続化モデルをそのまま返さず、PasswordHashを含まない公開項目だけに変換する。
	return userResponse{
		ID:        user.ID,
		Username:  user.Username,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func writeAuthError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, models.ErrInvalidUsername),
		errors.Is(err, models.ErrInvalidPassword):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid credentials format"})
	case errors.Is(err, services.ErrInvalidCredentials):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
	case errors.Is(err, services.ErrUsernameAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{"error": "username already exists"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
