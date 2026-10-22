package saved

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The store holds database passwords. When it cannot be encrypted it is
// written in the clear, and the user must be told: a silent downgrade
// leaves plain-text passwords on disk with nothing to show for it.
func TestPlainTextFallbackWarns(t *testing.T) {
	old := encryptPayload
	encryptPayload = func([]byte) ([]byte, error) { return nil, errors.New("no key") }
	defer func() { encryptPayload = old }()

	oldWarn := Warn
	var warned string
	Warn = func(msg string) { warned = msg }
	defer func() { Warn = oldWarn }()

	path := filepath.Join(t.TempDir(), "connections.json")
	s := &Store{Path: path}
	if err := s.Upsert("prod", "postgres://u:p@h/db"); err != nil {
		t.Fatal(err)
	}
	if warned == "" {
		t.Fatal("a plain-text fallback must be reported to the user")
	}
	if !strings.Contains(warned, "plain text") {
		t.Fatalf("the warning must say the file is in plain text: %q", warned)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "postgres://u:p@h/db") {
		t.Fatalf("the connection must still be saved: %s", data)
	}
}
