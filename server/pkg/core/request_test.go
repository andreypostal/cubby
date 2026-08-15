package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromStringToUintRoundTrip(t *testing.T) {
	for _, n := range []uint{0, 1, 42, 4294967295} {
		back, err := FromStringToUint(FromUintToString(n))
		require.NoError(t, err)
		assert.Equal(t, n, back)
	}
}

// Route params arrive as text, and anything that is not a plain positive number
// has to be an error rather than a silent zero
func TestFromStringToUintRejectsAnythingElse(t *testing.T) {
	for _, in := range []string{"", "-1", "1.5", "12abc", " 12", "0x10"} {
		n, err := FromStringToUint(in)
		assert.Error(t, err, "%q should not have parsed", in)
		assert.Zero(t, n)
	}
}

func TestProfileIsReadBackFromTheRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	// Nothing set yet: the middleware relies on this to know it has to authenticate
	_, ok := GetProfile(c)
	assert.False(t, ok)

	profile := Profile{
		SessionCode: "session-code",
		User:        ProfileUser{ID: 7, Username: "alice", Name: "Alice"},
		Workspace:   &ProfileWorkspace{ID: 3, Name: "Home", IsCreator: true},
	}
	SetProfile(c, profile)

	stored, ok := GetProfile(c)
	require.True(t, ok)
	assert.Equal(t, profile, stored)
}

func TestRespondErrorWritesItsOwnStatusAndBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	RespondError(c, NewError(http.StatusConflict, "already there", map[string]any{"name": "taken"}))

	assert.Equal(t, http.StatusConflict, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "already there", body["message"])
	assert.Equal(t, map[string]any{"name": "taken"}, body["details"])
}

// The status is not part of the payload, and an error without details does not
// carry an empty object into the response
func TestErrorWithoutDetailsOmitsThem(t *testing.T) {
	payload, err := json.Marshal(NewError(http.StatusBadRequest, "nope", nil))
	require.NoError(t, err)

	assert.JSONEq(t, `{"message":"nope"}`, string(payload))
}

type signup struct {
	Name  string `validate:"required"`
	Email string `validate:"required,email"`
	Plan  string `validate:"oneof=free pro"`
	Pass  string `validate:"min=8"`
}

func TestValidationErrorNamesEveryFieldThatFailed(t *testing.T) {
	err := validator.New().Struct(signup{Email: "not-an-email", Plan: "enterprise", Pass: "short"})
	require.Error(t, err)

	ve := ValidationError(err)

	assert.Equal(t, http.StatusUnprocessableEntity, ve.httpStatus)
	assert.Equal(t, "validation failed", ve.Message)

	assert.Equal(t, "Name is required", ve.Details["Name"])
	assert.Equal(t, "Email must be a valid email address", ve.Details["Email"])
	assert.Equal(t, "Plan must be one of: free pro", ve.Details["Plan"])
	assert.Equal(t, "Pass must be at least 8 characters", ve.Details["Pass"])
}

// A tag without a message of its own still says which field is at fault
func TestValidationErrorFallsBackToAGenericMessage(t *testing.T) {
	type payload struct {
		Age int `validate:"gte=18"`
	}

	ve := ValidationError(validator.New().Struct(payload{Age: 5}))
	assert.Equal(t, "Age is invalid", ve.Details["Age"])
}

// An error that is not a validation failure still produces a usable response
// instead of panicking on the type assertion
func TestValidationErrorToleratesAnUnrelatedError(t *testing.T) {
	ve := ValidationError(ErrBadRequest)

	assert.Equal(t, http.StatusUnprocessableEntity, ve.httpStatus)
	assert.Empty(t, ve.Details)
}
