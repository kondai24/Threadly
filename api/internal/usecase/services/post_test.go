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

func newPostServiceTest(t *testing.T) (*PostService, *mocks.MockPostRepository) {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	repo := mocks.NewMockPostRepository(ctrl)
	svc := NewPostService(repo)
	return svc, repo
}

func TestGetPostById(t *testing.T) {
	t.Run("IDをもとに正しく取得される", func(t *testing.T) {
		svc, repo := newPostServiceTest(t)

		expectedPost := &models.Post{
			BaseModel: models.BaseModel{ID: 1},
			Title:     "hello",
			Content:   "world",
		}

		repo.EXPECT().
			GetById(gomock.Any(), uint(1)).
			Return(expectedPost, nil)

		post, err := svc.GetPostById(context.Background(), 1)
		require.NoError(t, err)
		require.NotNil(t, post)
		assert.Equal(t, expectedPost.ID, post.ID)
		assert.Equal(t, expectedPost.Title, post.Title)
		assert.Equal(t, expectedPost.Content, post.Content)
	})

	t.Run("Repositoryのエラーがそのまま返される", func(t *testing.T) {
		svc, repo := newPostServiceTest(t)
		expectedErr := errors.New("db error")

		repo.EXPECT().
			GetById(gomock.Any(), uint(999)).
			Return(nil, expectedErr)

		post, err := svc.GetPostById(context.Background(), 999)
		require.Error(t, err)
		assert.Nil(t, post)
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestListPosts(t *testing.T) {
	t.Run("全ての投稿が正しく取得される", func(t *testing.T) {
		svc, repo := newPostServiceTest(t)

		expectedPosts := []*models.Post{
			{BaseModel: models.BaseModel{ID: 1}, Title: "hello", Content: "world"},
			{BaseModel: models.BaseModel{ID: 2}, Title: "foo", Content: "bar"},
		}

		repo.EXPECT().
			List(gomock.Any()).
			Return(expectedPosts, nil)

		posts, err := svc.ListPosts(context.Background())
		require.NoError(t, err)
		require.Len(t, posts, 2)
		assert.Equal(t, expectedPosts, posts)
	})

	t.Run("Repositoryのエラーがそのまま返される", func(t *testing.T) {
		svc, repo := newPostServiceTest(t)
		expectedErr := errors.New("db error")

		repo.EXPECT().
			List(gomock.Any()).
			Return(nil, expectedErr)
		posts, err := svc.ListPosts(context.Background())
		require.Error(t, err)
		assert.Nil(t, posts)
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestCreatePost(t *testing.T) {
	t.Run("入力が正しい場合に，投稿が作成されること", func(t *testing.T) {
		svc, repo := newPostServiceTest(t)

		repo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, post *models.Post) error {
				assert.Equal(t, "hello", post.Title)
				assert.Equal(t, "world", post.Content)
				return nil
			})

		err := svc.CreatePost(context.Background(), "hello", "world")
		require.NoError(t, err)
	})

	t.Run("タイトルがない場合に，バリデーションエラーが返されること", func(t *testing.T) {
		svc, _ := newPostServiceTest(t)

		err := svc.CreatePost(context.Background(), "", "world")
		require.Error(t, err)
		assert.EqualError(t, err, "titleが入力されていません")
	})

	t.Run("コンテンツがない場合に，バリデーションエラーが返されること", func(t *testing.T) {
		svc, _ := newPostServiceTest(t)

		err := svc.CreatePost(context.Background(), "hello", "")
		require.Error(t, err)
		assert.EqualError(t, err, "contentが入力されていません")
	})
}

func TestUpdatePost(t *testing.T) {
	t.Run("入力が正しい場合に，投稿が更新されること", func(t *testing.T) {
		svc, repo := newPostServiceTest(t)

		post := models.Post{
			Title:   "test",
			Content: "this is a test",
		}
		post.ID = 1

		repo.EXPECT().
			Update(gomock.Any(), &post).
			Return(nil)

		err := svc.UpdatePost(context.Background(), &post)
		require.NoError(t, err)
	})

	t.Run("タイトルがない場合に，バリデーションエラーが返されること", func(t *testing.T) {
		svc, _ := newPostServiceTest(t)

		post := models.Post{
			Content: "this is a test",
		}
		post.ID = 1

		err := svc.UpdatePost(context.Background(), &post)
		require.Error(t, err)
		assert.EqualError(t, err, "titleが入力されていません")
	})

	t.Run("コンテンツがない場合に，バリデーションエラーが返されること", func(t *testing.T) {
		svc, _ := newPostServiceTest(t)

		post := models.Post{
			Title: "test",
		}
		post.ID = 1

		err := svc.UpdatePost(context.Background(), &post)
		require.Error(t, err)
		assert.EqualError(t, err, "contentが入力されていません")
	})
}

func TestDeletePost(t *testing.T) {
	t.Run("IDをもとに正しく削除される", func(t *testing.T) {
		svc, repo := newPostServiceTest(t)

		repo.EXPECT().
			DeleteById(gomock.Any(), uint(1)).
			Return(int64(1), nil)

		err := svc.DeletePost(context.Background(), 1)
		require.NoError(t, err)
	})

	t.Run("存在しないIDを削除しようとした場合に，ErrPostNotFoundが返される", func(t *testing.T) {
		svc, repo := newPostServiceTest(t)

		repo.EXPECT().
			DeleteById(gomock.Any(), uint(999)).
			Return(int64(0), nil)

		err := svc.DeletePost(context.Background(), 999)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrPostNotFound)
	})
}
