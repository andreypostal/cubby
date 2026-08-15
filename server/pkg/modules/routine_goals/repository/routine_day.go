package repository

import (
	"fmt"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/routine_goals/models"
)

type RoutineDayRepository interface {
	Create(day *models.RoutineDay) error
	FindByRoutineAndDay(routineID uint, day time.Time) (*models.RoutineDay, error)
	ListForRoutineInRange(routineID uint, start, end time.Time) ([]*models.RoutineDay, error)
	Update(day *models.RoutineDay) error
	Delete(id uint) error
}

type rdr struct {
	db *core.DB
}

func NewRoutineDayRepository(db *core.DB) RoutineDayRepository {
	return &rdr{db: db}
}

func (r *rdr) Create(day *models.RoutineDay) error {
	result := r.db.Create(day)
	if result.Error != nil {
		return fmt.Errorf("%w: creating routine day: %w", core.ErrCreatingEntity, result.Error)
	}

	return nil
}

func (r *rdr) FindByRoutineAndDay(routineID uint, day time.Time) (*models.RoutineDay, error) {
	var routineDay models.RoutineDay

	result := r.db.Unscoped().First(&routineDay, "routine_id = ? AND day = ?", routineID, day)
	if result.Error != nil {
		if core.IsNotFoundError(result.Error) {
			return nil, fmt.Errorf("looking for routine day: %w", core.ErrResourceNotFound)
		}
		return nil, fmt.Errorf("looking for routine day: %w", result.Error)
	}

	return &routineDay, nil
}

func (r *rdr) ListForRoutineInRange(routineID uint, start, end time.Time) ([]*models.RoutineDay, error) {
	var days []*models.RoutineDay
	result := r.db.Where("routine_id = ? AND day >= ? AND day < ?", routineID, start, end).Find(&days)
	if result.Error != nil {
		return nil, fmt.Errorf("looking for routine days: %w", result.Error)
	}
	return days, nil
}

func (r *rdr) Update(day *models.RoutineDay) error {
	// Unscoped so restoring a soft-deleted day wont fail
	result := r.db.Unscoped().Save(day)
	if result.Error != nil {
		return fmt.Errorf("updating routine day: %w", result.Error)
	}

	return nil
}

func (r *rdr) Delete(id uint) error {
	result := r.db.Delete(&models.RoutineDay{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("deleting routine day: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: deleting routine day: %w", core.ErrEntityNotFound, result.Error)
	}
	return nil
}
