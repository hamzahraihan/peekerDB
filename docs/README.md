# docs

Historical design specs and implementation plans, kept as a record of
what was decided and why at the time.

**These are point-in-time documents, not current documentation.** The
project was renamed from `db-peek` to `peekerdb` in October 2025, so a
doc that mentions `db-peek`, `db-peek/connections.json` or
`github.com/hamzahraihan/db-peek` describes the project as it was
then. Line numbers, file paths and flag names in them may no longer
match the code.

**For current behaviour, read the [README](../README.md) and the code
itself.** The README is the source of truth for what the tool does.

## Layout

- `superpowers/specs/` — the design for a feature, written before it
  was built.
- `superpowers/plans/` — the step-by-step plan used to build it.

Each file is named `YYYY-MM-DD-<feature>-design.md` or
`YYYY-MM-DD-<feature>-plan.md`.

## Superseded

- `superpowers/specs/2026-10-01-readme-ci-releases-design.md` — the
  install command and release asset names in it predate the module
  path fix. The current install command is
  `go install github.com/hamzahraihan/peekerDB@latest`.
