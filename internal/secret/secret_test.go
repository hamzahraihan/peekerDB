package secret

import (
	"strings"
	"testing"
)

// Mask must never return the password, whatever shape the conn string
// takes. A password that url.Parse cannot read used to be returned
// as-is, which put it on screen, on stdout, in session.json and in the
// clipboard.
func TestMaskNeverLeaksPassword(t *testing.T) {
	for _, c := range []struct{ conn, secret string }{
		{"postgres://u:hunter2@h:5432/db", "hunter2"},
		{"postgres://u:p%40ss@h:5432/db", "p%40ss"},
		{"postgres://u:p ss@h:5432/db", "p ss"},
		{"postgres://u:50%off@h:5432/db", "50%off"},
		{"mysql://u:p%zz@h/db", "p%zz"},
		{"user:p@tcp(h:3306)/db", "p@tcp"},
		{"postgres://u@h/db?password=hunter2", "hunter2"},
		{"sqlserver://h?password=hunter2", "hunter2"},
		{"postgres://u@h/db?sslpassword=hunter2", "hunter2"},
		{"mongodb://u:hunter2@cluster/db", "hunter2"},
		{"clickhouse://u:hunter2@h:9000/db", "hunter2"},
		{"redis://:hunter2@h:6379", "hunter2"},
	} {
		if got := Mask(c.conn); strings.Contains(got, c.secret) {
			t.Errorf("Mask(%q) leaked %q: %q", c.conn, c.secret, got)
		}
	}
}

// The masked form marks the password slot, so an edit can leave it
// alone and keep the stored password.
func TestMaskMarksThePasswordSlot(t *testing.T) {
	got := Mask("postgres://u:hunter2@h:5432/db")
	if !strings.Contains(got, MaskPlaceholder) {
		t.Fatalf("Mask must mark the password slot: %q", got)
	}
	if !strings.Contains(got, "h:5432/db") {
		t.Fatalf("Mask must keep the host visible: %q", got)
	}
	if !strings.Contains(got, "u") {
		t.Fatalf("Mask must keep the user visible: %q", got)
	}
}

// A conn string with no password is shown as it is.
func TestMaskLeavesPasswordlessConnAlone(t *testing.T) {
	for _, conn := range []string{
		"postgres://u@h:5432/db",
		"./app.db",
		":memory:",
		"sqlite://app.db",
		"tcp(host:3306)/db",
	} {
		if got := Mask(conn); got != conn {
			t.Errorf("Mask(%q) = %q, want it unchanged", conn, got)
		}
	}
}

// A password that holds an "@" must be found whole: the userinfo ends
// at the last "@" of the authority, not the first.
func TestPasswordFindsAtInsideThePassword(t *testing.T) {
	pass, ok := Password("postgres://u:p@ss@h:5432/db")
	if !ok || pass != "p@ss" {
		t.Fatalf("Password = %q, %v; want %q", pass, ok, "p@ss")
	}
	// A path may hold an "@" without being part of the userinfo.
	pass, ok = Password("postgres://u:p@h/db@x")
	if !ok || pass != "p" {
		t.Fatalf("Password = %q, %v; want %q", pass, ok, "p")
	}
}

// Editing a connection must not replace the stored password with the
// placeholder, and must not keep the old password when the user typed a
// new one.
func TestMergeEditedConn(t *testing.T) {
	stored := "postgres://u:hunter2@h:5432/db"
	shown := Mask(stored)

	if got := MergeEditedConn(shown, stored); got != stored {
		t.Errorf("an untouched edit must keep the stored conn: %q", got)
	}
	if got := MergeEditedConn("postgres://u:newpass@h:5432/db", stored); got != "postgres://u:newpass@h:5432/db" {
		t.Errorf("a typed password must win: %q", got)
	}
	// The user changed the host but left the password slot alone.
	if got := MergeEditedConn("postgres://u:xxxxx@other:5432/db", stored); got != "postgres://u:hunter2@other:5432/db" {
		t.Errorf("a changed host must keep the stored password: %q", got)
	}
	// A conn string with no password is left alone.
	if got := MergeEditedConn("postgres://u@h:5432/db", "postgres://u@h:5432/db"); got != "postgres://u@h:5432/db" {
		t.Errorf("a conn with no password must be left alone: %q", got)
	}
	// The raw MySQL DSN form.
	dsn := "user:pw@tcp(h:3306)/db"
	if got := MergeEditedConn(Mask(dsn), dsn); got != dsn {
		t.Errorf("an untouched DSN edit must keep the stored conn: %q", got)
	}
	if got := MergeEditedConn("user:xxxxx@tcp(other:3306)/db", dsn); got != "user:pw@tcp(other:3306)/db" {
		t.Errorf("a changed DSN host must keep the stored password: %q", got)
	}
}
