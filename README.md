# dotfiles CLI

An agent-friendly self-describing CLI to manage your dotfiles. Files are
copied (no symlinks) between your live environment and a checked-in mirror in
a git repository.

## Install

```sh
go install github.com/dreikanter/dotfiles-cli/cmd/dotfiles@latest
```

This installs the `dotfiles` binary into `$GOBIN` (default `~/go/bin`); make
sure that directory is on your `PATH`.

## Manifest

Place a `dotfiles.json` at the root of your dotfiles repository (default
`~/.dotfiles`):

```json
{
  "git":   ["~/.gitconfig", "~/.gitignore_global"],
  "shell": ["~/.zshrc"],
  "nvim":  ["~/.config/nvim/"]
}
```

A trailing `/` marks an entry as a directory (its contents are tracked
recursively). Files mirror to `<repo>/config/<tool>/<rel>`.

## Commands

| Command           | Action                                            |
| ----------------- | ------------------------------------------------- |
| `dotfiles init`   | Scaffold a fresh dotfiles repository.             |
| `dotfiles save`   | Copy local files into the dotfiles repository.    |
| `dotfiles apply`  | Copy repository files into the local environment. |
| `dotfiles status` | Print files that are out of sync.                 |
| `dotfiles config` | Print the resolved dotfile-to-local mapping.      |

`apply` has alias `load`; `status` has alias `ls`.

### Flags

- `-n, --dry-run` — preview without writing
- `-v, --verbose` — log each file action (ignored with `--json`)
- `-p, --prune` — remove destination files no longer in the manifest
- `--json` — emit a single JSON object on stdout instead of plain text
- `--root <path>` — repository root (default `$DOTFILES_ROOT` or `~/.dotfiles`)
- `--config <path>` — manifest path (default `$DOTFILES_CONFIG` or `<root>/dotfiles.json`)
- `--tool <name>` — restrict `save`/`apply`/`status`/`config` to a single manifest tool
- `--file <path>` — restrict to specific files within `--tool` (repeatable; mutually exclusive with `--prune`). Values are expanded the same way manifest entries are (`~`, CWD-relative) and matched literally against the manifest, so the path you pass must equal the manifest entry exactly (no globs, no symlink resolution, no sub-file matching inside directory entries).

```sh
# Save a single tool's files
dotfiles save --tool git

# Save specific files within a tool
dotfiles save --tool git --file ~/.gitconfig --file ~/.gitignore_global

# Preview the same scope
dotfiles status --tool git --file ~/.gitconfig
dotfiles config --tool git
```

## JSON output

Pass `--json` to any command to receive a single JSON object on stdout. JSON
output is never mixed with plain text and `--verbose` is ignored. The exit
code is still non-zero on failure; failures emit an error envelope:

```json
{ "error": { "message": "load manifest: ..." } }
```

Per-command shapes are documented in `dotfiles <command> --help`. Use `jq` to
extract specific fields:

```sh
dotfiles status --json | jq '.summary.unsynced'
dotfiles save --json   | jq '.actions[] | select(.action=="error")'
```

## Example

```sh
$ dotfiles apply
dotfiles -> local environment
Files copied: 5; errors: 0

$ dotfiles status
5 files in sync
```

## Development

```sh
make test    # run tests with coverage
make lint    # run golangci-lint
```

See `CLAUDE.md` for project conventions and the release workflow.
