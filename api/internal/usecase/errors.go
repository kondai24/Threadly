package usecase

import "errors"

var (
	ErrPostNotFound           = errors.New("post not found")
	ErrCommentNotFound        = errors.New("comment not found")
	ErrCommentReplyNotAllowed = errors.New("comment reply not allowed")
	ErrUserNotFound           = errors.New("user not found")
)
