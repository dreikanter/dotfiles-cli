# Research: Generate Agent Skill

**Branch**: `001-generate-skill` | **Date**: 2026-05-13

This document captures the design decisions made during Phase 0 of the
implementation plan. The Technical Context in `plan.md` had no
`NEEDS CLARIFICATION` items remaining, but a handful of judgment calls
needed to be locked down before contracts and data model could be
written. They are recorded here so reviewers can interrogate or revise
them without re-reading commit history.

---

## R1. How is YAML frontmatter emitted?

**Decision**: Hand-emit the frontmatter as a fixed-shape string. No
`gopkg.in/yaml.v3` dependency.

**Rationale**:
- The frontmatter is exactly two scalar fields (`name`, `description`)
  plus opening/closing `---` delimiters — four lines total.
- A YAML marshaller would add ~20 KB of indirect dependencies (the
  full `yaml.v3` graph) to render four lines deterministically.
- Hand-emission is easier to audit for the determinism constraint
  (SC-005) than configuring a marshaller's flow style and key order.

**Alternatives considered**:
- `gopkg.in/yaml.v3` — rejected (dependency cost vs benefit).
- Templating via `text/template` — rejected (template syntax overhead
  is larger than the literal string it would produce).

---

## R2. What is the structure of the skill body?

**Decision**: Fixed scaffold with five sections, rendered in this order:
1. **Overview** — one paragraph: what `dotfiles` does, its manifest model.
2. **Global flags** — table of the persistent flags from `rootCmd`
   (`--json`, `--root`, `--config`), each with its `--help` blurb.
3. **Commands** — table of every direct child of `rootCmd` (excluding the
   Cobra-injected `help` and `completion` synthetic commands), with
   `cmd.Use` and `cmd.Short`. Each row points the reader at
   `dotfiles <name> --help` for the full flag set and JSON shape.
4. **Manifest model** — short paragraph describing `dotfiles.json` and the
   directory marker (`trailing slash`), in line with Constitution
   Principle II.
5. **JSON output and errors** — explains the `--json` contract and the
   standard `{ "error": { "message": "..." } }` envelope, in line with
   Constitution Principle I.

**Rationale**:
- The body is intentionally short and stable. The skill is a *map* to the
  CLI, not a duplicate of it; per FR-004 it points at `--help` for the
  authoritative per-command detail.
- Five sections is enough to brief an agent without growing into a manual.

**Alternatives considered**:
- Embedding every command's full `Long` text inline — rejected. Doubles
  the skill size, drifts from `--help`, and re-asserts content the
  constitution says should live in `--help` only.
- Free-form rendering driven by a markdown template file — rejected. A
  template file alongside the binary creates a second source of truth
  and breaks deterministic, version-locked output.

---

## R3. Which commands are enumerated in the skill body?

**Decision**: Iterate `rootCmd.Commands()`, sort by `cmd.Name()`, and
include every command where:
- `cmd.IsAvailableCommand()` returns true, AND
- `cmd.Name()` is not `help` and not `completion` (the two synthetic
  Cobra builtins).

The `skill` command itself is included — it is just another command.

**Rationale**:
- `IsAvailableCommand()` is Cobra's own filter for "user-visible
  command", so we don't second-guess Cobra.
- Sorting alphabetically guarantees deterministic output across builds,
  satisfying the SC-005 determinism constraint.

**Alternatives considered**:
- Preserve registration order — rejected. Cobra makes no guarantee
  about init() execution order across files.
- Exclude `skill` itself from the skill body — rejected. The user
  installing the skill will eventually want to know how to re-install
  or refresh it; that's discoverable via `dotfiles skill --help`, which
  the body should point at like any other command.

---

## R4. How are agent install locations represented?

**Decision**: A small `[]agentTarget` slice declared in
`internal/cli/skill.go`. Each entry is a struct of `Name string` (the
public flag value, e.g. `"claude"`), `PathFor func() (string, error)`
(returns the absolute install path), and `Detect func() (bool, error)`
(returns true if the agent appears installed on this machine). Adding a
new agent is one append to this slice.

**Rationale**:
- The set is small and changes rarely. A slice in the same file as the
  command handler is the cheapest possible representation.
- A function-valued field decouples path resolution from `os.UserHomeDir`
  failure modes, which differ between detection (best-effort) and
  installation (mandatory).
- This keeps the constitution's "no premature abstraction" rule honoured —
  no interfaces, no plugin loader.

**Alternatives considered**:
- A `map[string]agentTarget` — rejected. Iteration order would be
  non-deterministic and we'd need to sort keys at every read anyway.
- A plugin or registry package — rejected as over-engineering for the
  current scope (one agent today, possibly two or three over the
  product's lifetime).

---

## R5. What does "the agent is detected" mean?

**Decision**: For Claude Code, the agent is considered "detected" when
the directory `~/.claude/skills/` exists on the machine. The presence of
the *containing* skills directory is the strongest local signal that the
user has interacted with Claude Code at all; we deliberately do not look
for binaries, processes, or version files.

**Rationale**:
- Cheap, side-effect-free, and uses only `os.Stat`.
- A user who has the directory has chosen to put skill content there;
  installing into that location is the user's natural intent.
- A user without the directory either does not use Claude Code or
  expects to install it manually; failing loudly is safer than
  pre-creating an unfamiliar directory tree (per FR-014 edge case).

**Alternatives considered**:
- Check `~/.claude/` only — rejected. That directory exists for many
  reasons (chat history, settings) and is a false positive for
  "wants a skill installed."
- Probe `$PATH` for a `claude` binary — rejected. The CLI tool name
  collides with other binaries; not a reliable detection signal.

---

## R6. How does install mode decide between Create / Overwrite / Skip / Conflict?

**Decision**: The four actions are computed as follows, per target:

| Existing file? | Bytes equal to generated skill? | `--force`? | Action      | Writes? | Exit |
|----------------|---------------------------------|------------|-------------|---------|------|
| No             | —                               | —          | `create`    | yes     | 0    |
| Yes            | Yes                             | —          | `skip`      | no      | 0    |
| Yes            | No                              | No         | `conflict`  | no      | ≠ 0  |
| Yes            | No                              | Yes        | `overwrite` | yes     | 0    |

`--dry-run` reports the action without writing. Across multiple targets
(auto-detect path), the exit code is non-zero if *any* target ends up
in `conflict`; the other targets' writes still happen, and each action
is reported individually.

**Rationale**:
- This is the minimal action set that satisfies SC-003 ("unchanged
  target is a no-op") while also satisfying FR-011 ("refuse to
  overwrite without `--force`").
- The per-target reporting + aggregate exit-code rule matches how
  `save --prune` already behaves for batched filesystem actions.

**Alternatives considered**:
- A single boolean "overwrite vs skip" with no `conflict` distinction —
  rejected. Users running without `--force` would silently get
  `skip` even when their existing file is stale, which violates SC-003.
- Always overwrite — rejected. Trivially violates Constitution
  Principle III (Safe File Operations).

---

## R7. What does the JSON shape look like?

**Decision**: Two shapes, one per mode. Documented in detail in
`contracts/json.md`.

- **stdout mode** (default): a single object with `name`, `description`,
  and `body` fields. The body is the rendered markdown *without* the
  YAML frontmatter (frontmatter is reconstructable from `name` and
  `description`, so we do not duplicate it).
- **install mode**: a single object with an `actions` array. Each action
  has `agent`, `path`, `action` (one of `create | overwrite | skip |
  conflict`), and an optional `error` (string) when filesystem
  operations fail mid-batch.

**Rationale**:
- Splitting frontmatter from body in stdout JSON lets a consumer
  re-render either part without parsing markdown.
- The install-mode `actions` array generalizes to one-agent and
  multi-agent invocations under the same shape, which simplifies
  downstream parsing.

**Alternatives considered**:
- Returning the full markdown including frontmatter in the JSON body —
  rejected. Forces consumers to parse YAML to get the metadata they
  often want most.
- Single object per agent rather than an actions array — rejected.
  Inconsistent shape between `--agent=foo` and auto-detect.

---

## R8. How is `--dry-run` implemented?

**Decision**: The install code path is split into two stages:

1. **Plan**: a pure function that, given the rendered skill bytes and
   the resolved target list, returns a slice of `installAction{Agent,
   Path, Action}` — no filesystem writes.
2. **Apply**: a function that walks the planned actions and performs
   the writes. `--dry-run` skips this stage entirely.

**Rationale**:
- Matches the existing `save` command's plan/apply split pattern in
  `internal/cli/save.go`.
- Guarantees Constitution Principle III's "dry-run produces the exact
  same plan as it would execute" because both modes share the planner.

**Alternatives considered**:
- A boolean threaded through the writer — rejected. Conflates planning
  and execution and makes the planner harder to test.

---

## R9. How are filesystem paths under the user's home resolved in tests?

**Decision**: Provide a package-level `homeDir func() (string, error)`
defaulting to `os.UserHomeDir`. Tests assign a stub that returns
`t.TempDir()`. Reset between tests via the existing `resetFlags()`
helper convention.

**Rationale**:
- Matches the pattern already used to seed `HOME` in
  `stageRepoAndHome()` in `cli_test.go`. Test infrastructure is reused
  rather than invented.

**Alternatives considered**:
- Pass an injected interface through every function — rejected as
  over-abstracted for a single resolver.
- Mutate `$HOME` via `t.Setenv` only — accepted as a *complement* (some
  tests will do both: stub `homeDir` for explicit cases, `t.Setenv` for
  end-to-end ones). Not exclusive.

---

## R10. What about the version-bump and CHANGELOG entry?

**Decision**:
- This is a new top-level command → **MINOR** bump per Constitution V.
- A single `Unreleased` bullet is added in the implementing PR:
  *"Add `skill` command that prints an agent-installable skill document,
  with optional `--install` to write it directly to Claude Code's skills
  directory."*
- The PR number is appended in a follow-up commit once GitHub assigns it,
  per the CLAUDE.md changelog rule.

**Rationale**:
- The constitution and CLAUDE.md already prescribe this; the decision is
  recorded here only to make it explicit that no version heading is
  added in this PR and that the entry is written for the tool's user.

**Alternatives considered**:
- None — this follows existing project rules.
