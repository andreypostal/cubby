package dto

import "time"

type CreateCategoryReq struct {
	Name string `json:"name" validate:"required,min=2,max=100"`
}

type UpdateCategoryReq struct {
	Name string `json:"name" validate:"required,min=2,max=100"`
}

// CategoryPayload is the public representation of a category.
type CategoryPayload struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	// RecipeCount only counts recipes that are not archived, so the filter bar
	// can show how many results a category would actually bring back.
	RecipeCount int       `json:"recipe_count"`
	CreatedAt   time.Time `json:"created_at"`
}

type CategoryListResp struct {
	Categories []CategoryPayload `json:"categories"`
}
