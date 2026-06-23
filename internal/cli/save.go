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
	Unchanged int               `json:"unchanged"`
	Skipped   int               `json:"skipped"`
	Removed   int               `json:"removed"`
	Errors    int               `json:"errors"`
	Filter    *syncFilter       `json:"filter,omitempty"`
	Actions   []dotfiles.Action `json:"actions"`
}

const syncJSONShape = `JSON output shape:

  {
    "direction": "save"|"install",
    "dryRun":    bool,
    "copied":    N,
    "unchanged": N,
    "skipped":   N,
    "removed":   N,
    "errors":    N,
    "filter":    {"tool": "git", "files": [...]} | omitted,
    "actions":   [{"action": "copy"|"unchanged"|"skip"|"prune"|"error", "from", "to", "path", "message"}, ...]
  }`

const syncExamples = `  dotfiles save --tool git
  dotfiles save --tool git --file ~/.gitconfig --file ~/.gitignore_global
  dotfiles install --tool git --file ~/.gitconfig`

var saveCmd = &cobra.Command{
	Use:     "save",
	Short:   "Copy tracked files into the dotfiles repository",
	Long:    "Copy tracked files into the dotfiles repository.\n\n" + syncJSONShape,
	Example: syncExamples,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSync(cmd, dotfiles.DirSave, "save", "live -> saved")
	},
}

var installCmd = &cobra.Command{
	Use:     "install",
	Short:   "Copy tracked files to their live paths",
	Long:    "Copy tracked files to their live paths.\n\n" + syncJSONShape,
	Example: syncExamples,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSync(cmd, dotfiles.DirInstall, "install", "saved -> live")
	},
}

func init() {
	rootCmd.AddCommand(saveCmd)
	rootCmd.AddCommand(installCmd)
	for _, c := range []*cobra.Command{saveCmd, installCmd} {
		addFilterFlags(c)
		c.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "preview actions without writing")
		c.Flags().BoolVarP(&verbose, "verbose", "v", false, "log every file action (ignored when --json is set)")
		c.Flags().BoolVarP(&prune, "prune", "p", false, "remove destination files not in the manifest")
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
		filter, err := buildFilter(sel)
		if err != nil {
			return handleErr(cmd, err)
		}
		if writeErr := writeJSON(out, syncResponse{
			Direction: name,
			DryRun:    dryRun,
			Copied:    res.Copied,
			Unchanged: res.Unchanged,
			Skipped:   res.Skipped,
			Removed:   res.Removed,
			Errors:    res.Errors,
			Filter:    filter,
			Actions:   res.Actions,
		}); writeErr != nil {
			return writeErr
		}
		if res.Errors > 0 {
			return errSilent
		}
		return nil
	}
	fmt.Fprintf(out, "Copied: %d; unchanged: %d", res.Copied, res.Unchanged)
	if res.Skipped > 0 {
		fmt.Fprintf(out, "; skipped: %d", res.Skipped)
	}
	if prune {
		fmt.Fprintf(out, "; removed: %d", res.Removed)
	}
	fmt.Fprintf(out, "; errors: %d\n", res.Errors)
	if res.Errors > 0 {
		return fmt.Errorf("%s failed: %d errors", name, res.Errors)
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

func buildFilter(sel dotfiles.Selector) (*syncFilter, error) {
	if sel.IsEmpty() {
		return nil, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}
	return &syncFilter{Tool: sel.Tool, Files: sel.ResolvedFiles(home)}, nil
}
