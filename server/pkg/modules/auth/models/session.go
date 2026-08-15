package models

import (
	"time"

	"gorm.io/gorm"
)

type Session struct {
	gorm.Model

	UserID uint `gorm:"index;not null"`
	User   User `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	Code string `gorm:"uniqueIndex;not null"`

	WorkspaceID *uint      `gorm:"index"`
	Workspace   *Workspace `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	ExpiresAt time.Time
}
