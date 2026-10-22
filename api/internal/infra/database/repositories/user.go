package repository

import (
	"context"
	"errors"
	"fmt"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
	"Threadly/internal/tracing"

	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

// UserRepositoryは、UserRepository契約をGORMへ適配する。
type UserRepository struct {
	DB     *gorm.DB
	Tracer trace.Tracer
}

// NewUserRepositoryは、指定されたDB handleへ結び付いたUserRepositoryを生成する。
func NewUserRepository(db *gorm.DB, tracer trace.Tracer) repositories.UserRepository {
	return &UserRepository{DB: db, Tracer: tracer}
}

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (_ *models.User, err error) {
	ctx, span := tracing.Start(ctx, r.Tracer, tracing.LayerRepository, "UserRepository.FindByUsername")
	defer func() { tracing.End(span, repositoryResult(err), recover()) }()

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

func (r *UserRepository) FindByID(ctx context.Context, id models.UUID) (_ *models.User, err error) {
	ctx, span := tracing.Start(ctx, r.Tracer, tracing.LayerRepository, "UserRepository.FindByID")
	defer func() { tracing.End(span, repositoryResult(err), recover()) }()

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

func (r *UserRepository) Create(ctx context.Context, user *models.User) (err error) {
	ctx, span := tracing.Start(ctx, r.Tracer, tracing.LayerRepository, "UserRepository.Create")
	defer func() { tracing.End(span, repositoryResult(err), recover()) }()

	err = r.DB.WithContext(ctx).Create(user).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return repositories.ErrUsernameAlreadyExists
	}
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}
