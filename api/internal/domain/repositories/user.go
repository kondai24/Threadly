package repositories

import (
	"context"
	"errors"

	"Threadly/internal/domain/models"
)

var (
	ErrUserNotFound          = errors.New("user not found")
	ErrUsernameAlreadyExists = errors.New("username already exists")
)

type UserRepository interface {
	FindByUsername(ctx context.Context, username string) (*models.User, error)
	FindByID(ctx context.Context, id models.UUID) (*models.User, error)
	Create(ctx context.Context, user *models.User) error
}
