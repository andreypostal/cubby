package repository

import (
	"fmt"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/tasks/models"
)

type TaskRepository interface {
	Create(task *models.Task) error
	FindByID(id uint) (*models.Task, error)
	ListForBoard(boardID uint) ([]*models.Task, error)
	ListActiveInRange(boardID uint, from, to time.Time) ([]*models.Task, error)
	ListChain(rootID uint) ([]*models.Task, error)
	Update(task *models.Task) error
	Delete(id uint) error
}

type tr struct {
	db *core.DB
}

func NewTaskRepository(db *core.DB) TaskRepository {
	return &tr{db: db}
}

func (r *tr) Create(task *models.Task) error {
	result := r.db.Create(task)
	if result.Error != nil {
		return fmt.Errorf("%w: creating task: %w", core.ErrCreatingEntity, result.Error)
	}
	return nil
}

func (r *tr) FindByID(id uint) (*models.Task, error) {
	var task models.Task
	result := r.db.First(&task, "id = ?", id)
	if result.Error != nil {
		if core.IsNotFoundError(result.Error) {
			return nil, fmt.Errorf("looking for task: %w", core.ErrResourceNotFound)
		}
		return nil, fmt.Errorf("looking for task: %w", result.Error)
	}
	return &task, nil
}

// ListForBoard returns every task of a board, stopped ones included. The
// overview needs the whole set anyway: a stopped run only becomes meaningful
// next to the sibling that may have already taken over from it.
func (r *tr) ListForBoard(boardID uint) ([]*models.Task, error) {
	var tasks []*models.Task

	result := r.db.
		Where("board_id = ?", boardID).
		Order("pinned DESC, name ASC").
		Find(&tasks)
	if result.Error != nil {
		return nil, fmt.Errorf("looking for board tasks: %w", result.Error)
	}
	return tasks, nil
}

func (r *tr) ListActiveInRange(boardID uint, from, to time.Time) ([]*models.Task, error) {
	var tasks []*models.Task

	result := r.db.
		Where("board_id = ?", boardID).
		Where("starts_on <= ?", to).
		// Explicit parentheses: this OR must not fuse with the conditions above.
		Where("(ends_on IS NULL OR ends_on >= ?)", from).
		Order("pinned DESC, name ASC").
		Find(&tasks)
	if result.Error != nil {
		return nil, fmt.Errorf("looking for active tasks: %w", result.Error)
	}
	return tasks, nil
}

func (r *tr) ListChain(rootID uint) ([]*models.Task, error) {
	var tasks []*models.Task

	result := r.db.
		Where("id = ? OR origin_task_id = ?", rootID, rootID).
		Order("starts_on ASC").
		Find(&tasks)
	if result.Error != nil {
		return nil, fmt.Errorf("looking for task chain: %w", result.Error)
	}
	return tasks, nil
}

func (r *tr) Update(task *models.Task) error {
	result := r.db.Save(task)
	if result.Error != nil {
		return fmt.Errorf("updating task: %w", result.Error)
	}
	return nil
}

func (r *tr) Delete(id uint) error {
	result := r.db.Delete(&models.Task{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("deleting task: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: deleting task", core.ErrEntityNotFound)
	}
	return nil
}
