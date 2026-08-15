package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/recipes/dto"
	"andreypostal.com/cubby/server/pkg/modules/recipes/service"
	"andreypostal.com/cubby/server/pkg/modules/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type RecipeHandler struct {
	recipeService service.RecipeService
	validate      *validator.Validate
}

func NewRecipeHandler(recipeService service.RecipeService, validate *validator.Validate) *RecipeHandler {
	return &RecipeHandler{
		recipeService: recipeService,
		validate:      validate,
	}
}

func (h *RecipeHandler) SuggestIngredientName(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	var req dto.IngredientsSuggestionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		core.RespondError(c, core.ValidationError(err))
		return
	}

	resp, err := h.recipeService.Suggest(profile.Workspace.ID, req.Term)
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusInternalServerError, "failed to query ingredient suggestions", nil))
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *RecipeHandler) List(c *gin.Context) {
	logger := middleware.GetLogger(c)

	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	categoryIDs, err := parseCategoryIDs(c)
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid category_id", nil))
		return
	}

	query := service.ListQuery{
		Search:        c.Query("search"),
		CategoryIDs:   categoryIDs,
		Archived:      c.Query("archived"),
		FavoritesOnly: c.Query("favorite") == "true",
		Page:          parseIntQuery(c, "page"),
		PerPage:       parseIntQuery(c, "per_page"),
	}

	resp, err := h.recipeService.List(profile.Workspace.ID, query)
	if err != nil {
		logger.Warn("Failed to fetch recipes for workspace", slog.String("error", err.Error()))
		respondServiceError(c, "failed to retrieve recipes from workspace", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *RecipeHandler) Create(c *gin.Context) {
	logger := middleware.GetLogger(c)

	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	var req dto.CreateRecipeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		core.RespondError(c, core.ValidationError(err))
		return
	}

	resp, err := h.recipeService.Create(profile.Workspace.ID, &req)
	if err != nil {
		logger.Warn("error while creating new recipe", slog.String("reason", err.Error()))
		respondServiceError(c, "error while creating new recipe", err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *RecipeHandler) Get(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	recipeID, err := core.FromStringToUint(c.Param("recipeID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid recipe id", nil))
		return
	}

	resp, err := h.recipeService.Get(recipeID, profile.Workspace.ID)
	if err != nil {
		respondServiceError(c, "failed to fetch recipe", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *RecipeHandler) Update(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	recipeID, err := core.FromStringToUint(c.Param("recipeID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid recipe id", nil))
		return
	}

	var req dto.UpdateRecipeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		core.RespondError(c, core.ValidationError(err))
		return
	}

	resp, err := h.recipeService.Update(recipeID, profile.Workspace.ID, &req)
	if err != nil {
		respondServiceError(c, "failed to update recipe", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *RecipeHandler) SetFavorite(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	recipeID, err := core.FromStringToUint(c.Param("recipeID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid recipe id", nil))
		return
	}

	var req dto.SetFavoriteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	resp, err := h.recipeService.SetFavorite(recipeID, profile.Workspace.ID, req.Favorite)
	if err != nil {
		respondServiceError(c, "failed to update recipe favorite", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *RecipeHandler) Archive(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	recipeID, err := core.FromStringToUint(c.Param("recipeID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid recipe id", nil))
		return
	}

	resp, err := h.recipeService.Archive(recipeID, profile.Workspace.ID)
	if err != nil {
		respondServiceError(c, "failed to archive recipe", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *RecipeHandler) Unarchive(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	recipeID, err := core.FromStringToUint(c.Param("recipeID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid recipe id", nil))
		return
	}

	resp, err := h.recipeService.Unarchive(recipeID, profile.Workspace.ID)
	if err != nil {
		respondServiceError(c, "failed to unarchive recipe", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *RecipeHandler) ReorderSteps(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	recipeID, err := core.FromStringToUint(c.Param("recipeID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid recipe id", nil))
		return
	}

	var req dto.ReorderStepsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		core.RespondError(c, core.ValidationError(err))
		return
	}

	resp, err := h.recipeService.ReorderSteps(recipeID, profile.Workspace.ID, &req)
	if err != nil {
		respondServiceError(c, "failed to reorder recipe steps", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *RecipeHandler) Delete(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	recipeID, err := core.FromStringToUint(c.Param("recipeID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid recipe id", nil))
		return
	}

	if err := h.recipeService.Delete(recipeID, profile.Workspace.ID); err != nil {
		respondServiceError(c, "failed to remove recipe", err)
		return
	}

	c.Status(http.StatusNoContent)
}

// parseCategoryIDs accepts both repeated `category_id` params and a single
// comma separated one, since query builders disagree on how to send a list.
func parseCategoryIDs(c *gin.Context) ([]uint, error) {
	raw := c.QueryArray("category_id")

	var ids []uint
	for _, entry := range raw {
		for _, part := range strings.Split(entry, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := core.FromStringToUint(part)
			if err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
	}

	return ids, nil
}

// parseIntQuery reads a positive integer, treating anything unreadable as
// absent so a stray query param never turns a listing into an error.
func parseIntQuery(c *gin.Context, key string) int {
	value := c.Query(key)
	if value == "" {
		return 0
	}

	n, err := core.FromStringToUint(value)
	if err != nil {
		return 0
	}

	return int(n)
}
