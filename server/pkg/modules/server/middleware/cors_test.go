package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"andreypostal.com/cubby/server/pkg/core"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func corsConfig(origins ...string) core.CORSConfig {
	return core.CORSConfig{
		AllowedOrigins: origins,
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Origin", "Authorization"},
		ExposedHeaders: []string{"Content-Disposition"},
		MaxAge:         3600,
	}
}

// corsRequest runs one request through the middleware and reports whether the
// handler behind it was reached
func corsRequest(t *testing.T, cfg core.CORSConfig, method, origin string) (*httptest.ResponseRecorder, bool) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	reached := false
	router := gin.New()
	router.Use(CORS(cfg))
	router.Handle(method, "/things", func(c *gin.Context) {
		reached = true
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(method, "/things", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return rec, reached
}

// Same-origin requests carry no Origin header and must pass through untouched
func TestARequestWithoutAnOriginIsNotACORSRequest(t *testing.T) {
	rec, reached := corsRequest(t, corsConfig("https://app.example.com"), http.MethodGet, "")

	assert.True(t, reached)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestAnAllowedOriginIsEchoedBack(t *testing.T) {
	cfg := corsConfig("https://app.example.com", "https://admin.example.com")

	rec, reached := corsRequest(t, cfg, http.MethodGet, "https://admin.example.com")

	assert.True(t, reached)
	assert.Equal(t, "https://admin.example.com", rec.Header().Get("Access-Control-Allow-Origin"))
	// The response varies per origin, so caches have to be told
	assert.Equal(t, "Origin", rec.Header().Get("Vary"))
	assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
	assert.Equal(t, "GET,POST", rec.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Origin,Authorization", rec.Header().Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "Content-Disposition", rec.Header().Get("Access-Control-Expose-Headers"))
}

func TestAnUnknownOriginNeverReachesTheHandler(t *testing.T) {
	rec, reached := corsRequest(t, corsConfig("https://app.example.com"), http.MethodGet, "https://evil.example.com")

	assert.False(t, reached)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

// Matching is exact: a subdomain of an allowed host is still another origin
func TestOriginMatchingIsExact(t *testing.T) {
	cfg := corsConfig("https://app.example.com")

	for _, origin := range []string{
		"https://sub.app.example.com",
		"http://app.example.com",
		"https://app.example.com:8443",
		"https://app.example.com/",
	} {
		rec, reached := corsRequest(t, cfg, http.MethodGet, origin)
		assert.False(t, reached, "%s should not have been allowed", origin)
		assert.Equal(t, http.StatusForbidden, rec.Code, origin)
	}
}

func TestTheWildcardAllowsEveryOrigin(t *testing.T) {
	rec, reached := corsRequest(t, corsConfig("*"), http.MethodGet, "https://anywhere.example.com")

	assert.True(t, reached)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, rec.Header().Get("Vary"))
}

// A preflight is answered by the middleware itself and never runs the handler
func TestAPreflightIsAnsweredWithoutTheHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reached := false
	router := gin.New()
	router.Use(CORS(corsConfig("https://app.example.com")))
	router.POST("/things", func(c *gin.Context) { reached = true })

	req := httptest.NewRequest(http.MethodOptions, "/things", nil)
	req.Header.Set("Origin", "https://app.example.com")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.False(t, reached)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "3600", rec.Header().Get("Access-Control-Max-Age"))
	assert.Equal(t, "https://app.example.com", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestExposedHeadersAreOmittedWhenThereAreNone(t *testing.T) {
	cfg := corsConfig("*")
	cfg.ExposedHeaders = nil

	rec, _ := corsRequest(t, cfg, http.MethodGet, "https://app.example.com")

	_, present := rec.Header()["Access-Control-Expose-Headers"]
	assert.False(t, present)
}

// itoa is hand rolled, so it is checked against the standard library
func TestItoaMatchesStrconv(t *testing.T) {
	for _, n := range []int{0, 1, 7, 10, 99, 100, 3600, 86400, 1234567890} {
		assert.Equal(t, strconv.Itoa(n), itoa(n))
	}
}
