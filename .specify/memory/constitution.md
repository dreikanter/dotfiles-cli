<!--
SYNC IMPACT REPORT
==================
Version change: (template / unset) → 1.0.0
Rationale: First substantive ratification — replaces the unfilled template with
concrete principles derived from existing project conventions (README.md,
CLAUDE.md). MAJOR bump because this is the initial adoption of governing
principles; there is no prior numbered version to maintain compatibility with.

Modified principles:
  - [PRINCIPLE_1_NAME] → I. Agent-Friendly Interface (NON-NEGOTIABLE)
  - [PRINCIPLE_2_NAME] → II. Manifest as Source of Truth
  - [PRINCIPLE_3_NAME] → III. Safe File Operations
  - [PRINCIPLE_4_NAME] → IV. Quality Gates Before Merge (NON-NEGOTIABLE)
  - [PRINCIPLE_5_NAME] → V. Release Discipline

Added sections:
  - Development Workflow (replaces [SECTION_2_NAME])
  - Repository Conventions (replaces [SECTION_3_NAME])
  - Governance (filled)

Removed sections: none (template placeholders all replaced).

Templates requiring updates:
  - .specify/templates/plan-template.md ✅ aligned (Constitution Check section
    references this file; no changes needed — gates will be derived per plan).
  - .specify/templates/spec-template.md ✅ aligned (no constitution-specific
    references; user-story / requirements structure remains valid).
  - .specify/templates/tasks-template.md ✅ aligned (task categorization is
    compatible; tests remain explicitly optional per template's own guidance,
    matching this constitution's "tests where they earn their keep" stance).
  - .specify/templates/constitution-template.md ✅ unchanged (template stays
    as the seed for future fresh initializations; per command rules we operate
    on memory/constitution.md only).
  - .specify/templates/checklist-template.md ✅ not relevant to principles.
  - CLAUDE.md ✅ aligned (constitution formalizes rules already documented).
  - README.md ✅ aligned (commands, flags, and JSON-output behavior described
    in README match Principle I).

Follow-up TODOs: none.
-->

# dotfiles-cli Constitution

## Core Principles

### I. Agent-Friendly Interface (NON-NEGOTIABLE)

The CLI is designed to be driven by both humans and automated agents.

- Every command MUST support a `--json` flag that emits a single JSON object on
  stdout. JSON output MUST NOT be interleaved with plain-text output.
- When `--json` is set, `--verbose` MUST be ignored.
- Failures MUST emit a JSON error envelope of the form
  `{ "error": { "message": "..." } }` on stdout and MUST exit with a non-zero
  status.
- The per-command JSON shape is documented in `dotfiles <command> --help`,
  which is the single source of truth for that shape.
- Commands MUST be self-describing: `--help` MUST list every accepted flag,
  including the global flags (`--json`, `--root`, `--config`).

**Rationale**: This project's value proposition is being usable as a building
block by agents and scripts. Mixed-mode output, undocumented JSON shapes, or
silent failures break that contract.

### II. Manifest as Source of Truth

The `dotfiles.json` manifest defines what the CLI manages; nothing else does.

- Commands MUST operate only on paths reachable from the manifest. The CLI
  MUST NOT touch files outside the resolved live-to-saved mapping.
- The mapping from live paths to saved paths MUST be deterministic given the
  pair `(--root, --config)`. The same inputs MUST produce the same mapping.
- Manifest paths MAY reference individual files or directories; a trailing
  `/` marks a directory whose contents are tracked recursively. No other
  marker syntax (e.g. legacy `/*`) is supported.

**Rationale**: A predictable, manifest-driven scope is what makes the tool
safe to run unattended. Implicit file discovery would turn `save` and
`install` into unbounded operations.

### III. Safe File Operations

Commands that change the filesystem MUST be safe to run, predict, and undo.

- Any command that writes or deletes files MUST support `-n, --dry-run` and
  MUST produce the exact same plan in dry-run mode as it would execute.
- Destructive behavior MUST be opt-in via an explicit flag (`--prune`,
  `--force`). Default invocations MUST NOT delete or overwrite user data
  outside of the file currently being synced.
- `status` MUST distinguish, at minimum, between *in sync*, *out of sync*,
  *missing*, and *I/O error* states so callers can act on each.

**Rationale**: This CLI mutates the user's home directory and a tracked
repository. Surprises here corrupt config and erode trust; explicit opt-in
and dry-run previews are the cheapest insurance.

### IV. Quality Gates Before Merge (NON-NEGOTIABLE)

Every change passes the same automated gates before it lands on `main`.

- `make test` MUST pass on the branch before a PR is opened and before it is
  merged.
- `make lint` MUST pass on the branch before a PR is opened and before it is
  merged.
- Commits MUST be atomic: one logical change per commit.
- Commit messages MUST be a single short line with no body.
- PR bodies MUST follow `.github/pull_request_template.md`. An empty
  `References` section MUST be dropped rather than left as a placeholder.

**Rationale**: A green `main` and a readable history are what make small,
frequent releases possible. Skipping lint or test locally externalizes the
work to CI and to future readers of the log.

### V. Release Discipline

Releases follow a predictable, automated path driven by `CHANGELOG.md`.

- `CHANGELOG.md` is the source of truth for release notes. A
  `## [Unreleased]` section MUST be present at the top.
- PRs with user-visible changes MUST add an entry under `## [Unreleased]`,
  written for the tool's user — describing the conceptual change, not the
  implementation. Internal-only PRs MAY skip the changelog.
- Regular PRs MUST NOT bump the version number or add a new
  `## [X.Y.Z]` heading. Releases are cut by dedicated release PRs that
  convert `Unreleased` into `## [X.Y.Z] - YYYY-MM-DD` and add a fresh empty
  `Unreleased` section above it.
- The release tag `vX.Y.Z` is pushed by
  `.github/workflows/tag.yml` when the topmost numeric heading in
  `CHANGELOG.md` changes. The `Version` variable in
  `internal/cli/root.go` MUST remain `"dev"` and be overridden at build time
  via `-ldflags` from `git describe --tags`.
- Semantic Versioning rules:
  - **MAJOR**: breaking changes to CLI surface, manifest schema, JSON output
    shape, or the public Go API.
  - **MINOR**: new commands, new flags, new public APIs, or meaningful new
    behavior that is backward compatible.
  - **PATCH**: bug fixes, documentation updates, small behavior improvements,
    and internal changes worth releasing.
- Authorship or tooling attribution (e.g. "Generated by Claude Code",
  `Co-authored-by: Claude`) MUST NOT appear in commit messages, PR titles or
  descriptions, code comments, issue comments, or anywhere else in the
  repository.

**Rationale**: A changelog-driven release pipeline keeps version bumps and
user-facing notes from drifting apart, and removes guesswork about what
shipped in any given tag.

## Development Workflow

- Default toolchain: Go 1.25+ and `git`. Builds and installs go through
  `make` (`make build`, `make install`, `make update`, `make test`,
  `make lint`).
- Local install path is `~/go/bin/dotfiles`; `make update` rebuilds and
  re-installs after a merge.
- New CLI behavior SHOULD be exercised in `testdata/sample-repo` so that
  examples in `README.md` and integration tests stay self-contained.
- Changes that affect user-visible output MUST update both the human-readable
  format and the `--json` shape together; the two views MUST stay
  semantically equivalent.
- Spec Kit artifacts live under `specs/<feature>/` and `.specify/`. They are
  authored or updated by the corresponding `/speckit-*` commands and are
  governed by the rules in this constitution like any other change.

## Repository Conventions

- Source layout follows the standard Go project layout in use:
  - `cmd/dotfiles/` — `main` package, entrypoint only.
  - `internal/` — implementation packages; not part of any public API.
  - `testdata/` — fixtures used by tests and by README examples.
  - `docs/` — long-form documentation and assets (e.g. workflow diagram).
- Public Go API exposed under `cmd/dotfiles/...` is intentionally minimal;
  the CLI is the stable surface, not Go-level imports.
- Configuration discovery order for every command: explicit `--root` /
  `--config` flags, then `DOTFILES_ROOT` / `DOTFILES_CONFIG` environment
  variables, then defaults (`~/.dotfiles` and `<root>/dotfiles.json`). This
  order MUST NOT be changed without a MAJOR version bump.

## Governance

- This constitution supersedes ad-hoc conventions. Where a project document
  (README.md, CLAUDE.md, command help text) conflicts with this file, this
  file wins and the other document MUST be updated to match.
- Amendments are made by a PR that edits this file and, when the change
  modifies user-visible behavior, also adds a `CHANGELOG.md` entry.
- The PR description MUST state the new version, the bump type
  (MAJOR / MINOR / PATCH), and the reason. The reviewer MUST verify the
  Sync Impact Report at the top of this file is updated accordingly.
- Versioning of this constitution follows the same SemVer rules as the
  product (see Principle V), applied to governance content:
  - **MAJOR**: removal or backward-incompatible redefinition of a principle
    or governance rule.
  - **MINOR**: addition of a new principle or materially expanded guidance.
  - **PATCH**: clarifications, wording, typo fixes, non-semantic refinements.
- Compliance with this constitution is reviewed at PR time. Reviewers MUST
  block a PR that violates a NON-NEGOTIABLE principle unless the PR also
  amends the constitution to permit the change.
- Runtime development guidance for contributors lives in `CLAUDE.md` and
  `README.md`; both MUST stay consistent with the principles above.

**Version**: 1.0.0 | **Ratified**: 2026-05-13 | **Last Amended**: 2026-05-13
