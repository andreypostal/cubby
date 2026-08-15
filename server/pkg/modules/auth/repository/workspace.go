package repository

import (
	"fmt"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/auth/models"
	"gorm.io/gorm"
)

// WorkspaceRepository manages gorm data access for Workspaces
type WorkspaceRepository interface {
	Create(m *models.Workspace) error
	FindByID(id uint) (*models.Workspace, error)
	Update(m *models.Workspace) error
	Delete(id uint) error
	ListForUser(userID uint) ([]*models.Workspace, error)
	// ListCreatedBy returns only the workspaces the user owns
	ListCreatedBy(userID uint) ([]*models.Workspace, error)

	// HasAccess answers whether the user created the workspace or had it shared with them
	HasAccess(workspaceID, userID uint) (bool, error)
	// Members lists the users the workspace was shared with, the creator aside
	Members(workspaceID uint) ([]*models.User, error)
	AddMember(workspaceID uint, user *models.User) error
	RemoveMember(workspaceID, userID uint) error
	// RemoveMemberEverywhere drops the user from every workspace shared with them
	RemoveMemberEverywhere(userID uint) error
}

type wr struct {
	db *core.DB
}

func NewWorkspaceRepository(db *core.DB) WorkspaceRepository {
	return &wr{db: db}
}

func (r *wr) Create(m *models.Workspace) error {
	result := r.db.Create(m)
	if result.Error != nil {
		return fmt.Errorf("%w: workspace: %w", core.ErrCreatingEntity, result.Error)
	}
	return nil
}

func (r *wr) FindByID(id uint) (*models.Workspace, error) {
	var m models.Workspace
	result := r.db.First(&m, "id = ?", id)
	if result.Error != nil {
		if core.IsNotFoundError(result.Error) {
			return nil, fmt.Errorf("looking for workspace: %w", core.ErrEntityNotFound)
		}
		return nil, fmt.Errorf("looking for workspace: %w", result.Error)
	}
	return &m, nil
}

// ListForUser returns the workspaces the user created and the ones shared with them
func (r *wr) ListForUser(userID uint) ([]*models.Workspace, error) {
	var workspaces []*models.Workspace
	result := r.db.
		Distinct("workspaces.*").
		Joins("LEFT JOIN workspace_shared_users ON workspace_shared_users.workspace_id = workspaces.id").
		Where("workspaces.creator_id = ? OR workspace_shared_users.user_id = ?", userID, userID).
		Order("workspaces.name ASC").
		Find(&workspaces)
	if result.Error != nil {
		return nil, fmt.Errorf("looking for user workspaces: %w", result.Error)
	}
	return workspaces, nil
}

func (r *wr) ListCreatedBy(userID uint) ([]*models.Workspace, error) {
	var workspaces []*models.Workspace
	result := r.db.
		Where("creator_id = ?", userID).
		Order("name ASC").
		Find(&workspaces)
	if result.Error != nil {
		return nil, fmt.Errorf("looking for workspaces created by a user: %w", result.Error)
	}
	return workspaces, nil
}

func (r *wr) HasAccess(workspaceID, userID uint) (bool, error) {
	var count int64
	result := r.db.
		Model(&models.Workspace{}).
		Joins("LEFT JOIN workspace_shared_users ON workspace_shared_users.workspace_id = workspaces.id").
		Where("workspaces.id = ?", workspaceID).
		Where("workspaces.creator_id = ? OR workspace_shared_users.user_id = ?", userID, userID).
		Count(&count)
	if result.Error != nil {
		return false, fmt.Errorf("verifying workspace access: %w", result.Error)
	}
	return count > 0, nil
}

func (r *wr) Members(workspaceID uint) ([]*models.User, error) {
	var users []*models.User
	result := r.db.
		Model(&models.User{}).
		Joins("JOIN workspace_shared_users ON workspace_shared_users.user_id = users.id").
		Where("workspace_shared_users.workspace_id = ?", workspaceID).
		Order("users.name ASC").
		Find(&users)
	if result.Error != nil {
		return nil, fmt.Errorf("looking for workspace members: %w", result.Error)
	}
	return users, nil
}

func (r *wr) AddMember(workspaceID uint, user *models.User) error {
	err := r.db.
		Model(&models.Workspace{Model: gorm.Model{ID: workspaceID}}).
		Association("SharedWith").
		Append(user)
	if err != nil {
		return fmt.Errorf("sharing workspace: %w", err)
	}
	return nil
}

func (r *wr) RemoveMember(workspaceID, userID uint) error {
	err := r.db.
		Model(&models.Workspace{Model: gorm.Model{ID: workspaceID}}).
		Association("SharedWith").
		Delete(&models.User{Model: gorm.Model{ID: userID}})
	if err != nil {
		return fmt.Errorf("revoking workspace access: %w", err)
	}
	return nil
}

func (r *wr) RemoveMemberEverywhere(userID uint) error {
	result := r.db.Exec("DELETE FROM workspace_shared_users WHERE user_id = ?", userID)
	if result.Error != nil {
		return fmt.Errorf("revoking every workspace shared with a user: %w", result.Error)
	}
	return nil
}

func (r *wr) Update(m *models.Workspace) error {
	result := r.db.Save(m)
	if result.Error != nil {
		return fmt.Errorf("updating workspace: %w", result.Error)
	}
	return nil
}

func (r *wr) Delete(id uint) error {
	result := r.db.Delete(&models.Workspace{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("deleting workspace: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: deleting workspace: %w", core.ErrEntityNotFound, result.Error)
	}
	return nil
}
