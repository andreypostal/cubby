package repository

import (
	"fmt"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/shop/models"
)

type ListRepository interface {
	Create(list *models.List) error
	FindByID(id uint) (*models.List, error)
	ListForWorkspace(workspaceID uint) ([]*models.List, error)
	Update(list *models.List) error
	Delete(id uint) error
}

type lr struct {
	db *core.DB
}

func NewListRepository(db *core.DB) ListRepository {
	return &lr{db: db}
}

func (r *lr) Create(list *models.List) error {
	result := r.db.Create(list)
	if result.Error != nil {
		return fmt.Errorf("%w: creating list: %w", core.ErrCreatingEntity, result.Error)
	}

	return nil
}

func (r *lr) FindByID(id uint) (*models.List, error) {
	var list models.List
	result := r.db.First(&list, "id = ?", id)
	if result.Error != nil {
		if core.IsNotFoundError(result.Error) {
			return nil, fmt.Errorf("looking for list: %w", core.ErrResourceNotFound)
		}
		return nil, fmt.Errorf("looking for list: %w", result.Error)
	}

	return &list, nil
}

func (r *lr) ListForWorkspace(workspaceID uint) ([]*models.List, error) {
	var lists []*models.List
	result := r.db.Where("workspace_id = ?", workspaceID).Order("name ASC").Find(&lists)
	if result.Error != nil {
		return nil, fmt.Errorf("looking for workspace lists: %w", result.Error)
	}
	return lists, nil
}

func (r *lr) Update(list *models.List) error {
	result := r.db.Save(list)
	if result.Error != nil {
		return fmt.Errorf("updating list: %w", result.Error)
	}

	return nil
}

func (r *lr) Delete(id uint) error {
	result := r.db.Delete(&models.List{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("deleting list: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: deleting list: %w", core.ErrEntityNotFound, result.Error)
	}
	return nil
}
