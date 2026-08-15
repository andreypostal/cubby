package handler

import (
	"log/slog"
	"net/http"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/server/middleware"
	"andreypostal.com/cubby/server/pkg/modules/shop/dto"
	"andreypostal.com/cubby/server/pkg/modules/shop/service"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// ListHandler handles organization and membership routes.
type ListHandler struct {
	listService  service.ListService
	itemsService service.ListItemService
	validate     *validator.Validate
}

// NewListHandler returns an ListHandler.
func NewListHandler(listService service.ListService, itemsService service.ListItemService, validate *validator.Validate) *ListHandler {
	return &ListHandler{
		listService:  listService,
		itemsService: itemsService,
		validate:     validate,
	}
}

// List godoc
// GET /api/v1/workspaces
func (h *ListHandler) SuggestItemName(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	listID, err := core.FromStringToUint(c.Param("listID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid list id", nil))
	}

	var req dto.ListItemSuggestionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		core.RespondError(c, core.ValidationError(err))
		return
	}

	if !h.verifyWorkspaceOwnership(listID, profile.Workspace.ID) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	resp, err := h.itemsService.Suggest(profile.Workspace.ID, req.Term)
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusInternalServerError, "failed to query item name suggestions", nil))
		return
	}

	c.JSON(http.StatusOK, resp)
}

// List godoc
// GET /api/v1/workspaces
func (h *ListHandler) List(c *gin.Context) {
	logger := middleware.GetLogger(c)

	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	resp, err := h.listService.ListForWorkspace(profile.Workspace.ID)
	if err != nil {
		logger.Error("Failed to fetch list for workspace", slog.String("error", err.Error()))
		core.RespondError(c, core.NewError(http.StatusInternalServerError, "faile to retrieve lists from workspace", nil))
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Create godoc
// POST /api/v1/workspaces
func (h *ListHandler) CreateList(c *gin.Context) {
	logger := middleware.GetLogger(c)

	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	var req dto.CreateListReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		core.RespondError(c, core.ValidationError(err))
		return
	}

	resp, err := h.listService.Create(profile.Workspace.ID, &req)
	if err != nil {
		logger.Warn("error while creating new list", slog.String("reason", err.Error()))
		core.RespondError(c, core.NewError(http.StatusBadRequest, "error while creating new list", nil))
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *ListHandler) CreateItem(c *gin.Context) {
	logger := middleware.GetLogger(c)

	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	listID, err := core.FromStringToUint(c.Param("listID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid list id", nil))
	}

	var req dto.CreateListItemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		core.RespondError(c, core.ValidationError(err))
		return
	}

	if !h.verifyWorkspaceOwnership(listID, profile.Workspace.ID) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	resp, err := h.itemsService.Create(profile.Workspace.ID, listID, &req)
	if err != nil {
		logger.Warn("error while creating new list item", slog.String("reason", err.Error()))
		core.RespondError(c, core.NewError(http.StatusBadRequest, "error while creating new list item", nil))
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// Get godoc
// GET /api/v1/workspaces/:workspaceID
func (h *ListHandler) GetList(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	listID, err := core.FromStringToUint(c.Param("listID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid list id", nil))
	}

	resp, err := h.listService.Get(listID, profile.Workspace.ID)
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusForbidden, "failed to fetch list", nil))
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *ListHandler) GetItem(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	listID, err := core.FromStringToUint(c.Param("listID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid list id", nil))
	}

	itemID, err := core.FromStringToUint(c.Param("itemID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid item id", nil))
	}

	if !h.verifyWorkspaceOwnership(listID, profile.Workspace.ID) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	resp, err := h.itemsService.Get(itemID, listID)
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusForbidden, "failed to fetch list item", nil))
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Update godoc
// PATCH /api/v1/orgs/:orgID
func (h *ListHandler) UpdateList(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	listID, err := core.FromStringToUint(c.Param("listID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid list id", nil))
	}

	var req dto.UpdateListReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		core.RespondError(c, core.ValidationError(err))
		return
	}

	resp, err := h.listService.Update(listID, profile.Workspace.ID, &req)
	if err != nil {
		coreErr := core.NewError(http.StatusBadRequest, "failed to update list", map[string]any{
			"reason": err.Error(),
		})
		core.RespondError(c, coreErr)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *ListHandler) UpdateItem(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	listID, err := core.FromStringToUint(c.Param("listID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid list id", nil))
	}

	itemID, err := core.FromStringToUint(c.Param("itemID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid item id", nil))
	}

	var req dto.UpdateListItemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		core.RespondError(c, core.ValidationError(err))
		return
	}

	if !h.verifyWorkspaceOwnership(listID, profile.Workspace.ID) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	resp, err := h.itemsService.Update(itemID, listID, &req)
	if err != nil {
		coreErr := core.NewError(http.StatusBadRequest, "failed to update list item", map[string]any{
			"reason": err.Error(),
		})
		core.RespondError(c, coreErr)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Delete godoc
// DELETE /api/v1/orgs/:orgID
func (h *ListHandler) DeleteList(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	listID, err := core.FromStringToUint(c.Param("listID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid list id", nil))
	}

	if err := h.listService.Delete(listID, profile.Workspace.ID); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "failed to remove list", nil))
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *ListHandler) DeleteItem(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	listID, err := core.FromStringToUint(c.Param("listID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid list id", nil))
	}

	itemID, err := core.FromStringToUint(c.Param("itemID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid item id", nil))
	}

	if !h.verifyWorkspaceOwnership(listID, profile.Workspace.ID) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	if err := h.itemsService.Delete(itemID, listID); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "failed to remove list item", nil))
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *ListHandler) ToggleItem(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	listID, err := core.FromStringToUint(c.Param("listID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid list id", nil))
	}

	itemID, err := core.FromStringToUint(c.Param("itemID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid item id", nil))
	}

	if !h.verifyWorkspaceOwnership(listID, profile.Workspace.ID) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	resp, err := h.itemsService.ToggleBought(itemID, listID)
	if err != nil {
		coreErr := core.NewError(http.StatusBadRequest, "failed to toggle list item", map[string]any{
			"reason": err.Error(),
		})
		core.RespondError(c, coreErr)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *ListHandler) ClearBoughtItems(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	listID, err := core.FromStringToUint(c.Param("listID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid list id", nil))
		return
	}

	if !h.verifyWorkspaceOwnership(listID, profile.Workspace.ID) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	resp, err := h.itemsService.ClearBought(listID)
	if err != nil {
		coreErr := core.NewError(http.StatusBadRequest, "failed to clear bought list items", map[string]any{
			"reason": err.Error(),
		})
		core.RespondError(c, coreErr)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *ListHandler) verifyWorkspaceOwnership(listID, workspaceID uint) bool {
	_, err := h.listService.Get(listID, workspaceID)
	return err == nil
}
