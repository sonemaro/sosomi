// Application state and initialization for sosomi CLI
package main

import (
	"fmt"
	"os"

	"github.com/soroush/sosomi/internal/ai"
	"github.com/soroush/sosomi/internal/config"
	"github.com/soroush/sosomi/internal/history"
)

var (
	// Version info - set during build via ldflags
	version = "dev"
	commit  = "none"
	date    = "unknown"

	// Command flags
	autoExecute bool
	dryRun      bool
	explainOnly bool
	silent      bool
	profileName string

	// Global instances
	historyStore *history.Store
)

// initializeApp sets up the application configuration and stores
func initializeApp() error {
	// Check for first run
	if config.IsFirstRun() && profileName == "" {
		if !silent {
			fmt.Println("Welcome to sosomi! Run 'sosomi init' to set up your configuration.")
		}
	}

	// Initialize configuration with profile
	if err := config.InitWithProfile("", profileName); err != nil {
		return fmt.Errorf("failed to initialize config: %w", err)
	}

	cfg := config.Get()

	// Ensure directories exist
	if err := config.EnsureDirs(); err != nil {
		return err
	}

	// Initialize history store
	if cfg.History.Enabled {
		var err error
		historyStore, err = history.NewStore(cfg.History.DBPath)
		if err != nil {
			// Non-fatal, continue without history
			fmt.Fprintf(os.Stderr, "Warning: Could not initialize history: %v\n", err)
		}
	}

	return nil
}

// getAIProvider creates a new AI provider from the current configuration.
// This centralizes AI provider creation and error handling.
func getAIProvider() (ai.Provider, error) {
	provider, err := ai.NewProviderFromConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI provider: %w", err)
	}
	return provider, nil
}
