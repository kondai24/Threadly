package services

import (
	"context"
	"errors"
	"testing"

	"Threadly/internal/domain/models"
	"Threadly/internal/usecase/services/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const testUserID uint = 7

func newPostServiceTest(t *testing.T) (*PostService, *mocks.MockPostRepository) {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	repo := mocks.NewMockPostRepository(ctrl)
	service := NewPostService(repo)
	return service, repo
}

func TestPostService_GetPostByID(t *testing.T) {
	t.Run("所有者のPostを取得できる", func(t *testing.T) {
		service, repo := newPostServiceTest(t)
		expectedPost := &models.Post{
			BaseModel: models.BaseModel{ID: 1},
			AuthorID:  testUserID,
			Title:     "hello",
			Content:   "world",
		}
		repo.EXPECT().
			GetByID(gomock.Any(), uint(1)).
			Return(expectedPost, nil)

		post, err := service.GetPostByID(context.Background(), 1)

		require.NoError(t, err)
		require.NotNil(t, post)
		assert.Equal(t, expectedPost, post)
	})

	t.Run("Repositoryのエラーをそのまま返す", func(t *testing.T) {
		service, repo := newPostServiceTest(t)
		expectedErr := errors.New("db error")
		repo.EXPECT().
			GetByID(gomock.Any(), uint(999)).
			Return(nil, expectedErr)

		post, err := service.GetPostByID(context.Background(), 999)

		require.Error(t, err)
		assert.Nil(t, post)
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestPostService_ListAllPosts(t *testing.T) {
	t.Run("全Postを返す", func(t *testing.T) {
		service, repo := newPostServiceTest(t)
		expectedPosts := []*models.Post{
			{BaseModel: models.BaseModel{ID: 1}, AuthorID: testUserID, Title: "hello", Content: "world"},
			{BaseModel: models.BaseModel{ID: 2}, AuthorID: testUserID + 1, Title: "foo", Content: "bar"},
		}
		repo.EXPECT().
			ListAll(gomock.Any()).
			Return(expectedPosts, nil)

		posts, err := service.ListAllPosts(context.Background())

		require.NoError(t, err)
		assert.Equal(t, expectedPosts, posts)
	})

	t.Run("Repositoryのエラーをそのまま返す", func(t *testing.T) {
		service, repo := newPostServiceTest(t)
		expectedErr := errors.New("db error")
		repo.EXPECT().
			ListAll(gomock.Any()).
			Return(nil, expectedErr)

		posts, err := service.ListAllPosts(context.Background())

		require.Error(t, err)
		assert.Nil(t, posts)
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestPostService_CreatePost(t *testing.T) {
	t.Run("認証済みUserをauthorに設定する", func(t *testing.T) {
		service, repo := newPostServiceTest(t)
		repo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, post *models.Post) error {
				assert.Equal(t, testUserID, post.AuthorID)
				assert.Equal(t, "hello", post.Title)
				assert.Equal(t, "world", post.Content)
				return nil
			})

		err := service.CreatePost(context.Background(), testUserID, "hello", "world")

		require.NoError(t, err)
	})

	t.Run("空のtitleを拒否する", func(t *testing.T) {
		service, _ := newPostServiceTest(t)

		err := service.CreatePost(context.Background(), testUserID, "", "world")

		require.Error(t, err)
		assert.ErrorIs(t, err, models.ErrInvalidTitle)
	})

	t.Run("空のcontentを拒否する", func(t *testing.T) {
		service, _ := newPostServiceTest(t)

		err := service.CreatePost(context.Background(), testUserID, "hello", "")

		require.Error(t, err)
		assert.ErrorIs(t, err, models.ErrInvalidContent)
	})
}

func TestPostService_UpdatePost(t *testing.T) {
	t.Run("所有者のPostを更新できる", func(t *testing.T) {
		service, repo := newPostServiceTest(t)
		post := &models.Post{
			BaseModel: models.BaseModel{ID: 1},
			AuthorID:  testUserID,
			Title:     "test",
			Content:   "this is a test",
		}
		repo.EXPECT().Update(gomock.Any(), testUserID, post).Return(nil)

		err := service.UpdatePost(context.Background(), testUserID, post)

		require.NoError(t, err)
	})

	t.Run("他Userが所有するPostを拒否する", func(t *testing.T) {
		service, _ := newPostServiceTest(t)
		post := &models.Post{
			BaseModel: models.BaseModel{ID: 1},
			AuthorID:  testUserID + 1,
			Title:     "test",
			Content:   "this is a test",
		}

		err := service.UpdatePost(context.Background(), testUserID, post)

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrPostNotFound)
	})

	t.Run("空のtitleを拒否する", func(t *testing.T) {
		service, _ := newPostServiceTest(t)
		post := &models.Post{AuthorID: testUserID, Content: "content"}

		err := service.UpdatePost(context.Background(), testUserID, post)

		require.Error(t, err)
		assert.ErrorIs(t, err, models.ErrInvalidTitle)
	})

	t.Run("空のcontentを拒否する", func(t *testing.T) {
		service, _ := newPostServiceTest(t)
		post := &models.Post{AuthorID: testUserID, Title: "title"}

		err := service.UpdatePost(context.Background(), testUserID, post)

		require.Error(t, err)
		assert.ErrorIs(t, err, models.ErrInvalidContent)
	})
}

func TestPostService_DeletePost(t *testing.T) {
	t.Run("所有者のPostを削除できる", func(t *testing.T) {
		service, repo := newPostServiceTest(t)
		repo.EXPECT().
			DeleteByID(gomock.Any(), testUserID, uint(1)).
			Return(int64(1), nil)

		err := service.DeletePost(context.Background(), testUserID, 1)

		require.NoError(t, err)
	})

	t.Run("所有者のPostを削除できない場合はNotFoundを返す", func(t *testing.T) {
		service, repo := newPostServiceTest(t)
		repo.EXPECT().
			DeleteByID(gomock.Any(), testUserID, uint(999)).
			Return(int64(0), nil)

		err := service.DeletePost(context.Background(), testUserID, 999)

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrPostNotFound)
	})
}
