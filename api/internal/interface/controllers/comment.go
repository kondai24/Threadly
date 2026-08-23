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
	"gorm.io/gorm"
)

type CommentController struct {
	service *services.CommentService
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
// @Success 200 {array} dto.CommentResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/posts/{id}/comments [get]
func (cc *CommentController) ListCommentsHandler(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "unauthorized"})
		return
	}

	postID, ok := parseCommentIDParam(c, "post id")
	if !ok {
		return
	}

	comments, err := cc.service.ListCommentsForUser(c.Request.Context(), userID, postID)
	if err != nil {
		writeCommentError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.CommentResponsesFromRead(comments))
}

// CreateCommentHandler godoc
// @Summary Create a comment
// @Description Create a root comment or one-level reply for a post.
// @Tags comments
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path string true "Post ID"
// @Param request body dto.CreateCommentRequest true "Create comment payload"
// @Success 201
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/posts/{id}/comments [post]
func (cc *CommentController) CreateCommentHandler(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "unauthorized"})
		return
	}

	postID, ok := parseCommentIDParam(c, "post id")
	if !ok {
		return
	}

	var req dto.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid request body"})
		return
	}

	parentID, err := parseOptionalCommentID(req.ParentID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid parent id"})
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
// @Param request body dto.UpdateCommentRequest true "Update comment payload"
// @Success 200
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/comments/{id} [put]
func (cc *CommentController) UpdateCommentHandler(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "unauthorized"})
		return
	}

	commentID, ok := parseCommentIDParam(c, "comment id")
	if !ok {
		return
	}

	var req dto.UpdateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid request body"})
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
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/comments/{id} [delete]
func (cc *CommentController) DeleteCommentHandler(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "unauthorized"})
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
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid " + name})
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

func writeCommentError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, models.ErrInvalidCommentContent):
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid comment"})
	case errors.Is(err, services.ErrCommentReplyNotAllowed):
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "comment reply is not allowed"})
	case errors.Is(err, services.ErrPostNotFound),
		errors.Is(err, services.ErrCommentNotFound),
		errors.Is(err, repositories.ErrCommentNotFound),
		errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "comment or post not found"})
	default:
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
	}
}
