package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
	"Threadly/internal/tracing/tracingtest"
	"Threadly/internal/usecase/mocks"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/mock/gomock"
)

func TestPostUsecase_ListAllPostsForUserNestsLikeSummarySpan(t *testing.T) {
	ctrl := gomock.NewController(t)
	tracer, recorder := tracingtest.NewRecorder()
	postRepo := mocks.NewMockPostRepository(ctrl)
	postLikeRepo := mocks.NewMockPostLikeRepository(ctrl)
	likes := NewLikeUsecase(postRepo, mocks.NewMockCommentRepository(ctrl), postLikeRepo, mocks.NewMockCommentLikeRepository(ctrl), tracer)
	usecase := NewPostUsecaseWithLikeReader(postRepo, testUnitOfWork{}, likes, tracer)
	postRepo.EXPECT().ListAll(gomock.Any()).Return([]*models.Post{{UUIDBaseModel: models.UUIDBaseModel{ID: testPostID}}}, nil)
	postLikeRepo.EXPECT().CountByPostIDs(gomock.Any(), []models.UUID{testPostID}).Return(map[models.UUID]int64{}, nil)
	postLikeRepo.EXPECT().FindLikedPostIDs(gomock.Any(), testUserID, []models.UUID{testPostID}).Return(map[models.UUID]struct{}{}, nil)

	_, err := usecase.ListAllPostsForUser(context.Background(), testUserID)

	require.NoError(t, err)
	spans := recorder.Ended()
	// 同じUsecaseの一覧取得は入口Spanを重ねず、別UsecaseのLike集計だけを子Spanにする。
	require.Equal(t, []string{
		"LikeUsecase.PostSummaries success",
		"PostUsecase.ListAllPostsForUser success",
	}, tracingtest.Results(spans))
	require.Equal(t, spans[1].SpanContext().SpanID(), spans[0].Parent().SpanID())
	require.Equal(t, "usecase", tracingtest.Attribute(spans[1], "threadly.layer"))
}

// usecaseSpanFixtureは、全Usecaseを同じ記録用Tracerで組み立て、入口Spanを比較する。
type usecaseSpanFixture struct {
	users        *mocks.MockUserRepository
	posts        *mocks.MockPostRepository
	comments     *mocks.MockCommentRepository
	postLikes    *mocks.MockPostLikeRepository
	commentLikes *mocks.MockCommentLikeRepository
	tracer       trace.Tracer
	auth         *AuthUsecase
	post         *PostUsecase
	comment      *CommentUsecase
	like         *LikeUsecase
}

func newUsecaseSpanFixture(t *testing.T) (*usecaseSpanFixture, *tracetest.SpanRecorder) {
	t.Helper()
	ctrl := gomock.NewController(t)
	tracer, recorder := tracingtest.NewRecorder()
	f := &usecaseSpanFixture{
		users:        mocks.NewMockUserRepository(ctrl),
		posts:        mocks.NewMockPostRepository(ctrl),
		comments:     mocks.NewMockCommentRepository(ctrl),
		postLikes:    mocks.NewMockPostLikeRepository(ctrl),
		commentLikes: mocks.NewMockCommentLikeRepository(ctrl),
		tracer:       tracer,
	}
	uow := testUnitOfWork{repos: repositories.TransactionRepositories{
		Post:        f.posts,
		Comment:     f.comments,
		PostLike:    f.postLikes,
		CommentLike: f.commentLikes,
	}}
	f.auth = NewAuthUsecase(f.users, fakePasswordHasher{hash: "hash"}, fakeTokenIssuer{token: "token"}, tracer)
	f.like = NewLikeUsecase(f.posts, f.comments, f.postLikes, f.commentLikes, tracer)
	f.post = NewPostUsecase(f.posts, uow, tracer)
	f.comment = NewCommentUsecase(f.comments, f.posts, uow, tracer)
	return f, recorder
}

func TestUsecases_RecordOneEntrySpanPerPublicCall(t *testing.T) {
	post := &models.Post{UUIDBaseModel: models.UUIDBaseModel{ID: testPostID}, AuthorID: testUserID}
	comment := &models.Comment{UUIDBaseModel: models.UUIDBaseModel{ID: testCommentID}, PostID: testPostID, AuthorID: testOtherUserID}
	databaseErr := errors.New("database unavailable")
	cases := []struct {
		name string
		span string
		call func(context.Context, *usecaseSpanFixture) error
		want string
	}{
		{name: "登録の入力不正はinvalid_input", span: "AuthUsecase.Register", want: "invalid_input", call: func(ctx context.Context, f *usecaseSpanFixture) error {
			_, _, err := f.auth.Register(ctx, "", "")
			return err
		}},
		{name: "存在しないusernameのLoginはunauthenticated", span: "AuthUsecase.Login", want: "unauthenticated", call: func(ctx context.Context, f *usecaseSpanFixture) error {
			f.users.EXPECT().FindByUsername(gomock.Any(), "alice").Return(nil, repositories.ErrUserNotFound)
			_, _, err := f.auth.Login(ctx, "alice", "password")
			return err
		}},
		{name: "現在のUser取得はsuccess", span: "AuthUsecase.GetMe", want: "success", call: func(ctx context.Context, f *usecaseSpanFixture) error {
			f.users.EXPECT().FindByID(gomock.Any(), testUserID).Return(&models.User{}, nil)
			_, err := f.auth.GetMe(ctx, testUserID)
			return err
		}},
		{name: "存在しないPostの取得はnot_found", span: "PostUsecase.GetPostByID", want: "not_found", call: func(ctx context.Context, f *usecaseSpanFixture) error {
			f.posts.EXPECT().GetByID(gomock.Any(), testMissingID).Return(nil, repositories.ErrPostNotFound)
			_, err := f.post.GetPostByID(ctx, testMissingID)
			return err
		}},
		{name: "User向けPost取得は内部の閲覧取得Spanを重ねない", span: "PostUsecase.GetPostByIDForUser", want: "success", call: func(ctx context.Context, f *usecaseSpanFixture) error {
			f.posts.EXPECT().GetByID(gomock.Any(), testPostID).Return(post, nil)
			_, err := f.post.GetPostByIDForUser(ctx, testUserID, testPostID)
			return err
		}},
		{name: "所有者向けPost取得はsuccess", span: "PostUsecase.GetPostByIDForOwner", want: "success", call: func(ctx context.Context, f *usecaseSpanFixture) error {
			f.posts.EXPECT().GetByIDForOwner(gomock.Any(), testUserID, testPostID).Return(post, nil)
			_, err := f.post.GetPostByIDForOwner(ctx, testUserID, testPostID)
			return err
		}},
		{name: "Post一覧はsuccess", span: "PostUsecase.ListAllPosts", want: "success", call: func(ctx context.Context, f *usecaseSpanFixture) error {
			f.posts.EXPECT().ListAll(gomock.Any()).Return(nil, nil)
			_, err := f.post.ListAllPosts(ctx)
			return err
		}},
		{name: "User向けPost一覧は内部の一覧取得Spanを重ねない", span: "PostUsecase.ListAllPostsForUser", want: "success", call: func(ctx context.Context, f *usecaseSpanFixture) error {
			f.posts.EXPECT().ListAll(gomock.Any()).Return(nil, nil)
			_, err := f.post.ListAllPostsForUser(ctx, testUserID)
			return err
		}},
		{name: "Post作成の入力不正はinvalid_input", span: "PostUsecase.CreatePost", want: "invalid_input", call: func(ctx context.Context, f *usecaseSpanFixture) error {
			return f.post.CreatePost(ctx, testUserID, "", "content")
		}},
		{name: "非所有者のPost更新はnot_found", span: "PostUsecase.UpdatePost", want: "not_found", call: func(ctx context.Context, f *usecaseSpanFixture) error {
			return f.post.UpdatePost(ctx, testOtherUserID, post)
		}},
		{name: "Post削除のDB障害はinternal_error", span: "PostUsecase.DeletePost", want: "internal_error", call: func(ctx context.Context, f *usecaseSpanFixture) error {
			f.posts.EXPECT().GetByIDForUpdate(gomock.Any(), testPostID).Return(nil, databaseErr)
			return f.post.DeletePost(ctx, testUserID, testPostID)
		}},
		{name: "存在しないPostのComment一覧はnot_found", span: "CommentUsecase.ListComments", want: "not_found", call: func(ctx context.Context, f *usecaseSpanFixture) error {
			f.posts.EXPECT().GetByID(gomock.Any(), testMissingID).Return(nil, repositories.ErrPostNotFound)
			_, err := f.comment.ListComments(ctx, testMissingID)
			return err
		}},
		{name: "User向けComment一覧は内部の一覧取得Spanを重ねない", span: "CommentUsecase.ListCommentsForUser", want: "success", call: func(ctx context.Context, f *usecaseSpanFixture) error {
			f.posts.EXPECT().GetByID(gomock.Any(), testPostID).Return(post, nil)
			f.comments.EXPECT().ListByPostID(gomock.Any(), testPostID).Return(nil, nil)
			_, err := f.comment.ListCommentsForUser(ctx, testUserID, testPostID)
			return err
		}},
		{name: "Comment作成の入力不正はinvalid_input", span: "CommentUsecase.CreateComment", want: "invalid_input", call: func(ctx context.Context, f *usecaseSpanFixture) error {
			return f.comment.CreateComment(ctx, testUserID, testPostID, " ", nil)
		}},
		{name: "Comment更新はsuccess", span: "CommentUsecase.UpdateComment", want: "success", call: func(ctx context.Context, f *usecaseSpanFixture) error {
			f.comments.EXPECT().Update(gomock.Any(), testUserID, testCommentID, "updated").Return(int64(1), nil)
			return f.comment.UpdateComment(ctx, testUserID, testCommentID, "updated")
		}},
		{name: "非所有者のComment削除はnot_found", span: "CommentUsecase.DeleteComment", want: "not_found", call: func(ctx context.Context, f *usecaseSpanFixture) error {
			f.comments.EXPECT().GetByIDForUpdate(gomock.Any(), testCommentID).Return(comment, nil)
			return f.comment.DeleteComment(ctx, testUserID, testCommentID)
		}},
		{name: "Post Likeは操作後の集計Spanを重ねない", span: "LikeUsecase.LikePost", want: "success", call: func(ctx context.Context, f *usecaseSpanFixture) error {
			f.posts.EXPECT().GetByID(gomock.Any(), testPostID).Return(post, nil)
			f.postLikes.EXPECT().Ensure(gomock.Any(), testUserID, testPostID).Return(nil)
			f.postLikes.EXPECT().CountByPostIDs(gomock.Any(), gomock.Any()).Return(nil, nil)
			f.postLikes.EXPECT().FindLikedPostIDs(gomock.Any(), testUserID, gomock.Any()).Return(nil, nil)
			_, err := f.like.LikePost(ctx, testUserID, testPostID)
			return err
		}},
		{name: "存在しないPostのLike解除はnot_found", span: "LikeUsecase.UnlikePost", want: "not_found", call: func(ctx context.Context, f *usecaseSpanFixture) error {
			f.posts.EXPECT().GetByID(gomock.Any(), testMissingID).Return(nil, repositories.ErrPostNotFound)
			_, err := f.like.UnlikePost(ctx, testUserID, testMissingID)
			return err
		}},
		{name: "存在しないCommentのLikeはnot_found", span: "LikeUsecase.LikeComment", want: "not_found", call: func(ctx context.Context, f *usecaseSpanFixture) error {
			f.comments.EXPECT().GetByID(gomock.Any(), testMissingID).Return(nil, repositories.ErrCommentNotFound)
			_, err := f.like.LikeComment(ctx, testUserID, testMissingID)
			return err
		}},
		{name: "Comment Like解除は操作後の集計Spanを重ねない", span: "LikeUsecase.UnlikeComment", want: "success", call: func(ctx context.Context, f *usecaseSpanFixture) error {
			f.comments.EXPECT().GetByID(gomock.Any(), testCommentID).Return(comment, nil)
			f.posts.EXPECT().GetByID(gomock.Any(), testPostID).Return(post, nil)
			f.commentLikes.EXPECT().Delete(gomock.Any(), testUserID, testCommentID).Return(nil)
			f.commentLikes.EXPECT().CountByCommentIDs(gomock.Any(), gomock.Any()).Return(nil, nil)
			f.commentLikes.EXPECT().FindLikedCommentIDs(gomock.Any(), testUserID, gomock.Any()).Return(nil, nil)
			_, err := f.like.UnlikeComment(ctx, testUserID, testCommentID)
			return err
		}},
		{name: "空集合のPost集計も入口Spanを残す", span: "LikeUsecase.PostSummaries", want: "success", call: func(ctx context.Context, f *usecaseSpanFixture) error {
			_, err := f.like.PostSummaries(ctx, testUserID, nil)
			return err
		}},
		{name: "空集合のComment集計も入口Spanを残す", span: "LikeUsecase.CommentSummaries", want: "success", call: func(ctx context.Context, f *usecaseSpanFixture) error {
			_, err := f.like.CommentSummaries(ctx, testUserID, nil)
			return err
		}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			f, recorder := newUsecaseSpanFixture(t)

			_ = tt.call(context.Background(), f)

			spans := recorder.Ended()
			require.Equal(t, []string{tt.span + " " + tt.want}, tracingtest.Results(spans))
			require.Equal(t, "usecase", tracingtest.Attribute(spans[0], "threadly.layer"))
			wantStatus := codes.Unset
			if tt.want == "internal_error" {
				wantStatus = codes.Error
			}
			require.Equal(t, wantStatus, spans[0].Status().Code)
		})
	}
}

type failingHasher struct{ err error }

func (h failingHasher) Hash(string) (string, error)  { return "", h.err }
func (h failingHasher) Compare(string, string) error { return h.err }

type failingUnitOfWork struct{ err error }

func (u failingUnitOfWork) WithinTransaction(
	context.Context,
	func(repositories.TransactionRepositories) error,
) error {
	return u.err
}

func TestUsecases_ClassifyExpectedResultsAndUnexpectedFailures(t *testing.T) {
	databaseErr := errors.New("dummy-secret database failure")
	cases := []struct {
		name       string
		call       func(context.Context, *usecaseSpanFixture, trace.Tracer) error
		wantResult string
		wantStatus codes.Code
	}{
		{
			name:       "重複usernameはconflict",
			wantResult: "conflict",
			wantStatus: codes.Unset,
			call: func(ctx context.Context, f *usecaseSpanFixture, _ trace.Tracer) error {
				f.users.EXPECT().Create(gomock.Any(), gomock.Any()).Return(fmt.Errorf("wrapped: %w", repositories.ErrUsernameAlreadyExists))
				_, _, err := f.auth.Register(ctx, "alice", "password")
				return err
			},
		},
		{
			name:       "password不一致はunauthenticated",
			wantResult: "unauthenticated",
			wantStatus: codes.Unset,
			call: func(ctx context.Context, f *usecaseSpanFixture, tracer trace.Tracer) error {
				auth := NewAuthUsecase(f.users, fakePasswordHasher{compareErr: ErrPasswordMismatch}, fakeTokenIssuer{}, tracer)
				f.users.EXPECT().FindByUsername(gomock.Any(), "alice").Return(&models.User{PasswordHash: "hash"}, nil)
				_, _, err := auth.Login(ctx, "alice", "password")
				return err
			},
		},
		{
			name:       "不正hashなどのHasher障害はinternal_error",
			wantResult: "internal_error",
			wantStatus: codes.Error,
			call: func(ctx context.Context, f *usecaseSpanFixture, tracer trace.Tracer) error {
				auth := NewAuthUsecase(f.users, failingHasher{err: databaseErr}, fakeTokenIssuer{}, tracer)
				f.users.EXPECT().FindByUsername(gomock.Any(), "alice").Return(&models.User{PasswordHash: "broken"}, nil)
				_, _, err := auth.Login(ctx, "alice", "password")
				return err
			},
		},
		{
			name:       "context cancellationはinternal_error",
			wantResult: "internal_error",
			wantStatus: codes.Error,
			call: func(ctx context.Context, f *usecaseSpanFixture, _ trace.Tracer) error {
				f.posts.EXPECT().GetByID(gomock.Any(), testPostID).Return(nil, context.Canceled)
				_, err := f.post.GetPostByID(ctx, testPostID)
				return err
			},
		},
		{
			name:       "Transaction開始・commit失敗はinternal_error",
			wantResult: "internal_error",
			wantStatus: codes.Error,
			call: func(ctx context.Context, f *usecaseSpanFixture, tracer trace.Tracer) error {
				post := NewPostUsecase(f.posts, failingUnitOfWork{err: databaseErr}, tracer)
				return post.DeletePost(ctx, testUserID, testPostID)
			},
		},
		{
			name:       "wrapされた返信不可エラーはinvalid_input",
			wantResult: "invalid_input",
			wantStatus: codes.Unset,
			call: func(ctx context.Context, f *usecaseSpanFixture, tracer trace.Tracer) error {
				comment := NewCommentUsecase(f.comments, f.posts, failingUnitOfWork{err: fmt.Errorf("tx: %w", ErrCommentReplyNotAllowed)}, tracer)
				return comment.CreateComment(ctx, testUserID, testPostID, "reply", nil)
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			f, recorder := newUsecaseSpanFixture(t)

			err := tt.call(context.Background(), f, f.tracer)

			require.Error(t, err)
			spans := recorder.Ended()
			require.Len(t, spans, 1)
			require.Equal(t, tt.wantResult, tracingtest.Attribute(spans[0], "threadly.result"))
			require.Equal(t, tt.wantStatus, spans[0].Status().Code)
			for _, event := range spans[0].Events() {
				for _, attr := range event.Attributes {
					require.NotContains(t, attr.Value.String(), "dummy-secret")
				}
			}
		})
	}
}

func TestUsecases_PassSpanContextAndRequestContextToRepository(t *testing.T) {
	f, recorder := newUsecaseSpanFixture(t)
	type userKey struct{}
	ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), userKey{}, testUserID), time.Minute)
	defer cancel()
	wantDeadline, _ := ctx.Deadline()
	var repositoryCtx context.Context
	f.users.EXPECT().FindByID(gomock.Any(), testUserID).DoAndReturn(
		func(ctx context.Context, _ models.UUID) (*models.User, error) {
			repositoryCtx = ctx
			return &models.User{}, nil
		},
	)

	_, err := f.auth.GetMe(ctx, testUserID)

	require.NoError(t, err)
	spans := recorder.Ended()
	require.Len(t, spans, 1)
	require.Equal(t, spans[0].SpanContext().SpanID(), trace.SpanContextFromContext(repositoryCtx).SpanID())
	require.Equal(t, testUserID, repositoryCtx.Value(userKey{}))
	gotDeadline, ok := repositoryCtx.Deadline()
	require.True(t, ok)
	require.Equal(t, wantDeadline, gotDeadline)
}

func TestUsecases_PanicEndsSpanAndRepanics(t *testing.T) {
	const secret = "dummy-secret panic value"
	f, recorder := newUsecaseSpanFixture(t)
	f.posts.EXPECT().ListAll(gomock.Any()).DoAndReturn(func(context.Context) ([]*models.Post, error) {
		panic(secret)
	})

	require.PanicsWithValue(t, secret, func() {
		_, _ = f.post.ListAllPosts(context.Background())
	})

	spans := recorder.Ended()
	require.Equal(t, []string{"PostUsecase.ListAllPosts panic"}, tracingtest.Results(spans))
	require.Equal(t, codes.Error, spans[0].Status().Code)
	for _, event := range spans[0].Events() {
		for _, attr := range event.Attributes {
			require.NotContains(t, attr.Value.String(), secret)
		}
	}
}
