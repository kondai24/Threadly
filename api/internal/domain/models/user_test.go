package models

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{name: "3文字を許可する", username: "a_1"},
		{name: "32文字を許可する", username: strings.Repeat("a", 32)},
		{name: "ASCII英数字とアンダースコアを許可する", username: "Alice_123"},
		{name: "2文字を拒否する", username: "a1", wantErr: true},
		{name: "33文字を拒否する", username: strings.Repeat("a", 33), wantErr: true},
		{name: "先頭空白を拒否する", username: " alice", wantErr: true},
		{name: "末尾空白を拒否する", username: "alice ", wantErr: true},
		{name: "ASCII以外を拒否する", username: "abあ", wantErr: true},
		{name: "許可外記号を拒否する", username: "alice-1", wantErr: true},
		{name: "不正UTF-8を拒否する", username: string([]byte{'a', 'b', 0xff}), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUsername(tt.username)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidUsername) {
					t.Fatalf("ValidateUsername() error = %v, want ErrInvalidUsername", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateUsername() unexpected error = %v", err)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{name: "8文字を許可する", password: strings.Repeat("a", 8)},
		{name: "128文字を許可する", password: strings.Repeat("a", 128)},
		{name: "マルチバイト文字をRune数で数える", password: strings.Repeat("あ", 8)},
		{name: "7文字を拒否する", password: strings.Repeat("a", 7), wantErr: true},
		{name: "129文字を拒否する", password: strings.Repeat("a", 129), wantErr: true},
		{name: "不正UTF-8を拒否する", password: string([]byte{'p', 'a', 's', 's', 0xff}), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidPassword) {
					t.Fatalf("ValidatePassword() error = %v, want ErrInvalidPassword", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidatePassword() unexpected error = %v", err)
			}
		})
	}
}
