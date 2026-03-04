package models

import (
	"errors"
)

type Post struct {
	BaseModel
	Title   string `json:"title"`
	Content string `json:"content"`
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
