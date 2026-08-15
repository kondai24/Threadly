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

func TestToCommentResponseHidesInternalFieldsAndUsesEmptyReplies(t *testing.T) {
	comment := &models.Comment{
		UUIDBaseModel: models.UUIDBaseModel{ID: "77777777-7777-4777-8777-777777777777"},
		Content:       "comment",
		Author: models.User{
			UUIDBaseModel: models.UUIDBaseModel{ID: "11111111-1111-4111-8111-111111111111"},
			Username:      "alice",
			PasswordHash:  "must-not-leak",
		},
	}

	body, err := json.Marshal(toCommentResponse(comment))
	if err != nil {
		t.Fatalf("marshal comment response: %v", err)
	}
	bodyString := string(body)
	if strings.Contains(bodyString, comment.Author.PasswordHash) || strings.Contains(bodyString, "passwordHash") {
		t.Fatalf("comment response exposes password data: %s", bodyString)
	}
	if !strings.Contains(bodyString, `"replies":[]`) {
		t.Fatalf("comment response replies = %s, want empty array", bodyString)
	}
}

func TestWriteCommentErrorHidesInternalDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	writeCommentError(context, errors.New("database password leaked in internal error"))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "database password") {
		t.Fatalf("response exposes internal error: %s", recorder.Body.String())
	}
}
