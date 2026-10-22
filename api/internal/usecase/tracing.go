package usecase

import (
	"errors"

	"Threadly/internal/domain/models"
	"Threadly/internal/tracing"
)

// usecaseResultは、Usecaseが返す業務上のsentinelを想定内の結果として分類する。
// errorのwrapや戻り値は変えず、errors.Isで判定するためwrapped sentinelも同じ結果になる。
func usecaseResult(err error) tracing.Result {
	switch {
	case err == nil:
		return tracing.ResultSuccess
	case errors.Is(err, models.ErrInvalidUsername),
		errors.Is(err, models.ErrInvalidPassword),
		errors.Is(err, models.ErrInvalidTitle),
		errors.Is(err, models.ErrInvalidContent),
		errors.Is(err, models.ErrInvalidCommentContent),
		errors.Is(err, ErrCommentReplyNotAllowed):
		return tracing.ResultInvalidInput
	case errors.Is(err, ErrInvalidCredentials), errors.Is(err, ErrInvalidToken):
		return tracing.ResultUnauthenticated
	case errors.Is(err, ErrPostNotFound),
		errors.Is(err, ErrCommentNotFound),
		errors.Is(err, ErrUserNotFound),
		errors.Is(err, ErrLikeTargetNotFound):
		return tracing.ResultNotFound
	case errors.Is(err, ErrUsernameAlreadyExists):
		return tracing.ResultConflict
	default:
		return tracing.ResultInternalError
	}
}
