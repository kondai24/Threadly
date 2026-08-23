package dto

import "Threadly/internal/usecase"

type LikeActionResponse struct {
	TargetID  string `json:"targetId"`
	LikeCount int64  `json:"likeCount"`
	LikedByMe bool   `json:"likedByMe"`
}

func LikeActionResponseFromResult(result usecase.LikeActionResult) LikeActionResponse {
	return LikeActionResponse{
		TargetID:  string(result.TargetID),
		LikeCount: result.Summary.Count,
		LikedByMe: result.Summary.LikedByMe,
	}
}
