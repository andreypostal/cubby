package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gateRouter wires the gate in front of a handler that reports it was reached
func gateRouter(f *core.Freezer, handler gin.HandlerFunc) (*gin.Engine, *bool) {
	gin.SetMode(gin.TestMode)

	reached := false
	router := gin.New()
	router.Use(FreezeGate(f))

	wrapped := func(c *gin.Context) {
		reached = true
		if handler != nil {
			handler(c)
		}
		c.Status(http.StatusOK)
	}

	router.GET("/things", wrapped)
	router.POST("/things", wrapped)

	return router, &reached
}

func do(router *gin.Engine, method string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(method, "/things", nil))
	return rec
}

func newFreezer() *core.Freezer {
	return new(core.Freezer).New().(*core.Freezer)
}

func TestRequestsPassThroughAnUnfrozenGate(t *testing.T) {
	router, reached := gateRouter(newFreezer(), nil)

	assert.Equal(t, http.StatusOK, do(router, http.MethodGet).Code)
	assert.True(t, *reached)

	*reached = false
	assert.Equal(t, http.StatusOK, do(router, http.MethodPost).Code)
	assert.True(t, *reached)
}

// A backup freeze stops writes while the app stays readable
func TestAFrozenGateRefusesWritesButServesReads(t *testing.T) {
	f := newFreezer()

	unfreeze, err := f.Freeze(context.Background(), "backup")
	require.NoError(t, err)
	defer unfreeze()

	router, reached := gateRouter(f, nil)

	assert.Equal(t, http.StatusLocked, do(router, http.MethodPost).Code)
	assert.False(t, *reached, "a write ran while the system was frozen")

	assert.Equal(t, http.StatusOK, do(router, http.MethodGet).Code)
	assert.True(t, *reached)
}

// A restore freeze takes the whole app offline, reads included
func TestAnExclusiveFreezeRefusesEverything(t *testing.T) {
	f := newFreezer()

	unfreeze, err := f.FreezeAll(context.Background(), "restore")
	require.NoError(t, err)
	defer unfreeze()

	router, reached := gateRouter(f, nil)

	assert.Equal(t, http.StatusLocked, do(router, http.MethodGet).Code)
	assert.Equal(t, http.StatusLocked, do(router, http.MethodPost).Code)
	assert.False(t, *reached)
}

// The gate has to release its slot even when the handler is the one refusing,
// otherwise a freeze would wait on a request that already ended
func TestTheGateReleasesItsSlotWhenTheHandlerReturns(t *testing.T) {
	f := newFreezer()

	router, _ := gateRouter(f, nil)
	do(router, http.MethodPost)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	unfreeze, err := f.Freeze(ctx, "backup")
	require.NoError(t, err, "the gate kept the request slot after the handler returned")
	unfreeze()
}

// A handler that starts a long job leaves the gate early, and the gate leaving
// again on the way out must not double release the slot
func TestLeavingTheGateEarlyIsSafe(t *testing.T) {
	f := newFreezer()

	router, reached := gateRouter(f, func(c *gin.Context) {
		LeaveFreezeGate(c)
		// Handlers that finish their own work call it once; the middleware then
		// calls it again through its defer
		LeaveFreezeGate(c)
	})

	assert.Equal(t, http.StatusOK, do(router, http.MethodPost).Code)
	assert.True(t, *reached)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	unfreeze, err := f.Freeze(ctx, "backup")
	require.NoError(t, err)
	unfreeze()
}

// Nothing to leave is not an error: the helper is called from handlers that may
// run without the gate in front of them
func TestLeavingAGateThatWasNeverEnteredDoesNothing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	assert.NotPanics(t, func() { LeaveFreezeGate(c) })
}

func TestOnlyUnsafeMethodsCountAsWrites(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		assert.False(t, isMutating(method), "%s should be a read", method)
	}

	for _, method := range []string{
		http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete,
	} {
		assert.True(t, isMutating(method), "%s should be a write", method)
	}
}
