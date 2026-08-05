package controllers

import (
	"encoding/json"
	"strings"
	"testing"

	"Threadly/internal/domain/models"
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
