package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// idRouter runs one request through the middleware and reports the id the
// handler behind it saw
func idRouter(t *testing.T, header string) (*httptest.ResponseRecorder, string) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	var seen string
	router := gin.New()
	router.Use(RequestID())
	router.GET("/things", func(c *gin.Context) {
		seen = GetRequestID(c)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/things", nil)
	if header != "" {
		req.Header.Set(RequestIDHeader, header)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return rec, seen
}

func TestARequestWithoutAnIdGetsOneGenerated(t *testing.T) {
	rec, seen := idRouter(t, "")

	assert.NotEmpty(t, seen)
	// Echoed back so the caller can quote it when reporting a problem
	assert.Equal(t, seen, rec.Header().Get(RequestIDHeader))
}

func TestEachRequestGetsItsOwnId(t *testing.T) {
	_, first := idRouter(t, "")
	_, second := idRouter(t, "")

	assert.NotEqual(t, first, second)
}

// A proxy that already tracks an id keeps it, which is the point of reading the
// header at all
func TestAnIdSentByTheCallerIsKept(t *testing.T) {
	rec, seen := idRouter(t, "trace-abc.123_XYZ")

	assert.Equal(t, "trace-abc.123_XYZ", seen)
	assert.Equal(t, "trace-abc.123_XYZ", rec.Header().Get(RequestIDHeader))
}

func TestAnUnusableIdFromTheCallerIsReplaced(t *testing.T) {
	cases := map[string]string{
		"blank":            "   ",
		"too long":         strings.Repeat("a", maxRequestIDLen+1),
		"newline":          "abc\ndef",
		"space inside":     "abc def",
		"log line forgery": `abc" level=ERROR msg="forged`,
	}

	for name, header := range cases {
		t.Run(name, func(t *testing.T) {
			_, seen := idRouter(t, header)

			assert.NotEmpty(t, seen)
			assert.NotEqual(t, strings.TrimSpace(header), seen)
			// Whatever came in must not survive into the logs
			assert.NotContains(t, seen, " ")
			assert.NotContains(t, seen, "\n")
			assert.LessOrEqual(t, len(seen), maxRequestIDLen)
		})
	}
}

// An id nobody can read the request line with is worth nothing, so both the
// handler's own lines and the request line have to carry the same one
func TestTheIdReachesBothTheHandlerLoggerAndTheRequestLine(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, nil))

	previous := slog.Default()
	slog.SetDefault(base)
	t.Cleanup(func() { slog.SetDefault(previous) })

	router := gin.New()
	router.Use(RequestID())
	router.Use(InjectLogger(base))
	router.Use(Logger())
	router.GET("/things", func(c *gin.Context) {
		GetLogger(c).Error("handler failed")
		c.Status(http.StatusInternalServerError)
	})

	req := httptest.NewRequest(http.MethodGet, "/things", nil)
	req.Header.Set(RequestIDHeader, "shared-id")

	router.ServeHTTP(httptest.NewRecorder(), req)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 2, "expected the handler line and the request line")

	for _, line := range lines {
		var entry map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &entry))
		assert.Equal(t, "shared-id", entry["request_id"], "line without the id: %s", line)
	}
}

// The gate, the fallback and anything running outside the chain must not panic
// for want of an id
func TestGetRequestIDIsEmptyWithoutTheMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	assert.Empty(t, GetRequestID(c))
}
