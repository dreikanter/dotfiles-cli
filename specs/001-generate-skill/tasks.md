---
description: "Task list for Generate Agent Skill (issue #46)"
---

# Tasks: Generate Agent Skill

**Input**: Design documents from `specs/001-generate-skill/`

**Prerequisites**: plan.md, spec.md (both required); research.md, data-model.md,
contracts/, quickstart.md (all available and consulted below)

**Tests**: Tests are included per task. The project's constitution
(Principle IV) requires `make test` to pass before merge, and the existing
codebase has heavy in-process CLI coverage in `internal/cli/cli_test.go`;
new behavior MUST follow the same pattern in
`internal/cli/skill_test.go`. Tests are scheduled *after* their
implementation tasks within each story rather than strictly TDD-first.

**Organization**: Tasks are grouped by user story (US1 → US2 → US3) so
each story is a complete, independently shippable increment.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no incomplete dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)
- All paths are relative to the repository root

## Path Conventions

This is a single-project Go CLI layout (see `plan.md` → Project Structure).
All implementation lives under `internal/cli/`; tests live next to it as
`*_test.go`. No new packages are introduced.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Confirm the existing baseline is green before changes.

- [X] T001 Run `make test` and `make lint` from the repository root and confirm both pass on branch `001-generate-skill` before making any code changes (baseline guard against pre-existing failures masking new ones)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Stand up the bare Cobra wiring that every user story builds on.

**CRITICAL**: No user story work can begin until Phase 2 is complete.

- [X] T002 Add new file `internal/cli/skill.go` containing the stub `skillCmd` (`Use: "skill"`, placeholder `Short`/`Long`, `RunE` returning `nil`) and an `init()` that calls `rootCmd.AddCommand(skillCmd)` — register only; no behavior yet

**Checkpoint**: `dotfiles --help` now lists `skill`. User story phases can begin.

---

## Phase 3: User Story 1 — Print the skill to stdout (Priority: P1) — MVP

**Goal**: `dotfiles skill` prints a deterministic markdown-with-frontmatter
document covering every available command, the persistent flags, the
manifest model, and the JSON error envelope.

**Independent Test**: Run `dotfiles skill` on a fresh install. Verify the
output is a single self-contained markdown document beginning with a YAML
frontmatter block, that two invocations produce byte-identical output, and
that every available subcommand of `dotfiles` (sorted alphabetically) appears
in the Commands section.

### Implementation for User Story 1

- [ ] T003 [US1] Add the `Skill` struct (`Name`, `Description`, `Body`) plus a `Markdown()` string method to `internal/cli/skill.go` per `data-model.md`; constructor enforces invariants (name regex, description starts with `"Use when "`, body ends with `\n`)
- [ ] T004 [US1] Implement `renderSkill(root *cobra.Command) Skill` in `internal/cli/skill.go` per `research.md` R2/R3 and `contracts/cobra.md`: iterate `rootCmd.Commands()`, filter via `IsAvailableCommand()`, sort by `Name()`, build the five-section body (Overview, Global flags, Commands, Manifest model, JSON output and errors); iterate `rootCmd.PersistentFlags().VisitAll` for the global flags table
- [ ] T005 [US1] Define `skillJSONShape` constant in `internal/cli/skill.go` documenting the stdout-mode JSON shape from `contracts/json.md`, and concatenate it into the command's `Long` help text following the `initJSONShape` / `syncJSONShape` pattern used in `internal/cli/init.go` and `internal/cli/save.go`
- [ ] T006 [US1] Replace the stub `RunE` in `internal/cli/skill.go`: when the package-level `jsonOutput` is true, marshal `{name, description, body}` via the existing `writeJSON()` helper; otherwise print `skill.Markdown()` to stdout via `cmd.OutOrStdout()`; error paths use the existing `handleErr` helper (Constitution Principle I)
- [ ] T007 [P] [US1] Add `internal/cli/skill_test.go` using the existing `runCLI(t, args...)` helper from `cli_test.go`; cover: (a) `dotfiles skill` prints frontmatter `---\nname: dotfiles\n...---\n\n<body>\n`; (b) two consecutive invocations produce byte-identical output (determinism, SC-005); (c) `dotfiles skill --json` unmarshals to `{name, description, body}` with the expected fields; (d) every command in `rootCmd.Commands()` that `IsAvailableCommand()` returns true for appears in the body, alphabetized; (e) the synthetic `help`/`completion` commands do NOT appear; (f) every persistent flag (`--json`, `--root`, `--config`) appears in the global-flags section; (g) `dotfiles skill --help` includes the `skillJSONShape` text

**Checkpoint**: US1 is fully functional and independently shippable as the MVP.

---

## Phase 4: User Story 2 — Install into a known agent location (Priority: P2)

**Goal**: `dotfiles skill --install --agent=claude` writes the rendered
skill to `~/.claude/skills/dotfiles/SKILL.md` with proper create / skip /
conflict / overwrite semantics, honouring `--force` and `--dry-run`.

**Independent Test**: With `HOME` pointed at a sandbox temp directory in
which `.claude/skills/` exists, running `dotfiles skill --install
--agent=claude` produces a file at
`$HOME/.claude/skills/dotfiles/SKILL.md` whose bytes equal
`dotfiles skill` stdout. Re-running the same command without `--force`
is a no-op (`skip`). Mutating the file and re-running without `--force`
exits non-zero with action `conflict` and no write. Adding `--force`
overwrites and exits zero.

### Implementation for User Story 2

- [ ] T008 [US2] Add the `agentTarget` struct (`Name`, `PathFor`, `Detect`) and a package-level `var agents = []agentTarget{...}` slice in `internal/cli/skill.go` per `data-model.md`; include a single initial entry for `claude` with `PathFor` returning `<home>/.claude/skills/dotfiles/SKILL.md` and a `Detect` placeholder returning `false, nil` (real detection is wired in T015); also introduce the package-level resolver `var homeDir = os.UserHomeDir` (research R9) so tests can stub it
- [ ] T009 [US2] Add the install-mode flags to `skillCmd` in `internal/cli/skill.go`: `--install` (bool), `--agent` (string), `--force` (bool), `-n, --dry-run` (bool); validate via a pre-RunE check that `--agent` / `--force` without `--install` is a usage error, and that `--agent=<unknown>` is a usage error listing supported names (`contracts/cli.md` flag interactions)
- [ ] T010 [US2] Implement `planInstall(skill Skill, t agentTarget) installAction` in `internal/cli/skill.go` per `research.md` R6 and `data-model.md`: resolve `t.PathFor()`, attempt to read existing destination, decide `create` / `skip` / `conflict` / `overwrite` based on existence, byte-equality, and `--force`; populate `Error` on OS errors instead of an `Action` value
- [ ] T011 [US2] Implement `applyInstall(actions []installAction, dryRun bool) error` in `internal/cli/skill.go`: when `dryRun` is true, do nothing and return nil; otherwise iterate, write fresh content for `create` / `overwrite` only (creating intermediate directories only when the agent's containing skills dir already exists, per `contracts/cli.md`), capture per-target write errors into the action's `Error` field, and compute the aggregate exit status (non-zero if any action is `conflict` or has an `Error`)
- [ ] T012 [US2] Update `skillCmd.RunE` in `internal/cli/skill.go` to branch on `--install`: in install mode, build the target list from `--agent` (single entry) — auto-detect remains stubbed and is covered in US3 — call `planInstall` for each, call `applyInstall`, then either marshal the install-mode `{actions: [...]}` JSON via `writeJSON()` (when `jsonOutput`) or print a human-readable summary line per action; non-zero exit goes through `handleErr` (Constitution Principle I)
- [ ] T013 [US2] Extend `skillJSONShape` and the `Long` help text in `internal/cli/skill.go` to document the install-mode JSON shape from `contracts/json.md` and the supported `--agent=<name>` values, so `dotfiles skill --help` is the authoritative source per Constitution Principle I
- [ ] T014 [P] [US2] Extend `internal/cli/skill_test.go` with install-mode cases: install into a sandboxed `t.TempDir()` HOME (stub `homeDir` and seed `.claude/skills/` to satisfy detection later), assert (a) `create` on first run produces a file whose bytes equal `dotfiles skill` stdout; (b) re-running yields `skip` with no write and exit 0; (c) mutating the file then re-running without `--force` yields `conflict` with no write and a non-zero exit; (d) adding `--force` yields `overwrite` and exit 0; (e) `--dry-run` produces the same action plan but leaves the filesystem untouched; (f) `--agent=nopepants` is a usage error with the standard error envelope; (g) `--force` without `--install` is a usage error; (h) when the agent's parent skills directory is missing, the command exits non-zero with an actionable message; (i) install-mode `--json` unmarshals to `{actions: [{agent, path, action}]}` matching `contracts/json.md`

**Checkpoint**: US1 and US2 are both fully functional and independently
testable. The CLI now supports both stdout and explicit-agent install.

---

## Phase 5: User Story 3 — Auto-detect installed agents (Priority: P3)

**Goal**: `dotfiles skill --install` (no `--agent`) discovers which
supported agents are installed on this machine and installs the skill
into each of them; exits non-zero with an actionable error when none is
detected.

**Independent Test**: With `HOME` pointed at a sandbox where
`.claude/skills/` exists, `dotfiles skill --install` (no `--agent`)
installs into `~/.claude/skills/dotfiles/SKILL.md`. With `HOME` pointed
at a sandbox lacking `.claude/skills/`, the same command exits non-zero
with a message that lists supported agents and recommends passing
`--agent` explicitly.

### Implementation for User Story 3

- [ ] T015 [US3] Replace the placeholder `Detect` on the `claude` agent in `internal/cli/skill.go` with a real implementation that returns true when `<home>/.claude/skills/` exists (and false when missing); propagate any non-`IsNotExist` `os.Stat` error
- [ ] T016 [US3] Implement the auto-detect target-resolution branch in `internal/cli/skill.go`: when `--install` is set and `--agent` is empty, iterate the `agents` slice, call each `Detect()`, build the target list from those returning true; if the resulting list is empty, return a `handleErr`-wrapped usage error that names the supported agents and tells the user to pass `--agent` explicitly (per `contracts/cli.md`)
- [ ] T017 [P] [US3] Extend `internal/cli/skill_test.go` with auto-detect cases: (a) sandbox without `.claude/skills/` → no `--agent`, no `--install` value, command exits non-zero with the actionable error and supported-agents list; (b) sandbox with `.claude/skills/` → command auto-detects `claude`, installs, exit 0; (c) introduce a test-only helper that temporarily appends a fake second agent to the `agents` slice (and resets it via `t.Cleanup`) so the multi-target plan/apply path is exercised at least once (asserts both entries appear in `actions[]` of the JSON output)

**Checkpoint**: All three user stories are independently functional. The
feature is complete.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: User-facing documentation, final lint pass, end-to-end
manual smoke-test against `quickstart.md`.

- [ ] T018 [P] Add a single `[Unreleased]` bullet to `CHANGELOG.md` written for the tool's user per Constitution Principle V — high-level conceptual change, no implementation details, no file paths; reference the issue: `[#46]`. PR number is appended in a follow-up commit per CLAUDE.md changelog rules.
- [ ] T019 [P] Add a short `dotfiles skill` paragraph to `README.md` (`Commands` list and a Usage example), and include one `--json` example, consistent with the existing entries for `init`/`save`/`install`/`status`/`config`
- [ ] T020 Run `make lint` from the repository root and resolve any issues raised against the new code in `internal/cli/skill.go` and `internal/cli/skill_test.go` (Constitution Principle IV gate)
- [ ] T021 Run `make test` from the repository root and confirm full green across the new tests in `internal/cli/skill_test.go` and the existing suite in `internal/cli/cli_test.go` and `internal/cli/init_test.go` (Constitution Principle IV gate)
- [ ] T022 Walk through every command block in `specs/001-generate-skill/quickstart.md` against a freshly built `./dotfiles` binary using `HOME=$(mktemp -d)` sandboxes, confirming each documented expectation holds (build, stdout determinism, install, dry-run, conflict, force, agent selection, unknown-agent error)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; T001 is the baseline guard.
- **Foundational (Phase 2)**: Depends on Setup; T002 (stub registration) MUST land before any user story phase. Blocks all user stories.
- **User Story 1 (Phase 3, P1)**: Depends on Foundational. Independently shippable as MVP.
- **User Story 2 (Phase 4, P2)**: Depends on US1 — the install path reuses `renderSkill` (T004) and `Skill` (T003). Independently shippable on top of US1.
- **User Story 3 (Phase 5, P3)**: Depends on US2 — auto-detect reuses `planInstall` / `applyInstall` (T010 / T011) and the `agents` registry (T008). Independently shippable on top of US2.
- **Polish (Phase 6)**: Depends on every preceding phase being complete.

### Within Each User Story

Within US1 (T003 → T004 → T005 → T006 → T007):
- `Skill` struct (T003) before the renderer (T004) that constructs it.
- Renderer (T004) before `RunE` (T006) that calls it.
- `skillJSONShape` (T005) is independent of T003/T004 but referenced by T006's help text — keep it ahead of T006.
- Tests (T007) [P] is parallel-able with documentation/polish in other phases; depends on T003–T006 being complete to exercise real behavior.

Within US2 (T008 → T009 → T010 → T011 → T012 → T013 → T014):
- `agentTarget` registry (T008) before flag wiring (T009) and planner (T010).
- Flag validation (T009) before `RunE` branch (T012) so usage errors are caught up front.
- Planner (T010) before applier (T011); both before the new `RunE` branch (T012).
- JSON / help update (T013) is independent of T010–T012 but mentions the same shape — keep it after T012.
- Tests (T014) [P] depend on T008–T013 being complete.

Within US3 (T015 → T016 → T017):
- Real `Detect` (T015) before auto-detect resolution (T016).
- Tests (T017) [P] depend on T015–T016.

### Parallel Opportunities

- **Test tasks vs implementation tasks across phases**: Once US1 implementation (T003–T006) is in, T007 (US1 tests) is parallel-able with the start of US2 implementation (T008 etc.) if two contributors are on the branch. Within a single contributor's flow, follow the per-story sequence above.
- **Polish phase**: T018 (CHANGELOG.md) and T019 (README.md) touch different files and are independent — run in parallel.
- **Across the whole feature**: the implementation flow is largely sequential because most of the code lives in one file (`internal/cli/skill.go`). The natural parallelism is between code (`skill.go`) and tests (`skill_test.go`) and between docs (`CHANGELOG.md` / `README.md`).

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. T001 — baseline green.
2. T002 — stub command registered.
3. T003 → T007 — US1 complete: stdout mode lands.
4. **STOP and VALIDATE**: run T021 (`make test`) and T022 manually for the US1 portion of `quickstart.md`. The MVP is shippable here as a PR if you want to ship US1 standalone.

### Incremental Delivery

1. Land US1 as Plan A above. (Could ship as a standalone PR if scope is split; otherwise continue.)
2. Add US2 (T008 → T014). Re-run T021 / T022. The CLI now supports explicit-agent install.
3. Add US3 (T015 → T017). Re-run T021 / T022. The CLI now supports auto-detect.
4. Polish (T018 → T022). Open the PR.

### Single-PR Strategy (recommended)

Given the feature's small footprint (~500 LOC across two files), the
default expectation is a single PR covering US1 + US2 + US3 + polish,
with one atomic commit per task or per logical group, per Constitution
Principle IV (atomic commits, single-line messages). Each story
remains independently testable inside that PR via the checkpoint
verifications above.

---

## Notes

- Every task touches at most two files; same-file tasks are sequenced, not parallel.
- `[P]` is reserved for cross-file independence; within `internal/cli/skill.go` the order matters.
- Tests are scheduled after the implementation they cover so the running suite always represents reality; if a contributor prefers strict TDD ordering, they can reorder the test task ahead of its implementation tasks within a phase without changing the dependency graph.
- Every checkpoint exposes a story-level demo (run the binary and observe the documented behavior); commit at each checkpoint at minimum.
- No new external dependencies are introduced; do not add `gopkg.in/yaml.v3` or any other module for the YAML frontmatter — emit the four-line block by hand per research R1.
- Avoid: vague tasks, same-file [P] collisions, mixing CHANGELOG/README updates into code commits (Constitution IV: one logical change per commit).
