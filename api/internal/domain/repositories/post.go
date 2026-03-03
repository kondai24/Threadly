package repositories

import (
	"context"
	"Threadly/internal/domain/models"
)

type PostRepository interface {
	GetById(ctx context.Context, id uint) (*models.Post, error)
	Create(ctx context.Context, post *models.Post) error
	Update(ctx context.Context, post *models.Post) error
	DeleteById(ctx context.Context, id uint) (int64, error)
	List(ctx context.Context) ([]*models.Post, error)
}
