# Implementation Plan: Generate Agent Skill

**Branch**: `001-generate-skill` | **Date**: 2026-05-13 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/001-generate-skill/spec.md`

## Summary

Add a `dotfiles skill` Cobra subcommand that, by default, prints a single
self-contained markdown document (with YAML frontmatter) describing how an
AI agent should drive the CLI. The document is rendered at runtime from
Cobra's existing command tree (`rootCmd.Commands()`, persistent flags, each
`cmd.Short`/`cmd.Long`), so new commands and flags appear automatically once
the binary is rebuilt. The same command, invoked with `--install`, writes
the rendered skill to one or more agent-specific filesystem locations,
honouring the project's standard safety affordances (`--dry-run`,
`--force`, JSON error envelope). The first release supports Claude Code
(`~/.claude/skills/dotfiles/SKILL.md`); additional agent targets are
add-only entries in a small in-process registry.

## Technical Context

**Language/Version**: Go 1.25.0 (per `go.mod`)

**Primary Dependencies**:
- `github.com/spf13/cobra v1.10.2` — already in use; required for both command
  registration and the introspection (`rootCmd.Commands()`, `cmd.Short`,
  `cmd.Flags().VisitAll`) that powers skill generation.
- `github.com/stretchr/testify v1.11.1` — already in use; sufficient for new
  tests.
- No new direct dependencies. YAML frontmatter is short and fixed-shape, so
  it is emitted by hand from a string template rather than via `yaml.v3`.

**Storage**: None at the data-store level. The command reads only
in-memory Cobra metadata. In install mode it writes exactly one file per
selected agent target under the user's home directory; no project files
are touched.

**Testing**: Existing `runCLI(t, args...)` in-process helper in
`internal/cli/cli_test.go`. New tests live in `internal/cli/skill_test.go`
and cover both modes (stdout/JSON + install), all install actions
(create/overwrite/skip/conflict), and `--dry-run`. Filesystem tests use
`t.TempDir()` and override the home-dir resolver to avoid touching the
real `~/.claude`.

**Target Platform**: Same as the existing CLI — macOS and Linux, anywhere
Go 1.25 builds. No platform-specific code paths needed.

**Project Type**: Single-project Go CLI tool (Option 1 in the template).

**Performance Goals**: Not a hot path. Generation runs once per
invocation, output is ~5–20 KB, so wall-clock <100 ms on cold start is
plenty.

**Constraints**:
- **Deterministic output** (SC-005, FR-005): two invocations on the same
  binary in the same environment MUST produce byte-identical skill content.
  Implies: stable iteration order over commands and flags, no
  timestamps in output, no random ordering.
- **No network access.** The skill is generated locally from the binary.
- **No writes outside the explicit install target.** Stdout mode is a pure
  function; install mode writes exactly one file per target.

**Scale/Scope**: One new top-level subcommand. Estimated footprint:
~1 new file `internal/cli/skill.go` (~250 LOC), ~1 new test file
`internal/cli/skill_test.go` (~250 LOC), plus a one-line `CHANGELOG.md`
entry and a short README section. No new packages.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Agent-Friendly Interface (NON-NEGOTIABLE) | PASS | `--json` is supported via the same package-level `jsonOutput` global the other commands use. Error path goes through the shared `handleErr` envelope. `--help` documents the JSON shape inline, like `init`/`save`/`status`/`config`. |
| II. Manifest as Source of Truth | PASS (N/A) | The skill command does not read or write the `dotfiles.json` manifest. It operates on Cobra's command tree (read-only) and on a single agent-specific file under `~/`. The manifest principle scopes what the CLI "manages"; the skill artifact is tool configuration, not a managed dotfile. Plan keeps the two domains disjoint. |
| III. Safe File Operations | PASS | Install mode supports `-n, --dry-run` with identical planning output. Overwrite requires explicit `--force`. An unchanged target is detected by byte comparison and reported as `skip` (no write, exit 0); a diverging target without `--force` is reported as `conflict` (no write, exit non-zero). |
| IV. Quality Gates Before Merge (NON-NEGOTIABLE) | PASS at plan level | New tests in `internal/cli/skill_test.go`; `make test` and `make lint` will gate the PR. No design choice here requires accepting lint or test regressions. |
| V. Release Discipline | PASS | New top-level command → MINOR version bump. CHANGELOG.md adds a single `[Unreleased]` bullet written for the tool's user (e.g. "Add `skill` command that prints an agent-installable skill document"). No version heading change. |

No violations. **Complexity Tracking** section omitted.

## Project Structure

### Documentation (this feature)

```text
specs/001-generate-skill/
├── plan.md              # This file (/speckit-plan output)
├── spec.md              # /speckit-specify output
├── research.md          # Phase 0 output (this command)
├── data-model.md        # Phase 1 output (this command)
├── quickstart.md        # Phase 1 output (this command)
├── contracts/
│   ├── cli.md           # Command-surface contract: flags, exit codes, action set
│   ├── json.md          # JSON output shapes for stdout and install modes
│   └── cobra.md         # Which Cobra public-API fields the generator reads
└── checklists/
    └── requirements.md  # /speckit-specify quality checklist (already exists)
```

### Source Code (repository root)

```text
cmd/
└── dotfiles/
    └── main.go                       # unchanged — entrypoint delegates to internal/cli

internal/
├── cli/
│   ├── root.go                       # add no global flags; skill command self-registers
│   ├── init.go save.go status.go config.go   # unchanged
│   ├── output.go                     # unchanged; reused for error envelope + writeJSON
│   ├── skill.go                      # NEW — command def, render(), install(), agent registry
│   ├── skill_test.go                 # NEW — tests for stdout/JSON/install/dry-run/force
│   ├── cli_test.go                   # unchanged; runCLI helper is reused
│   └── …                             # other existing files unchanged
└── dotfiles/                         # unchanged
```

**Structure Decision**: Single-project layout, matches the existing
`internal/cli/<command>.go` per-command convention. No new package
introduced. The skill renderer, agent registry, and Cobra handler all
live in `internal/cli/skill.go` because they share the same data
(Cobra's command tree) and would only create indirection if split prematurely.
If the agent list grows beyond ~5 entries or detection logic becomes
platform-specific, splitting `skill_agents.go` is a follow-up refactor and
not part of this feature's scope.
