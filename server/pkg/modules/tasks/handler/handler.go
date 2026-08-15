package handler

import (
	"errors"
	"net/http"

	"andreypostal.com/cubby/server/pkg/core"
	"github.com/gin-gonic/gin"
)

func fetchProfile(c *gin.Context) (core.Profile, error) {
	profile, ok := core.GetProfile(c)
	if !ok {
		return core.Profile{}, errors.New("not able to retrieve user info")
	}

	if profile.Workspace == nil {
		return core.Profile{}, errors.New("profile must have a workspace")
	}

	return profile, nil
}

func respondServiceError(c *gin.Context, message string, err error) {
	if errors.Is(err, core.ErrNotAllowed) {
		core.RespondError(c, core.NewError(http.StatusForbidden, message, nil))
		return
	}

	core.RespondError(c, core.NewError(http.StatusBadRequest, message, map[string]any{
		"reason": err.Error(),
	}))
}
