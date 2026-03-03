package routes

import "github.com/gin-gonic/gin"

func registerPostRoutes(api *gin.RouterGroup, h Handlers) {
	posts := api.Group("/posts")
	{
		posts.GET("", h.Post.ListPostsHandler)
		posts.POST("", h.Post.CreatePostHandler)
		posts.GET("/:id", h.Post.GetPostByIdHandler)
		posts.PUT("/:id", h.Post.UpdatePostHandler)
		posts.DELETE("/:id", h.Post.DeletePostHandler)
	}
}
