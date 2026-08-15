package repository

import (
	"fmt"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/tasks/models"
)

type BoardRepository interface {
	GetOrCreateForWorkspace(workspaceID uint) (*models.TaskBoard, error)
	Update(board *models.TaskBoard) error
}

type br struct {
	db *core.DB
}

func NewBoardRepository(db *core.DB) BoardRepository {
	return &br{db: db}
}

func (r *br) GetOrCreateForWorkspace(workspaceID uint) (*models.TaskBoard, error) {
	board := models.TaskBoard{WorkspaceID: workspaceID}

	result := r.db.Where("workspace_id = ?", workspaceID).
		Attrs(models.TaskBoard{Timezone: "UTC"}).
		FirstOrCreate(&board)
	if result.Error != nil {
		return nil, fmt.Errorf("%w: getting task board: %w", core.ErrCreatingEntity, result.Error)
	}

	return &board, nil
}

func (r *br) Update(board *models.TaskBoard) error {
	result := r.db.Save(board)
	if result.Error != nil {
		return fmt.Errorf("updating task board: %w", result.Error)
	}
	return nil
}
