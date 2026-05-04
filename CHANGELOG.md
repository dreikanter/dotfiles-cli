# Changelog

## [Unreleased]

### Added

- Initial Go implementation of the dotfiles CLI with `save`, `apply`, `status`, and `config` commands.
- Cobra-based command surface with `--dry-run`, `--verbose`, and `--prune` flags.
- Sample dotfiles in `testdata/sample-repo` for tests and demonstration.
- Persistent `--json` flag on every command emitting a single JSON object on stdout, with a consistent `{"error": {"message": "..."}}` envelope on failure. `--verbose` is ignored in JSON mode and the process still exits non-zero on error.

### Changed

- `config` defaults to a plain-text table; pass `--json` for the structured form.
- `status --json` now emits `{"entries": [...], "summary": {...}}` instead of a bare array.
- `config --json` now emits `{"entries": [{"tool", "local", "dotfile"}, ...]}` instead of a flat dotfile→local map (the `tool` field is now preserved).
- `save --json` / `apply --json` emit `{"direction", "dryRun", "copied", "removed", "errors", "actions": [...]}` with per-file actions.
