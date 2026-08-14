package repositories

import (
	"Threadly/internal/domain/models"
	"context"
)

type PostRepository interface {
	GetByID(ctx context.Context, postID models.UUID) (*models.Post, error)
	GetByIDForOwner(ctx context.Context, userID models.UUID, postID models.UUID) (*models.Post, error)
	Create(ctx context.Context, post *models.Post) error
	Update(ctx context.Context, userID models.UUID, post *models.Post) error
	DeleteByID(ctx context.Context, userID models.UUID, postID models.UUID) (int64, error)
	ListAll(ctx context.Context) ([]*models.Post, error)
}
