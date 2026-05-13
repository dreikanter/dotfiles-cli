# Data Model: Generate Agent Skill

**Branch**: `001-generate-skill` | **Date**: 2026-05-13

This feature has no persistent data store. The entities below are
in-memory Go types used by the renderer and install planner inside
`internal/cli/skill.go`. Field names are normative; they appear in
the JSON output contract (`contracts/json.md`).

---

## Skill

The complete rendered skill document for the current binary.

**Fields**:

| Field         | Type     | Notes |
|---------------|----------|-------|
| `Name`        | `string` | Frontmatter `name`. Constant: `"dotfiles"`. Lowercased, no spaces — matches the Claude Code skill-naming convention. |
| `Description` | `string` | Frontmatter `description`. Begins with `"Use when "` per FR-003; mentions the manifest model and the `--json` contract. |
| `Body`        | `string` | The full markdown body *without* the leading frontmatter delimiters, ending in a single trailing newline. Deterministic for a given binary build. |

**Derived properties**:

- `Markdown() string` returns `"---\nname: " + Name + "\ndescription: " + Description + "\n---\n\n" + Body`. This is what stdout mode prints and what install mode writes to disk.
- `Bytes() []byte` is `[]byte(Markdown())`.

**Validation rules**:

- `Name` MUST be non-empty and contain only `[a-z0-9-]`.
- `Description` MUST be non-empty and start with `"Use when "`.
- `Body` MUST end with exactly one trailing `\n`.

These rules are enforced by the constructor at render time; a malformed
`Skill` is a programming error, not a user error, so a panic on
violation is acceptable (matches the project's existing style for
internal invariants).

---

## AgentTarget

A supported AI agent and the rules for installing the skill into it.

**Fields**:

| Field      | Type                          | Notes |
|------------|-------------------------------|-------|
| `Name`     | `string`                      | Stable short name accepted by `--agent=<name>`. Initial set: `"claude"`. MUST be unique within the registry. |
| `PathFor`  | `func() (string, error)`      | Resolves the absolute install path. For Claude Code: `<home>/.claude/skills/dotfiles/SKILL.md`. Returns the error from the home-dir resolver; the caller wraps it for user-facing messages. |
| `Detect`   | `func() (bool, error)`        | Returns `true` if the agent appears installed (its parent skills directory exists). Returns `(false, nil)` when the directory is simply missing, but propagates any other `os.Stat` error. |

**Registry**: a package-level `var agents = []AgentTarget{ ... }` slice
in `internal/cli/skill.go`. The slice is iterated in declaration order
when auto-detecting and looked up by `Name` when `--agent` is set.

**Initial registry contents**:

```go
var agents = []AgentTarget{
    {
        Name:    "claude",
        PathFor: func() (string, error) { return joinHome(".claude/skills/dotfiles/SKILL.md") },
        Detect:  func() (bool, error) { return dirExists(joinHome(".claude/skills")) },
    },
}
```

(Signatures abbreviated; the real version threads the `homeDir` resolver
from R9 through.)

**Validation rules**:

- `Name` matches `[a-z][a-z0-9-]*`.
- `PathFor` and `Detect` are non-nil.

---

## InstallAction

One planned filesystem action for one agent target.

**Fields**:

| Field    | Type            | Notes |
|----------|-----------------|-------|
| `Agent`  | `string`        | The `AgentTarget.Name` this action applies to. |
| `Path`   | `string`        | Absolute path the action would write to. |
| `Action` | `ActionKind`    | One of the four enum values below. |
| `Error`  | `string` (opt.) | Populated only when the action's pre-checks (read existing file, detect agent) failed; empty otherwise. |

### ActionKind (enum)

| Value       | Meaning                                                                                       | Writes? | Per-target exit contribution |
|-------------|-----------------------------------------------------------------------------------------------|---------|-------------------------------|
| `create`    | No file exists at `Path`; planner will write fresh content there.                             | yes     | 0                             |
| `overwrite` | File exists with different content; user passed `--force`; planner will replace it.           | yes     | 0                             |
| `skip`      | File exists with byte-identical content; nothing to do (SC-003).                              | no      | 0                             |
| `conflict`  | File exists with different content; user did NOT pass `--force`; planner refuses (FR-011).    | no      | ≠ 0                           |

`Error` is set instead of (not in addition to) an `Action` value when
the planner cannot decide because of an OS error (e.g. permission
denied when reading the existing file). In that case the per-target
exit contribution is also non-zero. JSON callers can tell the two
failure modes apart by the presence of the `error` field.

---

## InstallPlan

The full set of `InstallAction`s for a single CLI invocation.

**Fields**:

| Field     | Type              | Notes |
|-----------|-------------------|-------|
| `Actions` | `[]InstallAction` | One entry per target the planner considered. Stable order: registry-declaration order. |

**Aggregate exit code**:
- `0` if every action is `create`, `overwrite`, or `skip` and has no `Error`.
- Non-zero if any action is `conflict` or has a populated `Error`.

The planner returns the plan and the aggregate exit code; the executor
performs the writes for `create` / `overwrite` actions only (and only
when `--dry-run` is not set).

---

## Cobra metadata (read-only, sourced from `*cobra.Command`)

The skill renderer reads these fields from each enumerated command and
the root command's persistent flag set. They are *not* owned by this
feature; they are listed here so reviewers can see exactly which Cobra
APIs the rendering depends on (also captured in `contracts/cobra.md`):

| Source                            | Used as                                                  |
|-----------------------------------|----------------------------------------------------------|
| `rootCmd.Use`                     | Binary name in the rendered body.                        |
| `rootCmd.Version`                 | Version mentioned in the Overview section.               |
| `rootCmd.PersistentFlags().VisitAll` | Global flags table (`--json`, `--root`, `--config`).  |
| `rootCmd.Commands()`              | List of commands to enumerate.                           |
| `cmd.Name()`                      | Command label in the body and the `Skill.Name` namespace.|
| `cmd.IsAvailableCommand()`        | Filter — skip hidden/synthetic commands.                 |
| `cmd.Short`                       | One-line description in the Commands table.              |
