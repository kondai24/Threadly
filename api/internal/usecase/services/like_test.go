package services

import (
	"context"
	"errors"
	"testing"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
	"Threadly/internal/usecase/services/mocks"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newLikeServiceTest(t *testing.T) (
	*LikeService,
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
	service := NewLikeService(postRepo, commentRepo, postLikeRepo, commentLikeRepo)
	return service, postRepo, commentRepo, postLikeRepo, commentLikeRepo
}

func TestLikeService_PostSummariesBatchesCountAndCurrentUserState(t *testing.T) {
	service, _, _, postLikeRepo, _ := newLikeServiceTest(t)
	postIDs := []models.UUID{testPostID, testOtherPostID}
	postLikeRepo.EXPECT().
		CountByPostIDs(gomock.Any(), postIDs).
		Return(map[models.UUID]int64{testPostID: 2}, nil)
	postLikeRepo.EXPECT().
		FindLikedPostIDs(gomock.Any(), testUserID, postIDs).
		Return(map[models.UUID]struct{}{testPostID: {}}, nil)

	summaries, err := service.PostSummaries(context.Background(), testUserID, postIDs)

	require.NoError(t, err)
	require.Equal(t, models.LikeSummary{Count: 2, LikedByMe: true}, summaries[testPostID])
	require.Equal(t, models.LikeSummary{}, summaries[testOtherPostID])
}

func TestLikeService_EmptySummariesDoNotCallRepository(t *testing.T) {
	service, _, _, _, _ := newLikeServiceTest(t)

	summaries, err := service.CommentSummaries(context.Background(), testUserID, nil)

	require.NoError(t, err)
	require.Empty(t, summaries)
}

func TestLikeService_LikePostReturnsActionSummary(t *testing.T) {
	service, postRepo, _, postLikeRepo, _ := newLikeServiceTest(t)
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

	result, err := service.LikePost(context.Background(), testUserID, testPostID)

	require.NoError(t, err)
	require.Equal(t, LikeActionResult{
		TargetID: testPostID,
		Summary:  models.LikeSummary{Count: 3, LikedByMe: true},
	}, result)
}

func TestLikeService_CommentLikeRequiresActivePost(t *testing.T) {
	service, postRepo, commentRepo, _, _ := newLikeServiceTest(t)
	commentRepo.EXPECT().
		GetByID(gomock.Any(), testCommentID).
		Return(&models.Comment{
			UUIDBaseModel: models.UUIDBaseModel{ID: testCommentID},
			PostID:        testPostID,
		}, nil)
	postRepo.EXPECT().
		GetByID(gomock.Any(), testPostID).
		Return(nil, repositories.ErrPostNotFound)

	result, err := service.LikeComment(context.Background(), testUserID, testCommentID)

	require.ErrorIs(t, err, ErrLikeTargetNotFound)
	require.Empty(t, result)
}

func TestLikeService_RepositoryErrorsRemainInternalErrors(t *testing.T) {
	service, postRepo, _, _, _ := newLikeServiceTest(t)
	databaseErr := errors.New("database unavailable")
	postRepo.EXPECT().GetByID(gomock.Any(), testPostID).Return(nil, databaseErr)

	_, err := service.LikePost(context.Background(), testUserID, testPostID)

	require.Error(t, err)
	require.ErrorIs(t, err, databaseErr)
	require.NotErrorIs(t, err, ErrLikeTargetNotFound)
}
