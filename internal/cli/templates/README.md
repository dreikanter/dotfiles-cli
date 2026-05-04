# Dotfiles

Personal dotfiles managed with [dotfiles-cli](https://github.com/dreikanter/dotfiles-cli).

## Bootstrap on a new system

```sh
git clone <this-repo-url> ~/.dotfiles
go install github.com/dreikanter/dotfiles-cli/cmd/dotfiles@latest
dotfiles install
```

## Save local changes

```sh
dotfiles save
git -C ~/.dotfiles add -A
git -C ~/.dotfiles commit -m "update"
git -C ~/.dotfiles push
```

## Update from remote

```sh
git -C ~/.dotfiles pull
dotfiles install
```

## Manifest format

`dotfiles.json` maps tool names to lists of paths. A leading `~` expands to
your home directory; a trailing `/` marks a directory tracked recursively.

```json
{
  "git":   ["~/.gitconfig", "~/.gitignore_global"],
  "shell": ["~/.zshrc"],
  "nvim":  ["~/.config/nvim/"]
}
```
