package repository

import (
	"fmt"
	"strings"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/recipes/models"
	"gorm.io/gorm"
)

// ArchivedFilter decides which side of the archive a listing looks at.
type ArchivedFilter string

const (
	// ArchivedExclude is the default: only recipes still in use.
	ArchivedExclude ArchivedFilter = "exclude"
	// ArchivedOnly returns exclusively archived recipes.
	ArchivedOnly ArchivedFilter = "only"
	// ArchivedInclude ignores the archive entirely.
	ArchivedInclude ArchivedFilter = "include"
)

// ListFilter is the whole query surface of a recipe listing.
type ListFilter struct {
	WorkspaceID uint

	// Search matches any part of the name, not only its start.
	Search string

	// CategoryIDs keeps recipes assigned to at least one of them.
	CategoryIDs []uint

	Archived ArchivedFilter

	// FavoritesOnly narrows to favorites; favorites already sort first
	// regardless of this flag.
	FavoritesOnly bool

	Page    int
	PerPage int
}

type RecipeRepository interface {
	Create(recipe *models.Recipe) error
	FindByID(id uint) (*models.Recipe, error)
	// FindDetailByID loads the recipe with ingredients and steps already
	// ordered by position.
	FindDetailByID(id uint) (*models.Recipe, error)
	List(filter ListFilter) ([]*models.Recipe, int64, error)
	// ContentCounts returns how many ingredients and how many steps each of
	// the given recipes holds, so a listing never has to load them.
	ContentCounts(recipeIDs []uint) (map[uint]int, map[uint]int, error)
	CountSteps(recipeID uint) (int64, error)
	Update(recipe *models.Recipe) error
	// ReplaceContent swaps ingredients, steps and categories in one
	// transaction, so a failed save never leaves a half-written recipe.
	ReplaceContent(recipe *models.Recipe, ingredients []models.Ingredient, steps []models.Step, categories []models.Category) error
	ReorderSteps(recipeID uint, stepIDs []uint) error
	// Delete removes the recipe and everything it owns for good.
	Delete(id uint) error
}

type rr struct {
	db *core.DB
}

func NewRecipeRepository(db *core.DB) RecipeRepository {
	return &rr{db: db}
}

func (r *rr) Create(recipe *models.Recipe) error {
	// Categories are attached by id, so the association must not try to write
	// them back as new rows.
	result := r.db.Omit("Categories.*").Create(recipe)
	if result.Error != nil {
		return fmt.Errorf("%w: creating recipe: %w", core.ErrCreatingEntity, result.Error)
	}
	return nil
}

func (r *rr) FindByID(id uint) (*models.Recipe, error) {
	var recipe models.Recipe
	result := r.db.Preload("Categories").First(&recipe, "id = ?", id)
	if result.Error != nil {
		if core.IsNotFoundError(result.Error) {
			return nil, fmt.Errorf("looking for recipe: %w", core.ErrResourceNotFound)
		}
		return nil, fmt.Errorf("looking for recipe: %w", result.Error)
	}
	return &recipe, nil
}

func (r *rr) FindDetailByID(id uint) (*models.Recipe, error) {
	var recipe models.Recipe

	result := r.db.
		Preload("Categories", func(db *gorm.DB) *gorm.DB {
			return db.Order("categories.name ASC")
		}).
		Preload("Ingredients", func(db *gorm.DB) *gorm.DB {
			return db.Order("ingredients.position ASC, ingredients.id ASC")
		}).
		Preload("Steps", func(db *gorm.DB) *gorm.DB {
			return db.Order("steps.position ASC, steps.id ASC")
		}).
		First(&recipe, "id = ?", id)
	if result.Error != nil {
		if core.IsNotFoundError(result.Error) {
			return nil, fmt.Errorf("looking for recipe: %w", core.ErrResourceNotFound)
		}
		return nil, fmt.Errorf("looking for recipe: %w", result.Error)
	}

	return &recipe, nil
}

// scope builds the WHERE shared by the count and the page query, so the total
// can never describe a different set than the rows.
func (r *rr) scope(filter ListFilter) *gorm.DB {
	query := r.db.Model(&models.Recipe{}).Where("recipes.workspace_id = ?", filter.WorkspaceID)

	if term := strings.TrimSpace(filter.Search); term != "" {
		// Wrapped in wildcards on both ends: this is a "contains", not a
		// prefix match. LOWER on both sides keeps it case-insensitive
		// independent of the column collation.
		query = query.Where("LOWER(recipes.name) LIKE ?", "%"+strings.ToLower(term)+"%")
	}

	switch filter.Archived {
	case ArchivedOnly:
		query = query.Where("recipes.archived_at IS NOT NULL")
	case ArchivedInclude:
		// Nothing: both sides are wanted.
	default:
		query = query.Where("recipes.archived_at IS NULL")
	}

	if filter.FavoritesOnly {
		query = query.Where("recipes.favorite = ?", true)
	}

	if len(filter.CategoryIDs) > 0 {
		// A recipe assigned to two of the selected categories would come back
		// twice from the join, so the match is asked as a subquery instead.
		query = query.Where(
			"EXISTS (SELECT 1 FROM recipe_categories rc WHERE rc.recipe_id = recipes.id AND rc.category_id IN ?)",
			filter.CategoryIDs,
		)
	}

	return query
}

func (r *rr) List(filter ListFilter) ([]*models.Recipe, int64, error) {
	var total int64
	if err := r.scope(filter).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("counting workspace recipes: %w", err)
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	perPage := filter.PerPage
	if perPage < 1 {
		perPage = 12
	}

	var recipes []*models.Recipe
	result := r.scope(filter).
		Preload("Categories", func(db *gorm.DB) *gorm.DB {
			return db.Order("categories.name ASC")
		}).
		// Favorites first, then alphabetical so the order is stable between
		// two requests for the same page.
		Order("recipes.favorite DESC, recipes.name ASC, recipes.id ASC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&recipes)
	if result.Error != nil {
		return nil, 0, fmt.Errorf("looking for workspace recipes: %w", result.Error)
	}

	return recipes, total, nil
}

func (r *rr) ContentCounts(recipeIDs []uint) (map[uint]int, map[uint]int, error) {
	ingredients := make(map[uint]int)
	steps := make(map[uint]int)

	if len(recipeIDs) == 0 {
		return ingredients, steps, nil
	}

	type row struct {
		RecipeID uint
		Total    int
	}

	count := func(table string, into map[uint]int) error {
		var rows []row
		result := r.db.
			Table(table).
			Select("recipe_id, COUNT(*) AS total").
			Where("recipe_id IN ?", recipeIDs).
			Where("deleted_at IS NULL").
			Group("recipe_id").
			Scan(&rows)
		if result.Error != nil {
			return fmt.Errorf("counting %s: %w", table, result.Error)
		}
		for _, r := range rows {
			into[r.RecipeID] = r.Total
		}
		return nil
	}

	if err := count("ingredients", ingredients); err != nil {
		return nil, nil, err
	}
	if err := count("steps", steps); err != nil {
		return nil, nil, err
	}

	return ingredients, steps, nil
}

func (r *rr) Update(recipe *models.Recipe) error {
	// Only the scalar columns: the associations are handled by ReplaceContent.
	result := r.db.Omit("Ingredients", "Steps", "Categories").Save(recipe)
	if result.Error != nil {
		return fmt.Errorf("updating recipe: %w", result.Error)
	}
	return nil
}

func (r *rr) ReplaceContent(
	recipe *models.Recipe,
	ingredients []models.Ingredient,
	steps []models.Step,
	categories []models.Category,
) error {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Ingredients", "Steps", "Categories").Save(recipe).Error; err != nil {
			return fmt.Errorf("updating recipe: %w", err)
		}

		// Hard delete: an ingredient or a step only exists as part of its
		// recipe, so a soft-deleted leftover would be unreachable dead weight.
		if err := tx.Unscoped().Where("recipe_id = ?", recipe.ID).Delete(&models.Ingredient{}).Error; err != nil {
			return fmt.Errorf("clearing recipe ingredients: %w", err)
		}
		if err := tx.Unscoped().Where("recipe_id = ?", recipe.ID).Delete(&models.Step{}).Error; err != nil {
			return fmt.Errorf("clearing recipe steps: %w", err)
		}

		if len(ingredients) > 0 {
			for i := range ingredients {
				ingredients[i].RecipeID = recipe.ID
			}
			if err := tx.Create(&ingredients).Error; err != nil {
				return fmt.Errorf("creating recipe ingredients: %w", err)
			}
		}

		if len(steps) > 0 {
			for i := range steps {
				steps[i].RecipeID = recipe.ID
			}
			if err := tx.Create(&steps).Error; err != nil {
				return fmt.Errorf("creating recipe steps: %w", err)
			}
		}

		if err := tx.Model(recipe).Omit("Categories.*").Association("Categories").Replace(categories); err != nil {
			return fmt.Errorf("replacing recipe categories: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	recipe.Ingredients = ingredients
	recipe.Steps = steps
	recipe.Categories = categories

	return nil
}

func (r *rr) ReorderSteps(recipeID uint, stepIDs []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for position, id := range stepIDs {
			result := tx.Model(&models.Step{}).
				Where("id = ? AND recipe_id = ?", id, recipeID).
				Update("position", position)
			if result.Error != nil {
				return fmt.Errorf("reordering recipe steps: %w", result.Error)
			}
			if result.RowsAffected == 0 {
				return fmt.Errorf("%w: step %d does not belong to this recipe", core.ErrBadRequest, id)
			}
		}
		return nil
	})
}

func (r *rr) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM recipe_categories WHERE recipe_id = ?", id).Error; err != nil {
			return fmt.Errorf("detaching recipe categories: %w", err)
		}
		if err := tx.Unscoped().Where("recipe_id = ?", id).Delete(&models.Ingredient{}).Error; err != nil {
			return fmt.Errorf("deleting recipe ingredients: %w", err)
		}
		if err := tx.Unscoped().Where("recipe_id = ?", id).Delete(&models.Step{}).Error; err != nil {
			return fmt.Errorf("deleting recipe steps: %w", err)
		}

		result := tx.Unscoped().Delete(&models.Recipe{}, "id = ?", id)
		if result.Error != nil {
			return fmt.Errorf("deleting recipe: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("%w: deleting recipe", core.ErrEntityNotFound)
		}
		return nil
	})
}

// CountSteps tells how many steps a recipe has, used to reject a partial
// reorder payload.
func (r *rr) CountSteps(recipeID uint) (int64, error) {
	var total int64
	result := r.db.Model(&models.Step{}).Where("recipe_id = ?", recipeID).Count(&total)
	if result.Error != nil {
		return 0, fmt.Errorf("counting recipe steps: %w", result.Error)
	}
	return total, nil
}
