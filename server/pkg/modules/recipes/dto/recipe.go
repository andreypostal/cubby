package dto

import "time"

// IngredientReq is one line of the ingredient list. Order is taken from the
// position in the array, not from a field.
type IngredientReq struct {
	Name     string `json:"name" validate:"required,min=1,max=100"`
	Quantity string `json:"quantity" validate:"max=50"`
}

// StepReq is one instruction. Order is taken from the position in the array,
// which is what makes reordering a plain re-send of the list.
type StepReq struct {
	Title       string `json:"title" validate:"required,min=1,max=250"`
	Description string `json:"description" validate:"max=10000"`
}

type CreateRecipeReq struct {
	Name             string `json:"name" validate:"required,min=2,max=150"`
	Description      string `json:"description" validate:"max=250"`
	EstimatedMinutes int    `json:"estimated_minutes" validate:"min=0,max=100000"`
	Favorite         bool   `json:"favorite"`

	CategoryIDs []uint `json:"category_ids" validate:"max=50,dive,min=1"`

	Ingredients []IngredientReq `json:"ingredients" validate:"max=200,dive"`
	Steps       []StepReq       `json:"steps" validate:"max=200,dive"`
}

// UpdateRecipeReq replaces the whole recipe. Ingredients, steps and categories
// are sent in full: what is not in the payload is removed.
type UpdateRecipeReq struct {
	Name             string `json:"name" validate:"required,min=2,max=150"`
	Description      string `json:"description" validate:"max=250"`
	EstimatedMinutes int    `json:"estimated_minutes" validate:"min=0,max=100000"`
	Favorite         bool   `json:"favorite"`

	CategoryIDs []uint `json:"category_ids" validate:"max=50,dive,min=1"`

	Ingredients []IngredientReq `json:"ingredients" validate:"max=200,dive"`
	Steps       []StepReq       `json:"steps" validate:"max=200,dive"`
}

// ReorderStepsReq carries every step id of the recipe, in the wanted order.
type ReorderStepsReq struct {
	StepIDs []uint `json:"step_ids" validate:"required,min=1,max=200,dive,min=1"`
}

type SetFavoriteReq struct {
	Favorite bool `json:"favorite"`
}

// IngredientPayload is the public representation of an ingredient.
type IngredientPayload struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Quantity string `json:"quantity"`
	Position int    `json:"position"`
}

// StepPayload is the public representation of a step.
type StepPayload struct {
	ID          uint   `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Position    int    `json:"position"`
}

// RecipePayload is the summary used in listings: everything needed to render a
// card, without the ingredients and steps.
type RecipePayload struct {
	ID               uint   `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	EstimatedMinutes int    `json:"estimated_minutes"`
	Favorite         bool   `json:"favorite"`

	Archived   bool       `json:"archived"`
	ArchivedAt *time.Time `json:"archived_at,omitempty"`

	Categories []CategoryPayload `json:"categories"`

	IngredientCount int `json:"ingredient_count"`
	StepCount       int `json:"step_count"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RecipeDetailPayload is the full recipe, ingredients and steps included.
type RecipeDetailPayload struct {
	RecipePayload

	Ingredients []IngredientPayload `json:"ingredients"`
	Steps       []StepPayload       `json:"steps"`
}

type PaginationPayload struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

type RecipeListResp struct {
	Recipes    []RecipePayload   `json:"recipes"`
	Pagination PaginationPayload `json:"pagination"`
}

type IngredientsSuggestionReq struct {
	Term string `json:"term"`
}

type IngredientsSuggestions struct {
	Terms []string `json:"terms"`
}
