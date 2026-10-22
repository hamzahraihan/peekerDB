package db

import (
	"strings"
	"testing"
)

// A connection string that cannot be opened must not put its password
// into the error: the error is shown on screen, printed to stderr and
// copied into logs.
//
// Postgres and MongoDB are absent on purpose: both drivers connect
// lazily, so their errors surface at Ping time, and both redact the
// DSN themselves.
func TestOpenErrorDoesNotLeakPassword(t *testing.T) {
	for _, c := range []struct{ conn, secret string }{
		{"user:pass@tcp(h:3306)/db", "pass"},
		{"mysql://u:p ss@h/db", "p ss"},
		{"mariadb://u:p ss@h/db", "p ss"},
		{"redis://u:p ss@h:6379", "p ss"},
	} {
		_, err := Open(c.conn)
		if err == nil {
			t.Errorf("Open(%q) must fail", c.conn)
			continue
		}
		if strings.Contains(err.Error(), c.secret) {
			t.Errorf("Open(%q) leaked the password: %v", c.conn, err)
		}
	}
}
