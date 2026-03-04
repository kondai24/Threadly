package models

import (
	"time"

	"gorm.io/gorm"
)

// gorm.Model の代わりに使う。json タグ付きで DB/API 両方に対応。
// これによりswaggoとorvalが小文字で生成される
type BaseModel struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deletedAt"`
}
