package routes

import (
	docs "Threadly/docs"
	"Threadly/internal/interface/controllers"
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type Handlers struct {
	Post *controllers.PostController
}

func SetupRouter(h Handlers) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	docs.SwaggerInfo.BasePath = "/"
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	router.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })

	api := router.Group("/api")
	registerPostRoutes(api, h)

	return router
}
