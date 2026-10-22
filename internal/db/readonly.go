package db

// Read-only mode: deciding whether a statement would write.
//
// The gate is deliberately conservative. Anything it does not recognise
// as a read is treated as a write, so an unfamiliar dialect or syntax
// blocks rather than permits. The opposite mistake — calling a write a
// read — is what lets a write slip past --read-only.

import "strings"

// IsWriteStatement reports whether sql would modify data or schema
// (INSERT/UPDATE/DELETE/MERGE/REPLACE/DDL), including statements with
// a RETURNING clause or mutating CTEs. Callers gate read-only mode on this.
//
// The whole script is classified, not just its first statement:
// RunUserQuery executes every statement it is handed, so a script whose
// first statement is a SELECT still writes.
//
// Anything the gate does not recognise is treated as a write. That is
// what refuses the statements that would escape a read-only session —
// "START TRANSACTION READ WRITE", "SET transaction_read_only = 0",
// "PRAGMA query_only = OFF" — which the servers would otherwise accept.
func IsWriteStatement(sql string) bool {
	for _, stmt := range SplitStatements(sql) {
		if isWriteStatementOne(stmt) {
			return true
		}
	}
	return false
}

// isWriteStatementOne classifies a single statement.
func isWriteStatementOne(sql string) bool {
	s := stripLeadingComments(sql)
	if s == "" {
		return false
	}
	// EXPLAIN ANALYZE runs the statement it explains, so it is that
	// statement, not a read.
	if startsWithKeyword(s, "EXPLAIN") {
		return isWriteStatementOne(explainTarget(s))
	}
	toks := sqlKeywords(s)
	if len(toks) == 0 {
		return false
	}
	switch toks[0] {
	case "INSERT", "UPDATE", "DELETE", "MERGE", "REPLACE",
		"CREATE", "DROP", "ALTER", "TRUNCATE", "GRANT", "REVOKE":
		return true
	case "WITH":
		// A CTE can hide a write. Any write keyword after the leading WITH
		// makes the statement a write, whatever follows it: "INSERT"
		// without INTO and "MERGE ... THEN INSERT" are both valid in some
		// dialects.
		for _, t := range toks[1:] {
			switch t {
			case "INSERT", "UPDATE", "DELETE", "MERGE", "REPLACE",
				"TRUNCATE", "DROP", "CREATE", "ALTER":
				return true
			}
		}
		return false
	case "SELECT", "VALUES", "TABLE":
		return false
	default:
		return true
	}
}

// explainTarget returns the statement an EXPLAIN would run, or "" when
// there is none. "EXPLAIN ANALYZE DELETE FROM t" yields "DELETE FROM t".
func explainTarget(s string) string {
	rest := strings.TrimSpace(s[len("explain"):])
	// An option list: EXPLAIN (ANALYZE, BUFFERS) ...
	if strings.HasPrefix(rest, "(") {
		end := strings.Index(rest, ")")
		if end < 0 {
			return ""
		}
		rest = strings.TrimSpace(rest[end+1:])
	}
	// The optional ANALYZE keyword, which may carry its own options:
	// EXPLAIN ANALYZE (BUFFERS) DELETE FROM t
	if word, tail := firstWord(rest); word == "ANALYZE" {
		rest = tail
		if strings.HasPrefix(rest, "(") {
			end := strings.Index(rest, ")")
			if end < 0 {
				return ""
			}
			rest = strings.TrimSpace(rest[end+1:])
		}
	}
	return rest
}

// sqlKeywords returns the words of s in upper case, ignoring string
// literals, quoted identifiers and comments. A word inside a literal is
// data, not a keyword: "SELECT 'update'" is a read.
func sqlKeywords(s string) []string {
	var out []string
	var word strings.Builder
	flush := func() {
		if word.Len() > 0 {
			out = append(out, strings.ToUpper(word.String()))
			word.Reset()
		}
	}
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\'' || c == '"' || c == '`':
			flush()
			quote := c
			i++
			for i < len(s) {
				if s[i] != quote {
					i++
					continue
				}
				if i+1 < len(s) && s[i+1] == quote {
					i += 2 // an escaped quote, not the end of the literal
					continue
				}
				i++ // step over the closing quote
				break
			}
		case c == '-' && i+1 < len(s) && s[i+1] == '-':
			flush()
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(s) && s[i+1] == '*':
			flush()
			i += 2
			for i+1 < len(s) && !(s[i] == '*' && s[i+1] == '/') {
				i++
			}
			i++ // step over the '/'
		case isWordByte(c):
			word.WriteByte(c)
			i++
		default:
			flush()
			i++
		}
	}
	flush()
	return out
}

// firstWord returns the leading word of s in upper case and the rest of
// s after it. The word is "" when s starts with no word character.
func firstWord(s string) (string, string) {
	s = strings.TrimSpace(s)
	i := 0
	for i < len(s) && isWordByte(s[i]) {
		i++
	}
	return strings.ToUpper(s[:i]), strings.TrimSpace(s[i:])
}

// startsWithKeyword reports whether s begins with kw as a whole word.
func startsWithKeyword(s, kw string) bool {
	if len(s) < len(kw) || !strings.EqualFold(s[:len(kw)], kw) {
		return false
	}
	return len(s) == len(kw) || !isWordByte(s[len(kw)])
}

func isWordByte(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
