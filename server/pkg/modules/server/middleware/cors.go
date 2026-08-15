package middleware

import (
	"net/http"
	"slices"
	"strings"

	"andreypostal.com/cubby/server/pkg/core"
	"github.com/gin-gonic/gin"
)

func CORS(cfg core.CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		// Non-CORS request — skip.
		if origin == "" {
			c.Next()
			return
		}

		// Validate origin.
		allowAll := slices.Contains(cfg.AllowedOrigins, "*")
		if !allowAll && !isOriginAllowed(origin, cfg.AllowedOrigins) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		// Reflect the specific origin rather than echoing "*" when credentials
		// are enabled, since browsers reject "*" + credentials.
		if allowAll {
			c.Header("Access-Control-Allow-Origin", "*")
		} else {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		}

		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ","))
		c.Header("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ","))

		if len(cfg.ExposedHeaders) > 0 {
			c.Header("Access-Control-Expose-Headers", strings.Join(cfg.ExposedHeaders, ","))
		}

		// Handle preflight.
		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Max-Age", itoa(cfg.MaxAge))
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// isOriginAllowed checks whether the given origin matches any entry in the
// allowed list. Matching is exact; wildcard subdomains are not supported.
func isOriginAllowed(origin string, allowed []string) bool {
	return slices.Contains(allowed, origin)
}

// itoa converts an int to its decimal string without importing strconv
// (avoids an extra import just for this small helper).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}
