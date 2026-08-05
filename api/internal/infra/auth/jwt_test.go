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

func TestJWTIssuer_RejectsExpiredToken(t *testing.T) {
	issuer, err := NewJWTIssuer(strings.Repeat("s", minimumSecretLen))
	if err != nil {
		t.Fatalf("new jwt issuer: %v", err)
	}
	expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   "42",
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
	})
	rawToken, err := expiredToken.SignedString(issuer.secret)
	if err != nil {
		t.Fatalf("sign expired token: %v", err)
	}

	_, err = issuer.Parse(rawToken)

	if !errors.Is(err, services.ErrInvalidToken) {
		t.Fatalf("expired token error = %v, want ErrInvalidToken", err)
	}
}

func TestJWTIssuer_RejectsWrongAlgorithm(t *testing.T) {
	issuer, err := NewJWTIssuer(strings.Repeat("s", minimumSecretLen))
	if err != nil {
		t.Fatalf("new jwt issuer: %v", err)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.RegisteredClaims{Subject: "42"})
	rawToken, err := token.SignedString(issuer.secret)
	if err != nil {
		t.Fatalf("sign wrong algorithm token: %v", err)
	}

	_, err = issuer.Parse(rawToken)

	if !errors.Is(err, services.ErrInvalidToken) {
		t.Fatalf("wrong algorithm error = %v, want ErrInvalidToken", err)
	}
}

func TestNewJWTIssuer_RequiresLongSecret(t *testing.T) {
	_, err := NewJWTIssuer("short")

	if err == nil {
		t.Fatal("expected short secret to be rejected")
	}
}
