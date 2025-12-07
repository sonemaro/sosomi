// Init command for sosomi CLI
package main

import (
	"github.com/spf13/cobra"

	"github.com/soroush/sosomi/internal/config"
)

// initCmd returns the init subcommand
func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize sosomi with interactive setup",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := config.RunWizard()
			return err
		},
	}
}
