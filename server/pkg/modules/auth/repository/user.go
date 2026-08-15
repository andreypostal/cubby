package repository

import (
	"fmt"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/auth/models"
)

type UserRepository interface {
	Create(user *models.User) error
	FindByID(id uint) (*models.User, error)
	FindByUsername(username string) (*models.User, error)
	Update(user *models.User) error
	Delete(id uint) error
	List() ([]*models.User, error)

	// Count answers how many users exist
	Count() (int64, error)
	// CountSystemAdmins guards the operations that could leave the server with
	// nobody able to administer it
	CountSystemAdmins() (int64, error)
}

type ur struct {
	db *core.DB
}

func NewUserRepository(db *core.DB) UserRepository {
	return &ur{db: db}
}

func (r *ur) Create(user *models.User) error {
	result := r.db.Create(user)
	if result.Error != nil {
		if core.IsDuplicatedError(result.Error) {
			return fmt.Errorf("creating user: %w", core.ErrDuplicatedEntry)
		}
		return fmt.Errorf("%w: creating user: %w", core.ErrCreatingEntity, result.Error)
	}

	return nil
}

func (r *ur) FindByID(id uint) (*models.User, error) {
	var user models.User
	result := r.db.First(&user, "id = ?", id)
	if result.Error != nil {
		if core.IsNotFoundError(result.Error) {
			return nil, fmt.Errorf("looking for user: %w", core.ErrResourceNotFound)
		}
		return nil, fmt.Errorf("looking for user: %w", result.Error)
	}

	return &user, nil
}

func (r *ur) FindByUsername(username string) (*models.User, error) {
	var user models.User
	result := r.db.First(&user, "username = ?", username)
	if result.Error != nil {
		if core.IsNotFoundError(result.Error) {
			return nil, fmt.Errorf("looking for username: %w", core.ErrResourceNotFound)
		}
		return nil, fmt.Errorf("looking for username: %w", result.Error)
	}

	return &user, nil
}

func (r *ur) Update(user *models.User) error {
	result := r.db.Save(user)
	if result.Error != nil {
		return fmt.Errorf("updating user: %w", result.Error)
	}

	return nil
}

func (r *ur) Delete(id uint) error {
	result := r.db.Delete(&models.User{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("deleting user: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("deleting user: %w", core.ErrResourceNotFound)
	}

	return nil
}

func (r *ur) List() ([]*models.User, error) {
	var users []*models.User
	result := r.db.Order("users.name ASC").Find(&users)
	if result.Error != nil {
		return nil, fmt.Errorf("listing users: %w", result.Error)
	}

	return users, nil
}

func (r *ur) Count() (int64, error) {
	var count int64
	result := r.db.Model(&models.User{}).Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("counting users: %w", result.Error)
	}

	return count, nil
}

func (r *ur) CountSystemAdmins() (int64, error) {
	var count int64
	result := r.db.Model(&models.User{}).Where("is_system_admin = ?", true).Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("counting system admins: %w", result.Error)
	}

	return count, nil
}
