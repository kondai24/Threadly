package controllers

import (
	"errors"
	"net/http"
	"strconv"
	"Threadly/internal/domain/models"
	"Threadly/internal/usecase/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Postに関するHTTPリクエストを処理するコントローラ
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

func NewPostController(service *services.PostService) *PostController {
	return &PostController{service: service}
}

// ListPostsHandler godoc
// @Summary      List posts
// @Description  Get all posts
// @Tags         posts
// @Produce      json
// @Success      200  {array}   models.Post
// @Failure      500  {object}  gin.H
// @Router       /api/posts [get]
func (pc *PostController) ListPostsHandler(c *gin.Context) {
	// Postの取得
	var posts []*models.Post
	posts, err := pc.service.ListPosts(c.Request.Context())
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, posts)
}

// GetPostByIdHandler godoc
// @Summary      Get post by ID
// @Description  Get one post by its ID
// @Tags         posts
// @Produce      json
// @Param        id   path      int  true  "Post ID"
// @Success      200  {object}  models.Post
// @Failure      400  {object}  gin.H
// @Failure      404  {object}  gin.H
// @Router       /api/posts/{id} [get]
func (pc *PostController) GetPostByIdHandler(c *gin.Context) {
	// パスパラメータからIDを取得
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid id: " + err.Error()})
		return
	}

	// IDに対応するPostを取得
	var post *models.Post
	post, err = pc.service.GetPostById(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, post)
}

// CreatePostHandler godoc
// @Summary      Create post
// @Description  Create a new post
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        request  body      CreatePostRequest  true  "Create post payload"
// @Success      201
// @Failure      400  {object}  gin.H
// @Failure      500  {object}  gin.H
// @Router       /api/posts [post]
func (pc *PostController) CreatePostHandler(c *gin.Context) {
	var req CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	// Postの作成
	err := pc.service.CreatePost(c.Request.Context(), req.Title, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusCreated)
}

// UpdatePostHandler godoc
// @Summary      Update post
// @Description  Update title and/or content of a post
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        id       path      int     true   "Post ID"
// @Param        request  body      UpdatePostRequest  true  "Update post payload"
// @Success      200
// @Failure      400  {object}  gin.H
// @Failure      404  {object}  gin.H
// @Failure      500  {object}  gin.H
// @Router       /api/posts/{id} [put]
func (pc *PostController) UpdatePostHandler(c *gin.Context) {
	// パスパラメータからIDを取得
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid id: " + err.Error()})
		return
	}

	var post *models.Post
	post, err = pc.service.GetPostById(c.Request.Context(), uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch post"})
	}

	var req UpdatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}
	if req.Title == nil && req.Content == nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "title or content is required"})
		return
	}

	// Postの更新
	if req.Title != nil {
		post.Title = *req.Title
	}
	if req.Content != nil {
		post.Content = *req.Content
	}
	err = pc.service.UpdatePost(c.Request.Context(), post)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
}

// DeletePostHandler godoc
// @Summary      Delete post
// @Description  Delete a post by ID
// @Tags         posts
// @Produce      json
// @Param        id   path  int  true  "Post ID"
// @Success      204
// @Failure      400  {object}  gin.H
// @Failure      404  {object}  gin.H
// @Failure      500  {object}  gin.H
// @Router       /api/posts/{id} [delete]
func (pc *PostController) DeletePostHandler(c *gin.Context) {
	// パスパラメータからIDを取得
	idInt, err := strconv.Atoi(c.Param("id"))
	if err != nil || idInt <= 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid id: " + err.Error()})
		return
	}

	// Postの削除
	id := uint(idInt)
	err = pc.service.DeletePost(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrPostNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete post"})
		return
	}
	c.Status(http.StatusNoContent)
}
