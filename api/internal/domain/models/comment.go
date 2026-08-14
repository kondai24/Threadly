package models

import (
	"errors"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"
)

const (
	minCommentContentLength = 1
	maxCommentContentLength = 1000
)

var ErrInvalidCommentContent = errors.New("invalid comment content")

// CommentはPost直下またはPost直下Commentへの1段階返信を表す。
type Comment struct {
	UUIDBaseModel
	PostID   UUID       `gorm:"type:char(36);not null;index" json:"postId"`
	Post     Post       `gorm:"foreignKey:PostID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	AuthorID UUID       `gorm:"type:char(36);not null;index" json:"authorId"`
	Author   User       `gorm:"foreignKey:AuthorID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	ParentID *UUID      `gorm:"type:char(36);index" json:"parentId,omitempty"`
	Parent   *Comment   `gorm:"foreignKey:ParentID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	Replies  []*Comment `gorm:"foreignKey:ParentID" json:"-"`
	Content  string     `gorm:"type:text;not null" json:"content"`
}

func (c *Comment) BeforeCreate(_ *gorm.DB) error {
	if c.ID == "" {
		c.ID = NewUUID()
	}
	return nil
}

func (c *Comment) Validate() error {
	content := strings.TrimSpace(c.Content)
	if !utf8.ValidString(content) {
		return ErrInvalidCommentContent
	}

	contentLength := utf8.RuneCountInString(content)
	if contentLength < minCommentContentLength || contentLength > maxCommentContentLength {
		return ErrInvalidCommentContent
	}
	return nil
}
