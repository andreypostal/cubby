// Package service holds the business rules of the recipes module
package service

import (
	"errors"
	"fmt"
	"strings"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/recipes/dto"
	"andreypostal.com/cubby/server/pkg/modules/recipes/models"
	"andreypostal.com/cubby/server/pkg/modules/recipes/repository"
)

type CategoryService interface {
	List(workspaceID uint) (*dto.CategoryListResp, error)
	Create(workspaceID uint, req *dto.CreateCategoryReq) (*dto.CategoryPayload, error)
	Update(categoryID, workspaceID uint, req *dto.UpdateCategoryReq) (*dto.CategoryPayload, error)
	Delete(categoryID, workspaceID uint) error
}

type cs struct {
	categoryRepo repository.CategoryRepository
}

func NewCategoryService(categoryRepo repository.CategoryRepository) CategoryService {
	return &cs{categoryRepo: categoryRepo}
}

func (s *cs) List(workspaceID uint) (*dto.CategoryListResp, error) {
	categories, err := s.categoryRepo.ListForWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	counts, err := s.categoryRepo.CountActiveRecipes(workspaceID)
	if err != nil {
		return nil, err
	}

	out := make([]dto.CategoryPayload, len(categories))
	for i, c := range categories {
		out[i] = *toCategoryPayload(c, counts[c.ID])
	}

	return &dto.CategoryListResp{Categories: out}, nil
}

func (s *cs) Create(workspaceID uint, req *dto.CreateCategoryReq) (*dto.CategoryPayload, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("%w: category name is required", core.ErrBadRequest)
	}

	if err := s.assertNameFree(workspaceID, name, 0); err != nil {
		return nil, err
	}

	category := &models.Category{Name: name, WorkspaceID: workspaceID}
	if err := s.categoryRepo.Create(category); err != nil {
		return nil, err
	}

	return toCategoryPayload(category, 0), nil
}

func (s *cs) Update(categoryID, workspaceID uint, req *dto.UpdateCategoryReq) (*dto.CategoryPayload, error) {
	category, err := s.loadOwned(categoryID, workspaceID)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("%w: category name is required", core.ErrBadRequest)
	}

	if err := s.assertNameFree(workspaceID, name, category.ID); err != nil {
		return nil, err
	}

	category.Name = name
	if err := s.categoryRepo.Update(category); err != nil {
		return nil, err
	}

	counts, err := s.categoryRepo.CountActiveRecipes(workspaceID)
	if err != nil {
		return nil, err
	}

	return toCategoryPayload(category, counts[category.ID]), nil
}

// Delete drops the category and unassigns it from every recipe. The recipes
// themselves are untouched: a category is a label, not an owner.
func (s *cs) Delete(categoryID, workspaceID uint) error {
	category, err := s.loadOwned(categoryID, workspaceID)
	if err != nil {
		return err
	}
	return s.categoryRepo.Delete(category.ID)
}

func (s *cs) assertNameFree(workspaceID uint, name string, ignoreID uint) error {
	existing, err := s.categoryRepo.FindByName(workspaceID, name)
	if err != nil {
		if errors.Is(err, core.ErrResourceNotFound) {
			return nil
		}
		return err
	}

	if existing.ID == ignoreID {
		return nil
	}

	return fmt.Errorf("%w: a category named %q already exists", core.ErrDuplicatedEntry, name)
}

func (s *cs) loadOwned(categoryID, workspaceID uint) (*models.Category, error) {
	category, err := s.categoryRepo.FindByID(categoryID)
	if err != nil {
		return nil, normalizeError(err)
	}

	if category.WorkspaceID != workspaceID {
		return nil, core.ErrNotAllowed
	}

	return category, nil
}

func toCategoryPayload(category *models.Category, recipeCount int) *dto.CategoryPayload {
	return &dto.CategoryPayload{
		ID:          category.ID,
		Name:        category.Name,
		RecipeCount: recipeCount,
		CreatedAt:   category.CreatedAt,
	}
}

func normalizeError(err error) error {
	if errors.Is(err, core.ErrEntityNotFound) || errors.Is(err, core.ErrResourceNotFound) {
		return core.ErrNotAllowed
	}
	return err
}
