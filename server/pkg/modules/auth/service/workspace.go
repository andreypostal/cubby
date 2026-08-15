package service

import (
	"errors"
	"fmt"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/auth/dto"
	"andreypostal.com/cubby/server/pkg/modules/auth/models"
	"andreypostal.com/cubby/server/pkg/modules/auth/repository"
)

type WorkspaceService interface {
	Create(userID uint, workspace *dto.CreateWorkspaceReq) (*dto.WorkspaceResp, error)
	Get(workspaceID, requesterID uint) (*dto.WorkspaceResp, error)
	Update(workspaceID, requesterID uint, req *dto.UpdateWorkspaceReq) (*dto.WorkspaceResp, error)
	Delete(workspaceID, requesterID uint) error
	ListForUser(userID uint) (*dto.WorkspaceListResp, error)

	// Members answers to anyone with access to the workspace
	Members(workspaceID, requesterID uint) (*dto.WorkspaceMemberListResp, error)
	// Share and Revoke are the creator's alone
	Share(workspaceID, requesterID uint, req *dto.ShareWorkspaceReq) (*dto.WorkspaceMemberResp, error)
	Revoke(workspaceID, requesterID, memberID uint) error
	// Leave is how a shared user gives up their own access
	Leave(workspaceID, requesterID uint) error
}

type ws struct {
	wsRepo   repository.WorkspaceRepository
	userRepo repository.UserRepository
}

func NewWorkspaceService(
	wsRepo repository.WorkspaceRepository,
	userRepo repository.UserRepository,
) WorkspaceService {
	return &ws{wsRepo: wsRepo, userRepo: userRepo}
}

func (s *ws) Create(userID uint, req *dto.CreateWorkspaceReq) (*dto.WorkspaceResp, error) {
	workspace := &models.Workspace{
		Name:      req.Name,
		CreatorID: userID,
	}

	if err := s.wsRepo.Create(workspace); err != nil {
		return nil, err
	}

	return toWorkspaceResponse(workspace, userID), nil
}

func (s *ws) Get(workspaceID, requesterID uint) (*dto.WorkspaceResp, error) {
	workspace, err := s.member(workspaceID, requesterID)
	if err != nil {
		return nil, err
	}

	return toWorkspaceResponse(workspace, requesterID), nil
}

func (s *ws) Update(workspaceID, requesterID uint, req *dto.UpdateWorkspaceReq) (*dto.WorkspaceResp, error) {
	workspace, err := s.creator(workspaceID, requesterID)
	if err != nil {
		return nil, err
	}

	if req.Name == "" || req.Name == workspace.Name {
		return nil, fmt.Errorf("%w: nothing to change", core.ErrBadRequest)
	}

	workspace.Name = req.Name

	if err := s.wsRepo.Update(workspace); err != nil {
		return nil, err
	}

	return toWorkspaceResponse(workspace, requesterID), nil
}

func (s *ws) Delete(workspaceID, requesterID uint) error {
	if _, err := s.creator(workspaceID, requesterID); err != nil {
		return err
	}

	return s.wsRepo.Delete(workspaceID)
}

func (s *ws) ListForUser(userID uint) (*dto.WorkspaceListResp, error) {
	workspaces, err := s.wsRepo.ListForUser(userID)
	if err != nil {
		return nil, err
	}

	out := make([]dto.WorkspaceResp, len(workspaces))
	for i, m := range workspaces {
		out[i] = *toWorkspaceResponse(m, userID)
	}

	return &dto.WorkspaceListResp{Workspaces: out}, nil
}

func (s *ws) Members(workspaceID, requesterID uint) (*dto.WorkspaceMemberListResp, error) {
	if _, err := s.member(workspaceID, requesterID); err != nil {
		return nil, err
	}

	members, err := s.wsRepo.Members(workspaceID)
	if err != nil {
		return nil, err
	}

	out := make([]dto.WorkspaceMemberResp, len(members))
	for i, m := range members {
		out[i] = *toWorkspaceMemberResponse(m)
	}

	return &dto.WorkspaceMemberListResp{Members: out}, nil
}

func (s *ws) Share(workspaceID, requesterID uint, req *dto.ShareWorkspaceReq) (*dto.WorkspaceMemberResp, error) {
	workspace, err := s.creator(workspaceID, requesterID)
	if err != nil {
		return nil, err
	}

	user, err := s.userRepo.FindByUsername(req.Username)
	if err != nil {
		return nil, core.ErrResourceNotFound
	}

	// Do not allow sharing with the creator
	if user.ID == workspace.CreatorID {
		return nil, core.ErrDuplicatedEntry
	}

	shared, err := s.wsRepo.HasAccess(workspaceID, user.ID)
	if err != nil {
		return nil, err
	}
	if shared {
		return nil, core.ErrDuplicatedEntry
	}

	if err := s.wsRepo.AddMember(workspaceID, user); err != nil {
		return nil, err
	}

	return toWorkspaceMemberResponse(user), nil
}

func (s *ws) Revoke(workspaceID, requesterID, memberID uint) error {
	workspace, err := s.creator(workspaceID, requesterID)
	if err != nil {
		return err
	}

	// Creator access cannot be revoked
	if memberID == workspace.CreatorID {
		return core.ErrNotAllowed
	}

	return s.wsRepo.RemoveMember(workspaceID, memberID)
}

func (s *ws) Leave(workspaceID, requesterID uint) error {
	workspace, err := s.member(workspaceID, requesterID)
	if err != nil {
		return err
	}

	// The creator has nowhere to leave to: they delete the workspace instead
	if workspace.CreatorID == requesterID {
		return core.ErrNotAllowed
	}

	return s.wsRepo.RemoveMember(workspaceID, requesterID)
}

// member loads the workspace when the requester created it or it was shared
// with them, and refuses otherwise
func (s *ws) member(workspaceID, requesterID uint) (*models.Workspace, error) {
	workspace, err := s.wsRepo.FindByID(workspaceID)
	if err != nil {
		return nil, normalizeError(err)
	}

	if workspace.CreatorID != requesterID {
		shared, err := s.wsRepo.HasAccess(workspaceID, requesterID)
		if err != nil {
			return nil, err
		}
		if !shared {
			return nil, core.ErrNotAllowed
		}
	}

	return workspace, nil
}

// creator loads the workspace only for the user who created it
func (s *ws) creator(workspaceID, requesterID uint) (*models.Workspace, error) {
	workspace, err := s.wsRepo.FindByID(workspaceID)
	if err != nil {
		return nil, normalizeError(err)
	}

	if workspace.CreatorID != requesterID {
		return nil, core.ErrNotAllowed
	}

	return workspace, nil
}

func normalizeError(err error) error {
	if errors.Is(err, core.ErrEntityNotFound) {
		return core.ErrNotAllowed
	}
	return err
}

func toWorkspaceResponse(workspace *models.Workspace, requesterID uint) *dto.WorkspaceResp {
	return &dto.WorkspaceResp{
		ID:        workspace.ID,
		Name:      workspace.Name,
		IsCreator: workspace.CreatorID == requesterID,
	}
}

func toWorkspaceMemberResponse(user *models.User) *dto.WorkspaceMemberResp {
	return &dto.WorkspaceMemberResp{
		ID:       user.ID,
		Username: user.Username,
		Name:     user.Name,
	}
}
