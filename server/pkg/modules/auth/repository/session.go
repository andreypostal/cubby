package repository

import (
	"fmt"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/auth/models"
)

type SessionRepository interface {
	Create(session *models.Session) error
	FindByCode(code string) (*models.Session, error)
	Update(session *models.Session) error
	Delete(code string) error
	DeleteExpired(before time.Time) (int64, error)
	DeleteForUser(userID uint) (int64, error)
}

type sr struct {
	db *core.DB
}

func NewSessionRepository(db *core.DB) SessionRepository {
	return &sr{db: db}
}

func (r *sr) Create(session *models.Session) error {
	result := r.db.Create(session)
	if result.Error != nil {
		if core.IsDuplicatedError(result.Error) {
			return fmt.Errorf("creating session: %w", core.ErrDuplicatedEntry)
		}
		return fmt.Errorf("%w: creating session: %w", core.ErrCreatingEntity, result.Error)
	}

	return nil
}

func (r *sr) FindByCode(code string) (*models.Session, error) {
	var session models.Session
	result := r.db.Joins("User").Preload("Workspace").First(&session, "sessions.code = ?", code)
	if result.Error != nil {
		if core.IsNotFoundError(result.Error) {
			return nil, fmt.Errorf("looking for session: %w", core.ErrResourceNotFound)
		}
		return nil, fmt.Errorf("looking for session: %w", result.Error)
	}

	return &session, nil
}

func (r *sr) Update(session *models.Session) error {
	result := r.db.Save(session)
	if result.Error != nil {
		return fmt.Errorf("updating session: %w", result.Error)
	}

	return nil
}

// DeleteExpired erases every session that expired before the given instant and
// returns how many rows were removed.
func (r *sr) DeleteExpired(before time.Time) (int64, error) {
	result := r.db.Unscoped().Delete(&models.Session{}, "expires_at < ?", before)
	if result.Error != nil {
		return 0, fmt.Errorf("deleting expired sessions: %w", result.Error)
	}

	return result.RowsAffected, nil
}

// DeleteForUser erases every session belonging to a user for good. They are of
// no use to anyone once the credentials behind them changed.
func (r *sr) DeleteForUser(userID uint) (int64, error) {
	result := r.db.Unscoped().Delete(&models.Session{}, "user_id = ?", userID)
	if result.Error != nil {
		return 0, fmt.Errorf("deleting the sessions of a user: %w", result.Error)
	}

	return result.RowsAffected, nil
}

func (r *sr) Delete(code string) error {
	result := r.db.Delete(&models.Session{}, "code = ?", code)
	if result.Error != nil {
		return fmt.Errorf("deleting session: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("deleting session: %w", core.ErrResourceNotFound)
	}

	return nil
}
