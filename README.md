# peekerdb

[![CI](https://github.com/hamzahraihan/peekerDB/actions/workflows/ci.yml/badge.svg)](https://github.com/hamzahraihan/peekerDB/actions)
[![Release](https://github.com/hamzahraihan/peekerDB/actions/workflows/release.yml/badge.svg)](https://github.com/hamzahraihan/peekerDB/actions)
![Go 1.26+](https://img.shields.io/badge/go-1.26+-blue)

Peek at Postgres, MySQL, MariaDB, SQL Server, ClickHouse, Redis, MongoDB, and SQLite from the terminal. Interactive TUI plus one-shot CLI (`--list --schema --rows --query`).

## Install

```sh
go install github.com/hamzahraihan/peekerDB@latest
# or download peekerdb_<os>_<arch>.zip/tar.gz + checksums.txt from a v* release
go build -o peekerdb .
```

Requires Go 1.26+ for source builds.

`go install` names the binary after the repository, so it installs as
`peekerDB`; the release archives contain `peekerdb`. On a case-sensitive
filesystem the two are different names.

## Quickstart

```sh
peekerdb "postgres://user:pass@localhost:5432/db" --list
DATABASE_URL=postgres://user:pass@localhost:5432/db peekerdb --list
peekerdb ./app.db --rows users --limit 20 --offset 0
peekerdb ./app.db --schema users
peekerdb ./app.db --query "SELECT 1"
```

Conn forms: `postgres://...` | `postgresql://...` | `mysql://...` | `mariadb://...` | `sqlserver://...` | `mssql://...` | `clickhouse://...` | `redis://...` | `rediss://...` | `mongodb://...` | `mongodb+srv://...` | `./app.db` | `:memory:` | `sqlite://path` | `file:path`.
A bare saved NAME resolves to it. `DATABASE_URL` fills conn when no arg is given.
SQLite paths must exist; nothing is auto-created. To try it locally, create a
file first — `sqlite3 app.db "create table users(id integer primary key, name text)"` —
or point `peekerdb` at any existing SQLite database.

Table arguments accept a schema qualifier: `--schema public.users`,
`--rows public.users`. On Postgres a bare name that exists in more than one
schema is an error naming the candidates.

## CLI flags

| Flag | Effect |
|------|--------|
| `--db STR` | connection string |
| `--list` | print table names |
| `--schema [schema.]TABLE` | columns + indexes |
| `--rows [schema.]TABLE` | print N rows (`--limit`, `--offset`) |
| `--query SQL` | SELECT prints up to 200 rows; writes print N rows affected |
| `--limit N` | row limit for `--rows` (default 10) |
| `--offset N` | rows to skip (default 0) |
| `--save NAME [conn]` | save connection |
| `--forget NAME` | delete saved connection |
| `--conns` | list saved connections |
| `--read-only` | refuse writes and cell edits (SELECT/reads only) |
| `--version` | print version |

`--read-only` blocks a statement that would modify data or schema, including a
write hidden in a multi-statement script, in an `EXPLAIN ANALYZE`, or behind a
CTE. It also refuses the statements that would escape a read-only session,
such as `START TRANSACTION READ WRITE` or `SET transaction_read_only = 0`.

The session is also set read-only where the server supports it — Postgres,
MySQL, MariaDB, ClickHouse and SQLite. SQL Server has no session-level
read-only mode, so there the check above is the only guard; a read-only
login is the only server-side guarantee on that engine.

## TUI keys (`?` shows all in-app)

| Where | Keys |
|-------|------|
| Global | `q` quit, `tab` pane, `?` help, `esc` cancel query, `T`/`F3` theme picker |
| Sidebar | `/` filter, `enter` preview, `left/right` collapse/expand, `r` refresh, `c` connections |
| Detail | `1-5` schema/indexes/rows/query/er, `n/p` page, `s` size, `g/G` top/bottom, `,/.` column, `y/Y` yank cell/row, `F` filter cell, `v` view modal, `o` sort page, `/` find, `d` copy DDL, `e` edit cell, `E` export |
| Query editor | `ctrl+r/F5` run (selection, else statement at cursor), `ctrl+e/ctrl+y` explain, `ctrl+p/n` history, `ctrl+h` search history, `ctrl+t` new buf, `ctrl+w` close, `ctrl+d` del line, `ctrl+o` open `.sql`, `F2` save `.sql` |
| Results | `H/L` buf, `t/X` new/close, `n/p` page, `s` size, `y/Y` yank cell/row, `F` filter cell, `v` view modal, `o` sort page, `/` find |
| Connections | `a/e/d` add/edit/forget, `enter` connect |

## Saved connections

Stored under the OS config dir (`peekerdb/connections.json`, file `0600`); passwords masked in display. On Windows the file is encrypted with DPAPI (current user). History and session restore automatically.

The edit form shows the password as a placeholder. Leave it alone and the stored
password is kept; type a new one to replace it.

The query history and the last session are kept on disk too, in
`history.json` and `session.json`. They hold the SQL you ran, not the
connection string, but a query that embeds a secret puts that secret in the
history file.

## Export

In TUI press `E`: `.csv` `.tsv` `.json` `.md` `.sql` by extension. NULL is empty (JSON null).
Exports are written `0600`, and an existing file is replaced only after you confirm.

A `.csv`/`.tsv` cell starting with `=`, `+`, `-` or `@` is prefixed with an
apostrophe, so a spreadsheet does not evaluate it as a formula.

## Dev

```sh
go vet ./...
go test ./...
go build ./...
```

## License

MIT — see [LICENSE](LICENSE).
