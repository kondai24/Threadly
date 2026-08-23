package repository

import (
	"context"

	"Threadly/internal/domain/repositories"

	"gorm.io/gorm"
)

// UnitOfWorkは、root DBから業務Transactionを開始するGORM adapterである。
// Transaction中のRepositoryはcallback引数のtxから生成し、同じDB handleを共有する。
type UnitOfWork struct {
	DB *gorm.DB
}

// NewUnitOfWorkは、アプリケーション全体で共有するroot DBをUnitOfWorkへ注入する。
func NewUnitOfWork(db *gorm.DB) repositories.UnitOfWork {
	return &UnitOfWork{DB: db}
}

func (u *UnitOfWork) WithinTransaction(
	ctx context.Context,
	fn func(repositories.TransactionRepositories) error,
) error {
	return u.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// GORMのcallback引数からRepositoryを作ることで、callbackのエラーはRollbackへ伝播する。
		return fn(repositories.TransactionRepositories{
			Post:        NewPostRepository(tx),
			Comment:     NewCommentRepository(tx),
			PostLike:    NewPostLikeRepository(tx),
			CommentLike: NewCommentLikeRepository(tx),
		})
	})
}
