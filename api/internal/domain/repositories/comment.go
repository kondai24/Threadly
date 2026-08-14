package repositories

import (
	"context"
	"errors"

	"Threadly/internal/domain/models"
)

var ErrCommentNotFound = errors.New("comment not found")

type CommentRepository interface {
	Create(ctx context.Context, comment *models.Comment) error
	ListByPostID(ctx context.Context, postID models.UUID) ([]*models.Comment, error)
	GetByID(ctx context.Context, commentID models.UUID) (*models.Comment, error)
	Update(ctx context.Context, userID models.UUID, commentID models.UUID, content string) (int64, error)
	DeleteByID(ctx context.Context, userID models.UUID, commentID models.UUID) (int64, error)
}
