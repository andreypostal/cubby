package backup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/server/middleware"
	"github.com/gin-gonic/gin"
)

// ArchiveInfo describes one archive sitting in the backup folder
type ArchiveInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	// CreatedAt is the archive mtime
	CreatedAt time.Time `json:"created_at"`
}

type listResponse struct {
	Backups []ArchiveInfo `json:"backups"`
	Keep    int           `json:"keep"`
}

var (
	generateTimeout = 180 * time.Second
	restoreTimeout  = 60 * time.Second
)

// Generates a new archive and streams it back
func (sp *SimpleBackup) handleGenerate(c *gin.Context) {
	// Leave so we make sure that our current request wont block the freeze request
	middleware.LeaveFreezeGate(c)

	ctx, cancel := context.WithTimeout(context.TODO(), generateTimeout)
	defer cancel()

	unfreeze, err := sp.freezer.Freeze(ctx, "backup")
	if err != nil {
		slog.Error("Failed to hold the system for backup", slog.String("error", err.Error()))
		core.RespondError(c, core.NewError(http.StatusConflict, "the system is busy, try again", nil))
		return
	}
	defer unfreeze()

	// Wrap in a once func so we can ensure that we release any partially paused data
	// but also do a early unfreeze
	resume := sync.OnceFunc(func() {
		if err := sp.resume(); err != nil {
			slog.Error("Failed to resume after backup", slog.String("error", err.Error()))
		}
	})
	defer resume()

	if err := sp.pause(); err != nil {
		slog.Error("Failed to pause for backup", slog.String("error", err.Error()))
		core.RespondError(c, core.NewError(http.StatusInternalServerError, "could not pause the indexers", nil))
		return
	}

	backupFile, err := sp.generate()
	if err != nil {
		slog.Error("Failed to generate backup", slog.String("error", err.Error()))
		core.RespondError(c, core.NewError(http.StatusInternalServerError, "could not generate the backup", nil))
		return
	}

	resume()
	// We can do a early unfreeze before streaming the file
	unfreeze()

	slog.Info("Generated backup", slog.String("file", filepath.Base(backupFile)))
	sp.stream(c, backupFile)
}

// Replaces the live database and indexes with an archive, without a restart
func (sp *SimpleBackup) handleRestore(c *gin.Context) {
	path, err := sp.resolveArchive(c.Param("name"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		core.RespondError(c, core.NewError(http.StatusNotFound, "backup not found", nil))
		return
	}

	staged, err := sp.stage(path)
	if err != nil {
		slog.Error("Failed to stage backup", slog.String("error", err.Error()))
		core.RespondError(c, core.NewError(http.StatusBadRequest, "could not read this backup", nil))
		return
	}
	defer staged.discard()

	// Leave so we make sure that our current request wont block the freeze request
	middleware.LeaveFreezeGate(c)

	ctx, cancel := context.WithTimeout(context.TODO(), restoreTimeout)
	defer cancel()

	unfreeze, err := sp.freezer.FreezeAll(ctx, "restore")
	if err != nil {
		slog.Error("Failed to hold the system for restore", slog.String("error", err.Error()))
		core.RespondError(c, core.NewError(http.StatusConflict, "the system is busy, try again", nil))
		return
	}
	defer unfreeze()

	// Wrapped so a partially paused set is released even on an early return
	resume := sync.OnceFunc(func() {
		if err := sp.resume(); err != nil {
			slog.Error("Failed to resume after restore", slog.String("error", err.Error()))
		}
	})
	defer resume()

	if err := sp.pause(); err != nil {
		slog.Error("Failed to pause for restore", slog.String("error", err.Error()))
		core.RespondError(c, core.NewError(http.StatusInternalServerError, "could not pause the indexers", nil))
		return
	}

	if err := sp.apply(staged); err != nil {
		slog.Error("Failed to restore backup", slog.String("error", err.Error()))
		core.RespondError(c, core.NewError(http.StatusInternalServerError, "could not restore this backup", nil))
		return
	}

	resume()
	unfreeze()

	slog.Info("Restored backup", slog.String("file", filepath.Base(path)))
	c.Status(http.StatusNoContent)
}

func (sp *SimpleBackup) handleList(c *gin.Context) {
	entries, err := os.ReadDir(sp.saveDir)
	if err != nil {
		slog.Error("Failed to read backup folder", slog.String("error", err.Error()))
		core.RespondError(c, core.NewError(http.StatusInternalServerError, "could not read the backup folder", nil))
		return
	}

	backups := make([]ArchiveInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !isBackupArchive(e.Name()) {
			continue
		}

		// Skip a deleted backup file
		info, err := e.Info()
		if err != nil {
			continue
		}

		backups = append(backups, ArchiveInfo{
			Name:      e.Name(),
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
		})
	}

	// Newest first
	sort.Slice(backups, func(i, j int) bool { return backups[i].Name > backups[j].Name })

	c.JSON(http.StatusOK, listResponse{Backups: backups, Keep: sp.keep})
}

func (sp *SimpleBackup) handleDownload(c *gin.Context) {
	path, err := sp.resolveArchive(c.Param("name"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		core.RespondError(c, core.NewError(http.StatusNotFound, "backup not found", nil))
		return
	}

	sp.stream(c, path)
}

func (sp *SimpleBackup) handleDelete(c *gin.Context) {
	path, err := sp.resolveArchive(c.Param("name"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			core.RespondError(c, core.NewError(http.StatusNotFound, "backup not found", nil))
			return
		}

		slog.Error("Failed to delete backup", slog.String("file", filepath.Base(path)), slog.String("error", err.Error()))
		core.RespondError(c, core.NewError(http.StatusInternalServerError, "could not delete the backup", nil))
		return
	}

	slog.Info("Deleted backup", slog.String("file", filepath.Base(path)))
	c.Status(http.StatusNoContent)
}

// resolveArchive ensures backup path using user input is safe
func (sp *SimpleBackup) resolveArchive(name string) (string, error) {
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") || !isBackupArchive(name) {
		return "", fmt.Errorf("%q is not a backup archive name", name)
	}

	return filepath.Join(sp.saveDir, name), nil
}

func (sp *SimpleBackup) stream(c *gin.Context, path string) {
	filename := filepath.Base(path)

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "application/zip")
	c.File(path)
}
