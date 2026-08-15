package services

import "errors"

var (
	ErrPostNotFound           = errors.New("post not found")
	ErrCommentNotFound        = errors.New("comment not found")
	ErrCommentReplyNotAllowed = errors.New("comment reply not allowed")
)
