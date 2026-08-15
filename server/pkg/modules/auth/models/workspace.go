package models

import (
	"gorm.io/gorm"
)

type Workspace struct {
	gorm.Model

	Name string `gorm:"type:varchar(255);not null"`

	CreatorID uint `gorm:"index;not null"`
	Creator   User `gorm:"foreignKey:CreatorID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	// SharedWith are the users the creator granted access to
	SharedWith []User `gorm:"many2many:workspace_shared_users;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
