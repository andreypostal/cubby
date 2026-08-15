package core

import (
	"errors"
	"fmt"
	"log/slog"
	"path"
	"path/filepath"

	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

const dbFilename = "main.db"

var (
	ErrCreatingEntity = errors.New("error creating new entity")
	ErrEntityNotFound = errors.New("entity not found")
)

type DB struct {
	*gorm.DB

	dsn      string
	filepath string
	config   DBConfig
}

func (d *DB) Id() string {
	return "main-db"
}

func (d *DB) New() any {
	return new(DB)
}

func (d *DB) Initialize(ctx *Context) error {
	d.filepath = path.Join(ctx.Config.DataDir, dbFilename)
	d.dsn = fmt.Sprintf("file:%s?%s", d.filepath, ctx.Config.DB.SqliteParams)
	d.config = ctx.Config.DB

	return d.open()
}

func (d *DB) open() error {
	db, err := gorm.Open(gormlite.Open(d.dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("initializing db: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql handle: %w", err)
	}

	sqlDB.SetMaxOpenConns(d.config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(d.config.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(d.config.ConnMaxLifetime)

	d.DB = db

	// Also covers the reopen a restore performs
	slog.Info("Database open", slog.String("file", d.filepath))

	return nil
}

// Reopen drops the current handle and opens the database file again
func (d *DB) Reopen() error {
	if err := d.Close(); err != nil {
		return fmt.Errorf("closing the current handle: %w", err)
	}

	return d.open()
}

// Filepath returns the path to live db files
func (d *DB) Filepath() string {
	return d.filepath
}

// Filename returns the name of the main db file
func (d *DB) Filename() string {
	return dbFilename
}

func (d *DB) Snapshot(dest string) error {
	return d.Exec("VACUUM INTO ?", dest).Error
}

func (d *DB) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql handle for closing: %w", err)
	}
	return sqlDB.Close()
}

// VerifySqliteFile opens file read only and reads its schema
func VerifySqliteFile(file string) error {
	dsn := fmt.Sprintf("file:%s?mode=ro", filepath.ToSlash(file))

	db, err := gorm.Open(gormlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("opening %q: %w", file, err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("getting the sql handle: %w", err)
	}
	defer sqlDB.Close()

	var tables int64
	if err := db.Raw("SELECT count(*) FROM sqlite_master").Scan(&tables).Error; err != nil {
		return fmt.Errorf("reading the schema: %w", err)
	}

	return nil
}

// IsDuplicatedError answers whether a write lost to a unique index. The driver
// reports the constraint itself rather than going through gorm's translation,
// so both are checked.
func IsDuplicatedError(err error) bool {
	return errors.Is(err, gorm.ErrDuplicatedKey) ||
		errors.Is(err, sqlite3.CONSTRAINT_UNIQUE) ||
		errors.Is(err, sqlite3.CONSTRAINT_PRIMARYKEY)
}

func IsNotFoundError(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
