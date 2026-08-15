package repository

import (
	"fmt"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/routine_goals/models"
)

type RoutineRepository interface {
	Create(routine *models.Routine) error
	FindByID(id uint) (*models.Routine, error)
	ListForWorkspace(workspaceID uint) ([]*models.Routine, error)
	Update(routine *models.Routine) error
	Delete(id uint) error
}

type rr struct {
	db *core.DB
}

func NewRoutineRepository(db *core.DB) RoutineRepository {
	return &rr{db: db}
}

func (r *rr) Create(routine *models.Routine) error {
	result := r.db.Create(routine)
	if result.Error != nil {
		return fmt.Errorf("%w: creating routine: %w", core.ErrCreatingEntity, result.Error)
	}

	return nil
}

func (r *rr) FindByID(id uint) (*models.Routine, error) {
	var routine models.Routine
	result := r.db.First(&routine, "id = ?", id)
	if result.Error != nil {
		if core.IsNotFoundError(result.Error) {
			return nil, fmt.Errorf("looking for routine: %w", core.ErrResourceNotFound)
		}
		return nil, fmt.Errorf("looking for routine: %w", result.Error)
	}

	return &routine, nil
}

func (r *rr) ListForWorkspace(workspaceID uint) ([]*models.Routine, error) {
	var routines []*models.Routine
	result := r.db.Where("workspace_id = ?", workspaceID).
		Order("active DESC, pinned DESC, name ASC").
		Find(&routines)
	if result.Error != nil {
		return nil, fmt.Errorf("looking for workspace routines: %w", result.Error)
	}
	return routines, nil
}

func (r *rr) Update(routine *models.Routine) error {
	result := r.db.Save(routine)
	if result.Error != nil {
		return fmt.Errorf("updating routine: %w", result.Error)
	}

	return nil
}

func (r *rr) Delete(id uint) error {
	result := r.db.Delete(&models.Routine{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("deleting routine: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: deleting routine: %w", core.ErrEntityNotFound, result.Error)
	}
	return nil
}
