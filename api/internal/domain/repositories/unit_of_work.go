package repositories

import "context"

// TransactionRepositoriesは、同じTransactionで利用するRepositoryをまとめたもの。
type TransactionRepositories struct {
	Post        PostRepository
	Comment     CommentRepository
	PostLike    PostLikeRepository
	CommentLike CommentLikeRepository
}

type UnitOfWork interface {
	WithinTransaction(
		ctx context.Context,
		fn func(TransactionRepositories) error,
	) error
}
