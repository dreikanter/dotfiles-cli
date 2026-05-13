package cli

import (
	"github.com/spf13/cobra"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Print an agent-installable skill describing this CLI",
	Long:  `Print an agent-installable skill describing this CLI.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		return nil
	},
}

func init() {
	rootCmd.AddCommand(skillCmd)
}
