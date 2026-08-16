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

type PostController struct {
	service *services.PostService
}

func NewPostController(service *services.PostService) *PostController {
	return &PostController{service: service}
}

// ListPostsHandler godoc
// @Summary List posts
// @Description Get all posts visible to the authenticated user.
// @Tags posts
// @Produce json
// @Security SessionCookie
// @Success 200 {array} dto.PostListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/posts [get]
func (pc *PostController) ListPostsHandler(c *gin.Context) {
	posts, err := pc.service.ListAllPosts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
		return
	}
	c.JSON(http.StatusOK, dto.PostListResponsesFromModels(posts))
}

// GetPostByIDHandler godoc
// @Summary Get post by ID
// @Description Get a post by ID when it is visible to the authenticated user.
// @Tags posts
// @Produce json
// @Security SessionCookie
// @Param id path string true "Post ID"
// @Success 200 {object} dto.PostDetailResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/posts/{id} [get]
func (pc *PostController) GetPostByIDHandler(c *gin.Context) {
	postID, ok := parsePostIDParam(c)
	if !ok {
		return
	}

	post, err := pc.service.GetPostByID(c.Request.Context(), postID)
	if err != nil {
		writePostError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.PostDetailResponseFromModel(post))
}

// CreatePostHandler godoc
// @Summary Create post
// @Description Create a post owned by the authenticated user.
// @Tags posts
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body dto.CreatePostRequest true "Create post payload"
// @Success 201
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/posts [post]
func (pc *PostController) CreatePostHandler(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "unauthorized"})
		return
	}

	var req dto.CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid request body"})
		return
	}

	if err := pc.service.CreatePost(c.Request.Context(), userID, req.Title, req.Content); err != nil {
		writePostError(c, err)
		return
	}
	c.Status(http.StatusCreated)
}

// UpdatePostHandler godoc
// @Summary Update current user's post
// @Description Update a post when it is owned by the authenticated user.
// @Tags posts
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path string true "Post ID"
// @Param request body dto.UpdatePostRequest true "Update post payload"
// @Success 200
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/posts/{id} [put]
func (pc *PostController) UpdatePostHandler(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "unauthorized"})
		return
	}

	postID, ok := parsePostIDParam(c)
	if !ok {
		return
	}

	// 本文のバインドより先に所有者条件で取得し、非所有者へPostの存在を推測させない。
	post, err := pc.service.GetPostByIDForOwner(c.Request.Context(), userID, postID)
	if err != nil {
		writePostError(c, err)
		return
	}

	var req dto.UpdatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid request body"})
		return
	}
	if req.Title == nil && req.Content == nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "title or content is required"})
		return
	}
	if req.Title != nil {
		post.Title = *req.Title
	}
	if req.Content != nil {
		post.Content = *req.Content
	}

	if err := pc.service.UpdatePost(c.Request.Context(), userID, post); err != nil {
		writePostError(c, err)
		return
	}
	c.Status(http.StatusOK)
}

// DeletePostHandler godoc
// @Summary Delete current user's post
// @Description Delete a post when it is owned by the authenticated user.
// @Tags posts
// @Produce json
// @Security SessionCookie
// @Param id path string true "Post ID"
// @Success 204
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/posts/{id} [delete]
func (pc *PostController) DeletePostHandler(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "unauthorized"})
		return
	}

	postID, ok := parsePostIDParam(c)
	if !ok {
		return
	}

	if err := pc.service.DeletePost(c.Request.Context(), userID, postID); err != nil {
		writePostError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func parsePostIDParam(c *gin.Context) (models.UUID, bool) {
	rawPostID := c.Param("id")
	parsedPostID, err := models.ParseUUID(rawPostID)
	if err != nil || parsedPostID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid post id"})
		return "", false
	}
	return parsedPostID, true
}

func writePostError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrPostNotFound):
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "post not found"})
	case errors.Is(err, models.ErrInvalidTitle),
		errors.Is(err, models.ErrInvalidContent):
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid post"})
	default:
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
	}
}
