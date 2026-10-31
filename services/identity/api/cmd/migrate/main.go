package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/transcandence/campusplusplus/services/identity/api/internal/config"
)

const migrationsDir = "migrations"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: migrate <up|down [steps]|version|force <version>>")
	}

	// Load config. This also loads .env from the current directory.
	// JWT_SECRET is not required for migrations, but config.Load requires
	// it; if you want migration-only environments, add a LoadForMigrate()
	// that validates DATABASE_URL alone.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// golang-migrate's pgx/v5 driver registers the "pgx5://" scheme, not
	// "postgres://". Rewrite so .env can keep the standard scheme that
	// pgx.Connect (used by cmd/server) expects.
	dbURL := toPgx5URL(cfg.DatabaseURL)

	m, err := migrate.New("file://"+migrationsDir, dbURL)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			fmt.Fprintln(os.Stderr, "Warning: closing source:", srcErr)
		}
		if dbErr != nil {
			fmt.Fprintln(os.Stderr, "Warning: closing database:", dbErr)
		}
	}()

	switch args[0] {
	case "up":
		return cmdUp(m)
	case "down":
		return cmdDown(m, args[1:])
	case "version":
		return cmdVersion(m)
	case "force":
		return cmdForce(m, args[1:])
	default:
		return fmt.Errorf("unknown command %q (want up, down, version, or force)", args[0])
	}
}

func cmdUp(m *migrate.Migrate) error {
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("applying migrations: %w", err)
	}
	fmt.Println("Migrations up: database is at the latest version.")
	return nil
}

func cmdDown(m *migrate.Migrate, args []string) error {
	steps := 1
	if len(args) >= 1 {
		n, err := strconv.Atoi(args[0])
		if err != nil || n < 1 {
			return fmt.Errorf("invalid steps %q: must be a positive integer", args[0])
		}
		steps = n
	}
	if err := m.Steps(-steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("rolling back %d migration(s): %w", steps, err)
	}
	fmt.Printf("Migrations down: rolled back %d step(s).\n", steps)
	return nil
}

func cmdVersion(m *migrate.Migrate) error {
	v, dirty, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		fmt.Println("Version: none (no migrations applied)")
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading version: %w", err)
	}
	fmt.Printf("Version: %d (dirty=%v)\n", v, dirty)
	return nil
}

func cmdForce(m *migrate.Migrate, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: migrate force <version>")
	}
	v, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid version %q: must be an integer", args[0])
	}
	if err := m.Force(v); err != nil {
		return fmt.Errorf("forcing version %d: %w", v, err)
	}
	fmt.Printf("Forced version to %d.\n", v)
	return nil
}

// toPgx5URL rewrites a postgres:// URL to the pgx5:// scheme that
// golang-migrate's pgx/v5 driver registers. URLs that already use pgx5://
// are returned unchanged.
func toPgx5URL(url string) string {
	if strings.HasPrefix(url, "pgx5://") {
		return url
	}
	return "pgx5://" + strings.TrimPrefix(url, "postgres://")
}
