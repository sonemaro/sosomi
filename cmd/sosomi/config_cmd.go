// Config command for sosomi CLI
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/soroush/sosomi/internal/config"
)

// configCmd returns the config subcommand
func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()
			fmt.Printf("Provider: %s\n", cfg.Provider.Name)
			fmt.Printf("Model: %s\n", cfg.Model.Name)
			fmt.Printf("Endpoint: %s\n", config.GetEndpoint())
			fmt.Printf("Safety Level: %s\n", cfg.Safety.Level)
			fmt.Printf("Auto Execute Safe: %v\n", cfg.Safety.AutoExecuteSafe)
			fmt.Printf("History Enabled: %v\n", cfg.History.Enabled)
			if profile := config.GetActiveProfile(); profile != "" {
				fmt.Printf("Active Profile: %s\n", profile)
			}
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "set [key] [value]",
		Short: "Set a configuration value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.Set(args[0], args[1]); err != nil {
				return err
			}
			return config.Save()
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "validate",
		Short: "Validate current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			result := config.ValidateCurrent()
			if !result.IsValid() || result.HasWarnings() {
				fmt.Print(result.String())
			}
			if result.IsValid() {
				fmt.Println("✓ Configuration is valid")
				return nil
			}
			return fmt.Errorf("configuration has errors")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "edit",
		Short: "Open configuration file in editor",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := config.GetConfigPaths()
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vim"
			}
			return runEditor(editor, paths.User)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Show configuration file paths",
		Run: func(cmd *cobra.Command, args []string) {
			paths := config.GetConfigPaths()
			fmt.Printf("System config: %s\n", paths.System)
			fmt.Printf("User config:   %s\n", paths.User)
			fmt.Printf("Project config: %s\n", paths.Project)
			fmt.Printf("Profiles dir:  %s\n", paths.ProfileDir)
		},
	})

	return cmd
}
