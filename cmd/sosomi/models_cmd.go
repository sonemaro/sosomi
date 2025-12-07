// Models command for sosomi CLI
package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/soroush/sosomi/internal/ai"
)

// modelsCmd returns the models subcommand
func modelsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "List available models",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			aiProvider, err := getAIProvider()
			if err != nil {
				return err
			}

			models, err := aiProvider.ListModels(ctx)
			if err != nil {
				return err
			}

			fmt.Printf("Available models for %s:\n", aiProvider.Name())
			for _, m := range models {
				fmt.Printf("  - %s\n", m)
			}

			fmt.Println("\nRecommended models:")
			for provider, models := range ai.RecommendedModels() {
				fmt.Printf("  %s: %s\n", provider, strings.Join(models, ", "))
			}

			return nil
		},
	}
}
