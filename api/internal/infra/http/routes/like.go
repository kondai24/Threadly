package routes

import "github.com/gin-gonic/gin"

func registerLikeRoutes(api *gin.RouterGroup, h Handlers) {
	posts := api.Group("/posts")
	posts.PUT("/:id/like", h.Like.LikePostHandler)
	posts.DELETE("/:id/like", h.Like.UnlikePostHandler)

	comments := api.Group("/comments")
	comments.PUT("/:id/like", h.Like.LikeCommentHandler)
	comments.DELETE("/:id/like", h.Like.UnlikeCommentHandler)
}
