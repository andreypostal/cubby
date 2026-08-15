package models

import (
	"time"

	"gorm.io/gorm"

	authModels "andreypostal.com/cubby/server/pkg/modules/auth/models"
)

type List struct {
	gorm.Model

	Name string `gorm:"varchar(255);not null"`
	Icon string `gorm:"varchar(50)"`

	WorkspaceID uint                 `gorm:"index;not null"`
	Workspace   authModels.Workspace `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type ListItem struct {
	gorm.Model

	Name   string `gorm:"varchar(255);not null"`
	Notes  string `gorm:"varchar(255)"`
	Bought bool

	BoughtAt time.Time

	ListID uint `gorm:"index;not null"`
	List   List `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
