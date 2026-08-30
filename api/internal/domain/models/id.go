package models

import (
	"fmt"

	"github.com/google/uuid"
)

// UUIDはAPIとDBで使う正規化済みUUID文字列を表す。
type UUID string

// NewUUIDは、新しいPost・Comment・Userの識別子を生成する。
func NewUUID() UUID {
	return UUID(uuid.NewString())
}

// ParseUUIDは、入力を正規化したUUID文字列へ変換する。
func ParseUUID(value string) (UUID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil {
		return "", fmt.Errorf("parse uuid: %w", err)
	}
	return UUID(parsed.String()), nil
}
