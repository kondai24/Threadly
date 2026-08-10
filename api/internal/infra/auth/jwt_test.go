package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"Threadly/internal/usecase/services"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWTIssuer_IssueAndParse(t *testing.T) {
	issuer, err := NewJWTIssuer(strings.Repeat("s", minimumSecretLen))
	if err != nil {
		t.Fatalf("new jwt issuer: %v", err)
	}
	now := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	issuer.now = func() time.Time { return now }

	rawToken, err := issuer.Issue(42)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	userID, err := issuer.Parse(rawToken)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if userID != 42 {
		t.Fatalf("user ID = %d, want 42", userID)
	}
}

func TestJWTIssuer_RejectsInvalidTokens(t *testing.T) {
	issuer, err := NewJWTIssuer(strings.Repeat("s", minimumSecretLen))
	if err != nil {
		t.Fatalf("new jwt issuer: %v", err)
	}
	now := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	issuer.now = func() time.Time { return now }
	validClaims := jwt.RegisteredClaims{
		Subject:   "42",
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}

	tests := []struct {
		name       string
		method     jwt.SigningMethod
		claims     jwt.RegisteredClaims
		signingKey []byte
	}{
		{
			name:   "期限切れtokenを拒否する",
			method: jwt.SigningMethodHS256,
			claims: jwt.RegisteredClaims{
				Subject:   "42",
				IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
				ExpiresAt: jwt.NewNumericDate(now.Add(-time.Hour)),
			},
			signingKey: issuer.secret,
		},
		{
			name:   "expのないtokenを拒否する",
			method: jwt.SigningMethodHS256,
			claims: jwt.RegisteredClaims{
				Subject:  "42",
				IssuedAt: jwt.NewNumericDate(now),
			},
			signingKey: issuer.secret,
		},
		{
			name:   "未来のiatを拒否する",
			method: jwt.SigningMethodHS256,
			claims: jwt.RegisteredClaims{
				Subject:   "42",
				IssuedAt:  jwt.NewNumericDate(now.Add(time.Minute)),
				ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			},
			signingKey: issuer.secret,
		},
		{
			name:       "許可外アルゴリズムを拒否する",
			method:     jwt.SigningMethodHS512,
			claims:     validClaims,
			signingKey: issuer.secret,
		},
		{
			name:       "不正な署名鍵を拒否する",
			method:     jwt.SigningMethodHS256,
			claims:     validClaims,
			signingKey: []byte(strings.Repeat("x", minimumSecretLen)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := jwt.NewWithClaims(tt.method, tt.claims)
			rawToken, signErr := token.SignedString(tt.signingKey)
			if signErr != nil {
				t.Fatalf("sign token: %v", signErr)
			}

			_, parseErr := issuer.Parse(rawToken)
			if !errors.Is(parseErr, services.ErrInvalidToken) {
				t.Fatalf("Parse() error = %v, want ErrInvalidToken", parseErr)
			}
		})
	}
}

func TestNewJWTIssuer_RequiresLongSecret(t *testing.T) {
	_, err := NewJWTIssuer("short")

	if err == nil {
		t.Fatal("expected short secret to be rejected")
	}
}
