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
	"gorm.io/gorm"
)

type CommentController struct {
	service *services.CommentService
}

type createCommentRequest struct {
	Content  string  `json:"content" binding:"required" minLength:"1" maxLength:"1000" example:"コメント本文"`
	ParentID *string `json:"parentId,omitempty" example:"33333333-3333-4333-8333-333333333333"`
}

type updateCommentRequest struct {
	Content string `json:"content" binding:"required" minLength:"1" maxLength:"1000" example:"更新したコメント本文"`
}

type commentAuthorResponse struct {
	ID       models.UUID `json:"id"`
	Username string      `json:"username"`
}

type commentResponse struct {
	ID        models.UUID           `json:"id"`
	Content   string                `json:"content"`
	Author    commentAuthorResponse `json:"author"`
	CreatedAt time.Time             `json:"createdAt"`
	UpdatedAt time.Time             `json:"updatedAt"`
	Replies   []commentResponse     `json:"replies"`
}

func NewCommentController(service *services.CommentService) *CommentController {
	return &CommentController{service: service}
}

// ListCommentsHandler godoc
// @Summary List comments for a post
// @Description Get active comments nested by root comment and one-level replies.
// @Tags comments
// @Produce json
// @Security SessionCookie
// @Param id path string true "Post ID"
// @Success 200 {array} commentResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/posts/{id}/comments [get]
func (cc *CommentController) ListCommentsHandler(c *gin.Context) {
	postID, ok := parseCommentIDParam(c, "post id")
	if !ok {
		return
	}

	comments, err := cc.service.ListComments(c.Request.Context(), postID)
	if err != nil {
		writeCommentError(c, err)
		return
	}

	responses := make([]commentResponse, 0, len(comments))
	for _, comment := range comments {
		responses = append(responses, toCommentResponse(comment))
	}
	c.JSON(http.StatusOK, responses)
}

// CreateCommentHandler godoc
// @Summary Create a comment
// @Description Create a root comment or one-level reply for a post.
// @Tags comments
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path string true "Post ID"
// @Param request body createCommentRequest true "Create comment payload"
// @Success 201
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/posts/{id}/comments [post]
func (cc *CommentController) CreateCommentHandler(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	postID, ok := parseCommentIDParam(c, "post id")
	if !ok {
		return
	}

	var req createCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	parentID, err := parseOptionalCommentID(req.ParentID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid parent id"})
		return
	}

	if err := cc.service.CreateComment(
		c.Request.Context(),
		userID,
		postID,
		req.Content,
		parentID,
	); err != nil {
		writeCommentError(c, err)
		return
	}
	c.Status(http.StatusCreated)
}

// UpdateCommentHandler godoc
// @Summary Update a comment
// @Description Update a comment when the authenticated user is its author.
// @Tags comments
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path string true "Comment ID"
// @Param request body updateCommentRequest true "Update comment payload"
// @Success 200
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/comments/{id} [put]
func (cc *CommentController) UpdateCommentHandler(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	commentID, ok := parseCommentIDParam(c, "comment id")
	if !ok {
		return
	}

	var req updateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := cc.service.UpdateComment(
		c.Request.Context(),
		userID,
		commentID,
		req.Content,
	); err != nil {
		writeCommentError(c, err)
		return
	}
	c.Status(http.StatusOK)
}

// DeleteCommentHandler godoc
// @Summary Delete a comment
// @Description Soft-delete a comment and its direct replies when the authenticated user is its author.
// @Tags comments
// @Produce json
// @Security SessionCookie
// @Param id path string true "Comment ID"
// @Success 204
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/comments/{id} [delete]
func (cc *CommentController) DeleteCommentHandler(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	commentID, ok := parseCommentIDParam(c, "comment id")
	if !ok {
		return
	}

	if err := cc.service.DeleteComment(c.Request.Context(), userID, commentID); err != nil {
		writeCommentError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func parseCommentIDParam(c *gin.Context, name string) (models.UUID, bool) {
	parsedID, err := models.ParseUUID(c.Param("id"))
	if err != nil || parsedID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid " + name})
		return "", false
	}
	return parsedID, true
}

func parseOptionalCommentID(rawID *string) (*models.UUID, error) {
	if rawID == nil {
		return nil, nil
	}

	parsedID, err := models.ParseUUID(*rawID)
	if err != nil || parsedID == "" {
		return nil, errors.New("invalid parent id")
	}
	return &parsedID, nil
}

func toCommentResponse(comment *models.Comment) commentResponse {
	replies := make([]commentResponse, 0, len(comment.Replies))
	for _, reply := range comment.Replies {
		replies = append(replies, toCommentResponse(reply))
	}
	return commentResponse{
		ID:      comment.ID,
		Content: comment.Content,
		Author: commentAuthorResponse{
			ID:       comment.Author.ID,
			Username: comment.Author.Username,
		},
		CreatedAt: comment.CreatedAt,
		UpdatedAt: comment.UpdatedAt,
		Replies:   replies,
	}
}

func writeCommentError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, models.ErrInvalidCommentContent):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid comment"})
	case errors.Is(err, services.ErrCommentReplyNotAllowed):
		c.JSON(http.StatusBadRequest, gin.H{"error": "comment reply is not allowed"})
	case errors.Is(err, services.ErrPostNotFound),
		errors.Is(err, services.ErrCommentNotFound),
		errors.Is(err, repositories.ErrCommentNotFound),
		errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "comment or post not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
