package models

import (
	"gorm.io/gorm"

	authModels "andreypostal.com/cubby/server/pkg/modules/auth/models"
)

// Category groups recipes inside a workspace (desserts, breakfast, lunch...).
// A recipe can carry many of them and a category can be reused by many recipes.
type Category struct {
	gorm.Model

	Name string `gorm:"type:varchar(100);not null;index:idx_recipe_category_workspace_name"`

	WorkspaceID uint                 `gorm:"not null;index:idx_recipe_category_workspace_name"`
	Workspace   authModels.Workspace `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	Recipes []Recipe `gorm:"many2many:recipe_categories;"`
}
