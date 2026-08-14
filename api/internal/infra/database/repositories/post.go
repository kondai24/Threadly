package repository

import (
	"context"
	"fmt"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"

	"gorm.io/gorm"
)

type PostRepository struct {
	DB *gorm.DB
}

func NewPostRepository(db *gorm.DB) repositories.PostRepository {
	return &PostRepository{DB: db}
}

func (r *PostRepository) GetByID(ctx context.Context, postID models.UUID) (*models.Post, error) {
	var post models.Post
	result := r.DB.WithContext(ctx).
		Preload("Author").
		Where("id = ?", postID).
		First(&post)
	return &post, result.Error
}

func (r *PostRepository) GetByIDForOwner(ctx context.Context, userID models.UUID, postID models.UUID) (*models.Post, error) {
	var post models.Post
	result := r.DB.WithContext(ctx).
		Preload("Author").
		Where("id = ? AND author_id = ?", postID, userID).
		First(&post)
	return &post, result.Error
}

func (r *PostRepository) Create(ctx context.Context, post *models.Post) error {
	return r.DB.WithContext(ctx).Create(post).Error
}

func (r *PostRepository) Update(ctx context.Context, userID models.UUID, post *models.Post) error {
	// mapを使い、更新値が空文字でもGORMに無視されないようにする。
	result := r.DB.WithContext(ctx).
		Model(&models.Post{}).
		Where("id = ? AND author_id = ?", post.ID, userID).
		Updates(map[string]any{
			"title":   post.Title,
			"content": post.Content,
		})
	if result.Error != nil {
		return fmt.Errorf("update post: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *PostRepository) DeleteByID(ctx context.Context, userID models.UUID, postID models.UUID) (int64, error) {
	result := r.DB.WithContext(ctx).
		Where("id = ? AND author_id = ?", postID, userID).
		Delete(&models.Post{})
	return result.RowsAffected, result.Error
}

func (r *PostRepository) ListAll(ctx context.Context) ([]*models.Post, error) {
	posts := make([]*models.Post, 0)
	result := r.DB.WithContext(ctx).
		Preload("Author").
		Find(&posts)
	return posts, result.Error
}
