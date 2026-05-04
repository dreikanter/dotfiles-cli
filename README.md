# dotfiles-cli

An agent-friendly self-describing CLI to manage your dotfiles. Go port of
[dreikanter/dotfiles](https://github.com/dreikanter/dotfiles).

The CLI keeps a JSON manifest of paths to manage and copies files (no symlinks)
between your live environment and a checked-in mirror inside a git repository.

## Install

```sh
make install     # builds and installs to ~/go/bin/dotfiles
make build       # builds local ./dotfiles binary
```

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

| Command                  | Action                                               |
| ------------------------ | ---------------------------------------------------- |
| `dotfiles save`          | Copy local files into the dotfiles repository.       |
| `dotfiles apply`         | Copy repository files into the local environment.    |
| `dotfiles status`        | Print files that are out of sync.                    |
| `dotfiles status --json` | Same, machine-readable.                              |
| `dotfiles config`        | Print the resolved dotfile-to-local mapping as JSON. |

`apply` has alias `load`; `status` has alias `ls`.

### Flags

- `-n, --dry-run` — preview without writing
- `-v, --verbose` — log each file action
- `-p, --prune` — remove destination files no longer in the manifest
- `--root <path>` — repository root (default `$DOTFILES_ROOT` or `~/.dotfiles`)
- `--config <path>` — manifest path (default `$DOTFILES_CONFIG` or `<root>/dotfiles.json`)

## Example

```sh
$ dotfiles --root ./testdata/sample-repo apply
dotfiles -> local environment
Files copied: 5; errors: 0

$ dotfiles --root ./testdata/sample-repo status
5 files in sync
```

## Development

```sh
make test    # run tests with coverage
make lint    # run golangci-lint
```

See `CLAUDE.md` for project conventions and the release workflow.
