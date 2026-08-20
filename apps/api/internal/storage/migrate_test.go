package storage_test

import (
	"context"
	"os"
	"regexp"
	"sort"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/storage"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/testutil"
)

// migrationVersions lists the numbered migrations and checks each has an up and a down file.
func migrationVersions(t *testing.T) []int {
	t.Helper()
	entries, err := os.ReadDir(testutil.MigrationsPath())
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`^(\d+)_[a-z0-9_]+\.(up|down)\.sql$`)
	files := map[int]map[string]bool{}
	for _, e := range entries {
		m := re.FindStringSubmatch(e.Name())
		if m == nil {
			t.Errorf("unexpected file in migrations: %s", e.Name())
			continue
		}
		v, _ := strconv.Atoi(m[1])
		if files[v] == nil {
			files[v] = map[string]bool{}
		}
		files[v][m[2]] = true
	}
	var versions []int
	for v, kinds := range files {
		if !kinds["up"] || !kinds["down"] {
			t.Errorf("migration %06d needs both up and down files", v)
		}
		versions = append(versions, v)
	}
	sort.Ints(versions)
	for i, v := range versions {
		if v != i+1 {
			t.Fatalf("migration versions must be consecutive from 1, got %v", versions)
		}
	}
	return versions
}

// schema returns a description of every table, column and index so schemas can be compared.
func schema(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT 'column ' || table_name || '.' || column_name || ' ' || data_type || ' ' || is_nullable
		FROM information_schema.columns WHERE table_schema = 'public' AND table_name <> 'schema_migrations'
		UNION ALL
		SELECT 'index ' || indexname || ' ' || indexdef FROM pg_indexes
		WHERE schemaname = 'public' AND tablename <> 'schema_migrations'
		ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		out = append(out, line)
	}
	return out
}

func TestMigrationsApplyAndRollBackStepByStep(t *testing.T) {
	versions := migrationVersions(t)
	pool := testutil.NewEmptyDatabase(t)

	mg, err := storage.NewMigrator(pool, testutil.MigrationsPath())
	if err != nil {
		t.Fatal(err)
	}
	defer mg.Close()

	// Record the schema after each step up, then check each step down restores the previous one.
	schemas := [][]string{schema(t, pool)}
	for _, v := range versions {
		if err := mg.Steps(1); err != nil {
			t.Fatalf("migration %d up: %v", v, err)
		}
		schemas = append(schemas, schema(t, pool))
	}

	version, dirty, err := mg.Version()
	if err != nil || dirty || int(version) != versions[len(versions)-1] {
		t.Fatalf("after all up: version %d dirty %v err %v", version, dirty, err)
	}

	for i := len(versions) - 1; i >= 0; i-- {
		if err := mg.Down(1); err != nil {
			t.Fatalf("migration %d down: %v", versions[i], err)
		}
		if got, want := schema(t, pool), schemas[i]; !equal(got, want) {
			t.Fatalf("rolling back migration %d did not restore the previous schema:\n got %v\nwant %v", versions[i], got, want)
		}
	}

	if len(schema(t, pool)) != 0 {
		t.Fatal("tables remain after rolling back every migration")
	}

	// And everything applies again on the now-empty database.
	if err := mg.Up(); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	if got := schema(t, pool); !equal(got, schemas[len(schemas)-1]) {
		t.Fatal("re-applied schema differs from the first run")
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
