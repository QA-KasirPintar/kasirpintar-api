package models

import (
	"time"

	"gorm.io/gorm"
)

// PasswordReset menyimpan token reset password yang di-hash
type PasswordReset struct {
	gorm.Model
	UserID uint `gorm:"not null;index"`
	User   User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	TokenHash string    `gorm:"size:255;not null"`
	ExpiresAt time.Time `gorm:"index"`
}
