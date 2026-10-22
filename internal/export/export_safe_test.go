package export

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
)

// A cell that a spreadsheet would evaluate as a formula must not
// survive the export: Excel, LibreOffice and Google Sheets all evaluate
// a cell starting with "=", "+", "-" or "@", even a quoted one.
func TestSeparatedExportNeutralizesFormulas(t *testing.T) {
	hostile := [][]any{{"=cmd|'/c calc'!A0", "+1+1", "-2+3", "@SUM(A1)", "ok"}}
	for _, f := range []Format{FormatCSV, FormatTSV} {
		comma := ','
		if f == FormatTSV {
			comma = '\t'
		}
		var b bytes.Buffer
		if _, err := f.Write(&b, []string{"a", "b", "c", "d", "e"}, hostile); err != nil {
			t.Fatal(err)
		}
		r := csv.NewReader(bytes.NewReader(b.Bytes()))
		r.Comma = comma
		recs, err := r.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		for _, rec := range recs[1:] {
			for _, cell := range rec {
				if cell == "" {
					continue
				}
				switch cell[0] {
				case '=', '+', '-', '@':
					t.Errorf("%s: a formula must not survive the export: %q", f, cell)
				}
			}
		}
	}
}

// A plain number is data, not a formula: "-5" must stay a number
// rather than become text.
func TestSeparatedExportKeepsNumbers(t *testing.T) {
	var b bytes.Buffer
	if _, err := FormatCSV.Write(&b, []string{"n"}, [][]any{{int64(-5)}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "-5") {
		t.Fatalf("a negative number must stay a number: %q", b.String())
	}
	if strings.Contains(b.String(), "'-5") {
		t.Fatalf("a number must not be turned into text: %q", b.String())
	}
}

// The .sql export must be inert in every dialect: a value holding a
// backslash and a quote must not end the literal and start a new
// statement in MySQL, where a backslash escapes a quote.
func TestSQLLiteralIsInert(t *testing.T) {
	const hostile = `\'; DROP TABLE t; --`
	for _, d := range []Dialect{DialectStandard, DialectBackslash} {
		lit := sqlLiteral(hostile, d)
		if !sqlLiteralIsInert(lit, d) {
			t.Errorf("dialect %v: the literal must be inert, got %q", d, lit)
		}
	}
}

// The literal must also carry the value it stands for, so the dump
// still replays the data it exported.
func TestSQLLiteralKeepsTheValue(t *testing.T) {
	const val = `back\slash and 'quote'`
	for _, d := range []Dialect{DialectStandard, DialectBackslash} {
		lit := sqlLiteral(val, d)
		if !strings.HasPrefix(lit, "'") || !strings.HasSuffix(lit, "'") {
			t.Errorf("dialect %v: the literal must be quoted, got %q", d, lit)
		}
		if !strings.Contains(lit, "quote") {
			t.Errorf("dialect %v: the literal must carry the value, got %q", d, lit)
		}
	}
}

// sqlLiteralIsInert reports whether lit is a single, properly closed
// string literal under the dialect's escaping rules.
func sqlLiteralIsInert(lit string, d Dialect) bool {
	if len(lit) < 2 || lit[0] != '\'' {
		return false
	}
	body := lit[1 : len(lit)-1]
	for i := 0; i < len(body); i++ {
		if d == DialectBackslash && body[i] == '\\' {
			i++ // the next character is escaped
			continue
		}
		if body[i] != '\'' {
			continue
		}
		// A quote inside the literal must be doubled.
		if i+1 >= len(body) || body[i+1] != '\'' {
			return false
		}
		i++
	}
	return true
}
