package models

import (
	"fmt"

	"github.com/google/uuid"
)

// UUIDはAPIとDBで使う正規化済みUUID文字列を表す。
type UUID string

func NewUUID() UUID {
	return UUID(uuid.NewString())
}

func ParseUUID(value string) (UUID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil {
		return "", fmt.Errorf("parse uuid: %w", err)
	}
	return UUID(parsed.String()), nil
}
