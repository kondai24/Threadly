package controllers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"Threadly/internal/domain/models"
	"Threadly/internal/middleware"
	"Threadly/internal/usecase/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PostController struct {
	service *services.PostService
}

type CreatePostRequest struct {
	Title   string `json:"title" binding:"required"`
	Content string `json:"content" binding:"required"`
}

type UpdatePostRequest struct {
	Title   *string `json:"title"`
	Content *string `json:"content"`
}

type postAuthorResponse struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

type postListResponse struct {
	ID        uint               `json:"id"`
	Title     string             `json:"title"`
	Author    postAuthorResponse `json:"author"`
	CreatedAt time.Time          `json:"createdAt"`
}

type postDetailResponse struct {
	ID        uint               `json:"id"`
	Title     string             `json:"title"`
	Content   string             `json:"content"`
	Author    postAuthorResponse `json:"author"`
	CreatedAt time.Time          `json:"createdAt"`
	UpdatedAt time.Time          `json:"updatedAt"`
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
// @Success 200 {array} postListResponse
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/posts [get]
func (pc *PostController) ListPostsHandler(c *gin.Context) {
	posts, err := pc.service.ListAllPosts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	responses := make([]postListResponse, 0, len(posts))
	for _, post := range posts {
		responses = append(responses, toPostListResponse(post))
	}
	c.JSON(http.StatusOK, responses)
}

// GetPostByIDHandler godoc
// @Summary Get post by ID
// @Description Get a post by ID when it is visible to the authenticated user.
// @Tags posts
// @Produce json
// @Security SessionCookie
// @Param id path int true "Post ID"
// @Success 200 {object} postDetailResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
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
	c.JSON(http.StatusOK, toPostDetailResponse(post))
}

// CreatePostHandler godoc
// @Summary Create post
// @Description Create a post owned by the authenticated user.
// @Tags posts
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body CreatePostRequest true "Create post payload"
// @Success 201
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/posts [post]
func (pc *PostController) CreatePostHandler(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
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
// @Param id path int true "Post ID"
// @Param request body UpdatePostRequest true "Update post payload"
// @Success 200
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/posts/{id} [put]
func (pc *PostController) UpdatePostHandler(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	postID, ok := parsePostIDParam(c)
	if !ok {
		return
	}

	post, err := pc.service.GetPostByIDForOwner(c.Request.Context(), userID, postID)
	if err != nil {
		writePostError(c, err)
		return
	}

	var req UpdatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Title == nil && req.Content == nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "title or content is required"})
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
// @Param id path int true "Post ID"
// @Success 204
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/posts/{id} [delete]
func (pc *PostController) DeletePostHandler(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
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

func parsePostIDParam(c *gin.Context) (uint, bool) {
	rawPostID := c.Param("id")
	parsedPostID, err := strconv.ParseUint(rawPostID, 10, 64)
	if err != nil || parsedPostID == 0 || parsedPostID > uint64(^uint(0)) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid post id"})
		return 0, false
	}
	return uint(parsedPostID), true
}

func toPostAuthorResponse(author models.User) postAuthorResponse {
	return postAuthorResponse{
		ID:       author.ID,
		Username: author.Username,
	}
}

func toPostListResponse(post *models.Post) postListResponse {
	return postListResponse{
		ID:        post.ID,
		Title:     post.Title,
		Author:    toPostAuthorResponse(post.Author),
		CreatedAt: post.CreatedAt,
	}
}

func toPostDetailResponse(post *models.Post) postDetailResponse {
	return postDetailResponse{
		ID:        post.ID,
		Title:     post.Title,
		Content:   post.Content,
		Author:    toPostAuthorResponse(post.Author),
		CreatedAt: post.CreatedAt,
		UpdatedAt: post.UpdatedAt,
	}
}

func writePostError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrPostNotFound),
		errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
	case errors.Is(err, models.ErrInvalidTitle),
		errors.Is(err, models.ErrInvalidContent):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid post"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
