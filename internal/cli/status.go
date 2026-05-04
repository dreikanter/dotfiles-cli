package cli

import (
	"encoding/json"
	"fmt"

	"github.com/dreikanter/dotfiles-cli/internal/dotfiles"
	"github.com/spf13/cobra"
)

var statusJSON bool

var statusCmd = &cobra.Command{
	Use:     "status",
	Aliases: []string{"ls"},
	Short:   "Show out-of-sync files",
	RunE: func(cmd *cobra.Command, args []string) error {
		specs, err := loadSpecs()
		if err != nil {
			return err
		}
		entries, err := dotfiles.Status(specs)
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		if statusJSON {
			b, err := json.MarshalIndent(entries, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(out, string(b))
			return nil
		}
		var unsynced int
		for _, e := range entries {
			if e.State == dotfiles.StateInSync {
				continue
			}
			unsynced++
			fmt.Fprintf(out, "%-20s %s\n", e.State, e.Local)
		}
		if unsynced == 0 {
			fmt.Fprintf(out, "%d files in sync\n", len(entries))
		} else {
			fmt.Fprintf(out, "%d unsynced, %d total\n", unsynced, len(entries))
		}
		return nil
	},
}

func init() {
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "emit machine-readable JSON")
	rootCmd.AddCommand(statusCmd)
}
