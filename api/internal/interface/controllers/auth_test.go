package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Threadly/internal/domain/models"

	"github.com/gin-gonic/gin"
)

func TestToUserResponseDoesNotExposePasswordHash(t *testing.T) {
	user := &models.User{
		Username:     "alice",
		PasswordHash: "secret-hash-must-not-leak",
	}

	body, err := json.Marshal(toUserResponse(user))
	if err != nil {
		t.Fatalf("marshal user response: %v", err)
	}

	if strings.Contains(string(body), user.PasswordHash) {
		t.Fatalf("user response exposes password hash: %s", body)
	}
	if !strings.Contains(string(body), `"username":"alice"`) {
		t.Fatalf("user response does not contain username: %s", body)
	}
}

func TestWriteAuthErrorReturnsInternalServerErrorForPasswordHasherFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	writeAuthError(context, errors.New("compare password hash: invalid password hash"))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "invalid password hash") {
		t.Fatalf("response exposes internal error: %s", recorder.Body.String())
	}
}
