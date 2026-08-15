package models

import (
	"time"

	"gorm.io/gorm"

	authModels "andreypostal.com/cubby/server/pkg/modules/auth/models"
)

// Recipe is the aggregate root of the module: it owns its ingredients and its
// steps outright, and only borrows categories.
type Recipe struct {
	gorm.Model

	Name        string `gorm:"type:varchar(150);not null;index"`
	Description string `gorm:"type:varchar(250)"`

	// EstimatedMinutes is how long the whole recipe takes, cooking included.
	EstimatedMinutes int `gorm:"not null;default:0"`

	Favorite bool `gorm:"not null;default:false;index"`

	// ArchivedAt is what takes a recipe out of the default listing without
	// destroying it. Only an archived recipe can be permanently deleted.
	ArchivedAt *time.Time `gorm:"index"`

	WorkspaceID uint                 `gorm:"not null;index"`
	Workspace   authModels.Workspace `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	Ingredients []Ingredient `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Steps       []Step       `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Categories  []Category   `gorm:"many2many:recipe_categories;"`
}

func (r *Recipe) IsArchived() bool {
	return r.ArchivedAt != nil
}

// Ingredient belongs to exactly one recipe and is never shared, so it carries
// free text instead of pointing at a catalog.
type Ingredient struct {
	gorm.Model

	Name     string `gorm:"type:varchar(100);not null"`
	Quantity string `gorm:"type:varchar(50)"`

	// Position is the 0-based rank inside the recipe.
	Position int `gorm:"not null;default:0"`

	RecipeID uint   `gorm:"not null;index"`
	Recipe   Recipe `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// Step is one instruction of the recipe. Description is markdown authored with
// tiptap on the client.
type Step struct {
	gorm.Model

	Title       string `gorm:"type:varchar(250);not null"`
	Description string `gorm:"type:text"`

	// Position is the 0-based rank, which is what makes steps reorderable.
	Position int `gorm:"not null;default:0"`

	RecipeID uint   `gorm:"not null;index"`
	Recipe   Recipe `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
