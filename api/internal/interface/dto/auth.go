package dto

import (
	"time"

	"Threadly/internal/domain/models"
)

type CredentialsRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type UserResponse struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type AuthResponse struct {
	User UserResponse `json:"user"`
}

func UserResponseFromModel(user *models.User) UserResponse {
	if user == nil {
		return UserResponse{}
	}
	return UserResponse{
		ID:        string(user.ID),
		Username:  user.Username,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func AuthResponseFromModel(user *models.User) AuthResponse {
	return AuthResponse{
		User: UserResponseFromModel(user),
	}
}
