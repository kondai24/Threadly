package routes

import (
	"errors"
	"net/http"
	"testing"

	"Threadly/internal/domain/models"
	"Threadly/internal/interface/controllers"
	"Threadly/internal/usecase"
	"Threadly/internal/usecase/mocks"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type failingPasswordHasher struct {
	routePasswordHasher
	err error
}

func (h failingPasswordHasher) Compare(string, string) error { return h.err }

func TestSetupRouter_InternalErrorsHideDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	internalErr := errors.New("private database or hash diagnostic")
	cases := []struct {
		name   string
		method string
		path   string
		body   string
		router func(*testing.T) *gin.Engine
	}{
		{
			name:   "Hasher障害を認証失敗にしない",
			method: http.MethodPost,
			path:   "/api/auth/login",
			body:   `{"username":"alice","password":"password"}`,
			router: func(t *testing.T) *gin.Engine {
				repo := mocks.NewMockUserRepository(gomock.NewController(t))
				repo.EXPECT().FindByUsername(gomock.Any(), "alice").Return(&models.User{
					UUIDBaseModel: models.UUIDBaseModel{ID: routeUserID}, PasswordHash: "hash",
				}, nil)
				tokens := routeTokenIssuer{}
				auth := usecase.NewAuthUsecase(repo, failingPasswordHasher{err: internalErr}, tokens)
				return SetupRouter(Handlers{Auth: controllers.NewAuthController(auth), TokenIssuer: tokens})
			},
		},
		{
			name:   "Post一覧のDB障害を空一覧にしない",
			method: http.MethodGet,
			path:   "/api/posts",
			router: func(t *testing.T) *gin.Engine {
				router, repos := newTestRouter(t)
				repos.Post.EXPECT().ListAll(gomock.Any()).Return(nil, internalErr)
				return router
			},
		},
		{
			name:   "CommentのDB障害をNotFoundにしない",
			method: http.MethodGet,
			path:   "/api/posts/" + string(routePostID) + "/comments",
			router: func(t *testing.T) *gin.Engine {
				router, repos := newTestRouter(t)
				repos.Post.EXPECT().GetByID(gomock.Any(), routePostID).Return(nil, internalErr)
				return router
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			response := performRequest(tt.router(t), tt.method, tt.path, "user-"+string(routeUserID), tt.body)
			require.Equal(t, http.StatusInternalServerError, response.Code)
			require.JSONEq(t, `{"error":"internal server error"}`, response.Body.String())
		})
	}
}
