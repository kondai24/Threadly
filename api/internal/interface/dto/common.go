package dto

import "Threadly/internal/domain/models"

// PublicUserResponseはAPIで公開するUserの最小情報を表す。
type PublicUserResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

func PublicUserResponseFromModel(user models.User) PublicUserResponse {
	return PublicUserResponse{
		ID:       string(user.ID),
		Username: user.Username,
	}
}
