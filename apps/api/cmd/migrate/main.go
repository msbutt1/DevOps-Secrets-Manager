// Command migrate applies or rolls back the database schema migrations.
//
//	migrate up            apply all pending migrations
//	migrate down [N|all]  roll back N migrations (default 1) or all of them
//	migrate version       print the current schema version
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/config"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/storage"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: migrate up | down [N|all] | version")
	}

	if err := config.Load(); err != nil {
		return err
	}

	ctx := context.Background()
	pool, err := storage.NewPostgresPool(ctx, config.Database())
	if err != nil {
		return err
	}
	defer pool.Close()

	mg, err := storage.NewMigrator(pool, config.MigrationsPath())
	if err != nil {
		return err
	}
	defer mg.Close()

	switch args[0] {
	case "up":
		if err := mg.Up(); err != nil {
			return err
		}
	case "down":
		steps := 1
		if len(args) > 1 {
			if args[1] == "all" {
				steps = 0
			} else if steps, err = strconv.Atoi(args[1]); err != nil || steps < 1 {
				return fmt.Errorf("down expects a positive number of steps or 'all'")
			}
		}
		if err := mg.Down(steps); err != nil {
			return err
		}
	case "version":
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}

	version, dirty, err := mg.Version()
	if err != nil {
		return err
	}
	fmt.Printf("schema version %d (dirty: %t)\n", version, dirty)
	return nil
}
