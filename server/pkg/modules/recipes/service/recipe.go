package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/recipes/dto"
	"andreypostal.com/cubby/server/pkg/modules/recipes/models"
	"andreypostal.com/cubby/server/pkg/modules/recipes/repository"
)

const (
	// DefaultPerPage is what a listing returns when the client says nothing.
	DefaultPerPage = 12
	// MaxPerPage caps the page size so a single request cannot pull the whole
	// workspace.
	MaxPerPage = 60
)

// ListQuery is the raw, still unvalidated listing request.
type ListQuery struct {
	Search      string
	CategoryIDs []uint
	// Archived is one of "", "exclude", "only", "include".
	Archived      string
	FavoritesOnly bool
	Page          int
	PerPage       int
}

type RecipeService interface {
	List(workspaceID uint, query ListQuery) (*dto.RecipeListResp, error)
	Create(workspaceID uint, req *dto.CreateRecipeReq) (*dto.RecipeDetailPayload, error)
	Get(recipeID, workspaceID uint) (*dto.RecipeDetailPayload, error)
	Update(recipeID, workspaceID uint, req *dto.UpdateRecipeReq) (*dto.RecipeDetailPayload, error)
	SetFavorite(recipeID, workspaceID uint, favorite bool) (*dto.RecipeDetailPayload, error)
	Archive(recipeID, workspaceID uint) (*dto.RecipeDetailPayload, error)
	Unarchive(recipeID, workspaceID uint) (*dto.RecipeDetailPayload, error)
	ReorderSteps(recipeID, workspaceID uint, req *dto.ReorderStepsReq) (*dto.RecipeDetailPayload, error)
	Delete(recipeID, workspaceID uint) error
	Suggest(workspaceID uint, term string) (*dto.IngredientsSuggestions, error)
}

type rs struct {
	recipeRepo         repository.RecipeRepository
	categoryRepo       repository.CategoryRepository
	ingredientsIndexer *core.Indexer
	suggestTimeout     time.Duration
}

func NewRecipeService(
	recipeRepo repository.RecipeRepository,
	categoryRepo repository.CategoryRepository,
	ingredientsIndexer *core.Indexer,
	suggestTimeout time.Duration,
) RecipeService {
	return &rs{
		recipeRepo:         recipeRepo,
		categoryRepo:       categoryRepo,
		ingredientsIndexer: ingredientsIndexer,
		suggestTimeout:     suggestTimeout,
	}
}

func (s *rs) Suggest(workspaceID uint, term string) (*dto.IngredientsSuggestions, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.suggestTimeout)
	defer cancel()

	terms, err := s.ingredientsIndexer.Query(ctx, workspaceID, term)
	if err != nil {
		slog.Warn("Failed to query for ingredients suggestions", slog.String("error", err.Error()))
		return nil, errors.New("failed to find ingredients suggestion")
	}

	return &dto.IngredientsSuggestions{Terms: terms}, nil
}

func (s *rs) List(workspaceID uint, query ListQuery) (*dto.RecipeListResp, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}

	perPage := query.PerPage
	if perPage < 1 {
		perPage = DefaultPerPage
	}
	if perPage > MaxPerPage {
		perPage = MaxPerPage
	}

	var archived repository.ArchivedFilter
	switch query.Archived {
	case "", "exclude":
		archived = repository.ArchivedExclude
	case "only":
		archived = repository.ArchivedOnly
	case "include":
		archived = repository.ArchivedInclude
	default:
		return nil, fmt.Errorf("%w: archived must be one of exclude, only or include", core.ErrBadRequest)
	}

	recipes, total, err := s.recipeRepo.List(repository.ListFilter{
		WorkspaceID:   workspaceID,
		Search:        query.Search,
		CategoryIDs:   query.CategoryIDs,
		Archived:      archived,
		FavoritesOnly: query.FavoritesOnly,
		Page:          page,
		PerPage:       perPage,
	})
	if err != nil {
		return nil, err
	}

	ids := make([]uint, len(recipes))
	for i, r := range recipes {
		ids[i] = r.ID
	}

	ingredientCounts, stepCounts, err := s.recipeRepo.ContentCounts(ids)
	if err != nil {
		return nil, err
	}

	out := make([]dto.RecipePayload, len(recipes))
	for i, r := range recipes {
		out[i] = *toRecipePayload(r, ingredientCounts[r.ID], stepCounts[r.ID])
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(perPage) - 1) / int64(perPage))
	}

	return &dto.RecipeListResp{
		Recipes: out,
		Pagination: dto.PaginationPayload{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}

func (s *rs) Create(workspaceID uint, req *dto.CreateRecipeReq) (*dto.RecipeDetailPayload, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("%w: recipe name is required", core.ErrBadRequest)
	}

	categories, err := s.resolveCategories(workspaceID, req.CategoryIDs)
	if err != nil {
		return nil, err
	}

	recipe := &models.Recipe{
		Name:             name,
		Description:      strings.TrimSpace(req.Description),
		EstimatedMinutes: req.EstimatedMinutes,
		Favorite:         req.Favorite,
		WorkspaceID:      workspaceID,
		Ingredients:      buildIngredients(req.Ingredients),
		Steps:            buildSteps(req.Steps),
		Categories:       categories,
	}

	if err := s.recipeRepo.Create(recipe); err != nil {
		return nil, err
	}

	for _, ing := range recipe.Ingredients {
		if err := s.ingredientsIndexer.Insert(workspaceID, ing.Name); err != nil {
			slog.Warn("Failed to insert ingredient to indexer", slog.String("name", ing.Name), slog.String("err", err.Error()))
		}
	}

	return s.Get(recipe.ID, workspaceID)
}

func (s *rs) Get(recipeID, workspaceID uint) (*dto.RecipeDetailPayload, error) {
	recipe, err := s.loadOwnedDetail(recipeID, workspaceID)
	if err != nil {
		return nil, err
	}
	return toRecipeDetailPayload(recipe), nil
}

func (s *rs) Update(recipeID, workspaceID uint, req *dto.UpdateRecipeReq) (*dto.RecipeDetailPayload, error) {
	recipe, err := s.loadOwned(recipeID, workspaceID)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("%w: recipe name is required", core.ErrBadRequest)
	}

	categories, err := s.resolveCategories(workspaceID, req.CategoryIDs)
	if err != nil {
		return nil, err
	}

	recipe.Name = name
	recipe.Description = strings.TrimSpace(req.Description)
	recipe.EstimatedMinutes = req.EstimatedMinutes
	recipe.Favorite = req.Favorite

	if err := s.recipeRepo.ReplaceContent(
		recipe,
		buildIngredients(req.Ingredients),
		buildSteps(req.Steps),
		categories,
	); err != nil {
		return nil, err
	}

	for _, ing := range recipe.Ingredients {
		if err := s.ingredientsIndexer.Insert(workspaceID, ing.Name); err != nil {
			slog.Warn("Failed to insert ingredient to indexer", slog.String("name", ing.Name), slog.String("err", err.Error()))
		}
	}

	return s.Get(recipe.ID, workspaceID)
}

func (s *rs) SetFavorite(recipeID, workspaceID uint, favorite bool) (*dto.RecipeDetailPayload, error) {
	recipe, err := s.loadOwned(recipeID, workspaceID)
	if err != nil {
		return nil, err
	}

	recipe.Favorite = favorite
	if err := s.recipeRepo.Update(recipe); err != nil {
		return nil, err
	}

	return s.Get(recipe.ID, workspaceID)
}

func (s *rs) Archive(recipeID, workspaceID uint) (*dto.RecipeDetailPayload, error) {
	recipe, err := s.loadOwned(recipeID, workspaceID)
	if err != nil {
		return nil, err
	}

	if recipe.IsArchived() {
		return nil, fmt.Errorf("%w: recipe is already archived", core.ErrBadRequest)
	}

	now := time.Now().UTC()
	recipe.ArchivedAt = &now
	if err := s.recipeRepo.Update(recipe); err != nil {
		return nil, err
	}

	return s.Get(recipe.ID, workspaceID)
}

func (s *rs) Unarchive(recipeID, workspaceID uint) (*dto.RecipeDetailPayload, error) {
	recipe, err := s.loadOwned(recipeID, workspaceID)
	if err != nil {
		return nil, err
	}

	if !recipe.IsArchived() {
		return nil, fmt.Errorf("%w: recipe is not archived", core.ErrBadRequest)
	}

	recipe.ArchivedAt = nil
	if err := s.recipeRepo.Update(recipe); err != nil {
		return nil, err
	}

	return s.Get(recipe.ID, workspaceID)
}

// ReorderSteps takes the complete, ordered list of step ids. Requiring all of
// them is what stops a stale client from silently dropping a step's rank.
func (s *rs) ReorderSteps(recipeID, workspaceID uint, req *dto.ReorderStepsReq) (*dto.RecipeDetailPayload, error) {
	recipe, err := s.loadOwned(recipeID, workspaceID)
	if err != nil {
		return nil, err
	}

	seen := make(map[uint]bool, len(req.StepIDs))
	for _, id := range req.StepIDs {
		if seen[id] {
			return nil, fmt.Errorf("%w: step %d appears more than once", core.ErrBadRequest, id)
		}
		seen[id] = true
	}

	total, err := s.recipeRepo.CountSteps(recipe.ID)
	if err != nil {
		return nil, err
	}
	if int(total) != len(req.StepIDs) {
		return nil, fmt.Errorf(
			"%w: expected all %d step ids, got %d", core.ErrBadRequest, total, len(req.StepIDs),
		)
	}

	if err := s.recipeRepo.ReorderSteps(recipe.ID, req.StepIDs); err != nil {
		return nil, err
	}

	return s.Get(recipe.ID, workspaceID)
}

// Delete is permanent and only allowed once the recipe is archived, so a
// destructive action always takes two deliberate steps.
func (s *rs) Delete(recipeID, workspaceID uint) error {
	recipe, err := s.loadOwned(recipeID, workspaceID)
	if err != nil {
		return err
	}

	if !recipe.IsArchived() {
		return fmt.Errorf("%w: archive the recipe before deleting it", core.ErrBadRequest)
	}

	return s.recipeRepo.Delete(recipe.ID)
}

// resolveCategories turns the ids the client sent into rows, rejecting any id
// the workspace does not own instead of quietly skipping it.
func (s *rs) resolveCategories(workspaceID uint, ids []uint) ([]models.Category, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	unique := make([]uint, 0, len(ids))
	seen := make(map[uint]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		unique = append(unique, id)
	}

	categories, err := s.categoryRepo.FindManyForWorkspace(workspaceID, unique)
	if err != nil {
		return nil, err
	}

	if len(categories) != len(unique) {
		return nil, fmt.Errorf("%w: one or more categories do not exist in this workspace", core.ErrBadRequest)
	}

	return categories, nil
}

func (s *rs) loadOwned(recipeID, workspaceID uint) (*models.Recipe, error) {
	recipe, err := s.recipeRepo.FindByID(recipeID)
	if err != nil {
		return nil, normalizeError(err)
	}

	if recipe.WorkspaceID != workspaceID {
		return nil, core.ErrNotAllowed
	}

	return recipe, nil
}

func (s *rs) loadOwnedDetail(recipeID, workspaceID uint) (*models.Recipe, error) {
	recipe, err := s.recipeRepo.FindDetailByID(recipeID)
	if err != nil {
		return nil, normalizeError(err)
	}

	if recipe.WorkspaceID != workspaceID {
		return nil, core.ErrNotAllowed
	}

	return recipe, nil
}

// buildIngredients drops blank lines, which is what lets the form keep an empty
// row at the bottom without polluting the saved recipe.
func buildIngredients(reqs []dto.IngredientReq) []models.Ingredient {
	out := make([]models.Ingredient, 0, len(reqs))
	for _, r := range reqs {
		name := strings.TrimSpace(r.Name)
		if name == "" {
			continue
		}
		out = append(out, models.Ingredient{
			Name:     name,
			Quantity: strings.TrimSpace(r.Quantity),
			Position: len(out),
		})
	}
	return out
}

func buildSteps(reqs []dto.StepReq) []models.Step {
	out := make([]models.Step, 0, len(reqs))
	for _, r := range reqs {
		title := strings.TrimSpace(r.Title)
		if title == "" {
			continue
		}
		out = append(out, models.Step{
			Title:       title,
			Description: r.Description,
			Position:    len(out),
		})
	}
	return out
}

func toRecipePayload(recipe *models.Recipe, ingredientCount, stepCount int) *dto.RecipePayload {
	categories := make([]dto.CategoryPayload, len(recipe.Categories))
	for i := range recipe.Categories {
		categories[i] = *toCategoryPayload(&recipe.Categories[i], 0)
	}

	return &dto.RecipePayload{
		ID:               recipe.ID,
		Name:             recipe.Name,
		Description:      recipe.Description,
		EstimatedMinutes: recipe.EstimatedMinutes,
		Favorite:         recipe.Favorite,
		Archived:         recipe.IsArchived(),
		ArchivedAt:       recipe.ArchivedAt,
		Categories:       categories,
		IngredientCount:  ingredientCount,
		StepCount:        stepCount,
		CreatedAt:        recipe.CreatedAt,
		UpdatedAt:        recipe.UpdatedAt,
	}
}

func toRecipeDetailPayload(recipe *models.Recipe) *dto.RecipeDetailPayload {
	ingredients := make([]dto.IngredientPayload, len(recipe.Ingredients))
	for i, ing := range recipe.Ingredients {
		ingredients[i] = dto.IngredientPayload{
			ID:       ing.ID,
			Name:     ing.Name,
			Quantity: ing.Quantity,
			Position: ing.Position,
		}
	}

	steps := make([]dto.StepPayload, len(recipe.Steps))
	for i, st := range recipe.Steps {
		steps[i] = dto.StepPayload{
			ID:          st.ID,
			Title:       st.Title,
			Description: st.Description,
			Position:    st.Position,
		}
	}

	return &dto.RecipeDetailPayload{
		RecipePayload: *toRecipePayload(recipe, len(ingredients), len(steps)),
		Ingredients:   ingredients,
		Steps:         steps,
	}
}
