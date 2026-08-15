package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withAuthHeader builds a request context carrying the given Authorization header
func withAuthHeader(value string) *gin.Context {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	if value != "" {
		c.Request.Header.Set("Authorization", value)
	}

	return c
}

func TestBearerTokenIsPulledOutOfTheHeader(t *testing.T) {
	token, err := extractBearerToken(withAuthHeader("Bearer abc.def.ghi"))

	require.NoError(t, err)
	assert.Equal(t, "abc.def.ghi", token)
}

// The scheme is case insensitive per RFC 7235, and clients do vary
func TestTheBearerSchemeIsCaseInsensitive(t *testing.T) {
	for _, header := range []string{"bearer abc", "BEARER abc", "BeArEr abc"} {
		token, err := extractBearerToken(withAuthHeader(header))
		require.NoError(t, err, header)
		assert.Equal(t, "abc", token)
	}
}

// The token is trimmed, so extra spacing after the scheme is not part of it
func TestSurroundingSpaceIsNotPartOfTheToken(t *testing.T) {
	token, err := extractBearerToken(withAuthHeader("Bearer   abc  "))

	require.NoError(t, err)
	assert.Equal(t, "abc", token)
}

func TestAMalformedHeaderYieldsNoToken(t *testing.T) {
	for _, header := range []string{
		"",               // no header at all
		"abc.def.ghi",    // token without a scheme
		"Basic dXNlcg==", // the wrong scheme
		"Bearer",         // scheme without a token
		"Bearer ",        // scheme with an empty token
		"Bearer    ",     // scheme with only spaces
	} {
		token, err := extractBearerToken(withAuthHeader(header))
		assert.Error(t, err, "%q should not have produced a token", header)
		assert.Empty(t, token)
	}
}

// A token holding spaces is taken whole, since the split is limited to the
// scheme and the rest of the line
func TestTheTokenIsEverythingAfterTheScheme(t *testing.T) {
	token, err := extractBearerToken(withAuthHeader("Bearer one two"))

	require.NoError(t, err)
	assert.Equal(t, "one two", token)
}
