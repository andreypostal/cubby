package setup

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/auth/models"
	"andreypostal.com/cubby/server/pkg/modules/auth/repository"
	"andreypostal.com/cubby/server/pkg/modules/auth/service"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newBootstrap builds the module over a database of its own, already migrated
// and holding whatever users the test asked for
func newBootstrap(t *testing.T) *Bootstrap {
	t.Helper()

	// The dsn is a file: URI, so the drive letter has to come with forward slashes
	dataDir := path.Join(filepath.ToSlash(t.TempDir()), "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o755))

	registry := core.NewRegistry()
	registry.Register(new(core.DB))

	ctx := core.NewContext(registry, core.Config{
		DataDir: dataDir,
		DB: core.DBConfig{
			MaxOpenConns:    1,
			MaxIdleConns:    1,
			ConnMaxLifetime: time.Minute,
		},
	})

	db, err := core.GetModule[*core.DB](ctx, new(core.DB))
	require.NoError(t, err)

	// The database has to let go of the files before TempDir removes them
	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Session{}))

	userRepo := repository.NewUserRepository(db)

	return &Bootstrap{
		userRepo: userRepo,
		authSvc: service.NewAuthService(
			userRepo,
			repository.NewSessionRepository(db),
			core.BCryptHasher{},
			new(core.JWT),
		),
		validate: validator.New(),
	}
}

// call runs a request through an engine built from the module's own routes
func call(t *testing.T, b *Bootstrap, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)

	engine := gin.New()
	routes, err := b.Routes()
	require.NoError(t, err)

	for _, route := range routes {
		engine.Handle(route.Method, route.Path, route.Handlers...)
	}

	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")

	res := httptest.NewRecorder()
	engine.ServeHTTP(res, req)

	return res
}

func status(t *testing.T, res *httptest.ResponseRecorder) statusResponse {
	t.Helper()

	var out statusResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &out))

	return out
}

func users(t *testing.T, b *Bootstrap) []*models.User {
	t.Helper()

	list, err := b.userRepo.List()
	require.NoError(t, err)

	return list
}

// A server nobody has set up says so, which is what sends the UI to the setup
// screen instead of a sign-in form nobody has an account for
func TestStatusReportsAServerWithoutUsers(t *testing.T) {
	b := newBootstrap(t)

	res := call(t, b, http.MethodGet, "/setup/status", "")
	require.Equal(t, http.StatusOK, res.Code)

	assert.True(t, status(t, res).Required)
}

// The status is read from the database, so a user created through the CLI while
// the server runs closes the setup phase just as well as the endpoint would
func TestStatusStopsAskingOnceAUserExists(t *testing.T) {
	b := newBootstrap(t)
	require.NoError(t, b.Start())

	require.NoError(t, b.authSvc.CreateUser("Admin", "admin", "s3cret-enough", true))

	res := call(t, b, http.MethodGet, "/setup/status", "")
	assert.False(t, status(t, res).Required)
}

// The token is the whole credential, so the endpoint that hands it any weight
// must never be the one giving it away
func TestStatusNeverCarriesTheToken(t *testing.T) {
	b := newBootstrap(t)
	require.NoError(t, b.Start())
	require.NotEmpty(t, b.token)

	res := call(t, b, http.MethodGet, "/setup/status", "")

	assert.NotContains(t, res.Body.String(), b.token)
}

// The point of the token is that only whoever can read the server's log gets to
// claim it, so a wrong one creates nothing
func TestAWrongTokenCreatesNobody(t *testing.T) {
	b := newBootstrap(t)
	require.NoError(t, b.Start())

	res := call(t, b, http.MethodPost, "/setup",
		`{"token":"not-the-token","name":"Mallory","username":"mallory","password":"s3cret-enough"}`)

	assert.Equal(t, http.StatusForbidden, res.Code)
	assert.Empty(t, users(t, b))

	// The refusal did not burn the real token either
	assert.NotEmpty(t, b.token)
}

// The first user administers the server: there is nobody else who could promote
// them afterwards
func TestTheFirstUserIsCreatedAsASystemAdmin(t *testing.T) {
	b := newBootstrap(t)
	require.NoError(t, b.Start())

	res := call(t, b, http.MethodPost, "/setup",
		`{"token":"`+b.token+`","name":"Ada","username":"ada","password":"s3cret-enough"}`)
	require.Equal(t, http.StatusNoContent, res.Code)

	created := users(t, b)
	require.Len(t, created, 1)

	assert.Equal(t, "ada", created[0].Username)
	assert.Equal(t, "Ada", created[0].Name)
	assert.True(t, created[0].IsSystemAdmin)

	// The password is stored as a hash, never as what was typed
	assert.NotEqual(t, "s3cret-enough", created[0].Password)
	assert.NoError(t, core.BCryptHasher{}.Verify("s3cret-enough", created[0].Password))
}

// The token buys exactly one administrator. Replaying it must not open a second
// account on a server that is now live.
func TestTheTokenCannotBeUsedTwice(t *testing.T) {
	b := newBootstrap(t)
	require.NoError(t, b.Start())
	token := b.token

	first := call(t, b, http.MethodPost, "/setup",
		`{"token":"`+token+`","name":"Ada","username":"ada","password":"s3cret-enough"}`)
	require.Equal(t, http.StatusNoContent, first.Code)

	second := call(t, b, http.MethodPost, "/setup",
		`{"token":"`+token+`","name":"Mallory","username":"mallory","password":"s3cret-enough"}`)

	assert.Equal(t, http.StatusConflict, second.Code)
	assert.Len(t, users(t, b), 1)
}

// A server that already had users never announces a token, and without one
// there is nothing anybody could be holding
func TestAServerAlreadySetUpMintsNoToken(t *testing.T) {
	b := newBootstrap(t)
	require.NoError(t, b.authSvc.CreateUser("Admin", "admin", "s3cret-enough", true))

	require.NoError(t, b.Start())
	assert.Empty(t, b.token)

	res := call(t, b, http.MethodPost, "/setup",
		`{"token":"","name":"Mallory","username":"mallory","password":"s3cret-enough"}`)

	// An empty token is refused as a bad request before it can match the empty
	// one held in memory
	assert.NotEqual(t, http.StatusNoContent, res.Code)
	assert.Len(t, users(t, b), 1)
}

// Restarting rotates the token, so one that leaked out of an old log is worth
// nothing afterwards
func TestARestartIssuesANewToken(t *testing.T) {
	b := newBootstrap(t)

	require.NoError(t, b.Start())
	first := b.token

	require.NoError(t, b.Start())

	assert.NotEmpty(t, first)
	assert.NotEqual(t, first, b.token)
}

// A password too short to be worth anything is refused before it reaches the
// only account that administers the server
func TestAWeakPasswordIsRefused(t *testing.T) {
	b := newBootstrap(t)
	require.NoError(t, b.Start())

	res := call(t, b, http.MethodPost, "/setup",
		`{"token":"`+b.token+`","name":"Ada","username":"ada","password":"short"}`)

	assert.Equal(t, http.StatusUnprocessableEntity, res.Code)
	assert.Empty(t, users(t, b))
}

// Both routes answer before anybody can possibly hold a session, so they have
// to stay public
func TestTheSetupRoutesArePublic(t *testing.T) {
	routes, err := (&Bootstrap{}).Routes()
	require.NoError(t, err)

	require.Len(t, routes, 2)
	for _, route := range routes {
		assert.Equal(t, "public", route.AuthScope, "%s %s", route.Method, route.Path)
	}
}
