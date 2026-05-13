# Feature Specification: Generate Agent Skill

**Feature Branch**: `001-generate-skill`

**Created**: 2026-05-13

**Status**: Draft

**Input**: GitHub issue #46 — "Add a command to generate agentic skill"
(https://github.com/dreikanter/dotfiles-cli/issues/46)

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Print the skill to stdout (Priority: P1) — MVP

As a user (human or agent) of an AI coding assistant, I want a single command
that emits a ready-to-use *skill* describing how to drive `dotfiles`, so that
my assistant can learn the CLI's surface, conventions, and JSON output shapes
from one authoritative document instead of guessing from `--help` output
piecemeal.

**Why this priority**: Standalone value. With only stdout output, a user can
pipe the result anywhere they want — into a file, a clipboard tool, an agent
configuration, or a chat message. Every later story builds on this content;
without it, none of them are possible.

**Independent Test**: Run `dotfiles skill` on a fresh install. Confirm the
output is a single self-contained markdown document, beginning with a YAML
frontmatter block (`name`, `description`), and that the body covers every
top-level subcommand of the CLI and the global flags. Nothing else is written
to disk.

**Acceptance Scenarios**:

1. **Given** a working `dotfiles` binary, **When** the user runs
   `dotfiles skill`, **Then** a single markdown document is printed to stdout
   and the process exits with status `0`.
2. **Given** the same binary, **When** the user runs `dotfiles skill --json`,
   **Then** a single JSON object containing the skill content is written to
   stdout (one field for frontmatter metadata, one for body text), with no
   plain text mixed in.
3. **Given** the same binary, **When** the user runs `dotfiles skill --help`,
   **Then** the help text documents the command, its flags, and the shape of
   the `--json` output.
4. **Given** any command added to `dotfiles` in the future, **When** the
   skill is regenerated for that build, **Then** the new command appears in
   the skill body without manual edits to the skill itself.

---

### User Story 2 — Install the skill into a known agent location (Priority: P2)

As a user who already runs an AI coding assistant locally, I want a single
command that drops the skill into the assistant's well-known skills directory,
so that I don't have to know where the directory lives, what filename to use,
or how to format the frontmatter.

**Why this priority**: This is the "happy path" most humans will reach for
once US1 exists. It removes the manual copy/paste step but is not required
for an agent that can already act on stdout output.

**Independent Test**: On a machine where the target agent's skills directory
exists, run `dotfiles skill --install --agent=<name>`. Confirm a single file
is created at the agent's documented skill location, its content matches
`dotfiles skill` exactly, and the command reports the destination path.

**Acceptance Scenarios**:

1. **Given** the target agent's skills directory exists and no skill file is
   present, **When** the user runs `dotfiles skill --install --agent=<name>`,
   **Then** the skill file is created at the agent-specific path and the
   command prints (or returns in JSON) that path.
2. **Given** a skill file already exists at the target path, **When** the
   user runs the same command without `--force`, **Then** no file is
   modified, the command exits non-zero, and the message identifies the
   conflict.
3. **Given** the same precondition, **When** the user adds `--force`,
   **Then** the existing file is overwritten with the freshly generated
   skill content and the action is reported.
4. **Given** any of the above, **When** the user adds `-n, --dry-run`,
   **Then** the command prints the action it *would* take (create / skip /
   overwrite) and writes nothing to disk.
5. **Given** the user passes `--agent` with a value the binary does not
   recognize, **Then** the command exits non-zero with an error naming the
   unknown agent and listing the supported values.

---

### User Story 3 — Auto-detect installed agents (Priority: P3)

As a user who has more than one AI assistant installed, I want
`dotfiles skill --install` (without `--agent`) to discover which assistants
are present on this machine and install the skill into each one, so that I
don't have to learn each assistant's install location individually.

**Why this priority**: A convenience that depends on US2 already working.
A single-agent user gets the same result by passing `--agent` explicitly.

**Independent Test**: On a machine where two supported agents are
installed, run `dotfiles skill --install`. Confirm the skill is created in
both agents' skill directories and the command reports both destination
paths. On a machine with no supported agents detected, confirm the command
exits non-zero with an actionable error.

**Acceptance Scenarios**:

1. **Given** at least one supported agent is detected on the machine,
   **When** the user runs `dotfiles skill --install`, **Then** the skill is
   installed into every detected agent's location and each destination path
   is reported.
2. **Given** no supported agent is detected, **When** the same command is
   run, **Then** the command exits non-zero with a message listing supported
   agents and how to pass `--agent` explicitly.
3. **Given** auto-detect mode is in use, **When** the skill already exists
   in some locations but not others, **Then** without `--force` the command
   installs into empty locations and reports the conflicts in the
   already-populated locations.
4. **Given** `-n, --dry-run`, **Then** the command lists the actions it
   would take across all detected agents without writing to disk.

---

### Edge Cases

- The skill content references the binary's version (see FR-004); a stale
  binary therefore prints a stale skill. Users opt in to refresh by re-running
  the command.
- The target agent directory does not exist (e.g. assistant configured but
  never run): the command exits non-zero with a message explaining the
  missing directory; it does not create unfamiliar parent directories on the
  user's behalf.
- The user's home directory is not writable (e.g. a read-only mount): the
  install mode surfaces the OS error inside the standard JSON error envelope
  and exits non-zero.
- The user pipes `dotfiles skill` into a file that is also the install target
  for some agent: the stdout mode is unaffected; the install mode never
  reads its target.
- The skill content grows past a length some agent platforms truncate (no
  specific cap known today): the content is emitted as-is; the spec does not
  attempt to manage agent-side truncation limits.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The CLI MUST provide a `dotfiles skill` command, exposed as a
  top-level subcommand alongside `init`, `save`, `install`, `status`, and
  `config`.
- **FR-002**: Invoked with no extra arguments, the command MUST write the
  skill content to stdout as a single markdown document and MUST NOT modify
  the filesystem.
- **FR-003**: The skill content MUST begin with a YAML frontmatter block
  containing at minimum a `name` field and a `description` field that
  follows the convention "Use when …" so AI agents can match it to a task.
- **FR-004**: The skill body MUST cover, at minimum: every top-level
  subcommand and its purpose; the global flags `--json`, `--root`,
  `--config`; the role of `dotfiles.json` as the manifest; the JSON error
  envelope shape; and a pointer to `dotfiles <command> --help` as the
  authoritative source for each command's full flag list and JSON shape.
- **FR-005**: The skill content MUST be derived from the binary's own
  command registry, not from a hand-maintained text file checked in
  alongside it, so new commands or flags appear automatically once the
  binary is rebuilt.
- **FR-006**: The command MUST support the global `--json` flag and emit a
  single JSON object whose shape is documented in `dotfiles skill --help`.
  Plain text and JSON MUST NOT be mixed in the same invocation.
- **FR-007**: On any error path, the command MUST emit the standard
  `{ "error": { "message": "..." } }` envelope when `--json` is set and exit
  with a non-zero status.
- **FR-008**: The command MUST accept an `--install` flag that, when
  present, switches the command from stdout mode to filesystem mode.
- **FR-009**: In install mode, the command MUST accept an optional
  `--agent=<name>` flag selecting the destination. Supported `<name>`
  values MUST be enumerated in `--help`.
- **FR-010**: In install mode without `--agent`, the command MUST attempt
  to auto-detect installed agents from a known list and install into every
  detected agent's location. If none is detected, the command MUST exit
  non-zero with an actionable error.
- **FR-011**: In install mode, the command MUST refuse to overwrite an
  existing destination file unless `--force` is passed (per the Safe File
  Operations principle of the constitution).
- **FR-012**: In install mode, the command MUST support `-n, --dry-run`,
  which prints the actions it would take and writes nothing.
- **FR-013**: In install mode (both single-agent and auto-detect), the
  command MUST report each destination path on success, both in human
  output and inside the `--json` payload.
- **FR-014**: The set of supported agents and their install locations MUST
  be discoverable from `dotfiles skill --help` so that users can verify
  support without reading the source.

### Key Entities

- **Skill**: a self-contained markdown document with YAML frontmatter
  describing how an AI agent should drive the `dotfiles` CLI. Includes a
  short `name`, a discoverability `description`, and a body covering every
  command, the global flags, the manifest model, and the JSON error
  envelope. There is exactly one skill per `dotfiles` binary.
- **Agent target**: a named AI assistant (initially Claude Code, with
  others addable later) that has a documented filesystem location for
  user-installed skills. Each target is identified by a stable short name
  (e.g. `claude`) and resolves to an absolute path under the user's home
  directory.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user who has never read the project documentation can
  install the skill into their preferred AI assistant in a single command
  and the assistant can then perform `save`, `install`, and `status`
  workflows correctly on the first attempt, without reading the README.
- **SC-002**: The skill output for any given binary contains an entry for
  100% of the commands listed by `dotfiles --help` and for 100% of the
  documented global flags; no command or flag is silently omitted.
- **SC-003**: Re-running `dotfiles skill --install` without `--force`
  against an unchanged target is a no-op (no file modification, non-zero
  exit on conflict, clear message). Re-running with `--force` produces a
  destination file that byte-for-byte equals the stdout output of
  `dotfiles skill` on the same binary.
- **SC-004**: An agent that consumes the skill can call any command listed
  in it and parse the corresponding `--json` output using only the
  guidance the skill embeds (i.e. the skill is self-contained relative to
  the binary it ships with).
- **SC-005**: Adding a new top-level command to the CLI causes that
  command to appear in the regenerated skill output within the same build,
  with zero edits to a separately maintained skill template.

## Assumptions

- **Initial agent support is Claude Code only.** The `--agent=claude`
  target ships in the first version, and auto-detect (US3) initially looks
  for that one agent. Additional agents are added in follow-up work and
  enumerated in `--help` as they land.
- **The Claude Code skill location is the published convention** —
  `~/.claude/skills/dotfiles/SKILL.md` — without any custom directory
  layout. If the conventional location changes upstream, this command
  follows it in a subsequent release.
- **The skill content is generated at runtime** from the same command
  registry that powers `--help`, so version drift between the binary and
  the skill content is impossible by construction.
- **A single flat `skill` subcommand** is used rather than separate
  `skill` and `install-skill` commands (the issue uses
  `notes install-skill` as an example, but the existing CLI is flat:
  `init`, `save`, `install`, `status`, `config`). Install behavior is
  reached via `--install` to keep the surface consistent.
- **The skill describes the CLI it ships in**, not a generic dotfiles
  workflow. Users on an older binary get an older skill; users on a newer
  binary get a newer skill. There is no central registry of skills to
  fetch from.
- **`--force`, `--dry-run`, `--json`, `--root`, and `--config`** already
  exist on other commands and retain their established semantics here.

## Dependencies

- The CLI's existing command registry (the same source `--help` uses) must
  expose the metadata needed to render the skill body — at minimum, each
  command's name, short description, and flag list. If this metadata is
  not already centralized, the implementation phase will need to expose
  it; that is a planning-phase concern, not a specification gap.
- The destination directory for the Claude Code agent
  (`~/.claude/skills/dotfiles/`) must be created on demand only when the
  user passes `--install`; the command MUST NOT create unfamiliar agent
  directories during stdout mode.
