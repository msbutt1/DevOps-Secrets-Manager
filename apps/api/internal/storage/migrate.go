package storage

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Migrator wraps a golang-migrate instance bound to a database connection.
type Migrator struct {
	m  *migrate.Migrate
	db *sql.DB
}

// NewMigrator opens a migrate instance for the pool's database and the migrations directory.
func NewMigrator(pool *pgxpool.Pool, migrationsPath string) (*Migrator, error) {
	// Convert pgxpool to database/sql for golang-migrate compatibility
	db, err := sql.Open("pgx", pool.Config().ConnString())
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection for migrations: %w", err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create postgres driver instance: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(fmt.Sprintf("file://%s", migrationsPath), "postgres", driver)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create migrate instance: %w", err)
	}

	return &Migrator{m: m, db: db}, nil
}

// Close releases the migration source and database connection.
func (mg *Migrator) Close() error {
	srcErr, dbErr := mg.m.Close()
	return errors.Join(srcErr, dbErr)
}

// Up applies all pending migrations.
func (mg *Migrator) Up() error {
	if err := mg.m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}

// Steps applies n pending migrations.
func (mg *Migrator) Steps(n int) error {
	if err := mg.m.Steps(n); err != nil {
		return fmt.Errorf("failed to apply %d migrations: %w", n, err)
	}
	return nil
}

// Down rolls back the given number of migrations; steps <= 0 rolls back all of them.
func (mg *Migrator) Down(steps int) error {
	var err error
	if steps <= 0 {
		err = mg.m.Down()
	} else {
		err = mg.m.Steps(-steps)
	}
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to roll back migrations: %w", err)
	}
	return nil
}

// Version returns the current schema version and whether the last migration left it dirty.
// A database with no migrations applied reports version 0.
func (mg *Migrator) Version() (uint, bool, error) {
	version, dirty, err := mg.m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, nil
	}
	return version, dirty, err
}

// RunMigrations runs database migrations using golang-migrate
func RunMigrations(pool *pgxpool.Pool, migrationsPath string) error {
	mg, err := NewMigrator(pool, migrationsPath)
	if err != nil {
		return err
	}
	defer mg.Close()
	return mg.Up()
}
