// Package web serves the embedded ui SPA
package web

//go:generate npm --prefix ../../../../web ci
//go:generate npm --prefix ../../../../web run build

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var dist embed.FS

const (
	indexFile    = "index.html"
	assetsPrefix = "assets/"
	keepFile     = ".gitkeep"

	immutableCache  = "public, max-age=31536000, immutable"
	revalidateCache = "no-cache"
)

var extraMimeTypes = map[string]string{
	".woff2":       "font/woff2",
	".woff":        "font/woff",
	".ttf":         "font/ttf",
	".map":         "application/json",
	".webmanifest": "application/manifest+json",
}

type Web struct {
	enabled bool

	assets fs.FS

	// etags maps paths to its ETag so client can revalidate against
	etags map[string]string
}

func (w *Web) Id() string {
	return "web"
}

func (w *Web) New() any {
	return new(Web)
}

func (w *Web) Initialize(ctx *core.Context) error {
	w.enabled = ctx.Config.Web.Enabled
	if !w.enabled {
		return nil
	}

	assets, err := fs.Sub(dist, "dist")
	if err != nil {
		return fmt.Errorf("open embedded dist: %w", err)
	}
	w.assets = assets

	w.etags = make(map[string]string)
	err = fs.WalkDir(assets, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Base(p) == keepFile {
			return err
		}

		tag, err := etagFor(assets, p)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		w.etags[p] = tag

		return nil
	})
	if err != nil {
		return fmt.Errorf("index embedded assets: %w", err)
	}

	// Register types to builtin package so fonts and other are not served as stream
	for ext, typ := range extraMimeTypes {
		if err := mime.AddExtensionType(ext, typ); err != nil {
			return fmt.Errorf("register mime type %s: %w", ext, err)
		}
	}

	return nil
}

// Validate ensures that ui files were generated
func (w *Web) Validate() error {
	if !w.enabled {
		return nil
	}

	if _, ok := w.etags[indexFile]; !ok {
		return errors.New("embedded UI not generated")
	}

	return nil
}

// Fallback returns the fallback handler that serves the UI
func (w *Web) Fallback() gin.HandlerFunc {
	if !w.enabled {
		return nil
	}
	return w.serve
}

func (w *Web) serve(c *gin.Context) {
	name := strings.TrimPrefix(path.Clean(c.Request.URL.Path), "/")
	if name == "" || name == "." {
		name = indexFile
	}

	if tag, ok := w.etags[name]; ok {
		w.serveFile(c, name, tag)
		return
	}

	if !isNavigation(c.Request) {
		c.Data(http.StatusNotFound, "text/plain; charset=utf-8", []byte("not found\n"))
		return
	}

	w.serveFile(c, indexFile, w.etags[indexFile])
}

func (w *Web) serveFile(c *gin.Context, name, etag string) {
	f, err := w.assets.Open(name)
	if err != nil {
		_ = c.Error(fmt.Errorf("open embedded %s: %w", name, err))
		c.Status(http.StatusInternalServerError)
		return
	}
	defer f.Close()

	content, ok := f.(io.ReadSeeker)
	if !ok {
		_ = c.Error(fmt.Errorf("embedded %s is not seekable", name))
		c.Status(http.StatusInternalServerError)
		return
	}

	header := c.Writer.Header()
	header.Set("ETag", etag)
	header.Set("X-Content-Type-Options", "nosniff")
	if strings.HasPrefix(name, assetsPrefix) {
		header.Set("Cache-Control", immutableCache)
	} else {
		// index.html must be revalidated
		header.Set("Cache-Control", revalidateCache)
	}

	http.ServeContent(c.Writer, c.Request, name, time.Time{}, content)
}

func isNavigation(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}

	if mode := r.Header.Get("Sec-Fetch-Mode"); mode != "" {
		return mode == "navigate"
	}

	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

func etagFor(assets fs.FS, name string) (string, error) {
	f, err := assets.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()

	sum := sha256.New()
	if _, err := io.Copy(sum, f); err != nil {
		return "", err
	}

	return `"` + hex.EncodeToString(sum.Sum(nil)[:16]) + `"`, nil
}
