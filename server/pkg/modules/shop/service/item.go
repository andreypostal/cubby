package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/shop/dto"
	"andreypostal.com/cubby/server/pkg/modules/shop/models"
	"andreypostal.com/cubby/server/pkg/modules/shop/repository"
)

type ListItemService interface {
	Create(workspaceID, listID uint, list *dto.CreateListItemReq) (*dto.ListItemPayload, error)
	Get(itemID, listID uint) (*dto.ListItemPayload, error)
	Update(itemID, listID uint, req *dto.UpdateListItemReq) (*dto.ListItemPayload, error)
	Delete(itemID, listID uint) error
	ToggleBought(itemID, listID uint) (*dto.ListItemPayload, error)
	ClearBought(listID uint) ([]*dto.ListItemPayload, error)

	Suggest(workspaceID uint, term string) (*dto.ListItemSuggestions, error)
}

type lis struct {
	itemsRepo      repository.ListItemRepository
	itemsIndexer   *core.Indexer
	suggestTimeout time.Duration
}

func NewListItemService(itemsRepo repository.ListItemRepository, itemsIndexer *core.Indexer, suggestTimeout time.Duration) ListItemService {
	return &lis{itemsRepo: itemsRepo, itemsIndexer: itemsIndexer, suggestTimeout: suggestTimeout}
}

func (c *lis) Suggest(workspaceID uint, term string) (*dto.ListItemSuggestions, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.suggestTimeout)
	defer cancel()

	terms, err := c.itemsIndexer.Query(ctx, workspaceID, term)
	if err != nil {
		slog.Warn("FAILED TO QUERY FOR ITEM SUGGESTION", slog.String("error", err.Error()))
		return nil, errors.New("failed to find items suggestion")
	}

	return &dto.ListItemSuggestions{Terms: terms}, nil
}

func (s *lis) Create(workspaceID, listID uint, req *dto.CreateListItemReq) (*dto.ListItemPayload, error) {
	item := &models.ListItem{
		Name:   req.Name,
		ListID: listID,
	}

	if err := s.itemsRepo.Create(item); err != nil {
		return nil, err
	}

	if err := s.itemsIndexer.Insert(workspaceID, req.Name); err != nil {
		slog.Warn("FAILED TO INSERT ITEM INDEX", slog.String("error", err.Error()))
	}

	return toListItemResponse(item), nil
}

func (s *lis) Get(itemID, listID uint) (*dto.ListItemPayload, error) {
	item, err := s.itemsRepo.FindByID(itemID)
	if err != nil {
		return nil, normalizeError(err)
	}

	if item.ListID != listID {
		return nil, core.ErrNotAllowed
	}

	return toListItemResponse(item), nil
}

func (s *lis) Update(itemID, listID uint, req *dto.UpdateListItemReq) (*dto.ListItemPayload, error) {
	item, err := s.itemsRepo.FindByID(itemID)
	if err != nil {
		return nil, normalizeError(err)
	}

	if item.ListID != listID {
		return nil, core.ErrNotAllowed
	}

	changed := false

	if req.Name != "" && req.Name != item.Name {
		item.Name = req.Name
		changed = true
	}

	if req.Notes != nil && *req.Notes != item.Notes {
		item.Notes = *req.Notes
		changed = true
	}

	if !changed {
		return nil, errors.New("nothing to change")
	}

	if err := s.itemsRepo.Update(item); err != nil {
		return nil, err
	}

	return toListItemResponse(item), nil
}

func (s *lis) ToggleBought(itemID, listID uint) (*dto.ListItemPayload, error) {
	item, err := s.itemsRepo.FindByID(itemID)
	if err != nil {
		return nil, normalizeError(err)
	}

	if item.ListID != listID {
		return nil, core.ErrNotAllowed
	}

	item.Bought = !item.Bought
	if item.Bought {
		item.BoughtAt = time.Now()
	} else {
		item.BoughtAt = time.Time{}
	}

	if err := s.itemsRepo.Update(item); err != nil {
		return nil, err
	}

	return toListItemResponse(item), nil
}

func (s *lis) ClearBought(listID uint) ([]*dto.ListItemPayload, error) {
	if err := s.itemsRepo.ClearBought(listID); err != nil {
		return nil, err
	}

	items, err := s.itemsRepo.ListForList(listID)
	if err != nil {
		return nil, err
	}

	out := make([]*dto.ListItemPayload, len(items))
	for i, item := range items {
		out[i] = toListItemResponse(item)
	}

	return out, nil
}

func (s *lis) Delete(itemID, listID uint) error {
	item, err := s.itemsRepo.FindByID(itemID)
	if err != nil {
		return normalizeError(err)
	}

	if item.ListID != listID {
		return core.ErrNotAllowed
	}

	return s.itemsRepo.Delete(itemID)
}

func toListItemResponse(l *models.ListItem) *dto.ListItemPayload {
	return &dto.ListItemPayload{
		ID:        l.ID,
		Name:      l.Name,
		Notes:     l.Notes,
		Bought:    l.Bought,
		BoughtAt:  l.BoughtAt,
		CreatedAt: l.CreatedAt,
	}
}
