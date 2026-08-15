package service

import (
	"fmt"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/auth/dto"
	"andreypostal.com/cubby/server/pkg/modules/auth/models"
	"andreypostal.com/cubby/server/pkg/modules/auth/repository"
	"github.com/google/uuid"
)

type AuthService interface {
	GetSession(token string) (*models.Session, error)
	Login(req *dto.LoginReq) (*dto.AuthResponse, error)
	Logout(sessionCode string) error
	SwitchWorkspace(sessionCode string, req *dto.SwitchWorkspaceReq) error
	ResetWorkspace(sessionCode string) error

	// CLI actions
	CreateUser(name, username, password string, isSystemAdmin bool) error
	ChangeUserSystemAdminAccess(username string, isSystemAdmin bool) error
}

type authService struct {
	userRepo    repository.UserRepository
	sessionRepo repository.SessionRepository

	hasher core.PasswordHasher
	jwt    core.JwtProvider
}

func NewAuthService(
	userRepo repository.UserRepository,
	sessionRepo repository.SessionRepository,
	hasher core.PasswordHasher,
	jwt core.JwtProvider,
) AuthService {
	return &authService{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		hasher:      hasher,
		jwt:         jwt,
	}
}

func (s *authService) CreateUser(name, username, password string, isSystemAdmin bool) error {
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	return s.userRepo.Create(&models.User{
		Username:      username,
		Password:      hash,
		Name:          name,
		IsSystemAdmin: isSystemAdmin,
	})
}

func (s *authService) ChangeUserSystemAdminAccess(username string, isSystemAdmin bool) error {
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return err
	}

	user.IsSystemAdmin = isSystemAdmin

	return s.userRepo.Update(user)
}

// GetSession finds a session vinculated to an issued token
// it is not intended for public API usage
func (s *authService) GetSession(code string) (*models.Session, error) {
	return s.sessionRepo.FindByCode(code)
}

func (s *authService) Logout(sessionCode string) error {
	return s.sessionRepo.Delete(sessionCode)
}

func (s *authService) SwitchWorkspace(sessionCode string, req *dto.SwitchWorkspaceReq) error {
	session, err := s.sessionRepo.FindByCode(sessionCode)
	if err != nil {
		return core.ErrNotAllowed
	}

	if session.WorkspaceID != nil && *session.WorkspaceID == req.WorkspaceID {
		return nil
	}

	session.Workspace = nil
	session.WorkspaceID = &req.WorkspaceID

	if err = s.sessionRepo.Update(session); err != nil {
		return err
	}
	return nil
}

func (s *authService) ResetWorkspace(sessionCode string) error {
	session, err := s.sessionRepo.FindByCode(sessionCode)
	if err != nil {
		return core.ErrNotAllowed
	}

	if session.WorkspaceID == nil {
		return nil
	}

	session.Workspace = nil
	session.WorkspaceID = nil

	return s.sessionRepo.Update(session)
}

func (s *authService) Login(req *dto.LoginReq) (*dto.AuthResponse, error) {
	user, err := s.userRepo.FindByUsername(req.Username)
	if err != nil {
		return nil, core.ErrInvalidLogin
	}

	if err := s.hasher.Verify(req.Password, user.Password); err != nil {
		return nil, core.ErrInvalidLogin
	}

	token, err := s.doLogin(user.ID, user.Username)
	if err != nil {
		return nil, core.ErrInvalidLogin
	}

	return &dto.AuthResponse{
		Token: token,
		User:  toUserPayload(user),
	}, nil
}

// doLogin creates a session and returns the jwt token or an error
func (s *authService) doLogin(userId uint, username string) (string, error) {
	sessionCode := uuid.New()

	token, expiresAt, err := s.jwt.NewToken(sessionCode.String(), username, userId)
	if err != nil {
		return "", err
	}

	session := &models.Session{
		UserID:    userId,
		ExpiresAt: expiresAt,
		Code:      sessionCode.String(),
	}

	if err = s.sessionRepo.Create(session); err != nil {
		return "", err
	}

	return token, nil
}

func toUserPayload(u *models.User) dto.UserPayload {
	return dto.UserPayload{
		ID:            u.ID,
		Name:          u.Name,
		Username:      u.Username,
		IsSystemAdmin: u.IsSystemAdmin,
	}
}
