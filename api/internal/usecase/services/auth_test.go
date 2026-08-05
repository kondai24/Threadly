package services

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
	"Threadly/internal/usecase/services/mocks"

	"go.uber.org/mock/gomock"
)

type fakePasswordHasher struct {
	hash       string
	compareErr error
}

func (h fakePasswordHasher) Hash(string) (string, error) {
	return h.hash, nil
}

func (h fakePasswordHasher) Compare(string, string) error {
	return h.compareErr
}

type fakeTokenIssuer struct {
	token string
}

func (i fakeTokenIssuer) Issue(uint) (string, error) {
	return i.token, nil
}

func (i fakeTokenIssuer) Parse(string) (uint, error) {
	return 1, nil
}

func newAuthServiceTest(
	t *testing.T,
	hasher PasswordHasher,
	tokens TokenIssuer,
) (*AuthService, *mocks.MockUserRepository) {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	repo := mocks.NewMockUserRepository(ctrl)
	service := NewAuthService(repo, hasher, tokens)
	return service, repo
}

func TestAuthService_Register(t *testing.T) {
	service, repo := newAuthServiceTest(
		t,
		fakePasswordHasher{hash: "argon2id-hash"},
		fakeTokenIssuer{token: "access-token"},
	)
	repo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, user *models.User) error {
			if user.Username != "alice" {
				t.Errorf("username = %q, want alice", user.Username)
			}
			if user.PasswordHash != "argon2id-hash" {
				t.Errorf("password hash = %q, want argon2id-hash", user.PasswordHash)
			}
			user.ID = 1
			return nil
		})

	user, token, err := service.Register(
		context.Background(),
		"alice",
		"correct horse",
	)

	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if user.ID != 1 || user.Username != "alice" {
		t.Fatalf("user = %+v, want ID 1 and username alice", user)
	}
	if user.PasswordHash != "argon2id-hash" {
		t.Fatalf("password hash = %q, want hashed password", user.PasswordHash)
	}
	if token != "access-token" {
		t.Fatalf("token = %q, want access-token", token)
	}
}

func TestAuthService_RegisterRejectsDuplicateUsername(t *testing.T) {
	service, repo := newAuthServiceTest(
		t,
		fakePasswordHasher{hash: "argon2id-hash"},
		fakeTokenIssuer{token: "access-token"},
	)
	repo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(repositories.ErrUsernameAlreadyExists)

	_, _, err := service.Register(context.Background(), "alice", "correct horse")

	if !errors.Is(err, ErrUsernameAlreadyExists) {
		t.Fatalf("error = %v, want ErrUsernameAlreadyExists", err)
	}
}

func TestAuthService_Login(t *testing.T) {
	t.Run("正しい認証情報でtokenを返す", func(t *testing.T) {
		service, repo := newAuthServiceTest(
			t,
			fakePasswordHasher{},
			fakeTokenIssuer{token: "access-token"},
		)
		user := &models.User{
			BaseModel:    models.BaseModel{ID: 1},
			Username:     "alice",
			PasswordHash: "argon2id-hash",
		}
		repo.EXPECT().FindByUsername(gomock.Any(), "alice").Return(user, nil)

		user, token, err := service.Login(context.Background(), "alice", "correct horse")

		if err != nil {
			t.Fatalf("login: %v", err)
		}
		if user.Username != "alice" || token != "access-token" {
			t.Fatalf("user = %+v, token = %q", user, token)
		}
	})

	t.Run("誤ったpasswordを認証失敗として扱う", func(t *testing.T) {
		service, repo := newAuthServiceTest(
			t,
			fakePasswordHasher{compareErr: fmt.Errorf("verify: %w", ErrPasswordMismatch)},
			fakeTokenIssuer{token: "access-token"},
		)
		repo.EXPECT().FindByUsername(gomock.Any(), "alice").Return(&models.User{
			BaseModel:    models.BaseModel{ID: 1},
			Username:     "alice",
			PasswordHash: "argon2id-hash",
		}, nil)

		_, _, err := service.Login(context.Background(), "alice", "wrong pass")

		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("error = %v, want ErrInvalidCredentials", err)
		}
	})

	t.Run("PasswordHasherの内部エラーをラップして返す", func(t *testing.T) {
		expectedErr := errors.New("invalid password hash")
		service, repo := newAuthServiceTest(
			t,
			fakePasswordHasher{compareErr: expectedErr},
			fakeTokenIssuer{token: "access-token"},
		)
		repo.EXPECT().FindByUsername(gomock.Any(), "alice").Return(&models.User{
			BaseModel:    models.BaseModel{ID: 1},
			Username:     "alice",
			PasswordHash: "invalid-hash",
		}, nil)

		_, _, err := service.Login(context.Background(), "alice", "correct horse")

		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, want wrapped password hasher error", err)
		}
		if errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("error = %v, must not be ErrInvalidCredentials", err)
		}
	})

	t.Run("未知のusernameを認証失敗として扱う", func(t *testing.T) {
		service, repo := newAuthServiceTest(
			t,
			fakePasswordHasher{},
			fakeTokenIssuer{token: "access-token"},
		)
		repo.EXPECT().
			FindByUsername(gomock.Any(), "nobody").
			Return(nil, repositories.ErrUserNotFound)

		_, _, err := service.Login(context.Background(), "nobody", "wrong pass")

		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("error = %v, want ErrInvalidCredentials", err)
		}
	})
}

func TestAuthService_GetMe(t *testing.T) {
	t.Run("User IDに対応するUserを返す", func(t *testing.T) {
		service, repo := newAuthServiceTest(
			t,
			fakePasswordHasher{},
			fakeTokenIssuer{},
		)
		expected := &models.User{
			BaseModel: models.BaseModel{ID: 7},
			Username:  "alice",
		}
		repo.EXPECT().FindByID(gomock.Any(), uint(7)).Return(expected, nil)

		user, err := service.GetMe(context.Background(), 7)

		if err != nil {
			t.Fatalf("get me: %v", err)
		}
		if user != expected {
			t.Fatalf("user = %+v, want %+v", user, expected)
		}
	})

	t.Run("Userが存在しない場合はNotFoundを返す", func(t *testing.T) {
		service, repo := newAuthServiceTest(
			t,
			fakePasswordHasher{},
			fakeTokenIssuer{},
		)
		repo.EXPECT().
			FindByID(gomock.Any(), uint(999)).
			Return(nil, repositories.ErrUserNotFound)

		user, err := service.GetMe(context.Background(), 999)

		if user != nil {
			t.Fatalf("user = %+v, want nil", user)
		}
		if !errors.Is(err, repositories.ErrUserNotFound) {
			t.Fatalf("error = %v, want ErrUserNotFound", err)
		}
	})

	t.Run("Repositoryのエラーをラップして返す", func(t *testing.T) {
		service, repo := newAuthServiceTest(
			t,
			fakePasswordHasher{},
			fakeTokenIssuer{},
		)
		expectedErr := errors.New("db error")
		repo.EXPECT().FindByID(gomock.Any(), uint(7)).Return(nil, expectedErr)

		user, err := service.GetMe(context.Background(), 7)

		if user != nil {
			t.Fatalf("user = %+v, want nil", user)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, want wrapped db error", err)
		}
	})
}

func TestAuthService_ValidatesCredentials(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		wantErr  error
	}{
		{name: "短いusername", username: "ab", password: "correct horse", wantErr: models.ErrInvalidUsername},
		{name: "短いpassword", username: "alice", password: "short", wantErr: models.ErrInvalidPassword},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _ := newAuthServiceTest(
				t,
				fakePasswordHasher{hash: "hash"},
				fakeTokenIssuer{token: "token"},
			)
			_, _, err := service.Register(context.Background(), tt.username, tt.password)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
