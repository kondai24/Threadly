package repository

import (
	"context"
	"errors"
	"fmt"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, repositories.ErrPostNotFound
	}
	if result.Error != nil {
		return nil, fmt.Errorf("find post by id: %w", result.Error)
	}
	return &post, nil
}

func (r *PostRepository) GetByIDForUpdate(
	ctx context.Context,
	postID models.UUID,
) (*models.Post, error) {
	var post models.Post
	result := r.DB.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", postID).
		First(&post)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, repositories.ErrPostNotFound
	}
	if result.Error != nil {
		return nil, fmt.Errorf("find post for update: %w", result.Error)
	}
	return &post, nil
}

func (r *PostRepository) GetByIDForOwner(ctx context.Context, userID models.UUID, postID models.UUID) (*models.Post, error) {
	var post models.Post
	result := r.DB.WithContext(ctx).
		Preload("Author").
		Where("id = ? AND author_id = ?", postID, userID).
		First(&post)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, repositories.ErrPostNotFound
	}
	if result.Error != nil {
		return nil, fmt.Errorf("find post by owner: %w", result.Error)
	}
	return &post, nil
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
		return repositories.ErrPostNotFound
	}
	return nil
}

func (r *PostRepository) DeleteByID(ctx context.Context, userID models.UUID, postID models.UUID) (int64, error) {
	result := r.DB.WithContext(ctx).
		Where("id = ? AND author_id = ?", postID, userID).
		Delete(&models.Post{})
	if result.Error != nil {
		return 0, fmt.Errorf("delete post: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func (r *PostRepository) ListAll(ctx context.Context) ([]*models.Post, error) {
	posts := make([]*models.Post, 0)
	result := r.DB.WithContext(ctx).
		Preload("Author").
		Find(&posts)
	return posts, result.Error
}
