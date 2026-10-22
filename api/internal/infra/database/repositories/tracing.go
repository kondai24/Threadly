package repository

import (
	"errors"

	"Threadly/internal/domain/repositories"
	"Threadly/internal/tracing"
)

// repositoryResultは、Repository契約のsentinelだけを想定内の結果として分類する。
// rows=0は各操作の既存戻り値契約に従い、ここでは一律にnot_foundへ変換しない。
func repositoryResult(err error) tracing.Result {
	switch {
	case err == nil:
		return tracing.ResultSuccess
	case errors.Is(err, repositories.ErrUserNotFound),
		errors.Is(err, repositories.ErrPostNotFound),
		errors.Is(err, repositories.ErrCommentNotFound):
		return tracing.ResultNotFound
	case errors.Is(err, repositories.ErrUsernameAlreadyExists):
		return tracing.ResultConflict
	default:
		return tracing.ResultInternalError
	}
}
