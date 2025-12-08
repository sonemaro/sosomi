// Profile command for sosomi CLI
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sonemaro/sosomi/internal/config"
	"github.com/sonemaro/sosomi/internal/ui"
)

// profileCmd returns the profile subcommand
func profileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage configuration profiles",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List available profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			profiles, err := config.ListProfiles()
			if err != nil {
				return err
			}

			if len(profiles) == 0 {
				ui.PrintInfo("No profiles found. Create one with 'sosomi profile create <name>'")
				return nil
			}

			activeProfile := config.GetActiveProfile()
			defaultProfile := config.Get().DefaultProfile

			fmt.Println("Available profiles:")
			for _, name := range profiles {
				marker := "  "
				if name == activeProfile {
					marker = "▶ "
				}
				extra := ""
				if name == defaultProfile {
					extra = " (default)"
				}
				fmt.Printf("%s%s%s\n", marker, name, extra)
			}
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "create [name]",
		Short: "Create a new profile interactively",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Check if profile already exists
			if _, err := config.GetProfile(name); err == nil {
				return fmt.Errorf("profile '%s' already exists", name)
			}

			// Run wizard with profile focus
			result, err := config.RunWizard()
			if err != nil {
				return err
			}

			// Save as the specified profile
			profile := &config.Profile{
				Name:     name,
				Provider: result.Config.Provider,
				Model:    result.Config.Model,
				Safety:   config.SafetyConfig{Level: result.Config.Safety.Level},
			}

			if err := config.SaveProfile(profile); err != nil {
				return err
			}

			ui.PrintSuccess(fmt.Sprintf("Profile '%s' created!", name))
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "use [name]",
		Short: "Set the default profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if err := config.SetDefaultProfile(name); err != nil {
				return err
			}

			ui.PrintSuccess(fmt.Sprintf("Default profile set to '%s'", name))
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "show [name]",
		Short: "Show profile details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			profile, err := config.GetProfile(name)
			if err != nil {
				return fmt.Errorf("profile not found: %s", name)
			}

			fmt.Printf("Profile: %s\n", profile.Name)
			if profile.Description != "" {
				fmt.Printf("Description: %s\n", profile.Description)
			}
			fmt.Printf("Provider: %s\n", profile.Provider.Name)
			fmt.Printf("Model: %s\n", profile.Model.Name)
			fmt.Printf("Endpoint: %s\n", profile.Provider.Endpoint)
			fmt.Printf("Safety Level: %s\n", profile.Safety.Level)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "delete [name]",
		Short: "Delete a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			reader := bufio.NewReader(os.Stdin)
			fmt.Printf("Delete profile '%s'? (y/n): ", name)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))

			if input != "y" && input != "yes" {
				ui.PrintInfo("Canceled")
				return nil
			}

			if err := config.DeleteProfile(name); err != nil {
				return err
			}

			ui.PrintSuccess(fmt.Sprintf("Profile '%s' deleted", name))
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "test [name]",
		Short: "Test profile connectivity",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) > 0 {
				name = args[0]
			}

			fmt.Print("Testing connection...")
			if err := config.TestProfile(name); err != nil {
				fmt.Println(" ✗ Failed")
				return err
			}
			fmt.Println(" ✓ Connected")
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "export [name] [file]",
		Short: "Export a profile (without secrets)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			file := args[1]

			data, err := config.ExportProfile(name)
			if err != nil {
				return err
			}

			if err := os.WriteFile(file, data, 0644); err != nil {
				return err
			}

			ui.PrintSuccess(fmt.Sprintf("Profile exported to %s", file))
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "import [file]",
		Short: "Import a profile from file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			file := args[0]

			if err := config.ImportProfile(file); err != nil {
				return err
			}

			ui.PrintSuccess("Profile imported successfully")
			return nil
		},
	})

	return cmd
}
