package backup

import (
	"archive/zip"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testEnv is a backup service wired to a data and a backup folder of its own
type testEnv struct {
	sp      *SimpleBackup
	dataDir string
	saveDir string
}

// newTestEnv builds the module the same way the server does, over a registry
// holding only what the backup service reaches for
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	// The dsn is a file: URI, so the drive letter has to come with forward slashes
	root := filepath.ToSlash(t.TempDir())
	dataDir := path.Join(root, "data")
	saveDir := path.Join(root, "backups")

	require.NoError(t, os.MkdirAll(dataDir, 0o755))
	require.NoError(t, os.MkdirAll(saveDir, 0o755))

	registry := core.NewRegistry()
	registry.Register(new(core.DB))
	registry.Register(new(core.Freezer))
	registry.Register(new(SimpleBackup))

	ctx := core.NewContext(registry, core.Config{
		DataDir:   dataDir,
		BackupDir: saveDir,
		// Retention is off unless a test asks for it
		BackupKeep: 0,
		DB: core.DBConfig{
			MaxOpenConns:    1,
			MaxIdleConns:    1,
			ConnMaxLifetime: time.Minute,
		},
	})

	sp, err := core.GetModule[*SimpleBackup](ctx, new(SimpleBackup))
	require.NoError(t, err)

	// The database has to let go of the files before TempDir removes them
	t.Cleanup(func() { _ = sp.db.Close() })

	env := &testEnv{sp: sp, dataDir: dataDir, saveDir: saveDir}
	env.createNotes(t)

	return env
}

// createNotes gives the database a table of its own, standing in for whatever
// the modules would have migrated into it
func (e *testEnv) createNotes(t *testing.T) {
	t.Helper()
	require.NoError(t, e.sp.db.Exec("CREATE TABLE notes (body TEXT)").Error)
}

func (e *testEnv) writeNote(t *testing.T, body string) {
	t.Helper()
	require.NoError(t, e.sp.db.Exec("INSERT INTO notes (body) VALUES (?)", body).Error)
}

func (e *testEnv) notes(t *testing.T) []string {
	t.Helper()

	notes := make([]string, 0)
	require.NoError(t, e.sp.db.Table("notes").Order("body").Pluck("body", &notes).Error)

	return notes
}

// writeIndexFile puts a file where the search indexes live
func (e *testEnv) writeIndexFile(t *testing.T, name, content string) {
	t.Helper()

	p := filepath.Join(e.dataDir, core.IndexerFolder, filepath.FromSlash(name))
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
}

func (e *testEnv) readIndexFile(t *testing.T, name string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(e.dataDir, core.IndexerFolder, filepath.FromSlash(name)))
	require.NoError(t, err)

	return string(content)
}

// archives lists the archive names sitting in the backup folder, oldest first
func (e *testEnv) archives(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(e.saveDir)
	require.NoError(t, err)

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if isBackupArchive(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	return names
}

// archiveContents reads every entry of an archive into a name to content map
func archiveContents(t *testing.T, archivePath string) map[string]string {
	t.Helper()

	zr, err := zip.OpenReader(archivePath)
	require.NoError(t, err)
	defer zr.Close()

	return zipContents(t, zr.File)
}

func zipContents(t *testing.T, files []*zip.File) map[string]string {
	t.Helper()

	contents := make(map[string]string, len(files))
	for _, f := range files {
		if f.FileInfo().IsDir() {
			continue
		}

		rc, err := f.Open()
		require.NoError(t, err)

		body, err := io.ReadAll(rc)
		rc.Close()
		require.NoError(t, err)

		contents[filepath.ToSlash(f.Name)] = string(body)
	}

	return contents
}

// buildArchive writes an archive holding exactly the given entries, which is
// how a handcrafted or damaged upload reaches the restore
func buildArchive(t *testing.T, dest string, entries map[string]string) string {
	t.Helper()

	f, err := os.Create(dest)
	require.NoError(t, err)
	defer f.Close()

	zw := zip.NewWriter(f)
	for name, content := range entries {
		w, err := zw.Create(name)
		require.NoError(t, err)

		_, err = io.WriteString(w, content)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())

	return dest
}

// A generated archive carries the database and everything else in the data
// folder, so a restore has all it needs to rebuild the server state
func TestGenerateArchivesTheDatabaseAndTheIndexes(t *testing.T) {
	env := newTestEnv(t)
	env.writeNote(t, "archived")
	env.writeIndexFile(t, "1/segment.dat", "index payload")

	archive, err := env.sp.generate()
	require.NoError(t, err)

	assert.True(t, isBackupArchive(filepath.Base(archive)), "%q is not named like an archive", archive)
	assert.Equal(t, []string{filepath.Base(archive)}, env.archives(t))

	contents := archiveContents(t, archive)
	assert.Contains(t, contents, env.sp.db.Filename())
	assert.Equal(t, "index payload", contents[core.IndexerFolder+"/1/segment.dat"])

	// The snapshot stands in for the live files, the journals are not archived
	assert.NotContains(t, contents, env.sp.db.Filename()+"-wal")
	assert.NotContains(t, contents, env.sp.db.Filename()+"-shm")
}

// The signing secret sits in the data folder next to everything else that is
// archived. It must never travel with an archive: whoever downloads one would
// be able to forge a session against the server it came from.
func TestGenerateLeavesTheSigningSecretOut(t *testing.T) {
	env := newTestEnv(t)
	env.writeNote(t, "archived")

	secret := filepath.Join(env.dataDir, core.JWTSecretFile)
	require.NoError(t, os.WriteFile(secret, []byte("a-very-secret-value"), 0o600))

	archive, err := env.sp.generate()
	require.NoError(t, err)

	contents := archiveContents(t, archive)
	assert.Contains(t, contents, env.sp.db.Filename())
	assert.NotContains(t, contents, core.JWTSecretFile)

	for name, body := range contents {
		assert.NotContains(t, body, "a-very-secret-value", "entry %q carries the signing secret", name)
	}

	// Nothing removed it from the server that is still running on it
	assert.FileExists(t, secret)
}

// A restore replaces the data, never the identity of the server doing it: the
// live secret stays, so the sessions opened before it survive
func TestRestoreLeavesTheLiveSigningSecretAlone(t *testing.T) {
	env := newTestEnv(t)
	env.writeNote(t, "archived")

	archive, err := env.sp.generate()
	require.NoError(t, err)

	secret := filepath.Join(env.dataDir, core.JWTSecretFile)
	require.NoError(t, os.WriteFile(secret, []byte("the-live-secret"), 0o600))

	require.NoError(t, env.sp.restore(archive))

	stored, err := os.ReadFile(secret)
	require.NoError(t, err)
	assert.Equal(t, "the-live-secret", string(stored))
}

// The snapshot is taken from the live database, not copied off disk, so it has
// to hold what was written right before the backup
func TestGenerateSnapshotsTheCurrentDatabase(t *testing.T) {
	env := newTestEnv(t)
	env.writeNote(t, "written before the backup")

	archive, err := env.sp.generate()
	require.NoError(t, err)

	snapshot := filepath.Join(t.TempDir(), env.sp.db.Filename())
	require.NoError(t, os.WriteFile(snapshot, []byte(archiveContents(t, archive)[env.sp.db.Filename()]), 0o644))

	assert.NoError(t, checkSqliteFile(snapshot))
	assert.NoError(t, core.VerifySqliteFile(snapshot))
}

// An archive taken before a change puts the database back the way it was
func TestRestoreBringsBackTheArchivedState(t *testing.T) {
	env := newTestEnv(t)
	env.writeNote(t, "archived")
	env.writeIndexFile(t, "1/segment.dat", "archived index")

	archive, err := env.sp.generate()
	require.NoError(t, err)

	env.writeNote(t, "written after the backup")
	env.writeIndexFile(t, "1/segment.dat", "index written after the backup")

	require.NoError(t, env.sp.restore(archive))

	assert.Equal(t, []string{"archived"}, env.notes(t))
	assert.Equal(t, "archived index", env.readIndexFile(t, "1/segment.dat"))
}

// The handle is opened again against the restored file, so the database is
// usable without a restart
func TestRestoreLeavesTheDatabaseWritable(t *testing.T) {
	env := newTestEnv(t)
	env.writeNote(t, "archived")

	archive, err := env.sp.generate()
	require.NoError(t, err)

	require.NoError(t, env.sp.restore(archive))

	env.writeNote(t, "written after the restore")
	assert.Equal(t, []string{"archived", "written after the restore"}, env.notes(t))
}

// An archive taken before anything was indexed holds no index files, and the
// live ones go away with the database they described. This is what the restore
// does today: it parks the live indexes and, having none to put back, discards
// them along with the staging folder, so search stays empty until it is fed
// again.
func TestRestoreDropsTheLiveIndexesWhenTheArchiveHasNone(t *testing.T) {
	env := newTestEnv(t)
	env.writeNote(t, "archived")

	archive, err := env.sp.generate()
	require.NoError(t, err)

	env.writeIndexFile(t, "1/segment.dat", "indexed after the backup")

	require.NoError(t, env.sp.restore(archive))

	assert.Equal(t, []string{"archived"}, env.notes(t))

	_, err = os.Stat(filepath.Join(env.dataDir, core.IndexerFolder))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

// Nothing is swapped before the archive is known to be good, so a bad one
// leaves the running server exactly as it was
func TestRestoreRejectsABadArchiveWithoutTouchingTheLiveData(t *testing.T) {
	dbFile := new(core.DB).Filename()

	tests := []struct {
		name    string
		entries map[string]string
		wantErr string
	}{
		{
			name:    "no database entry",
			entries: map[string]string{core.IndexerFolder + "/1/segment.dat": "orphan index"},
			wantErr: "holds no",
		},
		{
			name:    "the database is not sqlite",
			entries: map[string]string{dbFile: "this is not a database"},
			wantErr: "is not a sqlite database",
		},
		{
			name:    "the database is empty",
			entries: map[string]string{dbFile: ""},
			wantErr: "reading the file header",
		},
		{
			name:    "an entry escapes the data folder",
			entries: map[string]string{dbFile: "", "../escaped.txt": "out of bounds"},
			wantErr: "escapes the data folder",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			env.writeNote(t, "live")
			env.writeIndexFile(t, "1/segment.dat", "live index")

			archive := buildArchive(t, filepath.Join(t.TempDir(), "handmade.zip"), tc.entries)

			require.ErrorContains(t, env.sp.restore(archive), tc.wantErr)

			// The database answers, and with what it held before the attempt
			assert.Equal(t, []string{"live"}, env.notes(t))
			assert.Equal(t, "live index", env.readIndexFile(t, "1/segment.dat"))
		})
	}
}

// A zip entry weighing a few kilobytes can decompress into gigabytes. Extraction
// has to stop at the limit instead of writing whatever the entry claims, or a
// handcrafted upload fills the data folder.
func TestExtractFileStopsAtTheLimit(t *testing.T) {
	const limit = 1024

	tests := []struct {
		name    string
		payload string
		wantErr error
	}{
		{name: "under the limit", payload: strings.Repeat("a", limit-1)},
		{name: "exactly the limit", payload: strings.Repeat("a", limit)},
		{name: "over the limit", payload: strings.Repeat("a", limit+1), wantErr: errRestoreTooLarge},
		{name: "far over the limit", payload: strings.Repeat("a", 64*limit), wantErr: errRestoreTooLarge},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			archive := buildArchive(t, filepath.Join(t.TempDir(), "handmade.zip"), map[string]string{
				"payload": tc.payload,
			})

			zr, err := zip.OpenReader(archive)
			require.NoError(t, err)
			defer zr.Close()

			dest := filepath.Join(t.TempDir(), "payload")
			written, err := extractFile(zr.File[0], dest, limit)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)

				// Whatever landed on disk stops at the limit, the entry is never written in full
				info, statErr := os.Stat(dest)
				require.NoError(t, statErr)
				assert.LessOrEqual(t, info.Size(), int64(limit)+1)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, int64(len(tc.payload)), written)

			body, err := os.ReadFile(dest)
			require.NoError(t, err)
			assert.Equal(t, tc.payload, string(body))
		})
	}
}

// The limit covers the archive as a whole, so entries that each fit cannot add
// up to more than one oversized entry is allowed to be
func TestStageSpendsOneBudgetOverTheWholeArchive(t *testing.T) {
	env := newTestEnv(t)
	env.writeNote(t, "archived")

	archive, err := env.sp.generate()
	require.NoError(t, err)

	zr, err := zip.OpenReader(archive)
	require.NoError(t, err)
	defer zr.Close()

	var dbEntry *zip.File
	for _, f := range zr.File {
		if f.Name == env.sp.db.Filename() {
			dbEntry = f
		}
	}
	require.NotNil(t, dbEntry)

	// The database alone already spends the budget, so nothing is left for what follows
	written, err := extractFile(dbEntry, filepath.Join(t.TempDir(), "db"), int64(dbEntry.UncompressedSize64))
	require.NoError(t, err)

	_, err = extractFile(dbEntry, filepath.Join(t.TempDir(), "again"), int64(dbEntry.UncompressedSize64)-written)
	require.ErrorIs(t, err, errRestoreTooLarge)
}

// Staging unpacks next to the live data, so the folder it used has to be gone
// once the restore is over
func TestRestoreCleansUpItsStagingFolder(t *testing.T) {
	env := newTestEnv(t)
	env.writeNote(t, "archived")

	archive, err := env.sp.generate()
	require.NoError(t, err)

	require.NoError(t, env.sp.restore(archive))

	entries, err := os.ReadDir(env.dataDir)
	require.NoError(t, err)

	for _, entry := range entries {
		assert.NotContains(t, entry.Name(), ".restore-", "the staging folder was left behind")
	}
}

// Generating prunes, so the backup folder does not grow forever
func TestGenerateKeepsOnlyTheMostRecentArchives(t *testing.T) {
	env := newTestEnv(t)
	env.sp.keep = 2

	var generated []string
	for range 3 {
		archive, err := env.sp.generate()
		require.NoError(t, err)
		generated = append(generated, filepath.Base(archive))
	}

	// The names carry a timestamp, the oldest is the one that goes
	assert.Equal(t, generated[1:], env.archives(t))
}

// A keep count of zero turns retention off rather than removing everything
func TestGenerateKeepsEveryArchiveWithoutRetention(t *testing.T) {
	env := newTestEnv(t)

	for range 3 {
		_, err := env.sp.generate()
		require.NoError(t, err)
	}

	assert.Len(t, env.archives(t), 3)
}

// The backup folder lives inside the data folder on a default install, and an
// archive must not end up archiving the ones taken before it
func TestGenerateSkipsTheArchivesAlreadyTaken(t *testing.T) {
	env := newTestEnv(t)
	env.sp.saveDir = env.dataDir
	env.writeNote(t, "archived")

	first, err := env.sp.generate()
	require.NoError(t, err)

	second, err := env.sp.generate()
	require.NoError(t, err)

	assert.NotContains(t, archiveContents(t, second), filepath.Base(first))
}
