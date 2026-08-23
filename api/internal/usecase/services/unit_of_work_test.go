package services

import (
	"context"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
)

type testUnitOfWork struct {
	repos repositories.TransactionRepositories
}

func (u testUnitOfWork) WithinTransaction(
	_ context.Context,
	fn func(repositories.TransactionRepositories) error,
) error {
	return fn(u.repos)
}

type testPostLikeRepository struct {
	deletedPostIDs []models.UUID
}

func (r *testPostLikeRepository) Ensure(
	_ context.Context,
	_ models.UUID,
	_ models.UUID,
) error {
	return nil
}

func (r *testPostLikeRepository) Delete(
	_ context.Context,
	_ models.UUID,
	_ models.UUID,
) error {
	return nil
}

func (r *testPostLikeRepository) DeleteByPostID(
	_ context.Context,
	postID models.UUID,
) error {
	r.deletedPostIDs = append(r.deletedPostIDs, postID)
	return nil
}

func (r *testPostLikeRepository) CountByPostIDs(
	_ context.Context,
	_ []models.UUID,
) (map[models.UUID]int64, error) {
	return map[models.UUID]int64{}, nil
}

func (r *testPostLikeRepository) FindLikedPostIDs(
	_ context.Context,
	_ models.UUID,
	_ []models.UUID,
) (map[models.UUID]struct{}, error) {
	return map[models.UUID]struct{}{}, nil
}

type testCommentLikeRepository struct {
	deletedCommentIDs [][]models.UUID
}

func (r *testCommentLikeRepository) Ensure(
	_ context.Context,
	_ models.UUID,
	_ models.UUID,
) error {
	return nil
}

func (r *testCommentLikeRepository) Delete(
	_ context.Context,
	_ models.UUID,
	_ models.UUID,
) error {
	return nil
}

func (r *testCommentLikeRepository) DeleteByCommentIDs(
	_ context.Context,
	commentIDs []models.UUID,
) error {
	ids := append([]models.UUID(nil), commentIDs...)
	r.deletedCommentIDs = append(r.deletedCommentIDs, ids)
	return nil
}

func (r *testCommentLikeRepository) DeleteByCommentIDWithReplies(
	_ context.Context,
	commentID models.UUID,
) error {
	r.deletedCommentIDs = append(r.deletedCommentIDs, []models.UUID{commentID})
	return nil
}

func (r *testCommentLikeRepository) DeleteByCommentsOfPostID(
	_ context.Context,
	postID models.UUID,
) error {
	r.deletedCommentIDs = append(r.deletedCommentIDs, []models.UUID{postID})
	return nil
}

func (r *testCommentLikeRepository) CountByCommentIDs(
	_ context.Context,
	_ []models.UUID,
) (map[models.UUID]int64, error) {
	return map[models.UUID]int64{}, nil
}

func (r *testCommentLikeRepository) FindLikedCommentIDs(
	_ context.Context,
	_ models.UUID,
	_ []models.UUID,
) (map[models.UUID]struct{}, error) {
	return map[models.UUID]struct{}{}, nil
}
