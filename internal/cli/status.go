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
	Use:   "status",
	Short: "Show the tracked files status",
	Long: `Show files whose live copy and saved repository copy disagree.

Plain text lists only out-of-sync entries. JSON includes every entry. Files
that exist but cannot be read (e.g. permission denied) surface as state
"error" with the underlying message in the "error" field.

JSON output shape:

  {
    "entries": [{"tool", "live", "saved", "state", "error"}, ...],
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
			if e.State == dotfiles.StateError {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", e.State.Display(), e.Tool, e.Live, e.Error)
				continue
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\n", e.State.Display(), e.Tool, e.Live)
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
