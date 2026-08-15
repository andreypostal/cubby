package handler

import (
	"errors"

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
