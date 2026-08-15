package setup

import (
	"crypto/subtle"
	"log/slog"
	"net/http"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/server/middleware"
	"github.com/gin-gonic/gin"
)

type statusResponse struct {
	Required bool `json:"required"`
}

// completeRequest creates the first system admin. The token proves the caller
// can read the server's log, which is the only credential a server without
// users has to offer.
type completeRequest struct {
	Token    string `json:"token" validate:"required"`
	Name     string `json:"name" validate:"required,min=2,max=100"`
	Username string `json:"username" validate:"required,min=2,max=100"`
	Password string `json:"password" validate:"required,min=8,max=100"`
}

func (b *Bootstrap) handleStatus(c *gin.Context) {
	required, err := b.setupRequired()
	if err != nil {
		middleware.GetLogger(c).Error("Failed to read the setup status", slog.String("error", err.Error()))
		core.RespondError(c, core.NewError(http.StatusInternalServerError, "failed to read the setup status", nil))
		return
	}

	c.JSON(http.StatusOK, statusResponse{Required: required})
}

func (b *Bootstrap) handleComplete(c *gin.Context) {
	logger := middleware.GetLogger(c)

	var req completeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	if err := b.validate.Struct(req); err != nil {
		core.RespondError(c, core.ValidationError(err))
		return
	}

	// Held across the whole check-and-create, to avoid racing setups
	b.mu.Lock()
	defer b.mu.Unlock()

	required, err := b.setupRequired()
	if err != nil {
		logger.Error("Failed to read the setup status", slog.String("error", err.Error()))
		core.RespondError(c, core.NewError(http.StatusInternalServerError, "failed to read the setup status", nil))
		return
	}

	if !required || b.token == "" {
		core.RespondError(c, core.NewError(http.StatusConflict, "this server is already set up", nil))
		return
	}

	if subtle.ConstantTimeCompare([]byte(req.Token), []byte(b.token)) != 1 {
		logger.Warn("Rejected a setup attempt carrying the wrong token")
		core.RespondError(c, core.NewError(http.StatusForbidden, "invalid setup token", nil))
		return
	}

	// The first user administers the server, there is nobody else to promote them
	if err := b.authSvc.CreateUser(req.Name, req.Username, req.Password, true); err != nil {
		logger.Error("Failed to create the first administrator", slog.String("error", err.Error()))
		core.RespondError(c, core.NewError(http.StatusInternalServerError, "failed to create the first administrator", nil))
		return
	}

	b.token = ""

	slog.Info("Setup completed", slog.String("username", req.Username))

	c.Status(http.StatusNoContent)
}
