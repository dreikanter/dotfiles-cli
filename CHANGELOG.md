# Changelog

## [Unreleased]

### Changed

- `status` reports the in-sync state as `synced` (previously `in sync`) in JSON output and the plain-text summary.

## [0.1.1] - 2026-05-04

### Changed

- Rewrote README for clarity.

### Fixed

- `dotfiles --version` no longer fails with `unknown flag: --version` when the binary is built with an empty `Version` ldflag value; it now falls back to Go module build info and finally to `unknown`.

## [0.1.0] - 2026-05-04

### Added

- `dotfiles init` command that scaffolds a fresh dotfiles repository: creates the target directory, an empty `dotfiles.json`, a short `README.md` written for the dotfiles user, and runs `git init`. Honors `--dry-run`, `--json`, `--verbose`; pass `--force` to overwrite existing `dotfiles.json` / `README.md`.
- Initial Go implementation of the dotfiles CLI with `save`, `apply`, `status`, and `config` commands.
- Cobra-based command surface with `--dry-run`, `--verbose`, and `--prune` flags.
- Sample dotfiles in `testdata/sample-repo` for tests and demonstration.
- Persistent `--json` flag on every command emitting a single JSON object on stdout, with a consistent `{"error": {"message": "..."}}` envelope on failure. `--verbose` is ignored in JSON mode and the process still exits non-zero on error.

### Changed

- `config` defaults to a plain-text table; pass `--json` for the structured form.
- `status --json` now emits `{"entries": [...], "summary": {...}}` instead of a bare array.
- `config --json` now emits `{"entries": [{"tool", "local", "dotfile"}, ...]}` instead of a flat dotfile→local map (the `tool` field is now preserved).
- `save --json` / `apply --json` emit `{"direction", "dryRun", "copied", "removed", "errors", "actions": [...]}` with per-file actions.
- `save`, `apply`, `status`, and `config` now accept `--tool <name>` to scope the operation to a single manifest tool, and `--tool <name> --file <path>` (repeatable) to scope it to specific files within that tool. `--file` is mutually exclusive with `--prune`. The plain-text header for `save`/`apply` echoes the active filter (`[tool=git, files=2]`) and `save`/`apply` JSON output gains a `filter` field with the resolved tool and absolute file paths.
