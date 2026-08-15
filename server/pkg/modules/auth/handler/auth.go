package handler

import (
	"log/slog"
	"net/http"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/auth/dto"
	"andreypostal.com/cubby/server/pkg/modules/auth/service"
	"andreypostal.com/cubby/server/pkg/modules/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// AuthHandler handles authentication routes.
type AuthHandler struct {
	authSvc service.AuthService
	wsSvc   service.WorkspaceService

	validate *validator.Validate
}

// NewAuthHandler returns an AuthHandler.
func NewAuthHandler(authSvc service.AuthService, wsSvc service.WorkspaceService, validate *validator.Validate) *AuthHandler {
	return &AuthHandler{authSvc: authSvc, wsSvc: wsSvc, validate: validate}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, core.ValidationError(err))
		return
	}

	logger := middleware.GetLogger(c)

	resp, err := h.authSvc.Login(&req)
	if err != nil {
		logger.Warn("Rejected a login attempt", slog.String("username", req.Username))
		core.RespondError(c, core.NewError(http.StatusForbidden, "failed to login", nil))
		return
	}

	logger.Info("User logged in",
		slog.String("username", resp.User.Username),
		slog.Uint64("user_id", uint64(resp.User.ID)))

	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	logger := middleware.GetLogger(c)

	profile, ok := core.GetProfile(c)
	if !ok {
		logger.Error("Failed to retrieve user from request context")
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	if err := h.authSvc.Logout(profile.SessionCode); err != nil {
		logger.Warn("Failed to end the session",
			slog.String("username", profile.User.Username),
			slog.String("error", err.Error()))
		core.RespondError(c, core.NewError(http.StatusForbidden, err.Error(), nil))
		return
	}

	logger.Info("User logged out", slog.String("username", profile.User.Username))

	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) SwitchWorkspace(c *gin.Context) {
	logger := middleware.GetLogger(c)

	profile, ok := core.GetProfile(c)
	if !ok {
		logger.Error("Failed to retrieve user from request context")
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	var req dto.SwitchWorkspaceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	if _, err := h.wsSvc.Get(req.WorkspaceID, profile.User.ID); err != nil {
		logger.Warn("Refused a workspace switch",
			slog.String("username", profile.User.Username),
			slog.Uint64("workspace_id", uint64(req.WorkspaceID)),
			slog.String("error", err.Error()))
		core.RespondError(c, core.NewError(http.StatusForbidden, err.Error(), nil))
		return
	}

	if err := h.authSvc.SwitchWorkspace(profile.SessionCode, &req); err != nil {
		logger.Error("Failed to switch the session workspace",
			slog.String("username", profile.User.Username),
			slog.Uint64("workspace_id", uint64(req.WorkspaceID)),
			slog.String("error", err.Error()))
		core.RespondError(c, core.NewError(http.StatusForbidden, err.Error(), nil))
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) ResetWorkspace(c *gin.Context) {
	logger := middleware.GetLogger(c)

	profile, ok := core.GetProfile(c)
	if !ok {
		logger.Error("Failed to retrieve user from request context")
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	if err := h.authSvc.ResetWorkspace(profile.SessionCode); err != nil {
		logger.Error("Failed to reset the session workspace",
			slog.String("username", profile.User.Username),
			slog.String("error", err.Error()))
		core.RespondError(c, core.NewError(http.StatusForbidden, err.Error(), nil))
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) Me(c *gin.Context) {
	profile, ok := core.GetProfile(c)
	if !ok {
		middleware.GetLogger(c).Error("Failed to retrieve user from request context")
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	resp := dto.MeResponse{
		User: dto.UserPayload{
			ID:            profile.User.ID,
			Name:          profile.User.Name,
			Username:      profile.User.Username,
			IsSystemAdmin: profile.User.IsSystemAdmin,
		},
	}

	if profile.Workspace != nil {
		resp.CurrentWorkspace = &dto.WorkspaceResp{
			ID:        profile.Workspace.ID,
			Name:      profile.Workspace.Name,
			IsCreator: profile.Workspace.IsCreator,
		}
	}

	c.JSON(http.StatusOK, resp)
}
