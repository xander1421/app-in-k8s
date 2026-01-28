package testutil

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestDB creates a test database connection
func TestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/test_db?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("Skipping test: cannot create pool: %v", err)
		return nil
	}

	// Test the connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("Skipping test: cannot connect to test database: %v", err)
		return nil
	}

	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}

// CleanupTables truncates tables for test isolation
func CleanupTables(t *testing.T, pool *pgxpool.Pool, tables ...string) {
	t.Helper()
	ctx := context.Background()

	for _, table := range tables {
		_, err := pool.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		if err != nil {
			t.Logf("Warning: failed to truncate %s: %v", table, err)
		}
	}
}

// StringPtr returns a pointer to a string
func StringPtr(s string) *string {
	return &s
}

// IntPtr returns a pointer to an int
func IntPtr(i int) *int {
	return &i
}

// TimePtr returns a pointer to a time
func TimePtr(t time.Time) *time.Time {
	return &t
}
