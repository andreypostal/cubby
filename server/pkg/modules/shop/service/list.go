package service

import (
	"errors"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/shop/dto"
	"andreypostal.com/cubby/server/pkg/modules/shop/models"
	"andreypostal.com/cubby/server/pkg/modules/shop/repository"
)

type ListService interface {
	Create(workspaceID uint, list *dto.CreateListReq) (*dto.ListPayload, error)
	Get(listID, workspaceID uint) (*dto.ListWithItemsResp, error)
	Update(listID, workspaceID uint, req *dto.UpdateListReq) (*dto.ListPayload, error)
	Delete(listID, workspaceID uint) error
	ListForWorkspace(workspaceID uint) (*dto.WorkspaceListsResp, error)
}

type ls struct {
	listRepo  repository.ListRepository
	itemsRepo repository.ListItemRepository
}

func NewListService(listRepo repository.ListRepository, itemsRepo repository.ListItemRepository) ListService {
	return &ls{listRepo: listRepo, itemsRepo: itemsRepo}
}

func (s *ls) Create(workspaceID uint, req *dto.CreateListReq) (*dto.ListPayload, error) {
	list := &models.List{
		Name:        req.Name,
		Icon:        req.Icon,
		WorkspaceID: workspaceID,
	}

	if err := s.listRepo.Create(list); err != nil {
		return nil, err
	}

	return toListResponse(list), nil
}

func (s *ls) Get(listID, workspaceID uint) (*dto.ListWithItemsResp, error) {
	list, err := s.listRepo.FindByID(listID)
	if err != nil {
		return nil, normalizeError(err)
	}

	if list.WorkspaceID != workspaceID {
		return nil, core.ErrNotAllowed
	}

	items, err := s.itemsRepo.ListForList(list.ID)
	if err != nil {
		return nil, err
	}

	resp := &dto.ListWithItemsResp{
		ID:    list.ID,
		Name:  list.Name,
		Icon:  list.Icon,
		Items: make([]*dto.ListItemPayload, len(items)),
	}

	for i, item := range items {
		resp.Items[i] = toListItemResponse(item)
	}

	return resp, nil
}

func (s *ls) Update(listID, workspaceID uint, req *dto.UpdateListReq) (*dto.ListPayload, error) {
	list, err := s.listRepo.FindByID(listID)
	if err != nil {
		return nil, normalizeError(err)
	}

	if list.WorkspaceID != workspaceID {
		return nil, core.ErrNotAllowed
	}

	list.Name = req.Name
	list.Icon = req.Icon

	if err := s.listRepo.Update(list); err != nil {
		return nil, err
	}

	return toListResponse(list), nil
}

func (s *ls) Delete(listID, workspaceID uint) error {
	list, err := s.listRepo.FindByID(listID)
	if err != nil {
		return normalizeError(err)
	}

	if list.WorkspaceID != workspaceID {
		return core.ErrNotAllowed
	}

	return s.listRepo.Delete(listID)
}

func (s *ls) ListForWorkspace(workspaceID uint) (*dto.WorkspaceListsResp, error) {
	lists, err := s.listRepo.ListForWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	out := make([]dto.ListPayload, len(lists))
	for i, m := range lists {
		out[i] = *toListResponse(m)
	}

	return &dto.WorkspaceListsResp{Lists: out}, nil
}

func normalizeError(err error) error {
	if errors.Is(err, core.ErrEntityNotFound) {
		return core.ErrNotAllowed
	}
	return err
}

func toListResponse(l *models.List) *dto.ListPayload {
	return &dto.ListPayload{
		ID:        l.ID,
		Name:      l.Name,
		Icon:      l.Icon,
		CreatedAt: l.CreatedAt,
	}
}
