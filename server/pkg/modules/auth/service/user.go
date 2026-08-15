package service

import (
	"errors"
	"fmt"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/auth/dto"
	"andreypostal.com/cubby/server/pkg/modules/auth/models"
	"andreypostal.com/cubby/server/pkg/modules/auth/repository"
)

// ErrUserOwnsWorkspaces refuses to remove a user while cubbies they created are
// still around
var ErrUserOwnsWorkspaces = errors.New("user still owns workspaces")

// ErrLastSystemAdmin refuses the operations that would leave the server with
// nobody able to administer it
var ErrLastSystemAdmin = errors.New("the last system admin cannot be removed or demoted")

// UserService is the system administrator's view of the user list. Everything
// here is already behind the system-admin route scope, so it enforces the rules
// that keep an administrator from locking themselves or everyone else out
// rather than any per-user permission.
type UserService interface {
	List() (*dto.UserListResp, error)
	Create(req *dto.CreateUserReq) (*dto.UserPayload, error)
	Update(id, requesterID uint, req *dto.UpdateUserReq) (*dto.UserPayload, error)
	ChangePassword(id uint, req *dto.ChangePasswordReq) error
	Delete(id, requesterID uint) error

	// OwnedWorkspaces names the cubbies blocking a deletion, so the caller can
	// tell the administrator what is in the way
	OwnedWorkspaces(id uint) ([]string, error)
}

type us struct {
	userRepo    repository.UserRepository
	wsRepo      repository.WorkspaceRepository
	sessionRepo repository.SessionRepository

	hasher core.PasswordHasher
}

func NewUserService(
	userRepo repository.UserRepository,
	wsRepo repository.WorkspaceRepository,
	sessionRepo repository.SessionRepository,
	hasher core.PasswordHasher,
) UserService {
	return &us{
		userRepo:    userRepo,
		wsRepo:      wsRepo,
		sessionRepo: sessionRepo,
		hasher:      hasher,
	}
}

func (s *us) List() (*dto.UserListResp, error) {
	users, err := s.userRepo.List()
	if err != nil {
		return nil, err
	}

	out := make([]dto.UserPayload, len(users))
	for i, u := range users {
		out[i] = toUserPayload(u)
	}

	return &dto.UserListResp{Users: out}, nil
}

func (s *us) Create(req *dto.CreateUserReq) (*dto.UserPayload, error) {
	hash, err := s.hasher.Hash(req.Password)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	user := &models.User{
		Username:      req.Username,
		Password:      hash,
		Name:          req.Name,
		IsSystemAdmin: req.IsSystemAdmin,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	payload := toUserPayload(user)

	return &payload, nil
}

func (s *us) Update(id, requesterID uint, req *dto.UpdateUserReq) (*dto.UserPayload, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if user.IsSystemAdmin && !req.IsSystemAdmin {
		if id == requesterID {
			return nil, fmt.Errorf("%w: an administrator cannot demote themselves", core.ErrNotAllowed)
		}

		if err := s.assertNotTheLastAdmin(); err != nil {
			return nil, err
		}
	}

	user.Name = req.Name
	user.IsSystemAdmin = req.IsSystemAdmin

	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}
	payload := toUserPayload(user)

	return &payload, nil
}

func (s *us) ChangePassword(id uint, req *dto.ChangePasswordReq) error {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return err
	}

	hash, err := s.hasher.Hash(req.Password)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	user.Password = hash

	if err := s.userRepo.Update(user); err != nil {
		return err
	}

	// Revoke already issued sessions
	if _, err := s.sessionRepo.DeleteForUser(id); err != nil {
		return err
	}

	return nil
}

func (s *us) Delete(id, requesterID uint) error {
	if id == requesterID {
		return fmt.Errorf("%w: an administrator cannot delete themselves", core.ErrNotAllowed)
	}

	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return err
	}

	if user.IsSystemAdmin {
		if err := s.assertNotTheLastAdmin(); err != nil {
			return err
		}
	}

	owned, err := s.wsRepo.ListCreatedBy(id)
	if err != nil {
		return err
	}

	if len(owned) > 0 {
		return fmt.Errorf("%w: %d of them", ErrUserOwnsWorkspaces, len(owned))
	}

	if _, err := s.sessionRepo.DeleteForUser(id); err != nil {
		return err
	}

	if err := s.wsRepo.RemoveMemberEverywhere(id); err != nil {
		return err
	}

	return s.userRepo.Delete(id)
}

func (s *us) OwnedWorkspaces(id uint) ([]string, error) {
	owned, err := s.wsRepo.ListCreatedBy(id)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(owned))
	for i, w := range owned {
		names[i] = w.Name
	}

	return names, nil
}

func (s *us) assertNotTheLastAdmin() error {
	admins, err := s.userRepo.CountSystemAdmins()
	if err != nil {
		return err
	}

	if admins <= 1 {
		return ErrLastSystemAdmin
	}

	return nil
}
