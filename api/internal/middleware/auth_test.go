package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type tokenIssuerStub struct {
	userID uint
	err    error
}

func (s tokenIssuerStub) Issue(uint) (string, error) {
	return "unused", nil
}

func (s tokenIssuerStub) Parse(string) (uint, error) {
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
		wantUserID    uint
	}{
		{
			name:       "Authorizationヘッダーがない",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:          "Authorizationヘッダーを拒否する",
			authorization: "Bearer token",
			issuer:        tokenIssuerStub{userID: 7},
			wantStatus:    http.StatusUnauthorized,
		},
		{
			name:       "正しいsession cookie",
			cookie:     "token",
			issuer:     tokenIssuerStub{userID: 8},
			wantStatus: http.StatusOK,
			wantUserID: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/", RequireAuth(tt.issuer), func(c *gin.Context) {
				userID, ok := UserIDFromContext(c.Request.Context())
				if !ok || userID != tt.wantUserID {
					t.Fatalf("context user ID = %d, ok = %t, want %d", userID, ok, tt.wantUserID)
				}
				c.Status(http.StatusOK)
			})

			request := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authorization != "" {
				request.Header.Set("Authorization", tt.authorization)
			}
			if tt.cookie != "" {
				request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: tt.cookie})
			}
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, tt.wantStatus)
			}
		})
	}
}
