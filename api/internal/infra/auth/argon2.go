package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	// argon2Memoryはargon2.IDKeyと同じKiB単位で指定する。
	argon2Memory      uint32 = 64 * 1024
	argon2Iterations  uint32 = 3
	argon2Parallelism uint8  = 2
	argon2SaltLength  uint32 = 16
	argon2KeyLength   uint32 = 32
)

var (
	ErrInvalidHash      = errors.New("invalid password hash")
	ErrPasswordMismatch = errors.New("password mismatch")
)

type Argon2idHasher struct{}

func NewArgon2idHasher() *Argon2idHasher {
	return &Argon2idHasher{}
}

func (h *Argon2idHasher) Hash(password string) (string, error) {
	salt := make([]byte, argon2SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	key := argon2.IDKey(
		[]byte(password),
		salt,
		argon2Iterations,
		argon2Memory,
		argon2Parallelism,
		argon2KeyLength,
	)

	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedKey := base64.RawStdEncoding.EncodeToString(key)
	// 検証に必要なパラメータ・salt・derived keyを、1つの文字列に保存する。
	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argon2Memory,
		argon2Iterations,
		argon2Parallelism,
		encodedSalt,
		encodedKey,
	), nil
}

func (h *Argon2idHasher) Compare(encodedHash string, password string) error {
	params, salt, expectedKey, err := parseArgon2idHash(encodedHash)
	if err != nil {
		return err
	}

	// 現在の期待するパラメータだけを受け入れ、想定外の設定で検証するのを防ぐ。
	actualKey := argon2.IDKey(
		[]byte(password),
		salt,
		params.iterations,
		params.memory,
		params.parallelism,
		params.keyLength,
	)
	if subtle.ConstantTimeCompare(actualKey, expectedKey) != 1 {
		return ErrPasswordMismatch
	}
	return nil
}

type argon2idParameters struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	keyLength   uint32
}

func parseArgon2idHash(encodedHash string) (argon2idParameters, []byte, []byte, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return argon2idParameters{}, nil, nil, ErrInvalidHash
	}

	params, err := parseArgon2idParameters(parts[3])
	if err != nil {
		return argon2idParameters{}, nil, nil, err
	}
	if params.memory != argon2Memory ||
		params.iterations != argon2Iterations ||
		params.parallelism != argon2Parallelism {
		return argon2idParameters{}, nil, nil, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != int(argon2SaltLength) {
		return argon2idParameters{}, nil, nil, ErrInvalidHash
	}
	expectedKey, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expectedKey) != int(argon2KeyLength) {
		return argon2idParameters{}, nil, nil, ErrInvalidHash
	}
	return params, salt, expectedKey, nil
}

func parseArgon2idParameters(encodedParams string) (argon2idParameters, error) {
	params := argon2idParameters{keyLength: argon2KeyLength}
	for _, value := range strings.Split(encodedParams, ",") {
		key, rawValue, ok := strings.Cut(value, "=")
		if !ok {
			return argon2idParameters{}, ErrInvalidHash
		}

		number, err := strconv.ParseUint(rawValue, 10, 32)
		if err != nil {
			return argon2idParameters{}, ErrInvalidHash
		}
		switch key {
		case "m":
			params.memory = uint32(number)
		case "t":
			params.iterations = uint32(number)
		case "p":
			if number > 255 {
				return argon2idParameters{}, ErrInvalidHash
			}
			params.parallelism = uint8(number)
		default:
			return argon2idParameters{}, ErrInvalidHash
		}
	}

	if params.memory == 0 || params.iterations == 0 || params.parallelism == 0 {
		return argon2idParameters{}, ErrInvalidHash
	}
	return params, nil
}
