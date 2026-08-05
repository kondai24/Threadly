package repositories

import (
	"Threadly/internal/domain/models"
	"context"
)

type PostRepository interface {
	GetByID(ctx context.Context, postID uint) (*models.Post, error)
	GetByIDForOwner(ctx context.Context, userID uint, postID uint) (*models.Post, error)
	Create(ctx context.Context, post *models.Post) error
	Update(ctx context.Context, userID uint, post *models.Post) error
	DeleteByID(ctx context.Context, userID uint, postID uint) (int64, error)
	ListAll(ctx context.Context) ([]*models.Post, error)
}
