package routes

import (
	docs "Threadly/docs"
	"Threadly/internal/interface/controllers"
	"Threadly/internal/middleware"
	"Threadly/internal/usecase/services"
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type Handlers struct {
	Auth        *controllers.AuthController
	Post        *controllers.PostController
	Comment     *controllers.CommentController
	TokenIssuer services.TokenIssuer
}

func SetupRouter(h Handlers) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	docs.SwaggerInfo.BasePath = "/"
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	router.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })

	api := router.Group("/api")
	registerAuthRoutes(api, h)

	// /meとPost APIはすべて同じ認証Middlewareを通す。
	protected := api.Group("")
	protected.Use(middleware.RequireAuth(h.TokenIssuer))
	protected.GET("/me", h.Auth.MeHandler)
	registerPostRoutes(protected, h)
	registerCommentRoutes(protected, h)

	return router
}
