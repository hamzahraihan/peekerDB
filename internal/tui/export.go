package tui

// Result export: E opens a one-line path prompt rendered in place of the
// pane's status line, so the row count above it never changes and the
// mouse math (detailTableTop) stays valid.

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textinput"

	dbpkg "github.com/hamzahraihan/peekerDB/internal/db"
	"github.com/hamzahraihan/peekerDB/internal/export"
)

// exportTarget is the result set the export overlay applies to.
func (m Model) exportTarget() (cols []string, raw [][]any, name string) {
	if m.tab == 3 {
		if m.querySample == nil {
			return nil, nil, "query"
		}
		return m.querySample.Columns, m.querySample.Raw, "query"
	}
	if m.sample == nil {
		return nil, nil, m.table
	}
	return m.sample.Columns, m.sample.Raw, m.table
}

// startExport opens the path prompt for the visible result set.
func (m *Model) startExport() {
	_, _, name := m.exportTarget()
	ext := ".csv"
	if m.tab == 3 {
		ext = ".json" // a query result is often heterogeneous
	}
	m.exporting = true
	m.exportInput = textinput.New()
	m.exportInput.Prompt = "export → "
	m.exportInput.CharLimit = 200
	m.exportInput.SetWidth(m.paneW())
	m.exportInput.SetValue(fmt.Sprintf("./peekerdb-%s%s", sanitizeFileStem(name), ext))
	m.exportInput.CursorEnd()
	m.exportInput.Focus()
}

// stopExport closes the prompt without writing anything.
func (m *Model) stopExport() {
	m.exporting = false
	m.exportInput.Blur()
}

// finishExport writes the visible result set to the entered path and
// reports the outcome on the status line.
func (m *Model) finishExport() {
	path := strings.TrimSpace(m.exportInput.Value())
	if path == "" {
		m.stopExport()
		return
	}
	format, err := export.FormatFromExt(path)
	if err != nil {
		m.stopExport()
		m.err = err.Error()
		return
	}
	cols, raw, _ := m.exportTarget()
	if len(raw) == 0 {
		m.stopExport()
		m.err = "nothing to export"
		return
	}
	// An existing file is not overwritten without asking: the default
	// export path is predictable, so the file that is already there may
	// be one the user cares about.
	if _, err := os.Lstat(path); err == nil {
		m.armExportOverwrite(path)
		return
	}
	m.writeExport(path, format, cols, raw, false)
}

// armExportOverwrite asks the user before replacing an existing file.
func (m *Model) armExportOverwrite(path string) {
	m.stopExport()
	m.exportConfirmPath = path
	m.status = "file exists — overwrite " + path + "? y overwrite, n cancel"
}

// confirmExportOverwrite replaces the file the user agreed to replace.
func (m *Model) confirmExportOverwrite(path string) {
	format, err := export.FormatFromExt(path)
	if err != nil {
		m.err = err.Error()
		return
	}
	cols, raw, _ := m.exportTarget()
	if len(raw) == 0 {
		m.err = "nothing to export"
		return
	}
	// The overwrite must not follow a symlink: the default export path
	// is predictable, so a planted link would redirect the rows into
	// another file.
	if fi, err := os.Lstat(path); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		m.err = "refusing to overwrite a symlink: " + path
		return
	}
	m.writeExport(path, format, cols, raw, true)
}

// writeExport writes the rows to path. The file is created 0600: an
// export holds database rows, which can be sensitive. A new file is
// created exclusively, so a link planted at the path is never followed.
func (m *Model) writeExport(path string, format export.Format, cols []string, raw [][]any, overwrite bool) {
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if overwrite {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	f, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		if os.IsExist(err) {
			m.armExportOverwrite(path)
			return
		}
		m.stopExport()
		m.err = err.Error()
		return
	}
	n, werr := format.WriteTo(f, cols, raw, exportDialect(m.db))
	cerr := f.Close()
	m.stopExport()
	switch {
	case werr != nil:
		m.err = werr.Error()
	case cerr != nil:
		m.err = cerr.Error()
	default:
		m.err = ""
		m.status = fmt.Sprintf("exported %d rows → %s", n, path)
	}
}

// exportDialect maps the connected driver to the SQL dialect a .sql
// export must target: MySQL, MariaDB and ClickHouse treat a backslash
// inside a string literal as an escape character, the others do not.
func exportDialect(d *dbpkg.DB) export.Dialect {
	if d == nil {
		return export.DialectStandard
	}
	switch d.Driver {
	case dbpkg.MySQL, dbpkg.MariaDB, dbpkg.ClickHouse:
		return export.DialectBackslash
	}
	return export.DialectStandard
}

// exportOverlayLine replaces a status line with the export prompt. It
// stays exactly one line so nothing below it moves.
func (m Model) exportOverlayLine(fallback string) string {
	if !m.exporting {
		return fallback
	}
	return fitText(m.exportInput.View(), m.paneW())
}

func sanitizeFileStem(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-.")
	if out == "" {
		return "query"
	}
	return out
}
