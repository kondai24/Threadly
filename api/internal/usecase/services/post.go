package services

import (
	"context"
	"errors"
	"fmt"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
)

type PostService struct {
	repo repositories.PostRepository
	uow  repositories.UnitOfWork
}

func NewPostService(
	repo repositories.PostRepository,
	uow repositories.UnitOfWork,
) *PostService {
	return &PostService{repo: repo, uow: uow}
}

// 認証済みUserが閲覧できるPostを取得する。閲覧時は所有者条件を付けない。
func (s *PostService) GetPostByID(ctx context.Context, postID models.UUID) (*models.Post, error) {
	post, err := s.repo.GetByID(ctx, postID)
	if err != nil {
		return nil, translatePostRepositoryError(err)
	}
	return post, nil
}

// 更新前の所有者確認など、所有者だけが扱うPostを取得する。
func (s *PostService) GetPostByIDForOwner(ctx context.Context, userID models.UUID, postID models.UUID) (*models.Post, error) {
	post, err := s.repo.GetByIDForOwner(ctx, userID, postID)
	if err != nil {
		return nil, translatePostRepositoryError(err)
	}
	return post, nil
}

// 認証済みUserが閲覧できる全Postを取得する。
func (s *PostService) ListAllPosts(ctx context.Context) ([]*models.Post, error) {
	return s.repo.ListAll(ctx)
}

// author_idはリクエストではなく、検証済みtokenのUser IDから設定する。
func (s *PostService) CreatePost(ctx context.Context, userID models.UUID, title string, content string) error {
	post := &models.Post{
		AuthorID: userID,
		Title:    title,
		Content:  content,
	}
	if err := post.Validate(); err != nil {
		return err
	}
	return s.repo.Create(ctx, post)
}

// 所有者でない場合はNotFoundとして扱い、他UserのPostの存在を隠す。
func (s *PostService) UpdatePost(ctx context.Context, userID models.UUID, post *models.Post) error {
	if post.AuthorID != userID {
		return ErrPostNotFound
	}
	if err := post.Validate(); err != nil {
		return err
	}
	if err := s.repo.Update(ctx, userID, post); err != nil {
		return translatePostRepositoryError(err)
	}
	return nil
}

// CommentLike、PostLike、Comment、Postを同じTransactionで削除し、部分削除を防ぐ。
func (s *PostService) DeletePost(ctx context.Context, userID models.UUID, postID models.UUID) error {
	var rows int64
	err := s.uow.WithinTransaction(ctx, func(tx repositories.TransactionRepositories) error {
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
