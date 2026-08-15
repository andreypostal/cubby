package core

import (
	"context"

	"github.com/urfave/cli/v3"
)

type Module interface {
	Id() string
	New() any
}

type Initializer interface {
	Initialize(*Context) error
}

type Validator interface {
	Validate() error
}

type Migrator interface {
	Migrate() error
}

type App interface {
	Start() error
	Stop(ctx context.Context) error
}

type Closer interface {
	Close() error
}

type Commander interface {
	Command() *cli.Command
}
