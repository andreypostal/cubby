package dto

import "time"

type CreateListReq struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
}

type UpdateListReq struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
}

type WorkspaceListsResp struct {
	Lists []ListPayload `json:"lists"`
}

// ListPayload is the public representation of a list
type ListPayload struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Icon      string    `json:"icon"`
	CreatedAt time.Time `json:"created_at"`
}

// ListWithItemsResp is the public representation of a list including the item list
type ListWithItemsResp struct {
	ID    uint               `json:"id"`
	Name  string             `json:"name"`
	Icon  string             `json:"icon"`
	Items []*ListItemPayload `json:"items"`
}

type CreateListItemReq struct {
	Name string `json:"name" validate:"required,max=255"`
}

type UpdateListItemReq struct {
	Name  string  `json:"name" validate:"omitempty,max=255"`
	Notes *string `json:"notes" validate:"omitempty,max=255"`
}

// ListItemPayload is the public representation of a list item
type ListItemPayload struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Notes     string    `json:"notes"`
	Bought    bool      `json:"bought"`
	BoughtAt  time.Time `json:"bought_at"`
	CreatedAt time.Time `json:"created_at"`
}

type ListItemSuggestionReq struct {
	Term string `json:"term"`
}

type ListItemSuggestions struct {
	Terms []string `json:"terms"`
}
