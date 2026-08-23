package usecase

import (
	"context"
	"errors"
	"testing"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
	"Threadly/internal/usecase/mocks"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newLikeUsecaseTest(t *testing.T) (
	*LikeUsecase,
	*mocks.MockPostRepository,
	*mocks.MockCommentRepository,
	*mocks.MockPostLikeRepository,
	*mocks.MockCommentLikeRepository,
) {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	postRepo := mocks.NewMockPostRepository(ctrl)
	commentRepo := mocks.NewMockCommentRepository(ctrl)
	postLikeRepo := mocks.NewMockPostLikeRepository(ctrl)
	commentLikeRepo := mocks.NewMockCommentLikeRepository(ctrl)
	usecase := NewLikeUsecase(postRepo, commentRepo, postLikeRepo, commentLikeRepo)
	return usecase, postRepo, commentRepo, postLikeRepo, commentLikeRepo
}

func TestLikeUsecase_PostSummariesBatchesCountAndCurrentUserState(t *testing.T) {
	usecase, _, _, postLikeRepo, _ := newLikeUsecaseTest(t)
	postIDs := []models.UUID{testPostID, testOtherPostID}
	postLikeRepo.EXPECT().
		CountByPostIDs(gomock.Any(), postIDs).
		Return(map[models.UUID]int64{testPostID: 2}, nil)
	postLikeRepo.EXPECT().
		FindLikedPostIDs(gomock.Any(), testUserID, postIDs).
		Return(map[models.UUID]struct{}{testPostID: {}}, nil)

	summaries, err := usecase.PostSummaries(context.Background(), testUserID, postIDs)

	require.NoError(t, err)
	require.Equal(t, models.LikeSummary{Count: 2, LikedByMe: true}, summaries[testPostID])
	require.Equal(t, models.LikeSummary{}, summaries[testOtherPostID])
}

func TestLikeUsecase_EmptySummariesDoNotCallRepository(t *testing.T) {
	usecase, _, _, _, _ := newLikeUsecaseTest(t)

	summaries, err := usecase.CommentSummaries(context.Background(), testUserID, nil)

	require.NoError(t, err)
	require.Empty(t, summaries)
}

func TestLikeUsecase_LikePostReturnsActionSummary(t *testing.T) {
	usecase, postRepo, _, postLikeRepo, _ := newLikeUsecaseTest(t)
	postRepo.EXPECT().
		GetByID(gomock.Any(), testPostID).
		Return(&models.Post{UUIDBaseModel: models.UUIDBaseModel{ID: testPostID}}, nil)
	postLikeRepo.EXPECT().Ensure(gomock.Any(), testUserID, testPostID).Return(nil)
	postLikeRepo.EXPECT().
		CountByPostIDs(gomock.Any(), []models.UUID{testPostID}).
		Return(map[models.UUID]int64{testPostID: 3}, nil)
	postLikeRepo.EXPECT().
		FindLikedPostIDs(gomock.Any(), testUserID, []models.UUID{testPostID}).
		Return(map[models.UUID]struct{}{testPostID: {}}, nil)

	result, err := usecase.LikePost(context.Background(), testUserID, testPostID)

	require.NoError(t, err)
	require.Equal(t, LikeActionResult{
		TargetID: testPostID,
		Summary:  models.LikeSummary{Count: 3, LikedByMe: true},
	}, result)
}

func TestLikeUsecase_CommentLikeRequiresActivePost(t *testing.T) {
	usecase, postRepo, commentRepo, _, _ := newLikeUsecaseTest(t)
	commentRepo.EXPECT().
		GetByID(gomock.Any(), testCommentID).
		Return(&models.Comment{
			UUIDBaseModel: models.UUIDBaseModel{ID: testCommentID},
			PostID:        testPostID,
		}, nil)
	postRepo.EXPECT().
		GetByID(gomock.Any(), testPostID).
		Return(nil, repositories.ErrPostNotFound)

	result, err := usecase.LikeComment(context.Background(), testUserID, testCommentID)

	require.ErrorIs(t, err, ErrLikeTargetNotFound)
	require.Empty(t, result)
}

func TestLikeUsecase_RepositoryErrorsRemainInternalErrors(t *testing.T) {
	usecase, postRepo, _, _, _ := newLikeUsecaseTest(t)
	databaseErr := errors.New("database unavailable")
	postRepo.EXPECT().GetByID(gomock.Any(), testPostID).Return(nil, databaseErr)

	_, err := usecase.LikePost(context.Background(), testUserID, testPostID)

	require.Error(t, err)
	require.ErrorIs(t, err, databaseErr)
	require.NotErrorIs(t, err, ErrLikeTargetNotFound)
}
