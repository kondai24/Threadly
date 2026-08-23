package repositories

import (
	"context"
	"errors"

	"Threadly/internal/domain/models"
)

var (
	ErrPostNotFound = errors.New("post not found")
)

type PostRepository interface {
	GetByID(ctx context.Context, postID models.UUID) (*models.Post, error)
	GetByIDForUpdate(ctx context.Context, postID models.UUID) (*models.Post, error)
	GetByIDForOwner(ctx context.Context, userID models.UUID, postID models.UUID) (*models.Post, error)
	Create(ctx context.Context, post *models.Post) error
	Update(ctx context.Context, userID models.UUID, post *models.Post) error
	DeleteByID(ctx context.Context, userID models.UUID, postID models.UUID) (int64, error)
	ListAll(ctx context.Context) ([]*models.Post, error)
}
