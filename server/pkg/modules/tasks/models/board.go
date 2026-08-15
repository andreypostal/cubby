package models

import (
	"gorm.io/gorm"

	authModels "andreypostal.com/cubby/server/pkg/modules/auth/models"
)

// TaskBoard is the single calendar board a workspace owns
// this can be used if at some point we want to allow more than one task board per workspace
type TaskBoard struct {
	gorm.Model

	WorkspaceID uint                 `gorm:"uniqueIndex;not null"`
	Workspace   authModels.Workspace `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	Timezone string `gorm:"varchar(64);not null;default:UTC"`
}
