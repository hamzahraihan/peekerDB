package saved

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// The temp file must have a name nobody can predict, so a file planted
// at the predictable path is never the one written.
func TestWriteAtomicIgnoresThePlantedTempName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "connections.json")
	planted := path + ".tmp"
	if err := os.WriteFile(planted, []byte("planted"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeAtomic(path, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(planted)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "planted" {
		t.Fatalf("the planted file must be untouched: %q", got)
	}
	got, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "payload" {
		t.Fatalf("the payload must land in the target: %q", got)
	}
}

// The store holds passwords. It must never be readable by anyone but
// its owner, whatever a stale temp file or a pre-existing config dir
// looked like.
func TestStoreStaysPrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission bits are not enforced on Windows")
	}
	dir := t.TempDir()
	// A config dir that already exists with loose permissions.
	cfg := filepath.Join(dir, "peekerdb")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cfg, "connections.json")
	// A stale temp file with loose permissions.
	if err := os.WriteFile(path+".tmp", []byte("stale"), 0o666); err != nil {
		t.Fatal(err)
	}
	s := &Store{Path: path}
	if err := s.Upsert("prod", "postgres://u:p@h/db"); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{path, cfg} {
		st, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm()&0o077 != 0 {
			t.Errorf("%s must not be group or world accessible: %v", p, st.Mode().Perm())
		}
	}
}

// A store that could not read its file must not overwrite it: the
// user's saved connections would be gone, with no backup.
func TestFailedLoadDoesNotWipeTheFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "connections.json")
	corrupt := []byte("{not json at all")
	if err := os.WriteFile(path, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := loadAt(path)
	if err == nil {
		t.Fatal("a corrupt file must fail to load")
	}
	if err := s.Upsert("prod", "postgres://u:p@h/db"); err == nil {
		t.Fatal("a store that failed to load must refuse to save")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(corrupt) {
		t.Fatalf("the unreadable file must be left alone: %q", got)
	}
}
