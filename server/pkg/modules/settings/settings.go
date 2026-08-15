package settings

import (
	"crypto/tls"
	"net/http"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	api "andreypostal.com/cubby/server/pkg/modules/server"
	"github.com/gin-gonic/gin"
)

type Settings struct {
	certFile string
	keyFile  string

	// material is what the API serves with, nil when no certificate was configured
	material *tlsMaterial
}

// tlsMaterial is the loaded certificate and when it was last written
type tlsMaterial struct {
	cert     *tls.Certificate
	loadedAt time.Time
}

func (s *Settings) Id() string {
	return "settings-service"
}

func (s *Settings) New() any {
	return new(Settings)
}

func (s *Settings) Initialize(ctx *core.Context) error {
	s.certFile = ctx.Config.API.TLS.CertFile
	s.keyFile = ctx.Config.API.TLS.KeyFile

	return s.load()
}

func (s *Settings) Routes() ([]api.Route, error) {
	return []api.Route{
		{
			Method:    http.MethodGet,
			Path:      "/settings/tls",
			Handlers:  []gin.HandlerFunc{s.handleTLSStatus},
			AuthScope: api.SystemAdminRouteType,
		},
	}, nil
}

// Certificate implements [api.TLSProvider]
func (s *Settings) Certificate() *tls.Certificate {
	if s.material == nil {
		return nil
	}

	return s.material.cert
}
