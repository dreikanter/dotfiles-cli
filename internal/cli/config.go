package cli

import (
	"fmt"
	"io"
	"strings"
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
	Root    string        `json:"root"`
	Entries []configEntry `json:"entries"`
}

const configExamples = `  dotfiles config --tool git
  dotfiles config --tool git --file ~/.gitconfig`

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Print the resolved dotfile-to-local mapping",
	Long: `Print the resolved mapping between local paths and dotfile mirror paths.

JSON output shape:

  {
    "root": "<dotfiles repository root>",
    "entries": [{"tool", "local", "dotfile"}, ...]
  }`,
	Example: configExamples,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := resolveRoot()
		if err != nil {
			return handleErr(cmd, err)
		}
		specs, err := loadSpecs(currentSelector())
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
			return writeJSON(w, configResponse{Root: root, Entries: out})
		}
		writeConfigTable(w, root, out)
		return nil
	},
}

func writeConfigTable(w io.Writer, root string, entries []configEntry) {
	fmt.Fprintf(w, "Root: %s\n", root)
	fmt.Fprintln(w, strings.Repeat("-", 40))
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, e := range entries {
		fmt.Fprintf(tw, "%s\t%s\n", e.Tool, e.Local)
	}
	_ = tw.Flush()
	fmt.Fprintf(w, "%d entries\n", len(entries))
}

func init() {
	rootCmd.AddCommand(configCmd)
	addFilterFlags(configCmd)
}
