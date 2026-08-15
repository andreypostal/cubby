package models

import (
	"gorm.io/gorm"
)

type User struct {
	gorm.Model

	Username string `gorm:"uniqueIndex;not null"`
	Password string `gorm:"type:varchar(255);not null"`

	Name          string `gorm:"varchar(255);not null"`
	IsSystemAdmin bool   `gorm:"not null;default:false"`

	Workspaces []Workspace `gorm:"foreignKey:CreatorID"`
}
