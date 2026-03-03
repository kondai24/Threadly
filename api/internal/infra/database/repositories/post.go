package repository

import (
	"context"
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

// `api/internal/domain/repositories/post.go`で定義したPostRepositoryインターフェースの実装
func (r *PostRepository) GetById(ctx context.Context, id uint) (*models.Post, error) {
	var post models.Post
	result := r.DB.WithContext(ctx).First(&post, id)
	return &post, result.Error
}

func (r *PostRepository) Create(ctx context.Context, post *models.Post) error {
	return r.DB.WithContext(ctx).Create(post).Error
}

func (r *PostRepository) Update(ctx context.Context, post *models.Post) error {
	return r.DB.WithContext(ctx).Save(post).Error
}

func (r *PostRepository) DeleteById(ctx context.Context, id uint) (int64, error) {
	result := r.DB.WithContext(ctx).Delete(&models.Post{}, id)
	return result.RowsAffected, result.Error
}

func (r *PostRepository) List(ctx context.Context) ([]*models.Post, error) {
	var posts []*models.Post
	result := r.DB.WithContext(ctx).Find(&posts)
	return posts, result.Error
}
