package repository

import (
	"context"

	"Threadly/internal/domain/repositories"

	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

// UnitOfWorkは、root DBから業務Transactionを開始するGORM adapterである。
// Transaction中のRepositoryはcallback引数のtxから生成し、同じDB handleを共有する。
type UnitOfWork struct {
	DB     *gorm.DB
	Tracer trace.Tracer
}

// NewUnitOfWorkは、アプリケーション全体で共有するroot DBとTracerをUnitOfWorkへ注入する。
func NewUnitOfWork(db *gorm.DB, tracer trace.Tracer) repositories.UnitOfWork {
	return &UnitOfWork{DB: db, Tracer: tracer}
}

func (u *UnitOfWork) WithinTransaction(
	ctx context.Context,
	fn func(repositories.TransactionRepositories) error,
) error {
	return u.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// GORMのcallback引数からRepositoryを作ることで、callbackのエラーはRollbackへ伝播する。
		// UoW自体はSpanを作らず、tx-bound Repositoryの各操作がcallerのContext配下にSpanを残す。
		return fn(repositories.TransactionRepositories{
			Post:        NewPostRepository(tx, u.Tracer),
			Comment:     NewCommentRepository(tx, u.Tracer),
			PostLike:    NewPostLikeRepository(tx, u.Tracer),
			CommentLike: NewCommentLikeRepository(tx, u.Tracer),
		})
	})
}
