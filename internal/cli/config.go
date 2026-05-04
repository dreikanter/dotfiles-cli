package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/dreikanter/dotfiles-cli/internal/dotfiles"
	"github.com/spf13/cobra"
)

type configEntry struct {
	Tool    string `json:"tool"`
	Local   string `json:"local"`
	Dotfile string `json:"dotfile"`
}

type configResponse struct {
	Entries []configEntry `json:"entries"`
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Print the resolved dotfile-to-local mapping",
	Long: `Print the resolved mapping between local paths and dotfile mirror paths.

JSON output shape:

  {
    "entries": [{"tool", "local", "dotfile"}, ...]
  }`,
	RunE: func(cmd *cobra.Command, args []string) error {
		specs, err := loadSpecs()
		if err != nil {
			return handleErr(cmd, err)
		}
		entries, err := dotfiles.ExpandAllUnion(specs)
		if err != nil {
			return handleErr(cmd, err)
		}
		out := make([]configEntry, 0, len(entries))
		for _, e := range entries {
			out = append(out, configEntry{Tool: e.Tool, Local: e.Local, Dotfile: e.Dotfile})
		}
		w := cmd.OutOrStdout()
		if jsonOutput {
			return writeJSON(w, configResponse{Entries: out})
		}
		writeConfigTable(w, out)
		return nil
	},
}

func writeConfigTable(w io.Writer, entries []configEntry) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, e := range entries {
		fmt.Fprintf(tw, "%s\t%s\t->\t%s\n", e.Tool, e.Local, e.Dotfile)
	}
	_ = tw.Flush()
	fmt.Fprintf(w, "%d entries\n", len(entries))
}

func init() {
	rootCmd.AddCommand(configCmd)
}
