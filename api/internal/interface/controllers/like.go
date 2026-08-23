package controllers

import (
	"errors"
	"net/http"

	"Threadly/internal/domain/models"
	"Threadly/internal/interface/dto"
	"Threadly/internal/middleware"
	"Threadly/internal/usecase/services"

	"github.com/gin-gonic/gin"
)

type LikeController struct {
	service *services.LikeService
}

func NewLikeController(service *services.LikeService) *LikeController {
	return &LikeController{service: service}
}

// LikePostHandler godoc
// @Summary Like a post
// @Description Create or keep the authenticated user's Like for a post.
// @Tags likes
// @Produce json
// @Security SessionCookie
// @Param id path string true "Post ID"
// @Success 200 {object} dto.LikeActionResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/posts/{id}/like [put]
func (lc *LikeController) LikePostHandler(c *gin.Context) {
	lc.handlePostLike(c, true)
}

// UnlikePostHandler godoc
// @Summary Unlike a post
// @Description Remove the authenticated user's Like from a post.
// @Tags likes
// @Produce json
// @Security SessionCookie
// @Param id path string true "Post ID"
// @Success 200 {object} dto.LikeActionResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/posts/{id}/like [delete]
func (lc *LikeController) UnlikePostHandler(c *gin.Context) {
	lc.handlePostLike(c, false)
}

// LikeCommentHandler godoc
// @Summary Like a comment
// @Description Create or keep the authenticated user's Like for a comment.
// @Tags likes
// @Produce json
// @Security SessionCookie
// @Param id path string true "Comment ID"
// @Success 200 {object} dto.LikeActionResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/comments/{id}/like [put]
func (lc *LikeController) LikeCommentHandler(c *gin.Context) {
	lc.handleCommentLike(c, true)
}

// UnlikeCommentHandler godoc
// @Summary Unlike a comment
// @Description Remove the authenticated user's Like from a comment.
// @Tags likes
// @Produce json
// @Security SessionCookie
// @Param id path string true "Comment ID"
// @Success 200 {object} dto.LikeActionResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/comments/{id}/like [delete]
func (lc *LikeController) UnlikeCommentHandler(c *gin.Context) {
	lc.handleCommentLike(c, false)
}

func (lc *LikeController) handlePostLike(c *gin.Context, like bool) {
	userID, targetID, ok := parseLikeRequest(c)
	if !ok {
		return
	}

	var (
		result services.LikeActionResult
		err    error
	)
	if like {
		result, err = lc.service.LikePost(c.Request.Context(), userID, targetID)
	} else {
		result, err = lc.service.UnlikePost(c.Request.Context(), userID, targetID)
	}
	writeLikeResult(c, result, err)
}

func (lc *LikeController) handleCommentLike(c *gin.Context, like bool) {
	userID, targetID, ok := parseLikeRequest(c)
	if !ok {
		return
	}

	var (
		result services.LikeActionResult
		err    error
	)
	if like {
		result, err = lc.service.LikeComment(c.Request.Context(), userID, targetID)
	} else {
		result, err = lc.service.UnlikeComment(c.Request.Context(), userID, targetID)
	}
	writeLikeResult(c, result, err)
}

func parseLikeRequest(c *gin.Context) (models.UUID, models.UUID, bool) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "unauthorized"})
		return "", "", false
	}
	targetID, err := models.ParseUUID(c.Param("id"))
	if err != nil || targetID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid like target id"})
		return "", "", false
	}
	return userID, targetID, true
}

func writeLikeResult(c *gin.Context, result services.LikeActionResult, err error) {
	switch {
	case errors.Is(err, services.ErrLikeTargetNotFound):
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "like target not found"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
	default:
		c.JSON(http.StatusOK, dto.LikeActionResponseFromResult(result))
	}
}
