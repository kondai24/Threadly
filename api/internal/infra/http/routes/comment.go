package routes

import "github.com/gin-gonic/gin"

func registerCommentRoutes(api *gin.RouterGroup, h Handlers) {
	posts := api.Group("/posts")
	posts.GET("/:id/comments", h.Comment.ListCommentsHandler)
	posts.POST("/:id/comments", h.Comment.CreateCommentHandler)

	comments := api.Group("/comments")
	comments.PUT("/:id", h.Comment.UpdateCommentHandler)
	comments.DELETE("/:id", h.Comment.DeleteCommentHandler)
}
