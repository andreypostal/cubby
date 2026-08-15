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

// UserHandler handles the system administrator's user management routes. Every
// route it serves is mounted behind the system-admin scope.
type UserHandler struct {
	userService service.UserService
	validate    *validator.Validate
}

func NewUserHandler(userService service.UserService, validate *validator.Validate) *UserHandler {
	return &UserHandler{userService: userService, validate: validate}
}

func (h *UserHandler) List(c *gin.Context) {
	resp, err := h.userService.List()
	if err != nil {
		respondUserError(c, err, "failed to list users")
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *UserHandler) Create(c *gin.Context) {
	var req dto.CreateUserReq
	if !h.bind(c, &req) {
		return
	}

	resp, err := h.userService.Create(&req)
	if err != nil {
		respondUserError(c, err, "failed to create the user")
		return
	}

	middleware.GetLogger(c).Info("Created a user",
		actor(c),
		slog.String("username", resp.Username),
		slog.Uint64("user_id", uint64(resp.ID)),
		slog.Bool("system_admin", resp.IsSystemAdmin),
	)

	c.JSON(http.StatusCreated, resp)
}

func (h *UserHandler) Update(c *gin.Context) {
	profile, userID, ok := h.userRequest(c)
	if !ok {
		return
	}

	var req dto.UpdateUserReq
	if !h.bind(c, &req) {
		return
	}

	resp, err := h.userService.Update(userID, profile.User.ID, &req)
	if err != nil {
		respondUserError(c, err, "failed to update the user")
		return
	}

	middleware.GetLogger(c).Info("Updated a user",
		actor(c),
		slog.String("username", resp.Username),
		slog.Uint64("user_id", uint64(resp.ID)),
		slog.Bool("system_admin", resp.IsSystemAdmin),
	)

	c.JSON(http.StatusOK, resp)
}

func (h *UserHandler) ChangePassword(c *gin.Context) {
	_, userID, ok := h.userRequest(c)
	if !ok {
		return
	}

	var req dto.ChangePasswordReq
	if !h.bind(c, &req) {
		return
	}

	if err := h.userService.ChangePassword(userID, &req); err != nil {
		respondUserError(c, err, "failed to change the password")
		return
	}

	middleware.GetLogger(c).Info("Changed a user password and revoked their sessions",
		actor(c),
		slog.Uint64("user_id", uint64(userID)),
	)

	c.Status(http.StatusNoContent)
}

func (h *UserHandler) Delete(c *gin.Context) {
	profile, userID, ok := h.userRequest(c)
	if !ok {
		return
	}

	err := h.userService.Delete(userID, profile.User.ID)

	// The cubbies the user created are what blocks the deletion, so the answer
	// names them instead of leaving the administrator to guess
	if errors.Is(err, service.ErrUserOwnsWorkspaces) {
		names, listErr := h.userService.OwnedWorkspaces(userID)
		if listErr != nil {
			middleware.GetLogger(c).Error("Failed to list the workspaces blocking a user deletion",
				slog.String("error", listErr.Error()))
			names = []string{}
		}

		core.RespondError(c, core.NewError(http.StatusConflict, "the user still owns cubbies", map[string]any{
			"workspaces": names,
		}))
		return
	}

	if err != nil {
		respondUserError(c, err, "failed to delete the user")
		return
	}

	middleware.GetLogger(c).Info("Deleted a user",
		actor(c),
		slog.Uint64("user_id", uint64(userID)),
	)

	c.Status(http.StatusNoContent)
}

// actor names the administrator behind a request
func actor(c *gin.Context) slog.Attr {
	profile, ok := core.GetProfile(c)
	if !ok {
		return slog.String("actor", "unknown")
	}

	return slog.String("actor", profile.User.Username)
}

// bind reads and validates a request body, answering whether the handler may
// carry on
func (h *UserHandler) bind(c *gin.Context, req any) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return false
	}

	if err := h.validate.Struct(req); err != nil {
		core.RespondError(c, core.ValidationError(err))
		return false
	}

	return true
}

// userRequest pulls the caller and the user being acted on out of the request
func (h *UserHandler) userRequest(c *gin.Context) (core.Profile, uint, bool) {
	profile, ok := core.GetProfile(c)
	if !ok {
		middleware.GetLogger(c).Error("Failed to retrieve user from request context")
		core.RespondError(c, core.NewError(http.StatusForbidden, "failed to retrieve user info", nil))
		return core.Profile{}, 0, false
	}

	userID, err := core.FromStringToUint(c.Param("userID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid user id", nil))
		return core.Profile{}, 0, false
	}

	return profile, userID, true
}

func respondUserError(c *gin.Context, err error, msg string) {
	switch {
	case errors.Is(err, service.ErrLastSystemAdmin):
		core.RespondError(c, core.NewError(http.StatusConflict, "the last system admin cannot be removed or demoted", nil))
	case errors.Is(err, core.ErrBadRequest):
		core.RespondError(c, core.NewError(http.StatusBadRequest, msg, map[string]any{
			"reason": err.Error(),
		}))
	case errors.Is(err, core.ErrResourceNotFound):
		core.RespondError(c, core.NewError(http.StatusNotFound, msg, nil))
	case errors.Is(err, core.ErrDuplicatedEntry):
		core.RespondError(c, core.NewError(http.StatusConflict, "that username is already taken", nil))
	case errors.Is(err, core.ErrNotAllowed):
		core.RespondError(c, core.NewError(http.StatusForbidden, msg, nil))
	default:
		middleware.GetLogger(c).Error(msg, slog.String("error", err.Error()))
		core.RespondError(c, core.NewError(http.StatusInternalServerError, msg, nil))
	}
}
