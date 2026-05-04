package cli

import (
	"fmt"

	"github.com/dreikanter/dotfiles-cli/internal/dotfiles"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Print the resolved dotfile-to-local mapping as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		specs, err := loadSpecs()
		if err != nil {
			return err
		}
		b, err := dotfiles.ConfigJSON(specs)
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}
