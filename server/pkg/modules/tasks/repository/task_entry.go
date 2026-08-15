package repository

import (
	"errors"
	"fmt"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/tasks/models"
	"gorm.io/gorm"
)

type TaskEntryRepository interface {
	Upsert(entry *models.TaskEntry) error
	FindByTaskAndDay(taskID uint, day time.Time) (*models.TaskEntry, error)
	ListForTasksInRange(taskIDs []uint, from, to time.Time) ([]*models.TaskEntry, error)
	Delete(id uint) error
}

type ter struct {
	db *core.DB
}

func NewTaskEntryRepository(db *core.DB) TaskEntryRepository {
	return &ter{db: db}
}

func (r *ter) Upsert(entry *models.TaskEntry) error {
	existing, err := r.findByTaskAndDayUnscoped(entry.TaskID, entry.Day)
	if err != nil && !errors.Is(err, core.ErrResourceNotFound) {
		return err
	}

	if existing == nil {
		result := r.db.Create(entry)
		if result.Error != nil {
			return fmt.Errorf("%w: creating task entry: %w", core.ErrCreatingEntity, result.Error)
		}
		return nil
	}

	existing.Status = entry.Status
	existing.Note = entry.Note
	// Clearing DeletedAt is what revives a previously cleared day.
	existing.DeletedAt = gorm.DeletedAt{}

	// Unscoped so the soft-deleted row is visible to the UPDATE.
	result := r.db.Unscoped().Save(existing)
	if result.Error != nil {
		return fmt.Errorf("updating task entry: %w", result.Error)
	}

	*entry = *existing
	return nil
}

// FindByTaskAndDay returns the live entry for a day, if the user marked it
func (r *ter) FindByTaskAndDay(taskID uint, day time.Time) (*models.TaskEntry, error) {
	var entry models.TaskEntry

	result := r.db.First(&entry, "task_id = ? AND day = ?", taskID, core.NormalizeDate(day))
	if result.Error != nil {
		if core.IsNotFoundError(result.Error) {
			return nil, fmt.Errorf("looking for task entry: %w", core.ErrResourceNotFound)
		}
		return nil, fmt.Errorf("looking for task entry: %w", result.Error)
	}

	return &entry, nil
}

// findByTaskAndDayUnscoped handles soft deletes
func (r *ter) findByTaskAndDayUnscoped(taskID uint, day time.Time) (*models.TaskEntry, error) {
	var entry models.TaskEntry

	result := r.db.Unscoped().First(&entry, "task_id = ? AND day = ?", taskID, core.NormalizeDate(day))
	if result.Error != nil {
		if core.IsNotFoundError(result.Error) {
			return nil, fmt.Errorf("looking for task entry: %w", core.ErrResourceNotFound)
		}
		return nil, fmt.Errorf("looking for task entry: %w", result.Error)
	}

	return &entry, nil
}

func (r *ter) ListForTasksInRange(taskIDs []uint, from, to time.Time) ([]*models.TaskEntry, error) {
	if len(taskIDs) == 0 {
		return nil, nil
	}

	var entries []*models.TaskEntry
	result := r.db.
		Where("task_id IN ?", taskIDs).
		Where("day >= ? AND day <= ?", core.NormalizeDate(from), core.NormalizeDate(to)).
		Find(&entries)
	if result.Error != nil {
		return nil, fmt.Errorf("looking for task entries: %w", result.Error)
	}

	return entries, nil
}

func (r *ter) Delete(id uint) error {
	result := r.db.Delete(&models.TaskEntry{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("deleting task entry: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: deleting task entry", core.ErrEntityNotFound)
	}
	return nil
}
