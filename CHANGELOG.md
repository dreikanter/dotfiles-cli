# Changelog

## [Unreleased]

## [0.2.0] - 2026-05-27

### Added

- `dotfiles track --tool <name> <path>` adds a path to the manifest. The path
  must exist on disk; directories are detected automatically and stored with a
  trailing slash. Paths under HOME are stored as `~/...` entries. [#55]
- `dotfiles untrack --tool <name> <path>` removes a path from the manifest.
  Pass `--purge` to also delete the saved copy from the repository. Both
  commands support `--dry-run` and `--json`. [#55]

### Changed

- A manifest path's trailing slash is now the only thing that marks it as a
  directory. Previously the tool would also guess based on what existed on
  disk, which could differ between machines. Directory entries must now end
  with `/`. [#54]

### Fixed

- Mismatches between a manifest entry and what is on disk are now reported
  loudly instead of being silently misread: a directory declared without a
  trailing slash, or a file declared with one, surfaces as an error in
  `status`, `save`, and `install` rather than being skipped or shown as in
  sync. [#54]

## [0.1.6] - 2026-05-27

### Fixed

- `dotfiles install` no longer fails with "source is a directory" for directory entries whose live path does not exist yet but whose saved path is already a directory in the repository. [#53]

## [0.1.5] - 2026-05-20

### Added

- `dotfiles skill` prints an agent-installable skill document describing the CLI — every available subcommand, the global flags, the manifest model, and the JSON error envelope. Pass `--install --agent=claude` to write it directly into Claude Code's skills directory, or `--install` on its own to auto-detect supported agents and install into each. Honors `--dry-run`, `--force`, byte-equal `skip` detection, and the standard `--json` output. Closes [#46].

### Changed

- README now explains why tracked files are copied rather than symlinked.

### Fixed

- `dotfiles --version` no longer prints a redundant `version v...`; the output is now `dotfiles vX.Y.Z`.
- `dotfiles status` plain-text rows now read as state descriptions (`live newer`, `not in repo`, `not on disk`, `saved newer`, `both missing`) instead of phrases like `saved missing` that parsed as verbs and resembled `save` command output. JSON state tokens are unchanged.

## [0.1.4] - 2026-05-04

### Removed

- Manifest paths no longer accept a trailing `/*` as a directory marker; use a trailing `/` instead.

## [0.1.3] - 2026-05-04

### Added

- `config` output now includes the resolved dotfiles repository root and manifest path: top-level `root` and `config` fields in `--json` mode, and `Root: <path>` and `Config: <path>` header lines above a blank line separating them from the entries in plain-text mode.
- `status` now surfaces real I/O errors (e.g. permission denied) as a dedicated `error` state instead of misreporting them as `neither-exists` / `live-missing` / `saved-missing`. JSON entries gain an optional `error` field with the underlying message; plain-text rows include the error after the path. Closes [#15].

### Changed

- `dotfiles init` now writes a minimal `README.md` that points at `dotfiles-cli` and `dotfiles --help`, rather than duplicating the tool's own README. Closes [#23].
- The two sides of every sync are now called `live` (the file at the path each tool reads) and `saved` (the copy in the dotfiles repository), replacing the previous `local`/`dotfile` terminology. This affects: status state values (`live-missing`, `saved-missing`, `live-changes`, `saved-changes`), `status --json` and `config --json` per-entry fields (`live`, `saved`), and plain-text headers for `save`/`install`. Closes [#14].
- Renamed the `apply` command to `install`. The `load` alias is dropped.
- Dropped the `ls` alias for `status`; the command only lists out-of-sync files, so the `ls` shorthand was misleading. Closes [#17].
- `--prune`/`-p`, `--verbose`/`-v`, and `--dry-run`/`-n` are no longer global flags; they are now declared only on the commands that consume them (`--prune` on `save`/`install`; `--verbose` and `--dry-run` on `save`/`install`/`init`). `status` and `config` no longer accept these flags. As a side effect, `dotfiles -v` now prints the version.
- Consolidated near-identical filesystem stat helpers. Closes [#22].
- README defers per-command JSON shapes to `dotfiles <command> --help` instead of repeating one example, removing a second source of truth that could drift. Closes [#18].

[#14]: https://github.com/dreikanter/dotfiles-cli/issues/14
[#15]: https://github.com/dreikanter/dotfiles-cli/issues/15
[#17]: https://github.com/dreikanter/dotfiles-cli/issues/17
[#18]: https://github.com/dreikanter/dotfiles-cli/issues/18
[#22]: https://github.com/dreikanter/dotfiles-cli/issues/22
[#23]: https://github.com/dreikanter/dotfiles-cli/issues/23
[#46]: https://github.com/dreikanter/dotfiles-cli/issues/46
[#53]: https://github.com/dreikanter/dotfiles-cli/pull/53
[#54]: https://github.com/dreikanter/dotfiles-cli/pull/54
[#55]: https://github.com/dreikanter/dotfiles-cli/pull/55

## [0.1.2] - 2026-05-04

### Changed

- `save` and `apply` now list each file they actually change or delete, and stay quiet about files that already match. The summary line gains an `unchanged` count (and the `--json` envelope an `unchanged` field plus `"action": "unchanged"` per-file entries). Dry-run mode produces the same report without touching the filesystem. `--verbose` additionally prints unchanged files.
- `status` plain-text output now includes the tool name alongside each out-of-sync file.
- `config` plain-text output now lists only the tool and live path; the dotfile mirror path is no longer echoed (use `--json` for the full mapping).
- `status --json` state labels are now hyphenated (e.g. `in-sync`, `dotfile-missing`); plain-text output is unchanged.
- Error messages from `save`, `apply`, `init`, `status`, and `config` now name the failed operation and the affected path (e.g. `open /home/foo/.bashrc: permission denied`) instead of returning bare OS errors. Improves both plain-text and `--json` output (the `error.message` and per-file `actions[].message` fields).

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
