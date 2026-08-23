package repositories

import "context"

// TransactionRepositoriesは、同じTransaction-bound DBを保持するRepositoryをまとめたもの。
// Usecaseはこのスコープを使い、起動時に注入されたroot DBのRepositoryへ戻らない。
type TransactionRepositories struct {
	Post        PostRepository
	Comment     CommentRepository
	PostLike    PostLikeRepository
	CommentLike CommentLikeRepository
}

// UnitOfWorkは、複数の永続化操作を一つの業務Transactionへ束ねる契約である。
type UnitOfWork interface {
	// WithinTransactionは、成功時にCommitし、callbackのエラー時にRollbackする。
	// callbackへ渡されるRepositoryは、同じTransactionへ結び付いている。
	WithinTransaction(
		ctx context.Context,
		fn func(TransactionRepositories) error,
	) error
}
