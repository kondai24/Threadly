package routes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
	"Threadly/internal/middleware"
	"Threadly/internal/tracing/tracingtest"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/mock/gomock"
)

const (
	routeCommentID models.UUID = "55555555-5555-4555-8555-555555555555"
	// routeSecretは、Spanへ漏れてはならない入力値の代わりに使うダミー値。
	routeSecret = "dummy-secret-value"
)

func newRecordingRouter(t *testing.T) (*gin.Engine, routeRepositories, *tracetest.SpanRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tracer, recorder := tracingtest.NewRecorder()
	router, repos := newRouteRouterWithTracer(t, true, tracer)
	return router, repos, recorder
}

func requireNoSecretInSpans(t *testing.T, spans []sdktrace.ReadOnlySpan) {
	t.Helper()
	for _, span := range spans {
		require.NotContains(t, span.Name(), routeSecret)
		require.NotContains(t, span.Status().Description, routeSecret)
		for _, attr := range span.Attributes() {
			require.NotContains(t, attr.Value.Emit(), routeSecret)
		}
		for _, event := range span.Events() {
			for _, attr := range event.Attributes {
				require.NotContains(t, attr.Value.Emit(), routeSecret)
			}
		}
	}
}

func TestHandlers_RecordHandlerSpanAboveUsecaseSpans(t *testing.T) {
	internalErr := errors.New(routeSecret)
	postPath := "/api/posts/" + string(routePostID)
	commentPath := "/api/comments/" + string(routeCommentID)
	cases := []struct {
		name   string
		method string
		path   string
		body   string
		setup  func(routeRepositories)
		want   []string
	}{
		{
			name: "登録はHandler配下にUsecase Spanを残す", method: http.MethodPost, path: "/api/auth/register",
			body: `{"username":"alice","password":"` + routeSecret + `"}`,
			want: []string{"AuthUsecase.Register success", "AuthController.RegisterHandler success"},
		},
		{
			name: "認証失敗のLoginは両層でunauthenticated", method: http.MethodPost, path: "/api/auth/login",
			body: `{"username":"alice","password":"` + routeSecret + `"}`,
			want: []string{"AuthUsecase.Login unauthenticated", "AuthController.LoginHandler unauthenticated"},
		},
		{
			name: "LogoutはUsecaseなしでHandler Spanだけを残す", method: http.MethodPost, path: "/api/auth/logout",
			want: []string{"AuthController.LogoutHandler success"},
		},
		{
			name: "存在しない現在UserはNotFound", method: http.MethodGet, path: "/api/me",
			want: []string{"AuthUsecase.GetMe not_found", "AuthController.MeHandler not_found"},
		},
		{
			name: "Post一覧はLike集計を含む階層を残す", method: http.MethodGet, path: "/api/posts",
			setup: func(repos routeRepositories) {
				repos.Post.EXPECT().ListAll(gomock.Any()).Return(nil, nil)
			},
			want: []string{
				"LikeUsecase.PostSummaries success",
				"PostUsecase.ListAllPostsForUser success",
				"PostController.ListPostsHandler success",
			},
		},
		{
			name: "不正なPost IDはHandlerだけでinvalid_input", method: http.MethodGet, path: "/api/posts/not-a-uuid",
			want: []string{"PostController.GetPostByIDHandler invalid_input"},
		},
		{
			name: "Post作成はsuccess", method: http.MethodPost, path: "/api/posts",
			body: `{"title":"title","content":"` + routeSecret + `"}`,
			setup: func(repos routeRepositories) {
				repos.Post.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			},
			want: []string{"PostUsecase.CreatePost success", "PostController.CreatePostHandler success"},
		},
		{
			name: "非所有者のPost更新はNotFound", method: http.MethodPut, path: postPath, body: `{"title":"title"}`,
			setup: func(repos routeRepositories) {
				repos.Post.EXPECT().GetByIDForOwner(gomock.Any(), routeUserID, routePostID).Return(nil, repositories.ErrPostNotFound)
			},
			want: []string{"PostUsecase.GetPostByIDForOwner not_found", "PostController.UpdatePostHandler not_found"},
		},
		{
			name: "Post削除のDB障害はinternal_error", method: http.MethodDelete, path: postPath,
			setup: func(repos routeRepositories) {
				repos.Post.EXPECT().GetByIDForUpdate(gomock.Any(), routePostID).Return(nil, internalErr)
			},
			want: []string{"PostUsecase.DeletePost internal_error", "PostController.DeletePostHandler internal_error"},
		},
		{
			name: "存在しないPostのComment一覧はNotFound", method: http.MethodGet, path: postPath + "/comments",
			setup: func(repos routeRepositories) {
				repos.Post.EXPECT().GetByID(gomock.Any(), routePostID).Return(nil, repositories.ErrPostNotFound)
			},
			want: []string{"CommentUsecase.ListCommentsForUser not_found", "CommentController.ListCommentsHandler not_found"},
		},
		{
			name: "不正なJSONのComment作成はHandlerだけでinvalid_input", method: http.MethodPost, path: postPath + "/comments",
			body: `{"content":`,
			want: []string{"CommentController.CreateCommentHandler invalid_input"},
		},
		{
			name: "Comment更新はsuccess", method: http.MethodPut, path: commentPath,
			body: `{"content":"updated"}`,
			setup: func(repos routeRepositories) {
				repos.Comment.EXPECT().Update(gomock.Any(), routeUserID, routeCommentID, "updated").Return(int64(1), nil)
			},
			want: []string{"CommentUsecase.UpdateComment success", "CommentController.UpdateCommentHandler success"},
		},
		{
			name: "不正なComment IDの削除はHandlerだけでinvalid_input", method: http.MethodDelete, path: "/api/comments/not-a-uuid",
			want: []string{"CommentController.DeleteCommentHandler invalid_input"},
		},
		{
			name: "存在しないPostのLikeはNotFound", method: http.MethodPut, path: postPath + "/like",
			setup: func(repos routeRepositories) {
				repos.Post.EXPECT().GetByID(gomock.Any(), routePostID).Return(nil, repositories.ErrPostNotFound)
			},
			want: []string{"LikeUsecase.LikePost not_found", "LikeController.LikePostHandler not_found"},
		},
		{
			name: "存在しないPostのLike解除はNotFound", method: http.MethodDelete, path: postPath + "/like",
			setup: func(repos routeRepositories) {
				repos.Post.EXPECT().GetByID(gomock.Any(), routePostID).Return(nil, repositories.ErrPostNotFound)
			},
			want: []string{"LikeUsecase.UnlikePost not_found", "LikeController.UnlikePostHandler not_found"},
		},
		{
			name: "存在しないCommentのLikeはNotFound", method: http.MethodPut, path: commentPath + "/like",
			setup: func(repos routeRepositories) {
				repos.Comment.EXPECT().GetByID(gomock.Any(), routeCommentID).Return(nil, repositories.ErrCommentNotFound)
			},
			want: []string{"LikeUsecase.LikeComment not_found", "LikeController.LikeCommentHandler not_found"},
		},
		{
			name: "不正なComment IDのLike解除はHandlerだけでinvalid_input", method: http.MethodDelete, path: "/api/comments/not-a-uuid/like",
			want: []string{"LikeController.UnlikeCommentHandler invalid_input"},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			router, repos, recorder := newRecordingRouter(t)
			if tt.setup != nil {
				tt.setup(repos)
			}

			performRequest(router, tt.method, tt.path, "user-"+string(routeUserID), tt.body)

			spans := recorder.Ended()
			require.Equal(t, tt.want, tracingtest.Results(spans))
			handler := spans[len(spans)-1]
			require.Equal(t, "handler", tracingtest.Attribute(handler, "threadly.layer"))
			require.Equal(t, tt.method, tracingtest.Attribute(handler, "http.request.method"))
			for _, span := range spans[:len(spans)-1] {
				if tracingtest.Attribute(span, "threadly.layer") == "usecase" &&
					!strings.HasPrefix(span.Name(), "LikeUsecase.PostSummaries") {
					require.Equal(t, handler.SpanContext().SpanID(), span.Parent().SpanID())
				}
			}
			wantStatus := codes.Unset
			if tracingtest.Attribute(handler, "threadly.result") == "internal_error" {
				wantStatus = codes.Error
			}
			require.Equal(t, wantStatus, handler.Status().Code)
			requireNoSecretInSpans(t, spans)
		})
	}
}

func TestUpdatePostHandler_RecordsOwnerLookupAndUpdateAsSiblings(t *testing.T) {
	router, repos, recorder := newRecordingRouter(t)
	post := &models.Post{UUIDBaseModel: models.UUIDBaseModel{ID: routePostID}, AuthorID: routeUserID, Title: "old", Content: "content"}
	repos.Post.EXPECT().GetByIDForOwner(gomock.Any(), routeUserID, routePostID).Return(post, nil)
	repos.Post.EXPECT().Update(gomock.Any(), routeUserID, gomock.Any()).Return(nil)

	response := performRequest(router, http.MethodPut, "/api/posts/"+string(routePostID), "user-"+string(routeUserID), `{"title":"new"}`)

	require.Equal(t, http.StatusOK, response.Code)
	spans := recorder.Ended()
	require.Equal(t, []string{
		"PostUsecase.GetPostByIDForOwner success",
		"PostUsecase.UpdatePost success",
		"PostController.UpdatePostHandler success",
	}, tracingtest.Results(spans))
	handler := spans[2]
	require.Equal(t, handler.SpanContext().SpanID(), spans[0].Parent().SpanID())
	require.Equal(t, handler.SpanContext().SpanID(), spans[1].Parent().SpanID())
	require.Equal(t, "/api/posts/:id", tracingtest.Attribute(handler, "http.route"))
	require.Equal(t, "200", tracingtest.Attribute(handler, "http.response.status_code"))
}

func TestHandlers_StartUnderIncomingSpanAndKeepRequestContext(t *testing.T) {
	router, _, recorder := newRecordingRouter(t)
	tracer, parentRecorder := tracingtest.NewRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, parent := tracer.Start(ctx, "HTTP parent")
	request := httptest.NewRequest(http.MethodGet, "/api/me", nil).WithContext(ctx)
	request.AddCookie(middleware.NewSessionCookie("user-" + string(routeUserID)))

	router.ServeHTTP(httptest.NewRecorder(), request)
	parent.End()

	spans := recorder.Ended()
	require.Equal(t, []string{"AuthUsecase.GetMe not_found", "AuthController.MeHandler not_found"}, tracingtest.Results(spans))
	require.Equal(t, parent.SpanContext().TraceID(), spans[1].SpanContext().TraceID())
	require.Equal(t, parent.SpanContext().SpanID(), spans[1].Parent().SpanID())
	require.Len(t, parentRecorder.Ended(), 1)
}

func TestHandlers_DoNotStartWhenAuthenticationRejectsRequest(t *testing.T) {
	router, _, recorder := newRecordingRouter(t)

	response := performRequest(router, http.MethodGet, "/api/posts", "", "")

	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.Empty(t, recorder.Ended())
}

func TestHandlers_PanicEndsSpansAndRecoveryReturns500(t *testing.T) {
	router, repos, recorder := newRecordingRouter(t)
	repos.Post.EXPECT().ListAll(gomock.Any()).DoAndReturn(func(context.Context) ([]*models.Post, error) {
		panic(routeSecret)
	})

	response := performRequest(router, http.MethodGet, "/api/posts", "user-"+string(routeUserID), "")

	require.Equal(t, http.StatusInternalServerError, response.Code)
	spans := recorder.Ended()
	require.Equal(t, []string{
		"PostUsecase.ListAllPostsForUser panic",
		"PostController.ListPostsHandler panic",
	}, tracingtest.Results(spans))
	for _, span := range spans {
		require.Equal(t, codes.Error, span.Status().Code)
	}
	require.Empty(t, tracingtest.Attribute(spans[1], "http.response.status_code"))
	requireNoSecretInSpans(t, spans)
}
