package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/dreikanter/dotfiles-cli/internal/dotfiles"
	"github.com/spf13/cobra"
)

type configEntry struct {
	Tool  string `json:"tool"`
	Live  string `json:"live"`
	Saved string `json:"saved"`
}

type configResponse struct {
	Root    string        `json:"root"`
	Config  string        `json:"config"`
	Entries []configEntry `json:"entries"`
}

const configExamples = `  dotfiles config --tool git
  dotfiles config --tool git --file ~/.gitconfig`

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Print the resolved live-to-saved mapping",
	Long: `Print the resolved mapping between live paths and saved repository paths.

JSON output shape:

  {
    "root": "<dotfiles repository root>",
    "config": "<manifest file path>",
    "entries": [{"tool", "live", "saved"}, ...]
  }`,
	Example: configExamples,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := resolveRoot()
		if err != nil {
			return handleErr(cmd, err)
		}
		manifest, err := resolveManifest(root)
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
			out = append(out, configEntry{Tool: e.Tool, Live: e.Live, Saved: e.Saved})
		}
		w := cmd.OutOrStdout()
		if jsonOutput {
			return writeJSON(w, configResponse{Root: root, Config: manifest, Entries: out})
		}
		writeConfigTable(w, root, manifest, out)
		return nil
	},
}

func writeConfigTable(w io.Writer, root, manifest string, entries []configEntry) {
	fmt.Fprintf(w, "Root: %s\n", root)
	fmt.Fprintf(w, "Config: %s\n", manifest)
	fmt.Fprintln(w)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, e := range entries {
		fmt.Fprintf(tw, "%s\t%s\n", e.Tool, e.Live)
	}
	_ = tw.Flush()
	fmt.Fprintf(w, "%d entries\n", len(entries))
}

func init() {
	rootCmd.AddCommand(configCmd)
	addFilterFlags(configCmd)
}
