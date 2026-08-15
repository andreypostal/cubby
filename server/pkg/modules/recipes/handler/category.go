package handler

import (
	"log/slog"
	"net/http"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/recipes/dto"
	"andreypostal.com/cubby/server/pkg/modules/recipes/service"
	"andreypostal.com/cubby/server/pkg/modules/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type CategoryHandler struct {
	categoryService service.CategoryService
	validate        *validator.Validate
}

func NewCategoryHandler(categoryService service.CategoryService, validate *validator.Validate) *CategoryHandler {
	return &CategoryHandler{
		categoryService: categoryService,
		validate:        validate,
	}
}

func (h *CategoryHandler) List(c *gin.Context) {
	logger := middleware.GetLogger(c)

	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	resp, err := h.categoryService.List(profile.Workspace.ID)
	if err != nil {
		logger.Error("Failed to fetch recipe categories", slog.String("error", err.Error()))
		core.RespondError(c, core.NewError(http.StatusInternalServerError, "failed to retrieve recipe categories", nil))
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *CategoryHandler) Create(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	var req dto.CreateCategoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		core.RespondError(c, core.ValidationError(err))
		return
	}

	resp, err := h.categoryService.Create(profile.Workspace.ID, &req)
	if err != nil {
		respondServiceError(c, "failed to create recipe category", err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *CategoryHandler) Update(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	categoryID, err := core.FromStringToUint(c.Param("categoryID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid category id", nil))
		return
	}

	var req dto.UpdateCategoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		core.RespondError(c, core.ValidationError(err))
		return
	}

	resp, err := h.categoryService.Update(categoryID, profile.Workspace.ID, &req)
	if err != nil {
		respondServiceError(c, "failed to update recipe category", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *CategoryHandler) Delete(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	categoryID, err := core.FromStringToUint(c.Param("categoryID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid category id", nil))
		return
	}

	if err := h.categoryService.Delete(categoryID, profile.Workspace.ID); err != nil {
		respondServiceError(c, "failed to remove recipe category", err)
		return
	}

	c.Status(http.StatusNoContent)
}
