package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"

	"gorm.io/gorm"
)

type CommentService struct {
	commentRepo repositories.CommentRepository
	postRepo    repositories.PostRepository
}

func NewCommentService(
	commentRepo repositories.CommentRepository,
	postRepo repositories.PostRepository,
) *CommentService {
	return &CommentService{
		commentRepo: commentRepo,
		postRepo:    postRepo,
	}
}

func (s *CommentService) ListComments(
	ctx context.Context,
	postID models.UUID,
) ([]*models.Comment, error) {
	if err := s.ensurePostExists(ctx, postID); err != nil {
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

	if err := s.ensurePostExists(ctx, postID); err != nil {
		return err
	}
	if parentID != nil {
		if err := s.validateParent(ctx, postID, *parentID); err != nil {
			return err
		}
	}

	if err := s.commentRepo.Create(ctx, comment); err != nil {
		return fmt.Errorf("create comment: %w", err)
	}
	return nil
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

func (s *CommentService) DeleteComment(
	ctx context.Context,
	userID models.UUID,
	commentID models.UUID,
) error {
	rows, err := s.commentRepo.DeleteByID(ctx, userID, commentID)
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	if rows == 0 {
		return ErrCommentNotFound
	}
	return nil
}

func (s *CommentService) ensurePostExists(ctx context.Context, postID models.UUID) error {
	_, err := s.postRepo.GetByID(ctx, postID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrPostNotFound
	}
	if err != nil {
		return fmt.Errorf("find post for comment: %w", err)
	}
	return nil
}

func (s *CommentService) validateParent(
	ctx context.Context,
	postID models.UUID,
	parentID models.UUID,
) error {
	parent, err := s.commentRepo.GetByID(ctx, parentID)
	if errors.Is(err, repositories.ErrCommentNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
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
