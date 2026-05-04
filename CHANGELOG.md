# Changelog

## [Unreleased]

### Added

- Initial Go implementation of the dotfiles CLI with `save`, `apply`, `status`, and `config` commands.
- Cobra-based command surface with `--dry-run`, `--verbose`, and `--prune` flags.
- JSON output mode for `status` (`--json`) and JSON dotfile-to-local mapping from `config`.
- Sample dotfiles in `testdata/sample-repo` for tests and demonstration.
