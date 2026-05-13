# Contract: Cobra introspection surface used by `skill`

**Branch**: `001-generate-skill` | **Date**: 2026-05-13

The skill renderer treats `github.com/spf13/cobra` as a *data source*.
This document pins down exactly which Cobra public-API fields and methods
the renderer reads, so a future Cobra upgrade can be reviewed against a
concrete list rather than a vague "uses Cobra somewhere" footprint.

If any of the items below disappears in a future Cobra release,
`internal/cli/skill.go` needs an explicit update.

---

## Read from `*cobra.Command` (the root command)

| API                                  | Use |
|--------------------------------------|-----|
| `rootCmd.Use`                        | Binary name token. Currently `"dotfiles"`. |
| `rootCmd.Version`                    | Version string mentioned in the body's Overview section. Defaults to `"dev"`; overridden at build time via `-ldflags`. |
| `rootCmd.Commands()`                 | The slice of registered subcommands. |
| `rootCmd.PersistentFlags()`          | The flag set whose `VisitAll` is iterated to render the Global Flags table. |

## Read from each enumerated `*cobra.Command` (child of root)

| API                       | Use |
|---------------------------|-----|
| `cmd.Name()`              | Label printed in the Commands table. |
| `cmd.IsAvailableCommand()`| Filter — only available commands are listed. Excludes `help`, `completion`, and any hidden ones. |
| `cmd.Short`               | One-line description in the Commands table. |

The renderer does NOT read `cmd.Long`. Per FR-004, the body points each
command's row at `dotfiles <name> --help` for the per-command flag list
and JSON shape, instead of duplicating that text.

## Read from `*pflag.FlagSet` (via `rootCmd.PersistentFlags()`)

| API              | Use |
|------------------|-----|
| `flags.VisitAll` | Iteration that emits the Global Flags table. |
| `flag.Name`      | Long flag name (e.g. `"json"`). |
| `flag.Shorthand` | Short flag, if any. Currently none of the persistent flags have shorthands. |
| `flag.Usage`     | The human-readable description shown in `--help`. Reused verbatim in the skill body. |

## Write back to Cobra

The renderer makes no calls that mutate the command tree. Cobra is a
pure read-only data source for this feature.

---

## Stability assumptions

- Cobra's `Commands()` returns commands in an unspecified order. The
  renderer MUST sort them alphabetically by `Name()` before rendering
  to preserve deterministic output (SC-005).
- Cobra's `IsAvailableCommand()` filters out `Hidden`, `help`, and
  `completion` synthetic commands as of v1.10.x. This filter is the
  source of truth — the renderer does NOT hard-code those names.
- `pflag.FlagSet.VisitAll` walks flags in sorted (lexicographic) order
  as of pflag v1.x. The renderer MUST NOT depend on registration order.
