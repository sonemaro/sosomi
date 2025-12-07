// Root command definition for sosomi CLI
package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// Execute runs the root command - this is the main entry point
func Execute() error {
	rootCmd := newRootCmd()
	return rootCmd.Execute()
}

// newRootCmd creates and configures the root command
func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "sosomi [prompt]",
		Short: "Safe AI Shell Assistant",
		Long: `Sosomi is a CLI tool that converts natural language to shell commands
with advanced safety features including risk analysis and dry-run mode.

Examples:
  sosomi "list all files larger than 100MB"
  sosomi "show disk usage" --auto
  sosomi "delete all .tmp files" --dry-run
  sosomi chat`,
		Args:    cobra.ArbitraryArgs,
		Version: fmt.Sprintf("%s (commit: %s)", version, commit),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return initializeApp()
		},
		RunE: runMain,
	}

	// Add flags
	addRootFlags(rootCmd)

	// Add subcommands
	rootCmd.AddCommand(chatCmd())
	rootCmd.AddCommand(llmCmd())
	rootCmd.AddCommand(askCmd())
	rootCmd.AddCommand(configCmd())
	rootCmd.AddCommand(historyCmd())
	rootCmd.AddCommand(modelsCmd())
	rootCmd.AddCommand(profileCmd())
	rootCmd.AddCommand(initCmd())

	return rootCmd
}

// addRootFlags adds command-line flags to the root command
func addRootFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&autoExecute, "auto", "a", false, "Auto-execute safe commands")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Simulate without executing")
	cmd.Flags().BoolVarP(&explainOnly, "explain", "e", false, "Show explanation only")
	cmd.Flags().BoolVarP(&silent, "silent", "s", false, "Minimal output")
	cmd.Flags().StringVarP(&profileName, "profile", "p", "", "Configuration profile to use")
}

// runMain handles the root command execution (single prompt mode)
func runMain(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}

	prompt := strings.Join(args, " ")
	return processPrompt(prompt)
}
