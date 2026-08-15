package repository

import (
	"fmt"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/recipes/models"
)

type CategoryRepository interface {
	Create(category *models.Category) error
	FindByID(id uint) (*models.Category, error)
	FindByName(workspaceID uint, name string) (*models.Category, error)
	ListForWorkspace(workspaceID uint) ([]*models.Category, error)
	// CountActiveRecipes returns, per category id, how many non-archived
	// recipes reference it.
	CountActiveRecipes(workspaceID uint) (map[uint]int, error)
	FindManyForWorkspace(workspaceID uint, ids []uint) ([]models.Category, error)
	Update(category *models.Category) error
	Delete(id uint) error
}

type cr struct {
	db *core.DB
}

func NewCategoryRepository(db *core.DB) CategoryRepository {
	return &cr{db: db}
}

func (r *cr) Create(category *models.Category) error {
	result := r.db.Create(category)
	if result.Error != nil {
		return fmt.Errorf("%w: creating recipe category: %w", core.ErrCreatingEntity, result.Error)
	}
	return nil
}

func (r *cr) FindByID(id uint) (*models.Category, error) {
	var category models.Category
	result := r.db.First(&category, "id = ?", id)
	if result.Error != nil {
		if core.IsNotFoundError(result.Error) {
			return nil, fmt.Errorf("looking for recipe category: %w", core.ErrResourceNotFound)
		}
		return nil, fmt.Errorf("looking for recipe category: %w", result.Error)
	}
	return &category, nil
}

// FindByName is what keeps names unique per workspace, since the comparison has
// to be case-insensitive and the unique index alone would not be.
func (r *cr) FindByName(workspaceID uint, name string) (*models.Category, error) {
	var category models.Category
	result := r.db.
		Where("workspace_id = ?", workspaceID).
		Where("LOWER(name) = LOWER(?)", name).
		First(&category)
	if result.Error != nil {
		if core.IsNotFoundError(result.Error) {
			return nil, fmt.Errorf("looking for recipe category: %w", core.ErrResourceNotFound)
		}
		return nil, fmt.Errorf("looking for recipe category: %w", result.Error)
	}
	return &category, nil
}

func (r *cr) ListForWorkspace(workspaceID uint) ([]*models.Category, error) {
	var categories []*models.Category
	result := r.db.
		Where("workspace_id = ?", workspaceID).
		Order("name ASC").
		Find(&categories)
	if result.Error != nil {
		return nil, fmt.Errorf("looking for workspace recipe categories: %w", result.Error)
	}
	return categories, nil
}

func (r *cr) CountActiveRecipes(workspaceID uint) (map[uint]int, error) {
	type row struct {
		CategoryID uint
		Total      int
	}

	var rows []row
	result := r.db.
		Table("recipe_categories").
		Select("recipe_categories.category_id AS category_id, COUNT(*) AS total").
		Joins("JOIN recipes ON recipes.id = recipe_categories.recipe_id").
		Where("recipes.workspace_id = ?", workspaceID).
		Where("recipes.deleted_at IS NULL").
		Where("recipes.archived_at IS NULL").
		Group("recipe_categories.category_id").
		Scan(&rows)
	if result.Error != nil {
		return nil, fmt.Errorf("counting recipes per category: %w", result.Error)
	}

	counts := make(map[uint]int, len(rows))
	for _, r := range rows {
		counts[r.CategoryID] = r.Total
	}
	return counts, nil
}

// FindManyForWorkspace resolves the ids the client sent, filtered by workspace
// so a recipe can never borrow another workspace's category.
func (r *cr) FindManyForWorkspace(workspaceID uint, ids []uint) ([]models.Category, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var categories []models.Category
	result := r.db.
		Where("workspace_id = ?", workspaceID).
		Where("id IN ?", ids).
		Find(&categories)
	if result.Error != nil {
		return nil, fmt.Errorf("looking for recipe categories: %w", result.Error)
	}
	return categories, nil
}

func (r *cr) Update(category *models.Category) error {
	result := r.db.Save(category)
	if result.Error != nil {
		return fmt.Errorf("updating recipe category: %w", result.Error)
	}
	return nil
}

func (r *cr) Delete(id uint) error {
	// The join rows have to go first, otherwise every recipe would keep
	// pointing at a category that no longer resolves.
	if err := r.db.Exec("DELETE FROM recipe_categories WHERE category_id = ?", id).Error; err != nil {
		return fmt.Errorf("detaching recipes from category: %w", err)
	}

	result := r.db.Delete(&models.Category{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("deleting recipe category: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: deleting recipe category", core.ErrEntityNotFound)
	}
	return nil
}
