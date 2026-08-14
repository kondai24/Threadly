package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"Threadly/internal/domain/models"

	"github.com/gin-gonic/gin"
)

type tokenIssuerStub struct {
	userID models.UUID
	err    error
}

func (s tokenIssuerStub) Issue(models.UUID) (string, error) {
	return "unused", nil
}

func (s tokenIssuerStub) Parse(string) (models.UUID, error) {
	return s.userID, s.err
}

func TestRequireAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name          string
		authorization string
		cookie        string
		issuer        tokenIssuerStub
		wantStatus    int
		wantUserID    models.UUID
	}{
		{
			name:       "Authorizationヘッダーがない",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:          "Authorizationヘッダーを拒否する",
			authorization: "Bearer token",
			issuer:        tokenIssuerStub{userID: "77777777-7777-4777-8777-777777777777"},
			wantStatus:    http.StatusUnauthorized,
		},
		{
			name:       "正しいsession cookie",
			cookie:     "token",
			issuer:     tokenIssuerStub{userID: "88888888-8888-4888-8888-888888888888"},
			wantStatus: http.StatusOK,
			wantUserID: "88888888-8888-4888-8888-888888888888",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/", RequireAuth(tt.issuer), func(c *gin.Context) {
				userID, ok := UserIDFromContext(c.Request.Context())
				if !ok || userID != tt.wantUserID {
					t.Fatalf("context user ID = %s, ok = %t, want %s", userID, ok, tt.wantUserID)
				}
				c.Status(http.StatusOK)
			})

			request := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authorization != "" {
				request.Header.Set("Authorization", tt.authorization)
			}
			if tt.cookie != "" {
				request.AddCookie(NewSessionCookie(tt.cookie))
			}
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, tt.wantStatus)
			}
		})
	}
}
