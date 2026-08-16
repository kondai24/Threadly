package models

import (
	"time"

	"gorm.io/gorm"
)

// LikeSummaryは、対象に対するLikeの集計結果を表す読み取り専用値である。
type LikeSummary struct {
	Count     int64
	LikedByMe bool
}

// PostLikeはUserとPostのLike関係を表す。Unlikeでは行を物理削除するため、論理削除を持たない。
type PostLike struct {
	ID        UUID `gorm:"type:char(36);primaryKey"`
	UserID    UUID `gorm:"type:char(36);not null;uniqueIndex:uidx_post_likes_user_post"`
	User      User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	PostID    UUID `gorm:"type:char(36);not null;index:idx_post_likes_post_id;uniqueIndex:uidx_post_likes_user_post"`
	Post      Post `gorm:"foreignKey:PostID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	CreatedAt time.Time
}

func (l *PostLike) BeforeCreate(_ *gorm.DB) error {
	if l.ID == "" {
		l.ID = NewUUID()
	}
	return nil
}

// CommentLikeはUserとCommentのLike関係を表す。Unlikeでは行を物理削除するため、論理削除を持たない。
type CommentLike struct {
	ID        UUID    `gorm:"type:char(36);primaryKey"`
	UserID    UUID    `gorm:"type:char(36);not null;uniqueIndex:uidx_comment_likes_user_comment"`
	User      User    `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	CommentID UUID    `gorm:"type:char(36);not null;index:idx_comment_likes_comment_id;uniqueIndex:uidx_comment_likes_user_comment"`
	Comment   Comment `gorm:"foreignKey:CommentID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	CreatedAt time.Time
}

func (l *CommentLike) BeforeCreate(_ *gorm.DB) error {
	if l.ID == "" {
		l.ID = NewUUID()
	}
	return nil
}
