package repository

import (
	"context"

	"Threadly/internal/domain/repositories"

	"gorm.io/gorm"
)

type UnitOfWork struct {
	DB *gorm.DB
}

func NewUnitOfWork(db *gorm.DB) repositories.UnitOfWork {
	return &UnitOfWork{DB: db}
}

func (u *UnitOfWork) WithinTransaction(
	ctx context.Context,
	fn func(repositories.TransactionRepositories) error,
) error {
	return u.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(repositories.TransactionRepositories{
			Post:        NewPostRepository(tx),
			Comment:     NewCommentRepository(tx),
			PostLike:    NewPostLikeRepository(tx),
			CommentLike: NewCommentLikeRepository(tx),
		})
	})
}
