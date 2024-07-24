package main

import (
	"context"
	"fmt"
	"github.com/sbowman/drawbridge/migrations"
	"github.com/sbowman/drawbridge/postgres/std"
	"github.com/urfave/cli/v2"
	"os"
	"strings"
	"time"
)

func main() {
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "migrations",
				EnvVars: []string{"DB_MIGRATIONS"},
				Value:   "./sql",
				Usage:   "directory containing the SQL migration files",
			},
			&cli.IntFlag{
				Name:    "revision",
				Aliases: []string{"v"},
				Usage:   "migrate the database to this revision (default latest)",
			},
			&cli.StringFlag{
				Name:    "metadata",
				EnvVars: []string{"DB_METADATA"},
				Value:   "drawbridge.schema_migrations",
				Usage:   "specify the name of the migrations metadata schema and table",
			},
			&cli.DurationFlag{
				Name:  "timeout",
				Value: 10 * time.Minute,
				Usage: "how long to wait for the migrations to be applied",
			},
			&cli.StringFlag{
				Name:  "uri",
				Usage: "database driver connection string",
			},
		},

		Usage:  "apply the latest migrations",
		Action: migrate,

		Commands: []*cli.Command{
			{
				Name:      "create",
				Usage:     "create a new migration",
				Args:      true,
				ArgsUsage: "[name]",
				Action:    create,
			},
			{
				Name: "rollback",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:    "steps",
						Aliases: []string{"s"},
						Value:   1,
						Usage:   "roll back this number of migrations",
					},
				},
				Action: rollback,
				Usage:  "rollback the migrations a number of steps",
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Create a migration.
func create(cctx *cli.Context) error {
	options := migrations.DefaultOptions()

	if cctx.IsSet("directory") {
		options = options.WithDirectory(cctx.String("directory"))
	}

	for idx := 0; idx < cctx.NArg(); idx++ {
		name := cctx.Args().Get(idx)
		path, err := options.Create(name)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Unable to create SQL migration file: %s\n", err)
			os.Exit(1)
		}

		fmt.Printf("Created %s\n", path)
	}

	return nil
}

func migrate(cctx *cli.Context) error {
	options := migrations.DefaultOptions()

	if cctx.IsSet("directory") {
		options = options.WithDirectory(cctx.String("directory"))
	}

	if cctx.IsSet("revision") {
		options = options.WithRevision(cctx.Int("revision"))
	}

	if cctx.IsSet("metadata") {
		options = options.WithSchemaTable(cctx.String("metadata"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), cctx.Duration("timeout"))
	defer cancel()

	uri := cctx.String("uri")
	if uri == "" {
		_, _ = fmt.Fprintln(os.Stderr, "Missing database driver connection string")
		os.Exit(1)
	}

	var db migrations.Span
	var err error

	if strings.HasPrefix(uri, "postgres") {
		db, err = std.Open(uri)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Unable to connect to the database: %s\n", err)
			os.Exit(1)
		}
	} else {
		_, _ = fmt.Fprintln(os.Stderr, "Database driver is not supported")
	}

	if err := options.Apply(ctx, db); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Database migration failed: %s\n", err)
		os.Exit(1)
	}

	return nil
}

func rollback(cctx *cli.Context) error {
	options := migrations.DefaultOptions()
	steps := 1

	if cctx.IsSet("directory") {
		options = options.WithDirectory(cctx.String("directory"))
	}

	if cctx.IsSet("steps") {
		steps = cctx.Int("steps")
	}

	if cctx.IsSet("metadata") {
		options = options.WithSchemaTable(cctx.String("metadata"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), cctx.Duration("timeout"))
	defer cancel()

	uri := cctx.String("uri")
	if uri == "" {
		_, _ = fmt.Fprintln(os.Stderr, "Missing database driver connection string")
		os.Exit(1)
	}

	var db migrations.Span
	var err error

	if strings.HasPrefix(uri, "postgres") {
		db, err = std.Open(uri)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Unable to connect to the database: %s\n", err)
			os.Exit(1)
		}
	} else {
		_, _ = fmt.Fprintln(os.Stderr, "Database driver is not supported")
	}

	if err := options.Rollback(ctx, db, steps); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Database rollback failed: %s\n", err)
		os.Exit(1)
	}

	return nil
}
