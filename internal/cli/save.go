package cli

import (
	"fmt"
	"io"

	"github.com/dreikanter/dotfiles-cli/internal/dotfiles"
	"github.com/spf13/cobra"
)

type syncResponse struct {
	Direction string            `json:"direction"`
	DryRun    bool              `json:"dryRun"`
	Copied    int               `json:"copied"`
	Removed   int               `json:"removed"`
	Errors    int               `json:"errors"`
	Actions   []dotfiles.Action `json:"actions"`
}

const syncJSONShape = `JSON output shape:

  {
    "direction": "save"|"apply",
    "dryRun":    bool,
    "copied":    N,
    "removed":   N,
    "errors":    N,
    "actions":   [{"action": "copy"|"prune"|"error", "from", "to", "path", "message"}, ...]
  }`

var saveCmd = &cobra.Command{
	Use:   "save",
	Short: "Copy local environment files into the dotfiles repository",
	Long:  "Copy local environment files into the dotfiles repository.\n\n" + syncJSONShape,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSync(cmd, dotfiles.DirSave, "save", "local environment -> dotfiles")
	},
}

var applyCmd = &cobra.Command{
	Use:     "apply",
	Aliases: []string{"load"},
	Short:   "Copy dotfiles repository files into the local environment",
	Long:    "Copy dotfiles repository files into the local environment.\n\n" + syncJSONShape,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSync(cmd, dotfiles.DirApply, "apply", "dotfiles -> local environment")
	},
}

func init() {
	rootCmd.AddCommand(saveCmd)
	rootCmd.AddCommand(applyCmd)
}

func runSync(cmd *cobra.Command, dir dotfiles.Direction, name, header string) error {
	specs, err := loadSpecs()
	if err != nil {
		return handleErr(cmd, err)
	}
	out := cmd.OutOrStdout()
	syncOut := out
	if jsonOutput {
		syncOut = io.Discard
	} else {
		suffix := ""
		if dryRun {
			suffix = " [DRY RUN]"
		}
		fmt.Fprintf(out, "%s%s\n", header, suffix)
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
