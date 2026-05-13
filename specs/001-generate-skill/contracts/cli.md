# Contract: `dotfiles skill` CLI surface

**Branch**: `001-generate-skill` | **Date**: 2026-05-13

The command's flag set, exit codes, and observable side effects.
This is the surface that test code asserts against and that downstream
agents drive. Implementation MUST conform exactly.

---

## Usage

```text
dotfiles skill [--install] [--agent=<name>] [--force] [-n|--dry-run] [--json]
```

Inherits the persistent flags from `rootCmd` (`--json`, `--root`, `--config`).
`--root` and `--config` are accepted but have no effect on this command —
the skill is generated from the in-memory command tree, not from the
manifest. They are documented in the global flags section of `--help`
for consistency with other commands.

## Flag reference

| Flag                 | Type    | Default  | Notes |
|----------------------|---------|----------|-------|
| `--install`          | bool    | `false`  | Switch from stdout mode to filesystem mode. |
| `--agent=<name>`     | string  | `""`     | In install mode, restrict to a single named agent. Without it, install mode auto-detects every supported agent installed on this machine. |
| `--force`            | bool    | `false`  | In install mode, permit overwriting a divergent existing file. |
| `-n, --dry-run`      | bool    | `false`  | In install mode, plan and report actions without writing. Ignored in stdout mode (already a no-op). |
| `--json` (inherited) | bool    | `false`  | Emit a single JSON object on stdout for both modes. Shape: see `contracts/json.md`. |

Flag interactions:

- `--agent` and `--force` are accepted only with `--install`. Passing
  them without `--install` is a usage error (exit non-zero, error
  envelope).
- `--dry-run` without `--install` is silently ignored (no-op in stdout
  mode); this matches how `save --dry-run` behaves when there is
  nothing to write.
- `--agent=<unknown>` is a usage error (exit non-zero, message lists
  the supported names).

## Exit codes

| Code | Condition |
|------|-----------|
| 0    | Stdout mode succeeded; or install mode succeeded with every action being `create`, `overwrite`, or `skip` and no errors. |
| ≠0   | Any usage error; any install action is `conflict`; any OS error encountered during planning or writing. |

## Observable side effects

**Stdout mode (default)**: writes a single markdown document or a single
JSON object to stdout. Reads no files. Writes no files. Exits 0 unless
rendering fails internally (which would be a programming bug, not a
user error).

**Install mode** (`--install` set):
- Reads zero or more existing skill files (one per resolved target) to
  decide between `create`/`overwrite`/`skip`/`conflict`.
- Writes zero or one file per resolved target — only for `create` and
  `overwrite` actions, and only when `--dry-run` is not set.
- Creates intermediate directories *only* when the agent's containing
  skill directory already exists (e.g. it will create
  `~/.claude/skills/dotfiles/` if `~/.claude/skills/` exists, but will
  NOT create `~/.claude/`).

## Help-output requirements (`dotfiles skill --help`)

Per Constitution Principle I (Agent-Friendly Interface), the help text MUST
include all of:

1. The flag list above.
2. The set of supported `--agent=<name>` values (the registry's
   `Name` fields). Currently: `claude`.
3. The JSON shapes documented in `contracts/json.md` (rendered as a
   constant in `internal/cli/skill.go` following the
   `initJSONShape` / `syncJSONShape` pattern in the existing commands).
4. An example invocation: `dotfiles skill | tee ~/some/path` for stdout
   mode and `dotfiles skill --install --agent=claude` for install mode.
