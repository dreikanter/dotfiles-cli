package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/dreikanter/dotfiles-cli/internal/dotfiles"
	"github.com/spf13/cobra"
)

type statusResponse struct {
	Entries []dotfiles.StatusEntry `json:"entries"`
	Summary statusSummary          `json:"summary"`
}

type statusSummary struct {
	Total    int `json:"total"`
	Unsynced int `json:"unsynced"`
}

const statusExamples = `  dotfiles status --tool git
  dotfiles status --tool git --file ~/.gitconfig`

var statusCmd = &cobra.Command{
	Use:     "status",
	Aliases: []string{"ls"},
	Short:   "Show out-of-sync files",
	Long: `Show files whose local copy and dotfile mirror disagree.

Plain text lists only out-of-sync entries. JSON includes every entry.

JSON output shape:

  {
    "entries": [{"tool", "local", "dotfile", "state"}, ...],
    "summary": {"total": N, "unsynced": N}
  }`,
	Example: statusExamples,
	RunE: func(cmd *cobra.Command, args []string) error {
		specs, err := loadSpecs(currentSelector())
		if err != nil {
			return handleErr(cmd, err)
		}
		entries, err := dotfiles.Status(specs)
		if err != nil {
			return handleErr(cmd, err)
		}
		unsynced := 0
		for _, e := range entries {
			if e.State != dotfiles.StateInSync {
				unsynced++
			}
		}
		out := cmd.OutOrStdout()
		if jsonOutput {
			return writeJSON(out, statusResponse{
				Entries: entries,
				Summary: statusSummary{Total: len(entries), Unsynced: unsynced},
			})
		}
		tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		for _, e := range entries {
			if e.State == dotfiles.StateInSync {
				continue
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\n", e.State.Display(), e.Tool, e.Local)
		}
		_ = tw.Flush()
		if unsynced == 0 {
			fmt.Fprintf(out, "%d files in sync\n", len(entries))
		} else {
			fmt.Fprintf(out, "%d unsynced, %d total\n", unsynced, len(entries))
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
	addFilterFlags(statusCmd)
}
