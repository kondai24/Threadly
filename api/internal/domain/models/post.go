package models

import (
	"errors"
)

var (
	ErrInvalidTitle   = errors.New("title is required")
	ErrInvalidContent = errors.New("content is required")
)

type Post struct {
	BaseModel
	AuthorID uint   `gorm:"not null;index" json:"authorId"`
	Author   User   `gorm:"foreignKey:AuthorID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	Title    string `json:"title"`
	Content  string `json:"content"`
}

func (p *Post) Validate() error {
	if p.Title == "" {
		return ErrInvalidTitle
	}
	if p.Content == "" {
		return ErrInvalidContent
	}
	return nil
}
