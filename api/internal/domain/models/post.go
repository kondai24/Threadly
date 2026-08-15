package models

import (
	"errors"

	"gorm.io/gorm"
)

var (
	ErrInvalidTitle   = errors.New("title is required")
	ErrInvalidContent = errors.New("content is required")
)

type Post struct {
	UUIDBaseModel
	AuthorID UUID `gorm:"type:char(36);not null;index"`
	Author   User `gorm:"foreignKey:AuthorID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	Title    string
	Content  string
}

func (p *Post) BeforeCreate(_ *gorm.DB) error {
	if p.ID == "" {
		p.ID = NewUUID()
	}
	return nil
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
