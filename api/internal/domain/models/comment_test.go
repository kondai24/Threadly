package models

import (
	"errors"
	"strings"
	"testing"
)

func TestCommentValidate(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{name: "1文字を許可する", content: "a"},
		{name: "空文字を拒否する", content: "", wantErr: true},
		{name: "空白だけを拒否する", content: " \t\n", wantErr: true},
		{name: "1001文字を拒否する", content: strings.Repeat("a", 1001), wantErr: true},
		{name: "Rune数で本文長を数える", content: strings.Repeat("あ", 1000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (&Comment{Content: tt.content}).Validate()
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidCommentContent) {
					t.Fatalf("Comment.Validate() error = %v, want ErrInvalidCommentContent", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Comment.Validate() unexpected error = %v", err)
			}
		})
	}
}
