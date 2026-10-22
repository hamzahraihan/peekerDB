package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hamzahraihan/peekerDB/internal/saved"
)

// The edit form must not put the password on screen, and saving an
// untouched edit must not replace the stored password with the
// placeholder.
func TestEditConnFormHidesPasswordAndKeepsIt(t *testing.T) {
	store := &saved.Store{Path: filepath.Join(t.TempDir(), "connections.json")}
	const conn = "postgres://u:hunter2@h:5432/db"
	if err := store.Upsert("prod", conn); err != nil {
		t.Fatal(err)
	}

	m := browseModel(t)
	m.store = store
	m.screen = screenConns
	m.refreshConns()

	// "e" opens the edit form for the selected profile.
	u, _ := m.Update(testKey("e"))
	m = u.(Model)
	if m.screen != screenForm {
		t.Fatalf("'e' must open the edit form, screen %d", m.screen)
	}
	if strings.Contains(m.connInput.Value(), "hunter2") {
		t.Fatalf("the edit form must not show the password: %q", m.connInput.Value())
	}
	if strings.Contains(m.formView(), "hunter2") {
		t.Fatal("the edit form view must not show the password")
	}
	if !strings.Contains(m.connInput.Value(), "h:5432/db") {
		t.Fatalf("the edit form must still show the host: %q", m.connInput.Value())
	}

	// Saving without touching the password must keep it.
	m.nameInput.SetValue("prod")
	u, _ = m.Update(testKey("enter"))
	m = u.(Model)
	got, ok := m.store.Get("prod")
	if !ok {
		t.Fatal("the profile must still exist")
	}
	if got.Conn != conn {
		t.Fatalf("the stored password must survive an untouched edit: %q", got.Conn)
	}
}

// A profile whose conn string has no password must round-trip through
// the edit form unchanged.
func TestEditConnFormRoundTripsPasswordlessConn(t *testing.T) {
	store := &saved.Store{Path: filepath.Join(t.TempDir(), "connections.json")}
	const conn = "postgres://u@h:5432/db"
	if err := store.Upsert("prod", conn); err != nil {
		t.Fatal(err)
	}

	m := browseModel(t)
	m.store = store
	m.screen = screenConns
	m.refreshConns()

	u, _ := m.Update(testKey("e"))
	m = u.(Model)
	m.nameInput.SetValue("prod")
	u, _ = m.Update(testKey("enter"))
	m = u.(Model)

	got, _ := m.store.Get("prod")
	if got.Conn != conn {
		t.Fatalf("a passwordless conn must round-trip: %q", got.Conn)
	}
}
