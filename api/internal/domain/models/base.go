package models

import (
	"time"

	"gorm.io/gorm"
)

// UUIDBaseModelはUUIDのIDと監査・論理削除フィールドを共通化する。
type UUIDBaseModel struct {
	ID        UUID `gorm:"type:char(36);primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
