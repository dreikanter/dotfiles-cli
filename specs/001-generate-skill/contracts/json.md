# Contract: `dotfiles skill` JSON output shapes

**Branch**: `001-generate-skill` | **Date**: 2026-05-13

Two shapes — one per mode. Documented at the field level so the help
text in `dotfiles skill --help` can mirror this contract verbatim
(per Constitution Principle I, `--help` is the authoritative source).

The standard error envelope `{ "error": { "message": "..." } }` applies
to any failure path and is documented in the global JSON section of the
README, not duplicated per command.

---

## Stdout mode (default, `dotfiles skill --json`)

```json
{
  "name": "dotfiles",
  "description": "Use when working with the user's dotfiles managed by the `dotfiles` CLI…",
  "body": "# dotfiles\n\nThe `dotfiles` CLI manages a checked-in mirror…\n"
}
```

| Field         | Type     | Notes |
|---------------|----------|-------|
| `name`        | string   | The skill name. Constant value `"dotfiles"`. Matches the frontmatter `name`. |
| `description` | string   | The discovery blurb; starts with `"Use when "`. Matches the frontmatter `description`. |
| `body`        | string   | The markdown body *without* the YAML frontmatter delimiters. Ends with exactly one trailing `\n`. UTF-8. |

To reconstruct what `dotfiles skill` (no `--json`) prints to stdout:

```text
---
name: <name>
description: <description>
---

<body>
```

---

## Install mode (`dotfiles skill --install --json`)

```json
{
  "actions": [
    {
      "agent": "claude",
      "path": "/Users/alice/.claude/skills/dotfiles/SKILL.md",
      "action": "create"
    },
    {
      "agent": "<another-agent>",
      "path": "/Users/alice/.../SKILL.md",
      "action": "conflict",
      "error": ""
    }
  ]
}
```

| Field             | Type     | Notes |
|-------------------|----------|-------|
| `actions`         | array    | One entry per resolved target. Order matches the agent registry's declaration order. Always present (possibly empty when no agents match `--agent` — though `--agent=<unknown>` is a usage error before this point). |
| `actions[].agent` | string   | The agent's `Name` from the registry. |
| `actions[].path`  | string   | Absolute filesystem path for this action. |
| `actions[].action`| string   | One of `"create"`, `"overwrite"`, `"skip"`, `"conflict"`. See `data-model.md` for semantics. |
| `actions[].error` | string   | Optional. Present and non-empty only when a per-target OS error prevented planning or writing this action. |

### `--dry-run` JSON

Identical shape to install mode. The set of `actions` and their
`action` values is exactly what a non-dry-run invocation would produce
in the same environment. Only the actual filesystem writes are
suppressed.

### Aggregate exit code in JSON mode

The JSON shape itself does not carry a top-level exit-code field; the
process exit code remains the canonical signal. Callers that need to
detect partial failure inspect `actions[].action` (looking for
`"conflict"`) and `actions[].error` (looking for non-empty).

---

## Error envelope (shared)

Any failure in either mode emits the standard envelope and exits non-zero:

```json
{ "error": { "message": "skill: unknown agent: foo (supported: claude)" } }
```

Examples of error sources for this command:
- Unknown `--agent` value.
- `--agent`/`--force` passed without `--install`.
- Auto-detect mode finds zero supported agents installed.
- The agent's parent skills directory does not exist (install mode).
- The user's home directory cannot be resolved.

These are usage or environment errors, not per-action OS errors. A
per-action OS error (e.g. permission denied while writing a particular
file) is reported in the `actions[].error` field of the install-mode
JSON, NOT in the global error envelope, so partial progress is still
visible.
