package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dreikanter/dotfiles-cli/internal/dotfiles"
	"github.com/spf13/cobra"
)

type syncFilter struct {
	Tool  string   `json:"tool,omitempty"`
	Files []string `json:"files,omitempty"`
}

type syncResponse struct {
	Direction string            `json:"direction"`
	DryRun    bool              `json:"dryRun"`
	Copied    int               `json:"copied"`
	Removed   int               `json:"removed"`
	Errors    int               `json:"errors"`
	Filter    *syncFilter       `json:"filter,omitempty"`
	Actions   []dotfiles.Action `json:"actions"`
}

const syncJSONShape = `JSON output shape:

  {
    "direction": "save"|"apply",
    "dryRun":    bool,
    "copied":    N,
    "removed":   N,
    "errors":    N,
    "filter":    {"tool": "git", "files": [...]} | omitted,
    "actions":   [{"action": "copy"|"prune"|"error", "from", "to", "path", "message"}, ...]
  }`

const syncExamples = `  dotfiles save --tool git
  dotfiles save --tool git --file ~/.gitconfig --file ~/.gitignore_global
  dotfiles apply --tool git --file ~/.gitconfig`

var saveCmd = &cobra.Command{
	Use:     "save",
	Short:   "Copy local environment files into the dotfiles repository",
	Long:    "Copy local environment files into the dotfiles repository.\n\n" + syncJSONShape,
	Example: syncExamples,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSync(cmd, dotfiles.DirSave, "save", "local environment -> dotfiles")
	},
}

var applyCmd = &cobra.Command{
	Use:     "apply",
	Aliases: []string{"load"},
	Short:   "Copy dotfiles repository files into the local environment",
	Long:    "Copy dotfiles repository files into the local environment.\n\n" + syncJSONShape,
	Example: syncExamples,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSync(cmd, dotfiles.DirApply, "apply", "dotfiles -> local environment")
	},
}

func init() {
	rootCmd.AddCommand(saveCmd)
	rootCmd.AddCommand(applyCmd)
	for _, c := range []*cobra.Command{saveCmd, applyCmd} {
		addFilterFlags(c)
		c.MarkFlagsMutuallyExclusive("file", "prune")
	}
}

func runSync(cmd *cobra.Command, dir dotfiles.Direction, name, header string) error {
	sel := currentSelector()
	specs, err := loadSpecs(sel)
	if err != nil {
		return handleErr(cmd, err)
	}
	out := cmd.OutOrStdout()
	syncOut := out
	if jsonOutput {
		syncOut = io.Discard
	} else {
		fmt.Fprintln(out, formatSyncHeader(header, sel, dryRun))
	}
	res, err := dotfiles.Sync(specs, dir, dotfiles.Options{
		DryRun:  dryRun,
		Verbose: verbose && !jsonOutput,
		Prune:   prune,
		Out:     syncOut,
	})
	if err != nil {
		return handleErr(cmd, err)
	}
	if jsonOutput {
		actions := res.Actions
		if actions == nil {
			actions = []dotfiles.Action{}
		}
		if writeErr := writeJSON(out, syncResponse{
			Direction: name,
			DryRun:    dryRun,
			Copied:    res.Copied,
			Removed:   res.Removed,
			Errors:    res.Errors,
			Filter:    buildFilter(sel),
			Actions:   actions,
		}); writeErr != nil {
			return writeErr
		}
		if res.Errors > 0 {
			return errSilent
		}
		return nil
	}
	fmt.Fprintf(out, "Files copied: %d; errors: %d", res.Copied, res.Errors)
	if prune {
		fmt.Fprintf(out, "; files removed: %d", res.Removed)
	}
	fmt.Fprintln(out)
	if res.Errors > 0 {
		return fmt.Errorf("%d errors", res.Errors)
	}
	return nil
}

// formatSyncHeader produces the plain-text header line, including any active
// filter and a [DRY RUN] suffix.
func formatSyncHeader(header string, sel dotfiles.Selector, dry bool) string {
	var b strings.Builder
	b.WriteString(header)
	if !sel.IsEmpty() {
		b.WriteString(" [tool=")
		b.WriteString(sel.Tool)
		if len(sel.Files) > 0 {
			fmt.Fprintf(&b, ", files=%d", len(sel.Files))
		}
		b.WriteString("]")
	}
	if dry {
		b.WriteString(" [DRY RUN]")
	}
	return b.String()
}

func buildFilter(sel dotfiles.Selector) *syncFilter {
	if sel.IsEmpty() {
		return nil
	}
	home, _ := os.UserHomeDir()
	return &syncFilter{Tool: sel.Tool, Files: sel.ResolvedFiles(home)}
}
