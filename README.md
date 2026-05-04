# dotfiles CLI

An agent-friendly self-describing CLI to manage your [dotfiles](https://dotfiles.github.io/).

## Install

Prerequisites: Go (1.25+) and `git`.

```sh
go install github.com/dreikanter/dotfiles-cli/cmd/dotfiles@latest
```

This installs the `dotfiles` binary into `$GOBIN` (default `~/go/bin`); make
sure that directory is on your `PATH`.

## Setup

Run `dotfiles init` to scaffold a new dotfiles repository (default
`~/.dotfiles`):

```sh
dotfiles init
```

It creates the directory, writes an empty `dotfiles.json` manifest and a
starter `README.md`, then runs `git init`.

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
- `dotfiles save` — copy installed files into the dotfiles repository
- `dotfiles install` — copy repository files into their installed locations
- `dotfiles status` — print files that are out of sync (alias: `ls`)
- `dotfiles config` — print the resolved installed-to-saved mapping

### Flags

These flags are accepted by every command:

- `--json` — emit a single JSON object on stdout instead of plain text (see [JSON output](#json-output))
- `--root <path>` — repository root (default `$DOTFILES_ROOT` or `~/.dotfiles`)
- `--config <path>` — manifest path (default `$DOTFILES_CONFIG` or `<root>/dotfiles.json`)

Command-specific flags (`-n, --dry-run`, `-v, --verbose`, `-p, --prune`,
`--tool`, `--file`, `--force`) are shown in the Usage examples below; run
`dotfiles <command> --help` for the full list.

## Usage

```sh
# Scaffold a fresh dotfiles repository
dotfiles init

# Inspect what init generated
tree ~/.dotfiles
# ~/.dotfiles
# ├── README.md
# └── dotfiles.json

# Re-scaffold an existing repository, overwriting dotfiles.json and README.md
dotfiles init --force

# Install the manifest into the live environment
dotfiles install

# Save a single tool's files
dotfiles save --tool git

# Save specific files within a tool
dotfiles save --tool git --file ~/.gitconfig --file ~/.gitignore_global

# Save and remove destination files no longer in the manifest
dotfiles save --prune

# Report which managed files are out of sync
dotfiles status

# Print the resolved installed-to-saved mapping
dotfiles config
# Root: /home/user/.dotfiles
# Config: /home/user/.dotfiles/dotfiles.json
#
# git    /home/user/.gitconfig
# git    /home/user/.gitignore_global
# nvim   /home/user/.config/nvim/init.lua
# nvim   /home/user/.config/nvim/lua/plugins.lua
# shell  /home/user/.zshrc
# 5 entries

# Scope status and config to a single tool or files
dotfiles status --tool git --file ~/.gitconfig
dotfiles config --tool git

# Run against a custom repository (handy for demos and testing)
dotfiles --root ./testdata/sample-repo status
# 5 files in sync
```

## JSON output

Pass `--json` to any command to receive a single JSON object on stdout. JSON
output is never mixed with plain text. For example, `dotfiles status --json`
prints a structured report:

```json
{
  "entries": [
    {
      "tool": "git",
      "installed": "/home/user/.gitconfig",
      "saved": "/home/user/.dotfiles/config/git/.gitconfig",
      "state": "in-sync"
    }
  ],
  "summary": { "total": 1, "unsynced": 0 }
}
```

The `--verbose` flag is ignored when `--json` is set.

The exit code is still non-zero on failure; failures emit an error envelope:

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
