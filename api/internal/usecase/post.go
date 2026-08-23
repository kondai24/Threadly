package usecase

import (
	"context"
	"errors"
	"fmt"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
)

// PostLikeSummaryReaderは、Postの閲覧結果に必要なLike集計だけを提供する。
// PostUsecaseがLike操作全体の実装へ依存しないよう、利用側で契約を定義する。
type PostLikeSummaryReader interface {
	PostSummaries(
		ctx context.Context,
		userID models.UUID,
		postIDs []models.UUID,
	) (map[models.UUID]models.LikeSummary, error)
}

// PostUsecaseは、PostControllerが必要とする業務操作の契約を定義する。
type PostUsecase interface {
	GetPostByID(ctx context.Context, postID models.UUID) (*models.Post, error)
	GetPostByIDForUser(
		ctx context.Context,
		userID models.UUID,
		postID models.UUID,
	) (*PostRead, error)
	GetPostByIDForOwner(
		ctx context.Context,
		userID models.UUID,
		postID models.UUID,
	) (*models.Post, error)
	ListAllPosts(ctx context.Context) ([]*models.Post, error)
	ListAllPostsForUser(ctx context.Context, userID models.UUID) ([]PostRead, error)
	CreatePost(ctx context.Context, userID models.UUID, title string, content string) error
	UpdatePost(ctx context.Context, userID models.UUID, post *models.Post) error
	DeletePost(ctx context.Context, userID models.UUID, postID models.UUID) error
}

// postUsecaseは、Postの公開取得・所有者操作・削除を組み立てるUsecaseである。
// Post削除のように複数Repositoryへまたがる処理はUnit of Workへ委譲する。
type postUsecase struct {
	repo       repositories.PostRepository
	uow        repositories.UnitOfWork
	likeReader PostLikeSummaryReader
}

// NewPostUsecaseは、Like集計を必要としないPost操作用のUsecaseを構築する。
func NewPostUsecase(
	repo repositories.PostRepository,
	uow repositories.UnitOfWork,
) PostUsecase {
	return &postUsecase{repo: repo, uow: uow}
}

// NewPostUsecaseWithLikeReaderは、Post取得結果へLike集計を付加するUsecaseを構築する。
func NewPostUsecaseWithLikeReader(
	repo repositories.PostRepository,
	uow repositories.UnitOfWork,
	likeReader PostLikeSummaryReader,
) PostUsecase {
	return &postUsecase{repo: repo, uow: uow, likeReader: likeReader}
}

// 認証済みUserが閲覧できるPostを取得する。閲覧時は所有者条件を付けない。
func (u *postUsecase) GetPostByID(ctx context.Context, postID models.UUID) (*models.Post, error) {
	post, err := u.repo.GetByID(ctx, postID)
	if err != nil {
		return nil, translatePostRepositoryError(err)
	}
	return post, nil
}

func (u *postUsecase) GetPostByIDForUser(
	ctx context.Context,
	userID models.UUID,
	postID models.UUID,
) (*PostRead, error) {
	post, err := u.GetPostByID(ctx, postID)
	if err != nil {
		return nil, err
	}
	summary, err := u.postSummary(ctx, userID, postID)
	if err != nil {
		return nil, err
	}
	return &PostRead{Post: post, Summary: summary}, nil
}

// 更新前の所有者確認など、所有者だけが扱うPostを取得する。
func (u *postUsecase) GetPostByIDForOwner(
	ctx context.Context,
	userID models.UUID,
	postID models.UUID,
) (*models.Post, error) {
	post, err := u.repo.GetByIDForOwner(ctx, userID, postID)
	if err != nil {
		return nil, translatePostRepositoryError(err)
	}
	return post, nil
}

// 認証済みUserが閲覧できる全Postを取得する。
func (u *postUsecase) ListAllPosts(ctx context.Context) ([]*models.Post, error) {
	return u.repo.ListAll(ctx)
}

func (u *postUsecase) ListAllPostsForUser(
	ctx context.Context,
	userID models.UUID,
) ([]PostRead, error) {
	posts, err := u.ListAllPosts(ctx)
	if err != nil {
		return nil, err
	}
	postIDs := make([]models.UUID, 0, len(posts))
	for _, post := range posts {
		if post != nil {
			postIDs = append(postIDs, post.ID)
		}
	}
	summaries, err := u.postSummaries(ctx, userID, postIDs)
	if err != nil {
		return nil, err
	}
	reads := make([]PostRead, 0, len(posts))
	for _, post := range posts {
		if post == nil {
			continue
		}
		reads = append(reads, PostRead{Post: post, Summary: summaries[post.ID]})
	}
	return reads, nil
}

func (u *postUsecase) postSummary(
	ctx context.Context,
	userID models.UUID,
	postID models.UUID,
) (models.LikeSummary, error) {
	if u.likeReader == nil {
		return models.LikeSummary{}, nil
	}
	summaries, err := u.likeReader.PostSummaries(ctx, userID, []models.UUID{postID})
	if err != nil {
		return models.LikeSummary{}, err
	}
	return summaries[postID], nil
}

func (u *postUsecase) postSummaries(
	ctx context.Context,
	userID models.UUID,
	postIDs []models.UUID,
) (map[models.UUID]models.LikeSummary, error) {
	if u.likeReader == nil {
		return makeLikeSummaries(postIDs), nil
	}
	return u.likeReader.PostSummaries(ctx, userID, postIDs)
}

// author_idはリクエストではなく、検証済みtokenのUser IDから設定する。
func (u *postUsecase) CreatePost(ctx context.Context, userID models.UUID, title string, content string) error {
	post := &models.Post{
		AuthorID: userID,
		Title:    title,
		Content:  content,
	}
	if err := post.Validate(); err != nil {
		return err
	}
	return u.repo.Create(ctx, post)
}

// 所有者でない場合はNotFoundとして扱い、他UserのPostの存在を隠す。
func (u *postUsecase) UpdatePost(ctx context.Context, userID models.UUID, post *models.Post) error {
	if post.AuthorID != userID {
		return ErrPostNotFound
	}
	if err := post.Validate(); err != nil {
		return err
	}
	if err := u.repo.Update(ctx, userID, post); err != nil {
		return translatePostRepositoryError(err)
	}
	return nil
}

// DeletePostは、認可・関連Likeのcleanup・Comment/Postの論理削除を同じTransactionで実行する。
// callback内ではroot DBのRepositoryを使わず、UoWから受け取った
// Transaction-bound Repositoryだけを使う。
func (u *postUsecase) DeletePost(ctx context.Context, userID models.UUID, postID models.UUID) error {
	var rows int64
	err := u.uow.WithinTransaction(ctx, func(tx repositories.TransactionRepositories) error {
		post, err := tx.Post.GetByIDForUpdate(ctx, postID)
		if err != nil {
			return err
		}
		if post.AuthorID != userID {
			return repositories.ErrPostNotFound
		}

		// CommentLikeは別Tableのため、Post配下CommentのLikeを先に同じTransactionで物理削除する。
		if err := tx.CommentLike.DeleteByCommentsOfPostID(ctx, post.ID); err != nil {
			return fmt.Errorf("delete post comment likes: %w", err)
		}
		if err := tx.PostLike.DeleteByPostID(ctx, post.ID); err != nil {
			return fmt.Errorf("delete post likes: %w", err)
		}
		if _, err := tx.Comment.DeleteByPostID(ctx, post.ID); err != nil {
			return fmt.Errorf("delete post comments: %w", err)
		}

		rows, err = tx.Post.DeleteByID(ctx, userID, post.ID)
		if err != nil {
			return fmt.Errorf("delete post: %w", err)
		}
		return nil
	})
	if err != nil {
		return translatePostRepositoryError(err)
	}
	if rows == 0 {
		return ErrPostNotFound
	}
	return nil
}

func translatePostRepositoryError(err error) error {
	if errors.Is(err, repositories.ErrPostNotFound) {
		return ErrPostNotFound
	}
	return err
}
