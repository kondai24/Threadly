package models

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"
)

var (
	ErrInvalidUsername = errors.New("invalid username")
	ErrInvalidPassword = errors.New("invalid password")
)

// usernameはAPI仕様どおり、ASCII英数字とアンダースコアだけを許可する。
var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{3,32}$`)

type User struct {
	UUIDBaseModel
	Username     string `gorm:"size:32;not null;uniqueIndex" json:"username"`
	PasswordHash string `gorm:"size:255;not null" json:"-"`
}

func (u *User) BeforeCreate(_ *gorm.DB) error {
	if u.ID == "" {
		u.ID = NewUUID()
	}
	return nil
}

func ValidateUsername(username string) error {
	if username != strings.TrimSpace(username) || !usernamePattern.MatchString(username) {
		return ErrInvalidUsername
	}
	return nil
}

func ValidatePassword(password string) error {
	if !utf8.ValidString(password) {
		return ErrInvalidPassword
	}

	passwordLength := utf8.RuneCountInString(password)
	if passwordLength < 8 || passwordLength > 128 {
		return ErrInvalidPassword
	}
	return nil
}
