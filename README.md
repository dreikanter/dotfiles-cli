# dotfiles CLI

An agent-friendly self-describing CLI to manage your [dotfiles](https://dotfiles.github.io/).

## Install

```sh
go install github.com/dreikanter/dotfiles-cli/cmd/dotfiles@latest
```

This installs the `dotfiles` binary into `$GOBIN` (default `~/go/bin`); make
sure that directory is on your `PATH`.

## Setup

Run `dotfiles init` to scaffold a new dotfiles repository (default
`~/.dotfiles`). It creates the directory, writes an empty `dotfiles.json`
manifest and a starter `README.md`, then runs `git init`.

The repository is a checked-in mirror of the live config files on your
machine. The manifest (`dotfiles.json`) maps tool names to lists of paths and
tells the CLI which files to track:

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

- `dotfiles init` — scaffold a fresh dotfiles repository
- `dotfiles save` — copy local files into the dotfiles repository
- `dotfiles apply` — copy repository files into the local environment (alias: `load`)
- `dotfiles status` — print files that are out of sync (alias: `ls`)
- `dotfiles config` — print the resolved dotfile-to-local mapping

### Flags

These flags are accepted by every command:

- `-n, --dry-run` — preview without writing
- `-v, --verbose` — log each file action (ignored with `--json`)
- `--json` — emit a single JSON object on stdout instead of plain text (see [JSON output](#json-output))
- `--root <path>` — repository root (default `$DOTFILES_ROOT` or `~/.dotfiles`)
- `--config <path>` — manifest path (default `$DOTFILES_CONFIG` or `<root>/dotfiles.json`)

Command-specific flags (`--tool`, `--file`, `--prune`, `--force`) are shown in
the Usage examples below; run `dotfiles <command> --help` for the full list.

## Usage

```sh
# Apply the manifest to the local environment
dotfiles apply

# Save and remove destination files no longer in the manifest
dotfiles save --prune

# Save a single tool's files
dotfiles save --tool git

# Save specific files within a tool
dotfiles save --tool git --file ~/.gitconfig --file ~/.gitignore_global

# Preview the same scope
dotfiles status --tool git --file ~/.gitconfig
dotfiles config --tool git

# Re-scaffold an existing repository, overwriting dotfiles.json and README.md
dotfiles init --force

# Run against a custom repository (handy for demos and testing)
$ dotfiles --root ./testdata/sample-repo status
5 files in sync
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

## Development

```sh
make test    # run tests with coverage
make lint    # run golangci-lint
```

See `CLAUDE.md` for project conventions and the release workflow.
