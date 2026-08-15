package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/auth/dto"
	"andreypostal.com/cubby/server/pkg/modules/auth/service"
	"andreypostal.com/cubby/server/pkg/modules/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// WorkspaceHandler handles organization and membership routes.
type WorkspaceHandler struct {
	wsService service.WorkspaceService
	validate  *validator.Validate
}

// NewWorkspaceHandler returns an WorkspaceHandler.
func NewWorkspaceHandler(wsService service.WorkspaceService, validate *validator.Validate) *WorkspaceHandler {
	return &WorkspaceHandler{
		wsService: wsService,
		validate:  validate,
	}
}

func (h *WorkspaceHandler) List(c *gin.Context) {
	logger := middleware.GetLogger(c)

	profile, ok := core.GetProfile(c)
	if !ok {
		logger.Error("Failed to retrieve user from request context")
		core.RespondError(c, core.NewError(http.StatusInternalServerError, "failed to retrieve user info", nil))
		return
	}

	resp, err := h.wsService.ListForUser(profile.User.ID)
	if err != nil {
		logger.Error("Failed to list user workspaces", slog.String("error", err.Error()))
		core.RespondError(c, core.NewError(http.StatusInternalServerError, "faile to retrieve user workspaces", nil))
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *WorkspaceHandler) Create(c *gin.Context) {
	logger := middleware.GetLogger(c)

	profile, ok := core.GetProfile(c)
	if !ok {
		logger.Error("Failed retrieve user from request context")
		core.RespondError(c, core.NewError(http.StatusInternalServerError, "failed to retrieve user info", nil))
		return
	}

	var req dto.CreateWorkspaceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		core.RespondError(c, core.ValidationError(err))
		return
	}

	resp, err := h.wsService.Create(profile.User.ID, &req)
	if err != nil {
		logger.Warn("error while creating new workspace", slog.String("reason", err.Error()))
		core.RespondError(c, core.NewError(http.StatusBadRequest, "error while creating new workspace", nil))
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *WorkspaceHandler) Get(c *gin.Context) {
	logger := middleware.GetLogger(c)

	profile, ok := core.GetProfile(c)
	if !ok {
		logger.Error("Failed retrieve user from request context")
		core.RespondError(c, core.NewError(http.StatusForbidden, "failed to retrieve user info", nil))
		return
	}

	workspaceID, err := core.FromStringToUint(c.Param("workspaceID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid workspace id", nil))
		return
	}

	resp, err := h.wsService.Get(workspaceID, profile.User.ID)
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusForbidden, "failed to fetch workspace", nil))
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *WorkspaceHandler) Update(c *gin.Context) {
	logger := middleware.GetLogger(c)

	profile, ok := core.GetProfile(c)
	if !ok {
		logger.Error("Failed retrieve user from request context")
		core.RespondError(c, core.NewError(http.StatusForbidden, "failed to retrieve user info", nil))
		return
	}

	workspaceID, err := core.FromStringToUint(c.Param("workspaceID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid workspace id", nil))
		return
	}

	var req dto.UpdateWorkspaceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		core.RespondError(c, core.ValidationError(err))
		return
	}

	resp, err := h.wsService.Update(workspaceID, profile.User.ID, &req)
	if err != nil {
		respondWorkspaceError(c, err, "failed to update workspace")
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *WorkspaceHandler) Delete(c *gin.Context) {
	logger := middleware.GetLogger(c)
	profile, ok := core.GetProfile(c)
	if !ok {
		logger.Error("Failed retrieve user from request context")
		core.RespondError(c, core.NewError(http.StatusForbidden, "failed to retrieve user info", nil))
		return
	}

	workspaceID, err := core.FromStringToUint(c.Param("workspaceID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid workspace id", nil))
		return
	}

	if err := h.wsService.Delete(workspaceID, profile.User.ID); err != nil {
		respondWorkspaceError(c, err, "failed to remove workspace")
		return
	}

	logger.Info("Deleted a workspace",
		actor(c),
		slog.Uint64("workspace_id", uint64(workspaceID)),
	)

	c.Status(http.StatusNoContent)
}

func (h *WorkspaceHandler) ListMembers(c *gin.Context) {
	profile, workspaceID, ok := h.workspaceRequest(c)
	if !ok {
		return
	}

	resp, err := h.wsService.Members(workspaceID, profile.User.ID)
	if err != nil {
		respondWorkspaceError(c, err, "failed to list workspace members")
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *WorkspaceHandler) Share(c *gin.Context) {
	profile, workspaceID, ok := h.workspaceRequest(c)
	if !ok {
		return
	}

	var req dto.ShareWorkspaceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		core.RespondError(c, core.ValidationError(err))
		return
	}

	resp, err := h.wsService.Share(workspaceID, profile.User.ID, &req)
	if err != nil {
		respondWorkspaceError(c, err, "failed to share workspace")
		return
	}

	middleware.GetLogger(c).Info("Shared a workspace",
		actor(c),
		slog.Uint64("workspace_id", uint64(workspaceID)),
		slog.String("with", req.Username),
	)

	c.JSON(http.StatusCreated, resp)
}

func (h *WorkspaceHandler) Revoke(c *gin.Context) {
	profile, workspaceID, ok := h.workspaceRequest(c)
	if !ok {
		return
	}

	memberID, err := core.FromStringToUint(c.Param("userID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid user id", nil))
		return
	}

	if err := h.wsService.Revoke(workspaceID, profile.User.ID, memberID); err != nil {
		respondWorkspaceError(c, err, "failed to revoke workspace access")
		return
	}

	middleware.GetLogger(c).Info("Revoked workspace access",
		actor(c),
		slog.Uint64("workspace_id", uint64(workspaceID)),
		slog.Uint64("member_id", uint64(memberID)),
	)

	c.Status(http.StatusNoContent)
}

func (h *WorkspaceHandler) Leave(c *gin.Context) {
	profile, workspaceID, ok := h.workspaceRequest(c)
	if !ok {
		return
	}

	if err := h.wsService.Leave(workspaceID, profile.User.ID); err != nil {
		respondWorkspaceError(c, err, "failed to leave workspace")
		return
	}

	c.Status(http.StatusNoContent)
}

// workspaceRequest pulls the caller and the workspace out of the request
func (h *WorkspaceHandler) workspaceRequest(c *gin.Context) (core.Profile, uint, bool) {
	logger := middleware.GetLogger(c)

	profile, ok := core.GetProfile(c)
	if !ok {
		logger.Error("Failed to retrieve user from request context")
		core.RespondError(c, core.NewError(http.StatusForbidden, "failed to retrieve user info", nil))
		return core.Profile{}, 0, false
	}

	workspaceID, err := core.FromStringToUint(c.Param("workspaceID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid workspace id", nil))
		return core.Profile{}, 0, false
	}

	return profile, workspaceID, true
}

func respondWorkspaceError(c *gin.Context, err error, msg string) {
	switch {
	case errors.Is(err, core.ErrBadRequest):
		core.RespondError(c, core.NewError(http.StatusBadRequest, msg, map[string]any{
			"reason": err.Error(),
		}))
	case errors.Is(err, core.ErrResourceNotFound):
		core.RespondError(c, core.NewError(http.StatusNotFound, msg, nil))
	case errors.Is(err, core.ErrDuplicatedEntry):
		core.RespondError(c, core.NewError(http.StatusConflict, msg, nil))
	case errors.Is(err, core.ErrNotAllowed):
		core.RespondError(c, core.NewError(http.StatusForbidden, msg, nil))
	default:
		middleware.GetLogger(c).Error(msg, slog.String("error", err.Error()))
		core.RespondError(c, core.NewError(http.StatusInternalServerError, msg, nil))
	}
}
