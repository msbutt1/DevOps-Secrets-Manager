// Package testutil provides a throwaway PostgreSQL database for integration tests.
package testutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/storage"
)

// MigrationsPath returns the absolute path of apps/api/migrations.
func MigrationsPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "migrations")
}

// NewEmptyDatabase creates a new, empty database on the server named by TEST_DATABASE_URL
// and drops it when the test finishes. Tests are skipped when the variable is not set.
func NewEmptyDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()

	baseURL := os.Getenv("TEST_DATABASE_URL")
	if baseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping database integration test")
	}

	ctx := context.Background()
	admin, err := pgx.Connect(ctx, baseURL)
	if err != nil {
		t.Fatalf("connect to TEST_DATABASE_URL: %v", err)
	}
	defer admin.Close(ctx)

	suffix := make([]byte, 6)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatal(err)
	}
	name := "dsm_test_" + hex.EncodeToString(suffix)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create test database: %v", err)
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	u.Path = "/" + name

	pool, err := storage.NewPostgresPool(ctx, storage.PostgresConfig{URL: u.String()})
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		conn, err := pgx.Connect(context.Background(), baseURL)
		if err != nil {
			t.Logf("drop test database: %v", err)
			return
		}
		defer conn.Close(context.Background())
		if _, err := conn.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Logf("drop test database: %v", err)
		}
	})

	return pool
}

// NewDatabase returns a fresh database with every migration applied.
func NewDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := NewEmptyDatabase(t)
	if err := storage.RunMigrations(pool, MigrationsPath()); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	return pool
}
