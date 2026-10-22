package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// readOnlyDB opens a throwaway SQLite database with one table.
func readOnlyDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ro.db")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.SQL.Exec(`CREATE TABLE t(a INTEGER)`); err != nil {
		d.Close()
		t.Fatal(err)
	}
	return d
}

// Read-only mode must be enforced by the session, not only by the
// client's own check: a write that reaches the server must be refused.
func TestApplyReadOnlyRefusesWrites(t *testing.T) {
	d := readOnlyDB(t)
	defer d.Close()
	ctx := context.Background()
	if err := d.ApplyReadOnly(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SQL.ExecContext(ctx, `INSERT INTO t VALUES (1)`); err == nil {
		t.Fatal("the session must refuse a write in read-only mode")
	}
	rows, err := d.SQL.QueryContext(ctx, `SELECT count(*) FROM t`)
	if err != nil {
		t.Fatalf("the session must still allow a read: %v", err)
	}
	rows.Close()
}

// The session setting is session-scoped, so it holds only for the
// connection it was applied to. The pool must be pinned to that
// connection, or a later statement would run on a fresh one without it.
func TestApplyReadOnlyPinsThePool(t *testing.T) {
	d := readOnlyDB(t)
	defer d.Close()
	if err := d.ApplyReadOnly(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := d.SQL.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("the pool must be pinned to one connection, got %d", got)
	}
}
