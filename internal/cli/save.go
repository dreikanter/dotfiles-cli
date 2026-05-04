package cli

import (
	"fmt"

	"github.com/dreikanter/dotfiles-cli/internal/dotfiles"
	"github.com/spf13/cobra"
)

var saveCmd = &cobra.Command{
	Use:   "save",
	Short: "Copy local environment files into the dotfiles repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSync(cmd, dotfiles.DirSave, "local environment -> dotfiles")
	},
}

var applyCmd = &cobra.Command{
	Use:     "apply",
	Aliases: []string{"load"},
	Short:   "Copy dotfiles repository files into the local environment",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSync(cmd, dotfiles.DirApply, "dotfiles -> local environment")
	},
}

func init() {
	rootCmd.AddCommand(saveCmd)
	rootCmd.AddCommand(applyCmd)
}

func runSync(cmd *cobra.Command, dir dotfiles.Direction, header string) error {
	specs, err := loadSpecs()
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	suffix := ""
	if dryRun {
		suffix = " [DRY RUN]"
	}
	fmt.Fprintf(out, "%s%s\n", header, suffix)
	res, err := dotfiles.Sync(specs, dir, dotfiles.Options{
		DryRun:  dryRun,
		Verbose: verbose,
		Prune:   prune,
		Out:     out,
	})
	if err != nil {
		return err
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
