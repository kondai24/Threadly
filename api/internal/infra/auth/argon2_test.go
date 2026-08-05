package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestArgon2idHasher_HashAndCompare(t *testing.T) {
	hasher := NewArgon2idHasher()
	password := "correct horse battery staple"

	firstHash, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	secondHash, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("hash password a second time: %v", err)
	}

	if !strings.HasPrefix(firstHash, "$argon2id$v=19$") {
		t.Fatalf("hash format = %q, want argon2id PHC format", firstHash)
	}
	if firstHash == secondHash {
		t.Fatal("hashes with random salts must differ")
	}
	if err := hasher.Compare(firstHash, password); err != nil {
		t.Fatalf("compare correct password: %v", err)
	}
	if err := hasher.Compare(firstHash, "wrong password"); !errors.Is(err, ErrPasswordMismatch) {
		t.Fatalf("compare wrong password error = %v, want ErrPasswordMismatch", err)
	}
}

func TestArgon2idHasher_RejectsMalformedHash(t *testing.T) {
	hasher := NewArgon2idHasher()

	err := hasher.Compare("not-an-argon2id-hash", "password")

	if !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("malformed hash error = %v, want ErrInvalidHash", err)
	}
}
