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
		{name: "1000文字を許可する", content: strings.Repeat("a", 1000)},
		{name: "前後空白を除いた本文を許可する", content: "  comment  "},
		{name: "空文字を拒否する", content: "", wantErr: true},
		{name: "空白だけを拒否する", content: " \t\n", wantErr: true},
		{name: "1001文字を拒否する", content: strings.Repeat("a", 1001), wantErr: true},
		{name: "Rune数で本文長を数える", content: strings.Repeat("あ", 1000)},
		{name: "不正UTF-8を拒否する", content: string([]byte{'a', 0xff}), wantErr: true},
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

func TestCommentBeforeCreateAssignsUUID(t *testing.T) {
	comment := &Comment{}
	if err := comment.BeforeCreate(nil); err != nil {
		t.Fatalf("Comment.BeforeCreate() error = %v", err)
	}
	if comment.ID == "" {
		t.Fatal("Comment.BeforeCreate() did not assign ID")
	}
	if _, err := ParseUUID(string(comment.ID)); err != nil {
		t.Fatalf("Comment.BeforeCreate() assigned invalid UUID: %v", err)
	}
}
