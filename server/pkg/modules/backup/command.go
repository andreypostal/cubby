package backup

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

func (sp *SimpleBackup) Command() *cli.Command {
	return &cli.Command{
		Name:  "backup",
		Usage: "Backup archive utils",
		Commands: []*cli.Command{
			sp.restoreCommand(),
		},
	}
}

func (sp *SimpleBackup) restoreCommand() *cli.Command {
	return &cli.Command{
		Name:  "restore",
		Usage: "Restore the database and the search indexes from a backup archive",
		Description: "Replaces the live database and search indexes with the contents of the given archive.\n" +
			"The HTTP API must be stopped before running it.\n" +
			"Whatever is currently in the data folder is discarded, take a backup first.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "file",
				Aliases:  []string{"f"},
				Usage:    "The full path to the backup archive that will be restored",
				Required: true,
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			archive := c.String("file")

			if err := sp.restore(archive); err != nil {
				return err
			}

			fmt.Printf("[BACKUP] database and indexes restored from %q\n", archive)

			return nil
		},
	}
}
