package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
)

// CommentLikeSummaryReaderは、Commentの閲覧結果に必要なLike集計だけを提供する。
// CommentServiceがLike操作全体の実装へ依存しないよう、利用側で契約を定義する。
type CommentLikeSummaryReader interface {
	CommentSummaries(
		ctx context.Context,
		userID models.UUID,
		commentIDs []models.UUID,
	) (map[models.UUID]models.LikeSummary, error)
}

// CommentServiceは、Commentの取得・作成・所有者操作を組み立てるUsecaseである。
// 返信作成と削除は、親子関係の確認と書き込みを同じUnit of Workへ束ねる。
type CommentService struct {
	commentRepo repositories.CommentRepository
	postRepo    repositories.PostRepository
	uow         repositories.UnitOfWork
	likeReader  CommentLikeSummaryReader
}

// NewCommentServiceは、Like集計を必要としないComment操作用のUsecaseを構築する。
func NewCommentService(
	commentRepo repositories.CommentRepository,
	postRepo repositories.PostRepository,
	uow repositories.UnitOfWork,
) *CommentService {
	return &CommentService{
		commentRepo: commentRepo,
		postRepo:    postRepo,
		uow:         uow,
	}
}

// NewCommentServiceWithLikeReaderは、Comment取得結果へLike集計を付加するUsecaseを構築する。
func NewCommentServiceWithLikeReader(
	commentRepo repositories.CommentRepository,
	postRepo repositories.PostRepository,
	uow repositories.UnitOfWork,
	likeReader CommentLikeSummaryReader,
) *CommentService {
	return &CommentService{
		commentRepo: commentRepo,
		postRepo:    postRepo,
		uow:         uow,
		likeReader:  likeReader,
	}
}

func (s *CommentService) ListComments(
	ctx context.Context,
	postID models.UUID,
) ([]*models.Comment, error) {
	// CommentRepositoryの検索結果だけではPost自体のNotFoundと空Comment一覧を
	// 区別できないため、先にPostを確認する。
	if err := s.ensurePostExists(ctx, s.postRepo, postID); err != nil {
		return nil, err
	}

	comments, err := s.commentRepo.ListByPostID(ctx, postID)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	return comments, nil
}

func (s *CommentService) ListCommentsForUser(
	ctx context.Context,
	userID models.UUID,
	postID models.UUID,
) (CommentListRead, error) {
	comments, err := s.ListComments(ctx, postID)
	if err != nil {
		return CommentListRead{}, err
	}
	commentIDs := make([]models.UUID, 0)
	for _, comment := range comments {
		appendCommentIDs(&commentIDs, comment)
	}
	var summaries map[models.UUID]models.LikeSummary
	if s.likeReader == nil {
		summaries = makeLikeSummaries(commentIDs)
	} else {
		summaries, err = s.likeReader.CommentSummaries(ctx, userID, commentIDs)
		if err != nil {
			return CommentListRead{}, err
		}
	}
	return CommentListRead{Comments: comments, Summaries: summaries}, nil
}

func appendCommentIDs(ids *[]models.UUID, comment *models.Comment) {
	if comment == nil {
		return
	}
	*ids = append(*ids, comment.ID)
	for _, reply := range comment.Replies {
		appendCommentIDs(ids, reply)
	}
}

func (s *CommentService) CreateComment(
	ctx context.Context,
	userID models.UUID,
	postID models.UUID,
	content string,
	parentID *models.UUID,
) error {
	comment := &models.Comment{
		PostID:   postID,
		AuthorID: userID,
		Content:  strings.TrimSpace(content),
		ParentID: parentID,
	}
	if err := comment.Validate(); err != nil {
		return err
	}

	// Postまたは親Commentが削除される競合を防ぐため、存在確認とComment作成を
	// 同じTransactionへ入れる。
	return s.uow.WithinTransaction(ctx, func(tx repositories.TransactionRepositories) error {
		if err := s.ensurePostExistsForUpdate(ctx, tx.Post, postID); err != nil {
			return err
		}
		if parentID != nil {
			if err := s.validateParent(ctx, tx.Comment, postID, *parentID); err != nil {
				return err
			}
		}

		if err := tx.Comment.Create(ctx, comment); err != nil {
			return fmt.Errorf("create comment: %w", err)
		}
		return nil
	})
}

func (s *CommentService) UpdateComment(
	ctx context.Context,
	userID models.UUID,
	commentID models.UUID,
	content string,
) error {
	comment := &models.Comment{Content: strings.TrimSpace(content)}
	if err := comment.Validate(); err != nil {
		return err
	}

	rows, err := s.commentRepo.Update(ctx, userID, commentID, comment.Content)
	if err != nil {
		if errors.Is(err, repositories.ErrCommentNotFound) {
			return ErrCommentNotFound
		}
		return fmt.Errorf("update comment: %w", err)
	}
	if rows == 0 {
		return ErrCommentNotFound
	}
	return nil
}

// DeleteCommentは、CommentLikeの物理削除とComment・直接返信の論理削除を
// 同じTransactionで実行する。
// CommentRepositoryはCommentの削除範囲を、CommentLikeRepositoryはLikeの削除範囲を担当する。
func (s *CommentService) DeleteComment(
	ctx context.Context,
	userID models.UUID,
	commentID models.UUID,
) error {
	var rows int64
	err := s.uow.WithinTransaction(ctx, func(tx repositories.TransactionRepositories) error {
		comment, err := tx.Comment.GetByIDForUpdate(ctx, commentID)
		if err != nil {
			return err
		}
		if comment.AuthorID != userID {
			return repositories.ErrCommentNotFound
		}

		// CommentLikeは別Table、Commentと直接返信はCommentRepositoryが扱うため、
		// 削除順序だけをUsecaseで組み合わせる。
		if err := tx.CommentLike.DeleteByCommentIDWithReplies(ctx, comment.ID); err != nil {
			return fmt.Errorf("delete comment likes: %w", err)
		}

		rows, err = tx.Comment.DeleteByIDWithReplies(ctx, userID, comment.ID)
		if err != nil {
			return fmt.Errorf("delete comment with replies: %w", err)
		}
		if rows == 0 {
			return repositories.ErrCommentNotFound
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, repositories.ErrCommentNotFound) {
			return ErrCommentNotFound
		}
		return fmt.Errorf("delete comment: %w", err)
	}
	if rows == 0 {
		return ErrCommentNotFound
	}
	return nil
}

func (s *CommentService) ensurePostExists(
	ctx context.Context,
	postRepo repositories.PostRepository,
	postID models.UUID,
) error {
	_, err := postRepo.GetByID(ctx, postID)
	if err != nil {
		if translated := translatePostRepositoryError(err); errors.Is(translated, ErrPostNotFound) {
			return translated
		}
		return fmt.Errorf("find post for comment: %w", err)
	}
	return nil
}

func (s *CommentService) ensurePostExistsForUpdate(
	ctx context.Context,
	postRepo repositories.PostRepository,
	postID models.UUID,
) error {
	_, err := postRepo.GetByIDForUpdate(ctx, postID)
	if err != nil {
		if translated := translatePostRepositoryError(err); errors.Is(translated, ErrPostNotFound) {
			return translated
		}
		return fmt.Errorf("find post for comment: %w", err)
	}
	return nil
}

func (s *CommentService) validateParent(
	ctx context.Context,
	commentRepo repositories.CommentRepository,
	postID models.UUID,
	parentID models.UUID,
) error {
	parent, err := commentRepo.GetByIDForUpdate(ctx, parentID)
	if errors.Is(err, repositories.ErrCommentNotFound) {
		return ErrCommentNotFound
	}
	if err != nil {
		return fmt.Errorf("find comment parent: %w", err)
	}

	// 返信先は同じPostの有効なルートCommentに限定し、別Post参照と2段目の返信を拒否する。
	if parent.PostID != postID {
		return ErrCommentNotFound
	}
	if parent.ParentID != nil {
		return ErrCommentReplyNotAllowed
	}
	return nil
}
