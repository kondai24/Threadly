package usecase

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

// LikeUsecaseは、LikeControllerが必要とする冪等なLike操作と、一覧表示用の集計を定義する。
type LikeUsecase interface {
	LikePost(
		ctx context.Context,
		userID models.UUID,
		postID models.UUID,
	) (LikeActionResult, error)
	UnlikePost(
		ctx context.Context,
		userID models.UUID,
		postID models.UUID,
	) (LikeActionResult, error)
	LikeComment(
		ctx context.Context,
		userID models.UUID,
		commentID models.UUID,
	) (LikeActionResult, error)
	UnlikeComment(
		ctx context.Context,
		userID models.UUID,
		commentID models.UUID,
	) (LikeActionResult, error)
	PostSummaries(
		ctx context.Context,
		userID models.UUID,
		postIDs []models.UUID,
	) (map[models.UUID]models.LikeSummary, error)
	CommentSummaries(
		ctx context.Context,
		userID models.UUID,
		commentIDs []models.UUID,
	) (map[models.UUID]models.LikeSummary, error)
}

// likeUsecaseは、Like対象の存在確認、冪等なLike操作、操作後の集計を組み立てるUsecaseである。
// Likeの内部行は公開せず、対象IDとLikeSummaryだけを上位層へ返す。
type likeUsecase struct {
	postRepo        repositories.PostRepository
	commentRepo     repositories.CommentRepository
	postLikeRepo    repositories.PostLikeRepository
	commentLikeRepo repositories.CommentLikeRepository
}

// NewLikeUsecaseは、Post/CommentとLikeテーブルの永続化契約を注入してUsecaseを構築する。
func NewLikeUsecase(
	postRepo repositories.PostRepository,
	commentRepo repositories.CommentRepository,
	postLikeRepo repositories.PostLikeRepository,
	commentLikeRepo repositories.CommentLikeRepository,
) LikeUsecase {
	return &likeUsecase{
		postRepo:        postRepo,
		commentRepo:     commentRepo,
		postLikeRepo:    postLikeRepo,
		commentLikeRepo: commentLikeRepo,
	}
}

// LikePostは、有効なPostへLikeを作成または維持し、最新の集計を返す。
func (u *likeUsecase) LikePost(
	ctx context.Context,
	userID models.UUID,
	postID models.UUID,
) (LikeActionResult, error) {
	if err := u.ensurePost(ctx, postID); err != nil {
		return LikeActionResult{}, err
	}
	if err := u.postLikeRepo.Ensure(ctx, userID, postID); err != nil {
		return LikeActionResult{}, fmt.Errorf("ensure post like: %w", err)
	}
	summary, err := u.postSummary(ctx, userID, postID)
	if err != nil {
		return LikeActionResult{}, err
	}
	return LikeActionResult{TargetID: postID, Summary: summary}, nil
}

// UnlikePostは、有効なPostから認証済みUserのLikeを削除し、最新の集計を返す。
func (u *likeUsecase) UnlikePost(
	ctx context.Context,
	userID models.UUID,
	postID models.UUID,
) (LikeActionResult, error) {
	if err := u.ensurePost(ctx, postID); err != nil {
		return LikeActionResult{}, err
	}
	if err := u.postLikeRepo.Delete(ctx, userID, postID); err != nil {
		return LikeActionResult{}, fmt.Errorf("delete post like: %w", err)
	}
	summary, err := u.postSummary(ctx, userID, postID)
	if err != nil {
		return LikeActionResult{}, err
	}
	return LikeActionResult{TargetID: postID, Summary: summary}, nil
}

// LikeCommentは、有効なCommentへLikeを作成または維持し、最新の集計を返す。
func (u *likeUsecase) LikeComment(
	ctx context.Context,
	userID models.UUID,
	commentID models.UUID,
) (LikeActionResult, error) {
	if err := u.ensureCommentTarget(ctx, commentID); err != nil {
		return LikeActionResult{}, err
	}
	if err := u.commentLikeRepo.Ensure(ctx, userID, commentID); err != nil {
		return LikeActionResult{}, fmt.Errorf("ensure comment like: %w", err)
	}
	summary, err := u.commentSummary(ctx, userID, commentID)
	if err != nil {
		return LikeActionResult{}, err
	}
	return LikeActionResult{TargetID: commentID, Summary: summary}, nil
}

// UnlikeCommentは、有効なCommentから認証済みUserのLikeを削除し、最新の集計を返す。
func (u *likeUsecase) UnlikeComment(
	ctx context.Context,
	userID models.UUID,
	commentID models.UUID,
) (LikeActionResult, error) {
	if err := u.ensureCommentTarget(ctx, commentID); err != nil {
		return LikeActionResult{}, err
	}
	if err := u.commentLikeRepo.Delete(ctx, userID, commentID); err != nil {
		return LikeActionResult{}, fmt.Errorf("delete comment like: %w", err)
	}
	summary, err := u.commentSummary(ctx, userID, commentID)
	if err != nil {
		return LikeActionResult{}, err
	}
	return LikeActionResult{TargetID: commentID, Summary: summary}, nil
}

// PostSummariesは、対象Post集合の件数とcurrent-userのLike状態を一括取得する。
func (u *likeUsecase) PostSummaries(
	ctx context.Context,
	userID models.UUID,
	postIDs []models.UUID,
) (map[models.UUID]models.LikeSummary, error) {
	summaries := makeLikeSummaries(postIDs)
	if len(postIDs) == 0 {
		return summaries, nil
	}

	counts, err := u.postLikeRepo.CountByPostIDs(ctx, postIDs)
	if err != nil {
		return nil, fmt.Errorf("count post likes: %w", err)
	}
	likedIDs, err := u.postLikeRepo.FindLikedPostIDs(ctx, userID, postIDs)
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

// CommentSummariesは、対象Comment集合の件数とcurrent-userのLike状態を一括取得する。
func (u *likeUsecase) CommentSummaries(
	ctx context.Context,
	userID models.UUID,
	commentIDs []models.UUID,
) (map[models.UUID]models.LikeSummary, error) {
	summaries := makeLikeSummaries(commentIDs)
	if len(commentIDs) == 0 {
		return summaries, nil
	}

	counts, err := u.commentLikeRepo.CountByCommentIDs(ctx, commentIDs)
	if err != nil {
		return nil, fmt.Errorf("count comment likes: %w", err)
	}
	likedIDs, err := u.commentLikeRepo.FindLikedCommentIDs(ctx, userID, commentIDs)
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

func (u *likeUsecase) ensurePost(ctx context.Context, postID models.UUID) error {
	_, err := u.postRepo.GetByID(ctx, postID)
	if errors.Is(err, repositories.ErrPostNotFound) {
		return ErrLikeTargetNotFound
	}
	if err != nil {
		return fmt.Errorf("find post for like: %w", err)
	}
	return nil
}

// ensureCommentTargetは、Commentだけでなく所属Postも有効であることを確認する。
// 親Postが削除済みなら、外部キーが残っていてもLike対象として公開しない。
func (u *likeUsecase) ensureCommentTarget(ctx context.Context, commentID models.UUID) error {
	comment, err := u.commentRepo.GetByID(ctx, commentID)
	if errors.Is(err, repositories.ErrCommentNotFound) {
		return ErrLikeTargetNotFound
	}
	if err != nil {
		return fmt.Errorf("find comment for like: %w", err)
	}
	if err := u.ensurePost(ctx, comment.PostID); err != nil {
		return err
	}
	return nil
}

func (u *likeUsecase) postSummary(
	ctx context.Context,
	userID models.UUID,
	postID models.UUID,
) (models.LikeSummary, error) {
	summaries, err := u.PostSummaries(ctx, userID, []models.UUID{postID})
	if err != nil {
		return models.LikeSummary{}, err
	}
	return summaries[postID], nil
}

func (u *likeUsecase) commentSummary(
	ctx context.Context,
	userID models.UUID,
	commentID models.UUID,
) (models.LikeSummary, error) {
	summaries, err := u.CommentSummaries(ctx, userID, []models.UUID{commentID})
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
