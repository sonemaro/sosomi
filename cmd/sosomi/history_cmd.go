// History command for sosomi CLI
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// historyCmd returns the history subcommand
func historyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "View command history",
		RunE: func(cmd *cobra.Command, args []string) error {
			if historyStore == nil {
				return fmt.Errorf("history is not enabled")
			}

			entries, err := historyStore.ListCommands(20, 0, "")
			if err != nil {
				return err
			}

			for _, entry := range entries {
				status := "⏸"
				if entry.Executed {
					if entry.ExitCode == 0 {
						status = "✓"
					} else {
						status = "✗"
					}
				}
				fmt.Printf("%s %s [%s] %s\n  └─ %s\n\n",
					status,
					entry.Timestamp.Format("2006-01-02 15:04:05"),
					entry.RiskLevel.String(),
					entry.Prompt,
					entry.GeneratedCmd,
				)
			}
			return nil
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "stats",
		Short: "Show history statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			if historyStore == nil {
				return fmt.Errorf("history is not enabled")
			}

			stats, err := historyStore.GetStats()
			if err != nil {
				return err
			}

			fmt.Printf("Total Commands: %v\n", stats["total_commands"])
			fmt.Printf("Executed: %v\n", stats["executed_commands"])
			fmt.Printf("By Risk Level: %v\n", stats["by_risk_level"])
			fmt.Printf("By Provider: %v\n", stats["by_provider"])
			return nil
		},
	})

	return cmd
}
