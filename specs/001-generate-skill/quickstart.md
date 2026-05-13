# Quickstart: Generate Agent Skill

**Branch**: `001-generate-skill` | **Date**: 2026-05-13

Local instructions for building, testing, and exercising the new
`dotfiles skill` command. Targets a contributor who has just checked
out this branch.

---

## Build

```sh
make build         # produces ./dotfiles bound to the current git tag
./dotfiles --help  # confirm the new "skill" subcommand is listed
```

## Smoke-test stdout mode

```sh
./dotfiles skill | head -40                # human/plain output
./dotfiles skill --json | jq .              # JSON output
./dotfiles skill --help                     # documents flags and JSON shape
diff <(./dotfiles skill) <(./dotfiles skill)  # MUST be empty (determinism)
```

## Smoke-test install mode against a sandbox HOME

Avoid touching the real `~/.claude`. Use a tempdir as HOME:

```sh
SANDBOX=$(mktemp -d)
mkdir -p "$SANDBOX/.claude/skills"  # signal Claude Code is "installed"

# Auto-detect mode
HOME="$SANDBOX" ./dotfiles skill --install --json | jq .
# Expect: actions[0].action == "create", path under $SANDBOX/.claude/skills/dotfiles/SKILL.md

# Re-run: should be a no-op (byte-identical)
HOME="$SANDBOX" ./dotfiles skill --install --json | jq '.actions[0].action'
# Expect: "skip"

# Mutate the file and re-run without --force: conflict
echo CHANGED >> "$SANDBOX/.claude/skills/dotfiles/SKILL.md"
HOME="$SANDBOX" ./dotfiles skill --install --json | jq '.actions[0].action'
# Expect: "conflict", process exit non-zero

# Re-run with --force: overwrites
HOME="$SANDBOX" ./dotfiles skill --install --force --json | jq '.actions[0].action'
# Expect: "overwrite", exit 0

# Dry-run never writes
rm "$SANDBOX/.claude/skills/dotfiles/SKILL.md"
HOME="$SANDBOX" ./dotfiles skill --install --dry-run --json | jq '.actions[0].action'
# Expect: "create"
test ! -e "$SANDBOX/.claude/skills/dotfiles/SKILL.md"   # nothing written
```

## Smoke-test agent selection and error paths

```sh
HOME="$SANDBOX" ./dotfiles skill --install --agent=claude    # explicit OK
HOME="$SANDBOX" ./dotfiles skill --install --agent=nopepants  # error envelope
./dotfiles skill --force                                       # usage error (no --install)
HOME=$(mktemp -d) ./dotfiles skill --install                   # no agents detected → error
```

## Run the test suite

```sh
make test    # in-process via the existing runCLI helper
make lint    # golangci-lint per Constitution IV
```

Both MUST pass before pushing or opening the PR.

## Inspect what the skill says

```sh
./dotfiles skill | bat -l markdown   # or `less`, or your editor of choice
```

The body should contain:

- An Overview paragraph naming the binary.
- A Global Flags table with `--json`, `--root`, `--config`.
- A Commands table with `config`, `init`, `install`, `save`, `skill`,
  `status` — one row per available command, alphabetized.
- A Manifest section pointing at `dotfiles.json`.
- A JSON output section pointing at `dotfiles <command> --help` and at
  the `{ "error": { "message": "..." } }` envelope.

## After-merge follow-ups (not part of this PR)

- Add an entry under `[Unreleased]` in `CHANGELOG.md` for the new command.
  Backfill the PR number in a follow-up commit once GitHub assigns it.
- Add a short paragraph in `README.md`'s Commands list describing
  `dotfiles skill`.
- (Future) Add the next agent target by appending a single entry to the
  `agents` slice in `internal/cli/skill.go` — no other code changes
  needed.
