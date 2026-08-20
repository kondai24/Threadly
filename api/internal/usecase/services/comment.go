package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
)

type CommentService struct {
	commentRepo repositories.CommentRepository
	postRepo    repositories.PostRepository
	uow         repositories.UnitOfWork
}

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

func (s *CommentService) ListComments(
	ctx context.Context,
	postID models.UUID,
) ([]*models.Comment, error) {
	if err := s.ensurePostExists(ctx, s.postRepo, postID); err != nil {
		return nil, err
	}

	comments, err := s.commentRepo.ListByPostID(ctx, postID)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	return comments, nil
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

// CommentLikeとCommentを同じTransactionで削除し、Likeだけが残る状態を防ぐ。
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

		commentIDs := []models.UUID{comment.ID}
		if comment.ParentID == nil {
			replyIDs, err := tx.Comment.ListIDsByParentID(ctx, comment.ID)
			if err != nil {
				return fmt.Errorf("list comment reply ids: %w", err)
			}
			commentIDs = append(commentIDs, replyIDs...)
		}
		if err := tx.CommentLike.DeleteByCommentIDs(ctx, commentIDs); err != nil {
			return fmt.Errorf("delete comment likes: %w", err)
		}

		rows, err = tx.Comment.DeleteByID(ctx, userID, comment.ID)
		if err != nil {
			return fmt.Errorf("delete comment: %w", err)
		}
		if rows == 0 {
			return repositories.ErrCommentNotFound
		}
		if comment.ParentID == nil {
			deletedReplies, err := tx.Comment.DeleteRepliesByParentID(ctx, comment.ID)
			if err != nil {
				return fmt.Errorf("delete comment replies: %w", err)
			}
			rows += deletedReplies
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
