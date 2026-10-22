package tui

// SQL file workflow: ctrl+o loads a .sql file into the query buffer,
// F2 saves the buffer back. The path prompt overlays the footer (like
// grid-find) so layout and mouse math never shift.

import (
	"os"
	"path/filepath"
	"strings"
)

// maxSQLFileBytes caps the open size so a multi-GB dump can never
// flood the 8-row editor.
const maxSQLFileBytes = 256 * 1024

// readSQLFile reads a file for the open path. It is a variable so a
// test can prove the size is checked before the read happens.
var readSQLFile = os.ReadFile

// openSQLFile arms the prompt in open mode, prefilled with the last path.
func (m *Model) openSQLFile() {
	if m.tab != 3 {
		return
	}
	m.dismissCompletion()
	m.sqlFiling, m.sqlFileMode = true, "open"
	m.sqlFileInput.SetWidth(max(20, m.paneW()-10))
	m.sqlFileInput.SetValue(m.lastSQLPath)
	m.sqlFileInput.CursorEnd()
}

// saveSQLFile arms the prompt in save mode.
func (m *Model) saveSQLFile() {
	if m.tab != 3 {
		return
	}
	m.dismissCompletion()
	m.sqlFiling, m.sqlFileMode = true, "save"
	m.sqlFileInput.SetWidth(max(20, m.paneW()-10))
	m.sqlFileInput.SetValue(m.lastSQLPath)
	m.sqlFileInput.CursorEnd()
}

// stopSQLFile closes the prompt without acting.
func (m *Model) stopSQLFile() {
	m.sqlFiling = false
	m.sqlFileInput.Blur()
}

// confirmSQLFile runs the armed open or save.
func (m *Model) confirmSQLFile() {
	path := strings.TrimSpace(m.sqlFileInput.Value())
	if path == "" {
		m.stopSQLFile()
		m.status = "no file chosen"
		return
	}
	if m.sqlFileMode == "save" {
		m.saveBufferToPath(path)
		return
	}
	m.loadBufferFromPath(path)
}

// loadBufferFromPath replaces the editor text with the file content.
// A non-empty buffer arms a y/n confirm instead of loading, so an
// accidental open can never clobber typed SQL.
func (m *Model) loadBufferFromPath(path string) {
	if strings.TrimSpace(m.editor.Text()) != "" && m.sqlConfirmPath == "" {
		m.stopSQLFile()
		m.sqlConfirmPath = path
		m.status = "buffer modified — open " + path + "? y replace, n keep"
		return
	}
	m.sqlConfirmPath = ""
	m.finishLoadBufferFromPath(path)
}

// finishLoadBufferFromPath performs the armed/confirmed load without any
// dirty check; loadBufferFromPath is the entry point that may arm first.
func (m *Model) finishLoadBufferFromPath(path string) {
	// The size is checked before the read, so a multi-GB dump is never
	// loaded into memory just to be rejected afterwards.
	fi, err := os.Stat(path)
	if err != nil {
		m.stopSQLFile()
		m.err = "open failed: " + err.Error()
		return
	}
	if fi.Size() > maxSQLFileBytes {
		m.stopSQLFile()
		m.err = "open failed: file too large (>256KB)"
		return
	}
	data, err := readSQLFile(path)
	if err != nil {
		m.stopSQLFile()
		m.err = "open failed: " + err.Error()
		return
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\x00", "")
	m.editor.SetText(text)
	m.editor.MoveToEnd()
	m.clampEditorScroll()
	m.refreshCompletionAfterEdit()
	m.lastSQLPath = path
	m.stopSQLFile()
	m.err = ""
	lines := len(strings.Split(text, "\n"))
	unit := "lines"
	if lines == 1 {
		unit = "line"
	}
	m.status = "loaded " + path + " (" + itoa(lines) + " " + unit + ")"
}

// saveBufferToPath writes the buffer atomically at 0600: queries can
// embed DSN passwords, so world-readable is never the default.
func (m *Model) saveBufferToPath(path string) {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			m.stopSQLFile()
			m.err = "save failed: " + err.Error()
			return
		}
	}
	// A randomly named temp file in the same directory: a predictable
	// name could be planted as a symlink and followed, and a leftover
	// temp file would keep its own permissions.
	f, err := os.CreateTemp(dir, ".peekerdb-*.sql")
	if err != nil {
		m.stopSQLFile()
		m.err = "save failed: " + err.Error()
		return
	}
	tmp := f.Name()
	defer os.Remove(tmp) // no-op once the rename succeeds
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		m.stopSQLFile()
		m.err = "save failed: " + err.Error()
		return
	}
	if _, err := f.WriteString(m.editor.Text()); err != nil {
		f.Close()
		m.stopSQLFile()
		m.err = "save failed: " + err.Error()
		return
	}
	if err := f.Close(); err != nil {
		m.stopSQLFile()
		m.err = "save failed: " + err.Error()
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		m.stopSQLFile()
		m.err = "save failed: " + err.Error()
		return
	}
	m.lastSQLPath = path
	m.stopSQLFile()
	m.err = ""
	m.status = "saved " + path
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
