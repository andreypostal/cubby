package backup

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"andreypostal.com/cubby/server/pkg/modules/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A handler that holds the system has to answer well inside this, whatever the
// freeze timeouts are. Blowing through it means it is waiting on itself.
const handlerDeadline = 15 * time.Second

// router mounts the routes the module exposes behind the freeze gate, which is
// what every request goes through on the running server
func (e *testEnv) router() *gin.Engine {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(middleware.FreezeGate(e.sp.freezer))

	r.POST("/backup", e.sp.handleGenerate)
	r.GET("/backups", e.sp.handleList)
	r.GET("/backups/:name", e.sp.handleDownload)
	r.DELETE("/backups/:name", e.sp.handleDelete)
	r.POST("/backups/:name/restore", e.sp.handleRestore)

	return r
}

// call runs a request and gives up rather than hanging, so a handler that
// freezes while still holding its own gate slot fails the test instead of
// stalling it until the freeze times out
func (e *testEnv) call(t *testing.T, method, target string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	done := make(chan struct{})

	go func() {
		defer close(done)
		e.router().ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	}()

	select {
	case <-done:
		return rec
	case <-time.After(handlerDeadline):
		t.Fatalf("%s %s did not answer within %s, the handler is waiting on the gate slot it holds itself", method, target, handlerDeadline)
		return nil
	}
}

// shortenFreezeWait keeps a handler that cannot hold the system from sitting on
// the default timeout, and makes one that never released the gate fail fast
func shortenFreezeWait(t *testing.T) {
	t.Helper()

	generate, restore := generateTimeout, restoreTimeout
	generateTimeout, restoreTimeout = 200*time.Millisecond, 200*time.Millisecond

	t.Cleanup(func() { generateTimeout, restoreTimeout = generate, restore })
}

func restorePath(name string) string {
	return "/backups/" + url.PathEscape(name) + "/restore"
}

// Generating is a POST, so the request holds a write slot on the gate and has
// to hand it back before asking to freeze
func TestGenerateOverTheGateAnswersWithTheArchive(t *testing.T) {
	env := newTestEnv(t)
	shortenFreezeWait(t)
	env.writeNote(t, "archived")

	rec := env.call(t, http.MethodPost, "/backup")

	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	saved := env.archives(t)
	require.Len(t, saved, 1)
	assert.Contains(t, rec.Header().Get("Content-Disposition"), saved[0])
	assert.Equal(t, "application/zip", rec.Header().Get("Content-Type"))

	// The body is the archive that was just written, not a description of it
	body := rec.Body.Bytes()
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	require.NoError(t, err)
	assert.Contains(t, zipContents(t, zr.File), env.sp.db.Filename())
}

// The system is handed back once the response is out, so the next request is
// served instead of being turned away by a freeze nobody released
func TestGenerateOverTheGateUnfreezesTheSystem(t *testing.T) {
	env := newTestEnv(t)
	shortenFreezeWait(t)

	require.Equal(t, http.StatusOK, env.call(t, http.MethodPost, "/backup").Code)
	require.Equal(t, http.StatusOK, env.call(t, http.MethodPost, "/backup").Code)

	assert.Len(t, env.archives(t), 2)
}

func TestRestoreOverTheGateSwapsTheLiveData(t *testing.T) {
	env := newTestEnv(t)
	shortenFreezeWait(t)
	env.writeNote(t, "archived")
	env.writeIndexFile(t, "1/segment.dat", "archived index")

	archive, err := env.sp.generate()
	require.NoError(t, err)

	env.writeNote(t, "written after the backup")

	rec := env.call(t, http.MethodPost, restorePath(filepath.Base(archive)))

	require.Equal(t, http.StatusNoContent, rec.Code, "body: %s", rec.Body.String())
	assert.Equal(t, []string{"archived"}, env.notes(t))
	assert.Equal(t, "archived index", env.readIndexFile(t, "1/segment.dat"))
}

// A write that has not finished is what the freeze waits on, and the wait is
// bounded so the client is told to come back rather than left hanging
func TestFreezingHandlersGiveUpWhileAWriteIsInflight(t *testing.T) {
	tests := []struct {
		name   string
		target func(env *testEnv) (string, string)
	}{
		{
			name: "generate",
			target: func(*testEnv) (string, string) {
				return http.MethodPost, "/backup"
			},
		},
		{
			name: "restore",
			target: func(env *testEnv) (string, string) {
				archive, err := env.sp.generate()
				require.NoError(t, err)
				return http.MethodPost, restorePath(filepath.Base(archive))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			shortenFreezeWait(t)
			env.writeNote(t, "live")

			method, target := tc.target(env)
			before := env.archives(t)

			// Stands in for another request still writing when the freeze starts
			leave, err := env.sp.freezer.Enter()
			require.NoError(t, err)
			defer leave()

			rec := env.call(t, method, target)

			assert.Equal(t, http.StatusConflict, rec.Code)
			assert.Contains(t, rec.Body.String(), "the system is busy")

			// Nothing was written and the freeze was let go on the way out
			assert.Equal(t, before, env.archives(t))
			leave()
			assert.Equal(t, http.StatusOK, env.call(t, http.MethodGet, "/backups").Code)
		})
	}
}

// The archive name comes from the client, and it only ever names a file in the
// backup folder
func TestRestoreRejectsANameThatIsNotAnArchive(t *testing.T) {
	env := newTestEnv(t)
	shortenFreezeWait(t)

	tests := []struct {
		name string
		want int
	}{
		// A separator inside the name never gets past the handler
		{name: "..%5C..%5Cwindows%5Cwin.ini", want: http.StatusBadRequest},
		// An escaped slash is not even routed, the path stops matching
		{name: "..%2F..%2Fetc%2Fpasswd", want: http.StatusNotFound},
		{name: "notes.txt", want: http.StatusBadRequest},
		{name: "cubby-backup-20200101-000000.000.zip", want: http.StatusNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := env.call(t, http.MethodPost, "/backups/"+tc.name+"/restore")

			assert.Equal(t, tc.want, rec.Code)
			// The live database was never closed for a request that got nowhere
			assert.Equal(t, []string{}, env.notes(t))
		})
	}
}

// Listing is what the admin screen reloads after every action
func TestListReportsTheArchivesAndTheRetention(t *testing.T) {
	env := newTestEnv(t)
	env.sp.keep = 3

	first, err := env.sp.generate()
	require.NoError(t, err)
	second, err := env.sp.generate()
	require.NoError(t, err)

	rec := env.call(t, http.MethodGet, "/backups")
	require.Equal(t, http.StatusOK, rec.Code)

	var got listResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))

	assert.Equal(t, 3, got.Keep)
	require.Len(t, got.Backups, 2)

	// Newest first, which is the order the screen shows them in
	assert.Equal(t, filepath.Base(second), got.Backups[0].Name)
	assert.Equal(t, filepath.Base(first), got.Backups[1].Name)
	assert.NotZero(t, got.Backups[0].Size)
	assert.False(t, got.Backups[0].CreatedAt.IsZero())
}

func TestDownloadAndDeleteWorkOnAGeneratedArchive(t *testing.T) {
	env := newTestEnv(t)
	env.writeNote(t, "archived")

	archive, err := env.sp.generate()
	require.NoError(t, err)
	name := filepath.Base(archive)

	rec := env.call(t, http.MethodGet, "/backups/"+name)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Disposition"), name)

	body := rec.Body.Bytes()
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	require.NoError(t, err)
	assert.Contains(t, zipContents(t, zr.File), env.sp.db.Filename())

	require.Equal(t, http.StatusNoContent, env.call(t, http.MethodDelete, "/backups/"+name).Code)
	assert.Empty(t, env.archives(t))

	assert.Equal(t, http.StatusNotFound, env.call(t, http.MethodDelete, "/backups/"+name).Code)
}

// The gate turns reads away only while a handler holds the system exclusively,
// which is what restore does and backup does not
func TestOnlyRestoreLocksOutReaders(t *testing.T) {
	env := newTestEnv(t)

	unfreeze, err := env.sp.freezer.Freeze(t.Context(), "backup")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, env.call(t, http.MethodGet, "/backups").Code)
	unfreeze()

	unfreezeAll, err := env.sp.freezer.FreezeAll(t.Context(), "restore")
	require.NoError(t, err)
	assert.Equal(t, http.StatusLocked, env.call(t, http.MethodGet, "/backups").Code)
	unfreezeAll()

	assert.Equal(t, http.StatusOK, env.call(t, http.MethodGet, "/backups").Code)
}
