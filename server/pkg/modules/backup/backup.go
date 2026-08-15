package backup

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	api "andreypostal.com/cubby/server/pkg/modules/server"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
)

type Pausable interface {
	Pause() error
	Resume() error
}

type SimpleBackup struct {
	// ctx is kept to re-run the module migrations after a restore
	ctx *core.Context

	pausables []Pausable
	freezer   *core.Freezer

	dataDir string
	saveDir string
	keep    int

	apiHost string
	apiPort int

	db *core.DB
}

func (sp *SimpleBackup) Id() string {
	return "backup-service"
}

func (sp *SimpleBackup) New() any {
	return new(SimpleBackup)
}

func (sp *SimpleBackup) Initialize(ctx *core.Context) error {
	sp.ctx = ctx
	sp.dataDir = ctx.Config.DataDir
	sp.saveDir = ctx.Config.BackupDir
	sp.keep = ctx.Config.BackupKeep

	sp.apiHost = ctx.Config.API.Host
	sp.apiPort = ctx.Config.API.Port

	freezer, err := core.GetModule[*core.Freezer](ctx, new(core.Freezer))
	if err != nil {
		return fmt.Errorf("getting system freezer: %w", err)
	}
	sp.freezer = freezer

	db, err := ctx.Module(new(core.DB))
	if err != nil {
		return fmt.Errorf("load db: %w", err)
	}
	sp.db = db.(*core.DB)

	pausables, err := ctx.Modules(new(Pausable))
	if err != nil {
		return fmt.Errorf("getting pausables: %w", err)
	}

	for _, p := range pausables {
		sp.pausables = append(sp.pausables, p.(Pausable))
	}

	return nil
}

func (sp *SimpleBackup) Routes() ([]api.Route, error) {
	return []api.Route{
		{
			Method:    http.MethodPost,
			Path:      "/backup",
			Handlers:  []gin.HandlerFunc{sp.handleGenerate},
			AuthScope: api.SystemAdminRouteType,
		},
		{
			Method:    http.MethodGet,
			Path:      "/backups",
			Handlers:  []gin.HandlerFunc{sp.handleList},
			AuthScope: api.SystemAdminRouteType,
		},
		{
			Method:    http.MethodGet,
			Path:      "/backups/:name",
			Handlers:  []gin.HandlerFunc{sp.handleDownload},
			AuthScope: api.SystemAdminRouteType,
		},
		{
			Method:    http.MethodDelete,
			Path:      "/backups/:name",
			Handlers:  []gin.HandlerFunc{sp.handleDelete},
			AuthScope: api.SystemAdminRouteType,
		},
		{
			Method:    http.MethodPost,
			Path:      "/backups/:name/restore",
			Handlers:  []gin.HandlerFunc{sp.handleRestore},
			AuthScope: api.SystemAdminRouteType,
		},
	}, nil
}

// restore replaces the live database and search indexes with the ones held by
// the archive at archivePath
func (sp *SimpleBackup) restore(archivePath string) error {
	staged, err := sp.stage(archivePath)
	if err != nil {
		return err
	}
	defer staged.discard()

	return sp.apply(staged)
}

// stagedRestore is an archive already unpacked next to the live data, waiting
// to be applied or thrown away
type stagedRestore struct {
	dir        string
	hasIndexes bool
}

func (s *stagedRestore) discard() {
	if err := os.RemoveAll(s.dir); err != nil {
		slog.Error("Failed to clean the restore folder", slog.String("error", err.Error()))
	}
}

// stage unpacks the database and the indexes held by the archive into a folder
// next to the live ones and checks the database opens
func (sp *SimpleBackup) stage(archivePath string) (*stagedRestore, error) {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, fmt.Errorf("opening archive %q: %w", archivePath, err)
	}
	defer zr.Close()

	dbFile := sp.db.Filename()

	var (
		dbEntry    *zip.File
		idxEntries []archiveEntry
		ignored    int
	)

	for _, f := range zr.File {
		name := strings.ReplaceAll(f.Name, `\`, "/")

		// Directory entries are recreated while extracting the files
		if strings.HasSuffix(name, "/") {
			continue
		}

		if !filepath.IsLocal(filepath.FromSlash(name)) {
			return nil, fmt.Errorf("archive entry %q escapes the data folder", f.Name)
		}

		switch {
		case name == dbFile:
			dbEntry = f
		case strings.HasPrefix(name, core.IndexerFolder+"/"):
			idxEntries = append(idxEntries, archiveEntry{file: f, name: name})
		default:
			ignored++
		}
	}

	if dbEntry == nil {
		return nil, fmt.Errorf("archive %q holds no %q entry", filepath.Base(archivePath), dbFile)
	}

	staging, err := os.MkdirTemp(sp.dataDir, ".restore-*")
	if err != nil {
		return nil, fmt.Errorf("creating restore folder: %w", err)
	}

	staged := &stagedRestore{dir: staging, hasIndexes: len(idxEntries) > 0}

	// The whole archive shares one budget
	budget := int64(maxExtractedBytes)

	stagedDB := filepath.Join(staging, dbFile)
	written, err := extractFile(dbEntry, stagedDB, budget)
	if err != nil {
		staged.discard()
		return nil, fmt.Errorf("extracting database: %w", err)
	}
	budget -= written

	// An empty file is a valid but unwanted database, so we check the header before opening it
	if err := checkSqliteFile(stagedDB); err != nil {
		staged.discard()
		return nil, fmt.Errorf("archived database %q: %w", dbFile, err)
	}

	if err := core.VerifySqliteFile(stagedDB); err != nil {
		staged.discard()
		return nil, fmt.Errorf("archived database %q: %w", dbFile, err)
	}

	for _, e := range idxEntries {
		written, err := extractFile(e.file, filepath.Join(staging, filepath.FromSlash(e.name)), budget)
		if err != nil {
			staged.discard()
			return nil, fmt.Errorf("extracting index file %q: %w", e.name, err)
		}
		budget -= written
	}

	slog.Info("Staged backup contents",
		slog.String("archive", filepath.Base(archivePath)),
		slog.Int("index_files", len(idxEntries)),
		slog.Int("ignored_files", ignored),
	)

	return staged, nil
}

func (sp *SimpleBackup) apply(staged *stagedRestore) error {
	// Sqlite holds the database files open
	if err := sp.db.Close(); err != nil {
		return fmt.Errorf("closing database: %w", err)
	}

	if err := sp.swapRestored(staged.dir, staged.hasIndexes); err != nil {
		// The live files are back, the handle still has to be usable again
		if reopenErr := sp.db.Reopen(); reopenErr != nil {
			return errors.Join(err, fmt.Errorf("reopening the previous database: %w", reopenErr))
		}
		return err
	}

	if err := sp.db.Reopen(); err != nil {
		return fmt.Errorf("opening the restored database: %w", err)
	}

	if err := sp.migrate(); err != nil {
		return fmt.Errorf("migrating the restored database: %w", err)
	}

	return nil
}

// migrate runs every module migration against the database currently open,
// bringing an archive taken on an older build up to the running schema
func (sp *SimpleBackup) migrate() error {
	migrators, err := sp.ctx.Modules(new(core.Migrator))
	if err != nil {
		return fmt.Errorf("getting migrators: %w", err)
	}

	for _, m := range migrators {
		if err := m.(core.Migrator).Migrate(); err != nil {
			return err
		}
	}

	return nil
}

// archiveEntry pairs an archive entry with its separator normalized name
type archiveEntry struct {
	file *zip.File
	name string
}

// swapRestored parks the live database and indexes inside staging and moves the
// staged ones into the data folder, so we can rollback if needed
func (sp *SimpleBackup) swapRestored(staging string, hasIndexes bool) error {
	parked := filepath.Join(staging, ".replaced")
	if err := os.Mkdir(parked, 0o755); err != nil {
		return fmt.Errorf("creating rollback folder: %w", err)
	}

	sw := &restoreSwap{parked: parked}

	dbFile := sp.db.Filename()
	liveDB := sp.db.Filepath()
	liveIdx := filepath.Join(sp.dataDir, core.IndexerFolder)

	steps := []func() error{
		// The journal files describe the database being replaced, they must go with it
		func() error { return sw.park(liveDB+"-wal", dbFile+"-wal") },
		func() error { return sw.park(liveDB+"-shm", dbFile+"-shm") },
		func() error { return sw.park(liveDB, dbFile) },
		func() error { return sw.park(liveIdx, core.IndexerFolder) },
		func() error { return sw.place(filepath.Join(staging, dbFile), liveDB) },
	}

	// We only restore indexes if they existed when taking the backup
	if hasIndexes {
		steps = append(steps, func() error {
			return sw.place(filepath.Join(staging, core.IndexerFolder), liveIdx)
		})
	}

	for _, step := range steps {
		if err := step(); err != nil {
			sw.rollback()
			return fmt.Errorf("replacing live data: %w", err)
		}
	}

	slog.Info("Restored database and indexes", slog.String("folder", sp.dataDir))

	return nil
}

// restoreSwap records every move it performs so they can be undone in reverse
type restoreSwap struct {
	parked string
	undo   []func() error
}

// park moves a live path out of the way, a path that does not exist is a no-op
func (s *restoreSwap) park(live, name string) error {
	dest := filepath.Join(s.parked, name)

	if err := os.Rename(live, dest); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	s.undo = append(s.undo, func() error { return os.Rename(dest, live) })

	return nil
}

// place moves a staged path to the location it is restored to
func (s *restoreSwap) place(staged, live string) error {
	if err := os.Rename(staged, live); err != nil {
		return err
	}

	s.undo = append(s.undo, func() error { return os.Rename(live, staged) })

	return nil
}

func (s *restoreSwap) rollback() {
	for i := len(s.undo) - 1; i >= 0; i-- {
		if err := s.undo[i](); err != nil {
			slog.Error("Failed to roll back the restore", slog.String("error", err.Error()))
		}
	}
}

// maxExtractedBytes caps how much a single archive is allowed to expand to
const maxExtractedBytes = 2 << 30

var errRestoreTooLarge = errors.New("contents are larger than the restore limit")

// extractFile writes an archive entry to dest, reading at most limit bytes
func extractFile(f *zip.File, dest string, limit int64) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return 0, err
	}

	src, err := f.Open()
	if err != nil {
		return 0, err
	}
	defer src.Close()

	perm := f.Mode().Perm()
	if perm == 0 {
		perm = 0o644
	}

	// #nosec G304 -- dest is the staging folder joined with an entry name that
	// stage already ran through filepath.IsLocal
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return 0, err
	}

	written, err := io.CopyN(out, src, limit+1)
	if errors.Is(err, io.EOF) {
		err = nil
	}

	if err == nil && written > limit {
		err = errRestoreTooLarge
	}

	if closeErr := out.Close(); err == nil {
		err = closeErr
	}

	if err != nil {
		return 0, err
	}

	return written, nil
}

const sqliteHeader = "SQLite format 3\x00"

func checkSqliteFile(path string) error {
	// #nosec G304 -- the caller passes the database it just staged, never a
	// path that came in with the request
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	header := make([]byte, len(sqliteHeader))
	if _, err := io.ReadFull(f, header); err != nil {
		return fmt.Errorf("reading the file header: %w", err)
	}

	if string(header) != sqliteHeader {
		return errors.New("is not a sqlite database")
	}

	return nil
}

func (sp *SimpleBackup) generate() (string, error) {
	tmpDir, err := os.MkdirTemp(sp.saveDir, ".snapshot-*")
	if err != nil {
		return "", fmt.Errorf("creating snapshot folder: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	snapshot := filepath.Join(tmpDir, sp.db.Filename())
	if err := sp.db.Snapshot(snapshot); err != nil {
		return "", fmt.Errorf("snapshotting database: %w", err)
	}

	saved, err := sp.zipFolder(tmpDir, snapshot)
	if err != nil {
		return "", err
	}

	// The archive is already on disk, a failed cleanup must not fail the backup
	if err := sp.prune(); err != nil {
		slog.Error("Failed to prune old backups", slog.String("error", err.Error()))
	}

	return saved, nil
}

// prune keeps only the sp.keep most recent archives in saveDir, removing the
// older ones. A keep count of zero or less disables retention.
func (sp *SimpleBackup) prune() error {
	if sp.keep <= 0 {
		return nil
	}

	entries, err := os.ReadDir(sp.saveDir)
	if err != nil {
		return fmt.Errorf("reading backup folder: %w", err)
	}

	var backups []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !isBackupArchive(name) {
			continue
		}
		backups = append(backups, name)
	}

	if len(backups) <= sp.keep {
		return nil
	}

	// The timestamp in the filename sorts chronologically, so the oldest come first
	sort.Strings(backups)

	var errs []error
	for _, name := range backups[:len(backups)-sp.keep] {
		if err := os.Remove(filepath.Join(sp.saveDir, name)); err != nil {
			errs = append(errs, err)
			continue
		}
		slog.Info("Removed old backup", slog.String("file", name))
	}

	return errors.Join(errs...)
}

func (sp *SimpleBackup) pause() error {
	var g errgroup.Group
	for _, p := range sp.pausables {
		g.Go(p.Pause)
	}
	return g.Wait()
}

func (sp *SimpleBackup) resume() error {
	var g errgroup.Group
	for _, p := range sp.pausables {
		g.Go(p.Resume)
	}
	return g.Wait()
}

func (sp *SimpleBackup) zipFolder(tmpDir, snapshot string) (string, error) {
	zipFile, destFile, err := sp.createArchive()
	if err != nil {
		return "", err
	}

	zw := zip.NewWriter(zipFile)

	if err := sp.writeArchive(zw, tmpDir, snapshot); err != nil {
		_ = zw.Close()
		_ = zipFile.Close()
		_ = os.Remove(destFile)
		return "", err
	}

	if err := zw.Close(); err != nil {
		_ = zipFile.Close()
		_ = os.Remove(destFile)
		return "", fmt.Errorf("finishing archive: %w", err)
	}

	if err := zipFile.Close(); err != nil {
		_ = os.Remove(destFile)
		return "", fmt.Errorf("closing archive: %w", err)
	}

	return destFile, nil
}

func (sp *SimpleBackup) writeArchive(zw *zip.Writer, tmpDir, snapshot string) error {
	dbFile := sp.db.Filename()

	if err := addToArchive(zw, snapshot, dbFile); err != nil {
		return fmt.Errorf("archiving database: %w", err)
	}

	absData, err := filepath.Abs(sp.dataDir)
	if err != nil {
		return fmt.Errorf("resolving data folder: %w", err)
	}

	absSave, err := filepath.Abs(sp.saveDir)
	if err != nil {
		return fmt.Errorf("resolving backup folder: %w", err)
	}

	absTmp, err := filepath.Abs(tmpDir)
	if err != nil {
		return fmt.Errorf("resolving snapshot folder: %w", err)
	}

	return filepath.Walk(sp.dataDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sp.dataDir, p)
		if err != nil {
			return err
		}
		abs := filepath.Join(absData, relPath)

		if info.IsDir() {
			if abs == absTmp || (abs == absSave && abs != absData) {
				return filepath.SkipDir
			}
			return nil
		}

		// ignore db files, they are covered by the db snapshot
		if relPath == dbFile || relPath == dbFile+"-wal" || relPath == dbFile+"-shm" {
			return nil
		}

		// avoid generated jwt secret from reaching the exported backup file
		if relPath == core.JWTSecretFile {
			return nil
		}

		// skip backup files in dataDir
		if isBackupArchive(filepath.Base(p)) {
			return nil
		}

		return addToArchive(zw, p, relPath)
	})
}

func addToArchive(zw *zip.Writer, src, name string) error {
	// #nosec G304 -- src is a path the walk over the data folder handed us
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(name)
	header.Method = zip.Deflate

	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, file)
	return err
}

const (
	backupPrefix     = "cubby-backup-"
	backupSuffix     = ".zip"
	backupTimeLayout = "20060102-150405.000"
)

func isBackupArchive(name string) bool {
	name = filepath.Base(name)
	return filepath.IsLocal(name) && strings.HasPrefix(name, backupPrefix) && strings.HasSuffix(name, backupSuffix)
}

func (sp *SimpleBackup) createArchive() (*os.File, string, error) {
	base := backupPrefix + time.Now().Format(backupTimeLayout)

	for i := range 100 {
		name := base + backupSuffix
		if i > 0 {
			name = fmt.Sprintf("%s-%d%s", base, i, backupSuffix)
		}

		p := filepath.Join(sp.saveDir, name)

		// #nosec G304 -- p is the backup folder joined with a name this function
		// built out of the current time
		f, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			return f, p, nil
		}

		if !errors.Is(err, os.ErrExist) {
			return nil, "", fmt.Errorf("creating archive: %w", err)
		}
	}

	return nil, "", fmt.Errorf("creating archive: %q is already taken", base+backupSuffix)
}
