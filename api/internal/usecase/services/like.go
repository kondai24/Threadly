package services

import (
	"context"
	"errors"
	"fmt"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
)

var ErrLikeTargetNotFound = errors.New("like target not found")

// LikeActionResultはLike操作後に返す対象IDと集計結果を表す。
type LikeActionResult struct {
	TargetID models.UUID
	Summary  models.LikeSummary
}

// PostReadはPostの公開Modelとcurrent-user依存のLike集計を分離して保持する。
type PostRead struct {
	Post    *models.Post
	Summary models.LikeSummary
}

// CommentListReadはCommentのネスト結果とIDごとのLike集計を保持する。
type CommentListRead struct {
	Comments  []*models.Comment
	Summaries map[models.UUID]models.LikeSummary
}

type LikeService struct {
	postRepo        repositories.PostRepository
	commentRepo     repositories.CommentRepository
	postLikeRepo    repositories.PostLikeRepository
	commentLikeRepo repositories.CommentLikeRepository
}

func NewLikeService(
	postRepo repositories.PostRepository,
	commentRepo repositories.CommentRepository,
	postLikeRepo repositories.PostLikeRepository,
	commentLikeRepo repositories.CommentLikeRepository,
) *LikeService {
	return &LikeService{
		postRepo:        postRepo,
		commentRepo:     commentRepo,
		postLikeRepo:    postLikeRepo,
		commentLikeRepo: commentLikeRepo,
	}
}

func (s *LikeService) LikePost(
	ctx context.Context,
	userID models.UUID,
	postID models.UUID,
) (LikeActionResult, error) {
	if err := s.ensurePost(ctx, postID); err != nil {
		return LikeActionResult{}, err
	}
	if err := s.postLikeRepo.Ensure(ctx, userID, postID); err != nil {
		return LikeActionResult{}, fmt.Errorf("ensure post like: %w", err)
	}
	summary, err := s.postSummary(ctx, userID, postID)
	if err != nil {
		return LikeActionResult{}, err
	}
	return LikeActionResult{TargetID: postID, Summary: summary}, nil
}

func (s *LikeService) UnlikePost(
	ctx context.Context,
	userID models.UUID,
	postID models.UUID,
) (LikeActionResult, error) {
	if err := s.ensurePost(ctx, postID); err != nil {
		return LikeActionResult{}, err
	}
	if err := s.postLikeRepo.Delete(ctx, userID, postID); err != nil {
		return LikeActionResult{}, fmt.Errorf("delete post like: %w", err)
	}
	summary, err := s.postSummary(ctx, userID, postID)
	if err != nil {
		return LikeActionResult{}, err
	}
	return LikeActionResult{TargetID: postID, Summary: summary}, nil
}

func (s *LikeService) LikeComment(
	ctx context.Context,
	userID models.UUID,
	commentID models.UUID,
) (LikeActionResult, error) {
	if err := s.ensureCommentTarget(ctx, commentID); err != nil {
		return LikeActionResult{}, err
	}
	if err := s.commentLikeRepo.Ensure(ctx, userID, commentID); err != nil {
		return LikeActionResult{}, fmt.Errorf("ensure comment like: %w", err)
	}
	summary, err := s.commentSummary(ctx, userID, commentID)
	if err != nil {
		return LikeActionResult{}, err
	}
	return LikeActionResult{TargetID: commentID, Summary: summary}, nil
}

func (s *LikeService) UnlikeComment(
	ctx context.Context,
	userID models.UUID,
	commentID models.UUID,
) (LikeActionResult, error) {
	if err := s.ensureCommentTarget(ctx, commentID); err != nil {
		return LikeActionResult{}, err
	}
	if err := s.commentLikeRepo.Delete(ctx, userID, commentID); err != nil {
		return LikeActionResult{}, fmt.Errorf("delete comment like: %w", err)
	}
	summary, err := s.commentSummary(ctx, userID, commentID)
	if err != nil {
		return LikeActionResult{}, err
	}
	return LikeActionResult{TargetID: commentID, Summary: summary}, nil
}

func (s *LikeService) PostSummaries(
	ctx context.Context,
	userID models.UUID,
	postIDs []models.UUID,
) (map[models.UUID]models.LikeSummary, error) {
	summaries := makeLikeSummaries(postIDs)
	if len(postIDs) == 0 {
		return summaries, nil
	}

	counts, err := s.postLikeRepo.CountByPostIDs(ctx, postIDs)
	if err != nil {
		return nil, fmt.Errorf("count post likes: %w", err)
	}
	likedIDs, err := s.postLikeRepo.FindLikedPostIDs(ctx, userID, postIDs)
	if err != nil {
		return nil, fmt.Errorf("find liked posts: %w", err)
	}
	for postID, count := range counts {
		summary := summaries[postID]
		summary.Count = count
		summaries[postID] = summary
	}
	for postID := range likedIDs {
		summary := summaries[postID]
		summary.LikedByMe = true
		summaries[postID] = summary
	}
	return summaries, nil
}

func (s *LikeService) CommentSummaries(
	ctx context.Context,
	userID models.UUID,
	commentIDs []models.UUID,
) (map[models.UUID]models.LikeSummary, error) {
	summaries := makeLikeSummaries(commentIDs)
	if len(commentIDs) == 0 {
		return summaries, nil
	}

	counts, err := s.commentLikeRepo.CountByCommentIDs(ctx, commentIDs)
	if err != nil {
		return nil, fmt.Errorf("count comment likes: %w", err)
	}
	likedIDs, err := s.commentLikeRepo.FindLikedCommentIDs(ctx, userID, commentIDs)
	if err != nil {
		return nil, fmt.Errorf("find liked comments: %w", err)
	}
	for commentID, count := range counts {
		summary := summaries[commentID]
		summary.Count = count
		summaries[commentID] = summary
	}
	for commentID := range likedIDs {
		summary := summaries[commentID]
		summary.LikedByMe = true
		summaries[commentID] = summary
	}
	return summaries, nil
}

func (s *LikeService) ensurePost(ctx context.Context, postID models.UUID) error {
	_, err := s.postRepo.GetByID(ctx, postID)
	if errors.Is(err, repositories.ErrPostNotFound) {
		return ErrLikeTargetNotFound
	}
	if err != nil {
		return fmt.Errorf("find post for like: %w", err)
	}
	return nil
}

func (s *LikeService) ensureCommentTarget(ctx context.Context, commentID models.UUID) error {
	comment, err := s.commentRepo.GetByID(ctx, commentID)
	if errors.Is(err, repositories.ErrCommentNotFound) {
		return ErrLikeTargetNotFound
	}
	if err != nil {
		return fmt.Errorf("find comment for like: %w", err)
	}
	if err := s.ensurePost(ctx, comment.PostID); err != nil {
		return err
	}
	return nil
}

func (s *LikeService) postSummary(
	ctx context.Context,
	userID models.UUID,
	postID models.UUID,
) (models.LikeSummary, error) {
	summaries, err := s.PostSummaries(ctx, userID, []models.UUID{postID})
	if err != nil {
		return models.LikeSummary{}, err
	}
	return summaries[postID], nil
}

func (s *LikeService) commentSummary(
	ctx context.Context,
	userID models.UUID,
	commentID models.UUID,
) (models.LikeSummary, error) {
	summaries, err := s.CommentSummaries(ctx, userID, []models.UUID{commentID})
	if err != nil {
		return models.LikeSummary{}, err
	}
	return summaries[commentID], nil
}

func makeLikeSummaries(ids []models.UUID) map[models.UUID]models.LikeSummary {
	summaries := make(map[models.UUID]models.LikeSummary, len(ids))
	for _, id := range ids {
		summaries[id] = models.LikeSummary{}
	}
	return summaries
}
