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

func TestRequireAuthRejectsAuthorizationHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	issuer := tokenIssuerStub{userID: "77777777-7777-4777-8777-777777777777"}
	router.GET("/", RequireAuth(issuer), func(c *gin.Context) {
		t.Fatal("unauthenticated request reached handler")
	})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
}
