package models

import (
	"errors"

	"gorm.io/gorm"
)

type Post struct {
	*gorm.Model
	Title   string
	Content string
}

func (p *Post) Validate() error {
	if p.Title == "" {
		return errors.New("titleが入力されていません")
	}
	if p.Content == "" {
		return errors.New("contentが入力されていません")
	}
	return nil
}
