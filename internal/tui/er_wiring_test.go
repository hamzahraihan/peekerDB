package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/hamzahraihan/peekerDB/internal/db"
)

// erFixtureDDL is the schema the ER diagram needs: a parent table and a
// child table joined by a foreign key, so the diagram has a real link to
// draw. It is created per test run, so the test never depends on a
// database file in the working tree and can never be skipped.
var erFixtureDDL = []string{
	`CREATE TABLE categories (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		name        TEXT NOT NULL UNIQUE,
		description TEXT NOT NULL DEFAULT '',
		parent_id   INTEGER REFERENCES categories (id)
	)`,
	`CREATE TABLE products (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		category_id INTEGER NOT NULL REFERENCES categories (id) ON DELETE RESTRICT,
		name        TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE INDEX idx_products_category ON products (category_id)`,
	`INSERT INTO categories (id, name) VALUES (1, 'tools'), (2, 'garden')`,
	`INSERT INTO products (id, category_id, name) VALUES (1, 1, 'hammer'), (2, 1, 'spade'), (3, 2, 'trowel')`,
}

// newERFixtureDB opens a throwaway SQLite database holding the ER
// fixture schema. The file lives in t.TempDir() so the test is
// self-contained: it runs in CI, in a fresh clone, and on any machine.
func newERFixtureDB(t *testing.T) *db.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "er.db")
	// A zero-byte file is a valid empty SQLite database, and db.Open
	// refuses to create the file itself — it must exist first.
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("create fixture file: %v", err)
	}
	d, err := db.Open(path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	for _, stmt := range erFixtureDDL {
		if _, err := d.SQL.Exec(stmt); err != nil {
			d.Close()
			t.Fatalf("fixture DDL failed: %v\n%s", err, stmt)
		}
	}
	return d
}

// Entering the ER tab issues two commands — the per-table FK load and the
// full-schema load. They must share one cancellable context: a second
// context cancels the first, and the legacy view would keep saying
// "(no foreign keys)" forever.
func TestEnteringERTabLoadsRealForeignKeys(t *testing.T) {
	d := newERFixtureDB(t)
	defer d.Close()

	m := browseModel(t)
	m = apply(t, m, connectMsg{db: d})
	m = apply(t, m, schemasLoadedMsg{
		schemas: []string{"main"},
		tables:  map[string][]db.TableRef{"main": {{Name: "categories"}, {Name: "products"}}},
	})
	if m.table == "" {
		t.Fatalf("the schema load must select a table, got %q", m.table)
	}

	m = apply(t, m, testKey("5"))
	m = applyAll(t, m, m.setTab(4))

	if !m.erSchema.loaded {
		t.Fatalf("the ER schema must load: %+v", m.erSchema)
	}
	if len(m.erSchema.links) == 0 {
		t.Fatal("the fixture declares foreign keys; none were loaded")
	}
	if len(m.erLinks) == 0 {
		t.Fatal("the per-table foreign keys must be cached: the legacy view would show (no foreign keys)")
	}
	if m.erCache[m.table] == nil {
		t.Fatalf("erCache must hold %q, got %+v", m.table, m.erCache)
	}
	if out := m.erView(100, 20); strings.Contains(out, "(no foreign keys") {
		t.Fatalf("the diagram must render real connectors:\n%s", out)
	}
}

// apply sends one message to the model and ignores the reply.
func apply(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	nm, _ := m.Update(msg)
	return nm.(Model)
}

// applyAll runs a command and feeds every message it produces back
// through Update, descending into tea.Batch.
func applyAll(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	return applyCmd(t, m, cmd, 0)
}

func applyCmd(t *testing.T, m Model, cmd tea.Cmd, depth int) Model {
	t.Helper()
	if cmd == nil || depth > 4 {
		return m
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		nm, next := m.Update(msg)
		return applyCmd(t, nm.(Model), next, depth+1)
	}
	var next tea.Cmd
	for i, sub := range batch {
		if sub == nil {
			continue
		}
		subMsg := sub()
		nm, subNext := m.Update(subMsg)
		m = nm.(Model)
		if i == 0 {
			next = subNext
		}
	}
	return applyCmd(t, m, next, depth+1)
}
