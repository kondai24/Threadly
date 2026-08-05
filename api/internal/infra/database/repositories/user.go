package repository

import (
	"context"
	"errors"
	"fmt"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"

	"gorm.io/gorm"
)

type UserRepository struct {
	DB *gorm.DB
}

func NewUserRepository(db *gorm.DB) repositories.UserRepository {
	return &UserRepository{DB: db}
}

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	result := r.DB.WithContext(ctx).Where("username = ?", username).First(&user)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, repositories.ErrUserNotFound
	}
	if result.Error != nil {
		return nil, fmt.Errorf("find user by username: %w", result.Error)
	}
	return &user, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id uint) (*models.User, error) {
	var user models.User
	result := r.DB.WithContext(ctx).First(&user, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, repositories.ErrUserNotFound
	}
	if result.Error != nil {
		return nil, fmt.Errorf("find user by id: %w", result.Error)
	}
	return &user, nil
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	err := r.DB.WithContext(ctx).Create(user).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return repositories.ErrUsernameAlreadyExists
	}
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}
