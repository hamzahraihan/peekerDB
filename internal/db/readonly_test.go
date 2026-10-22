package db

import (
	"testing"
)

func TestIsWriteStatement(t *testing.T) {
	for sql, want := range map[string]bool{
		"SELECT * FROM t":                         false,
		"  -- comment\nUPDATE t SET a = 1":        true,
		"WITH x AS (SELECT 1) SELECT * FROM x":    false,
		"INSERT INTO t(a) VALUES (1)":             true,
		"DELETE FROM t":                           true,
		"CREATE TABLE t(a INT)":                   true,
		"EXPLAIN SELECT * FROM t":                 false,
		"INSERT INTO t(a) VALUES (1) RETURNING a": true,
		"DELETE FROM t RETURNING id":              true,
		"WITH d AS (DELETE FROM t) SELECT 1":      true,
		"WITH u AS (UPDATE t SET a = 1) SELECT 1": true,
	} {
		if got := IsWriteStatement(sql); got != want {
			t.Fatalf("IsWriteStatement(%q) = %v, want %v", sql, got, want)
		}
	}
}

// Read-only mode refuses writes, so the gate must classify the whole
// script: RunUserQuery executes every statement it is handed, and a
// script whose first statement is a SELECT still writes.
func TestIsWriteStatementCoversWholeScript(t *testing.T) {
	for sql, want := range map[string]bool{
		"SELECT 1; DROP TABLE users":                    true,
		"SELECT 'a;'; DELETE FROM t":                    true,
		"SELECT 1; SELECT 2":                            false,
		"SELECT 1; SELECT 2; UPDATE t SET a = 1":        true,
		"INSERT INTO t(a) VALUES (1); SELECT 1":         true,
		"SELECT 1; CREATE TABLE t(a INT)":               true,
		"SELECT 1;WITH x AS (SELECT 1) SELECT * FROM x": false,
		"-- lead\nSELECT 1; DELETE FROM t":              true,
		"SELECT 1 /* ; DROP TABLE t */; DELETE FROM t":  true,
		"SELECT 'DELETE FROM t'":                        false,
		"SELECT 1; -- trailing comment\n":               false,
		"":                                              false,
		";":                                             false,
		";;":                                            false,
	} {
		if got := IsWriteStatement(sql); got != want {
			t.Fatalf("IsWriteStatement(%q) = %v, want %v", sql, got, want)
		}
	}
}

// EXPLAIN ANALYZE runs the statement it explains, so it is classified
// as that statement. Plain EXPLAIN of a read stays a read.
func TestIsWriteStatementSeesThroughExplainAnalyze(t *testing.T) {
	for sql, want := range map[string]bool{
		"EXPLAIN ANALYZE DELETE FROM t":                 true,
		"EXPLAIN ANALYZE UPDATE t SET a = 1":            true,
		"EXPLAIN (ANALYZE, BUFFERS) UPDATE t SET a = 1": true,
		"EXPLAIN (ANALYZE) DELETE FROM t":               true,
		"explain analyze delete from t":                 true,
		"EXPLAIN ANALYZE SELECT * FROM t":               false,
		"EXPLAIN SELECT * FROM t":                       false,
		"EXPLAIN (ANALYZE, BUFFERS) SELECT * FROM t":    false,
		"EXPLAIN": false,
	} {
		if got := IsWriteStatement(sql); got != want {
			t.Fatalf("IsWriteStatement(%q) = %v, want %v", sql, got, want)
		}
	}
}

// A CTE can hide a write behind any keyword spelling: "INSERT" without
// INTO and "MERGE ... THEN INSERT" are both valid in some dialects.
func TestIsWriteStatementSeesThroughMutatingCTE(t *testing.T) {
	for sql, want := range map[string]bool{
		"WITH x AS (SELECT 1) MERGE INTO t USING x ON true WHEN NOT MATCHED THEN INSERT VALUES (1)": true,
		"WITH x AS (SELECT 1) MERGE INTO t USING x ON true WHEN MATCHED THEN DELETE":                true,
		"WITH x AS (SELECT 1) INSERT t SELECT * FROM x":                                             true,
		"WITH x AS (SELECT 1) DELETE t":                                                             true,
		"WITH x AS (SELECT 1) TRUNCATE t":                                                           true,
		// A keyword inside a literal is not a keyword: the statement reads.
		"WITH x AS (SELECT 'update') SELECT 1":     false,
		"WITH x AS (SELECT 'x update y') SELECT 1": false,
		"WITH x AS (SELECT 1) SELECT * FROM x":     false,
	} {
		if got := IsWriteStatement(sql); got != want {
			t.Fatalf("IsWriteStatement(%q) = %v, want %v", sql, got, want)
		}
	}
}

// A read-only session can be escaped from the server side: Postgres and
// MySQL let a transaction be started read-write explicitly, and every
// driver can have its read-only setting turned off again. The client
// gate must refuse all of them, whatever the session allows.
func TestIsWriteStatementBlocksReadOnlyEscapeHatches(t *testing.T) {
	for _, sql := range []string{
		"START TRANSACTION READ WRITE",
		"START TRANSACTION READ WRITE; INSERT INTO t VALUES (1)",
		"BEGIN READ WRITE",
		"SET default_transaction_read_only = off",
		"SET transaction_read_only = 0",
		"SET tx_read_only = 0",
		"SET readonly = 0",
		"PRAGMA query_only = OFF",
	} {
		if !IsWriteStatement(sql) {
			t.Errorf("IsWriteStatement(%q) = false, want true: it would escape the read-only session", sql)
		}
	}
}

func TestIsRowReturning(t *testing.T) {
	for sql, want := range map[string]bool{
		"SELECT * FROM t":                         true,
		"INSERT INTO t(a) VALUES (1) RETURNING a": true,
		"DELETE FROM t RETURNING id":              true,
		"WITH x AS (SELECT 1) SELECT * FROM x":    true,
		"EXPLAIN SELECT * FROM t":                 true,
		"INSERT INTO t(a) VALUES (1)":             false,
		"DELETE FROM t":                           false,
		"UPDATE t SET a = 1":                      false,
		"CREATE TABLE t(a INT)":                   false,
	} {
		if got := IsRowReturning(sql); got != want {
			t.Fatalf("IsRowReturning(%q) = %v, want %v", sql, got, want)
		}
	}
}
