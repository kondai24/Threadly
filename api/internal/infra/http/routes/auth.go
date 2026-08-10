package routes

import "github.com/gin-gonic/gin"

func registerAuthRoutes(api *gin.RouterGroup, h Handlers) {
	auth := api.Group("/auth")
	auth.POST("/register", h.Auth.RegisterHandler)
	auth.POST("/login", h.Auth.LoginHandler)
	auth.POST("/logout", h.Auth.LogoutHandler)
}
