package storage

import (
	"database/sql"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// RunMigrations runs database migrations using golang-migrate
func RunMigrations(pool *pgxpool.Pool, migrationsPath string) error {
	// Convert pgxpool to database/sql for golang-migrate compatibility
	connString := pool.Config().ConnString()
	db, err := sql.Open("pgx", connString)
	if err != nil {
		return fmt.Errorf("failed to open database connection for migrations: %w", err)
	}
	defer db.Close()

	// Create postgres driver instance
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create postgres driver instance: %w", err)
	}

	// Create migrate instance
	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsPath),
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	// Run migrations up
	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			// No new migrations to run - this is not an error
			return nil
		}
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}
