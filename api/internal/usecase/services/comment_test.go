package services

import (
	"context"
	"testing"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
	"Threadly/internal/usecase/services/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

const (
	testOtherPostID models.UUID = "44444444-4444-4444-8444-444444444444"
	testCommentID   models.UUID = "55555555-5555-4555-8555-555555555555"
	testReplyID     models.UUID = "66666666-6666-4666-8666-666666666666"
)

func newCommentServiceTest(
	t *testing.T,
) (*CommentService, *mocks.MockCommentRepository, *mocks.MockPostRepository) {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	commentRepo := mocks.NewMockCommentRepository(ctrl)
	postRepo := mocks.NewMockPostRepository(ctrl)
	service := NewCommentService(commentRepo, postRepo)
	return service, commentRepo, postRepo
}

func TestCommentService_ListComments(t *testing.T) {
	service, commentRepo, postRepo := newCommentServiceTest(t)
	expectedComments := []*models.Comment{
		{UUIDBaseModel: models.UUIDBaseModel{ID: testCommentID}, PostID: testPostID},
	}
	postRepo.EXPECT().
		GetByID(gomock.Any(), testPostID).
		Return(&models.Post{UUIDBaseModel: models.UUIDBaseModel{ID: testPostID}}, nil)
	commentRepo.EXPECT().
		ListByPostID(gomock.Any(), testPostID).
		Return(expectedComments, nil)

	comments, err := service.ListComments(context.Background(), testPostID)

	require.NoError(t, err)
	assert.Equal(t, expectedComments, comments)
}

func TestCommentService_CreateComment(t *testing.T) {
	t.Run("Post直下Commentを作成する", func(t *testing.T) {
		service, commentRepo, postRepo := newCommentServiceTest(t)
		postRepo.EXPECT().
			GetByID(gomock.Any(), testPostID).
			Return(&models.Post{UUIDBaseModel: models.UUIDBaseModel{ID: testPostID}}, nil)
		commentRepo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, comment *models.Comment) error {
				assert.Equal(t, testPostID, comment.PostID)
				assert.Equal(t, testUserID, comment.AuthorID)
				assert.Equal(t, "comment", comment.Content)
				assert.Nil(t, comment.ParentID)
				return nil
			})

		err := service.CreateComment(
			context.Background(),
			testUserID,
			testPostID,
			"  comment  ",
			nil,
		)

		require.NoError(t, err)
	})

	t.Run("同じPostの親Commentへ返信する", func(t *testing.T) {
		service, commentRepo, postRepo := newCommentServiceTest(t)
		parentID := testCommentID
		postRepo.EXPECT().
			GetByID(gomock.Any(), testPostID).
			Return(&models.Post{UUIDBaseModel: models.UUIDBaseModel{ID: testPostID}}, nil)
		commentRepo.EXPECT().
			GetByID(gomock.Any(), parentID).
			Return(&models.Comment{
				UUIDBaseModel: models.UUIDBaseModel{ID: parentID},
				PostID:        testPostID,
			}, nil)
		commentRepo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, comment *models.Comment) error {
				if comment.ParentID == nil || *comment.ParentID != parentID {
					t.Fatalf("parent ID = %v, want %s", comment.ParentID, parentID)
				}
				return nil
			})

		err := service.CreateComment(
			context.Background(),
			testOtherUserID,
			testPostID,
			"reply",
			&parentID,
		)

		require.NoError(t, err)
	})

	t.Run("返信Commentへの返信を拒否する", func(t *testing.T) {
		service, commentRepo, postRepo := newCommentServiceTest(t)
		parentID := testReplyID
		postRepo.EXPECT().
			GetByID(gomock.Any(), testPostID).
			Return(&models.Post{UUIDBaseModel: models.UUIDBaseModel{ID: testPostID}}, nil)
		commentRepo.EXPECT().
			GetByID(gomock.Any(), parentID).
			Return(&models.Comment{
				UUIDBaseModel: models.UUIDBaseModel{ID: parentID},
				PostID:        testPostID,
				ParentID:      pointerToUUID(testCommentID),
			}, nil)

		err := service.CreateComment(
			context.Background(),
			testUserID,
			testPostID,
			"second reply",
			&parentID,
		)

		require.ErrorIs(t, err, ErrCommentReplyNotAllowed)
	})

	t.Run("別PostのCommentへの返信をNotFoundにする", func(t *testing.T) {
		service, commentRepo, postRepo := newCommentServiceTest(t)
		parentID := testCommentID
		postRepo.EXPECT().
			GetByID(gomock.Any(), testPostID).
			Return(&models.Post{UUIDBaseModel: models.UUIDBaseModel{ID: testPostID}}, nil)
		commentRepo.EXPECT().
			GetByID(gomock.Any(), parentID).
			Return(&models.Comment{
				UUIDBaseModel: models.UUIDBaseModel{ID: parentID},
				PostID:        testOtherPostID,
			}, nil)

		err := service.CreateComment(
			context.Background(),
			testUserID,
			testPostID,
			"reply",
			&parentID,
		)

		require.ErrorIs(t, err, ErrCommentNotFound)
	})

	t.Run("削除済みの親Commentへの返信をNotFoundにする", func(t *testing.T) {
		service, commentRepo, postRepo := newCommentServiceTest(t)
		parentID := testCommentID
		postRepo.EXPECT().
			GetByID(gomock.Any(), testPostID).
			Return(&models.Post{UUIDBaseModel: models.UUIDBaseModel{ID: testPostID}}, nil)
		commentRepo.EXPECT().
			GetByID(gomock.Any(), parentID).
			Return(nil, repositories.ErrCommentNotFound)

		err := service.CreateComment(
			context.Background(),
			testUserID,
			testPostID,
			"reply",
			&parentID,
		)

		require.ErrorIs(t, err, ErrCommentNotFound)
	})

	t.Run("削除済みPostへのCommentを拒否する", func(t *testing.T) {
		service, _, postRepo := newCommentServiceTest(t)
		postRepo.EXPECT().
			GetByID(gomock.Any(), testPostID).
			Return(nil, gorm.ErrRecordNotFound)

		err := service.CreateComment(
			context.Background(),
			testUserID,
			testPostID,
			"comment",
			nil,
		)

		require.ErrorIs(t, err, ErrPostNotFound)
	})
}

func TestCommentService_UpdateComment(t *testing.T) {
	t.Run("本人のCommentを更新する", func(t *testing.T) {
		service, commentRepo, _ := newCommentServiceTest(t)
		commentRepo.EXPECT().
			Update(gomock.Any(), testUserID, testCommentID, "updated").
			Return(int64(1), nil)

		err := service.UpdateComment(
			context.Background(),
			testUserID,
			testCommentID,
			"  updated  ",
		)

		require.NoError(t, err)
	})

	t.Run("他UserのCommentをNotFoundにする", func(t *testing.T) {
		service, commentRepo, _ := newCommentServiceTest(t)
		commentRepo.EXPECT().
			Update(gomock.Any(), testOtherUserID, testCommentID, "updated").
			Return(int64(0), nil)

		err := service.UpdateComment(
			context.Background(),
			testOtherUserID,
			testCommentID,
			"updated",
		)

		require.ErrorIs(t, err, ErrCommentNotFound)
	})
}

func TestCommentService_DeleteComment(t *testing.T) {
	t.Run("本人のCommentを削除する", func(t *testing.T) {
		service, commentRepo, _ := newCommentServiceTest(t)
		commentRepo.EXPECT().
			DeleteByID(gomock.Any(), testUserID, testCommentID).
			Return(int64(2), nil)

		err := service.DeleteComment(context.Background(), testUserID, testCommentID)

		require.NoError(t, err)
	})

	t.Run("他Userまたは削除済みのCommentをNotFoundにする", func(t *testing.T) {
		service, commentRepo, _ := newCommentServiceTest(t)
		commentRepo.EXPECT().
			DeleteByID(gomock.Any(), testOtherUserID, testCommentID).
			Return(int64(0), nil)

		err := service.DeleteComment(context.Background(), testOtherUserID, testCommentID)

		require.ErrorIs(t, err, ErrCommentNotFound)
	})
}

func pointerToUUID(value models.UUID) *models.UUID {
	return &value
}
