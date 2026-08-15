package repository

import (
	"fmt"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/shop/models"
)

type ListItemRepository interface {
	Create(item *models.ListItem) error
	FindByID(id uint) (*models.ListItem, error)
	ListForList(listId uint) ([]*models.ListItem, error)
	Update(item *models.ListItem) error
	Delete(id uint) error
	ClearBought(listID uint) error
}

type lir struct {
	db *core.DB
}

func NewListItemRepository(db *core.DB) ListItemRepository {
	return &lir{db: db}
}

func (r *lir) Create(item *models.ListItem) error {
	result := r.db.Create(item)
	if result.Error != nil {
		return fmt.Errorf("%w: creating list item: %w", core.ErrCreatingEntity, result.Error)
	}

	return nil
}

func (r *lir) FindByID(id uint) (*models.ListItem, error) {
	var item models.ListItem
	result := r.db.First(&item, "id = ?", id)
	if result.Error != nil {
		if core.IsNotFoundError(result.Error) {
			return nil, fmt.Errorf("looking for list item: %w", core.ErrResourceNotFound)
		}
		return nil, fmt.Errorf("looking for list item: %w", result.Error)
	}

	return &item, nil
}

func (r *lir) ListForList(listId uint) ([]*models.ListItem, error) {
	var items []*models.ListItem
	result := r.db.Where("list_id = ?", listId).Order("bought ASC, name ASC").Find(&items)
	if result.Error != nil {
		return nil, fmt.Errorf("looking for list items: %w", result.Error)
	}
	return items, nil
}

func (r *lir) Update(item *models.ListItem) error {
	result := r.db.Save(item)
	if result.Error != nil {
		return fmt.Errorf("updating list item: %w", result.Error)
	}

	return nil
}

func (r *lir) ClearBought(listID uint) error {
	result := r.db.Model(&models.ListItem{}).
		Where("list_id = ? AND bought = ?", listID, true).
		Updates(map[string]any{
			"bought":    false,
			"bought_at": time.Time{},
		})
	if result.Error != nil {
		return fmt.Errorf("clearing bought list items: %w", result.Error)
	}
	return nil
}

func (r *lir) Delete(id uint) error {
	result := r.db.Delete(&models.ListItem{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("deleting list item: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: deleting list item: %w", core.ErrEntityNotFound, result.Error)
	}
	return nil
}
