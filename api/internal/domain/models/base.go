package models

import (
	"time"

	"gorm.io/gorm"
)

// UUIDBaseModelはUUIDのIDと監査・論理削除フィールドを共通化する。
type UUIDBaseModel struct {
	ID        UUID           `gorm:"type:char(36);primaryKey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deletedAt"`
}
