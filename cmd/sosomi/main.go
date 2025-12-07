// Sosomi - Safe AI Shell Assistant
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/peterh/liner"
	"github.com/spf13/cobra"

	"github.com/soroush/sosomi/internal/ai"
	"github.com/soroush/sosomi/internal/config"
	"github.com/soroush/sosomi/internal/conversation"
	"github.com/soroush/sosomi/internal/history"
	"github.com/soroush/sosomi/internal/mcp"
	"github.com/soroush/sosomi/internal/safety"
	"github.com/soroush/sosomi/internal/session"
	"github.com/soroush/sosomi/internal/shell"
	"github.com/soroush/sosomi/internal/types"
	"github.com/soroush/sosomi/internal/ui"
	"github.com/soroush/sosomi/internal/undo"
)

var (
	// Version info
	version = "0.1.0"
	commit  = "dev"

	// Flags
	autoExecute  bool
	dryRun       bool
	explainOnly  bool
	silent       bool
	profileName  string
	forceExecute bool

	// Global instances
	historyStore  *history.Store
	backupManager *undo.Manager
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "sosomi [prompt]",
		Short: "Safe AI Shell Assistant",
		Long: `Sosomi is a CLI tool that converts natural language to shell commands
with advanced safety features including risk analysis, dry-run mode, and undo/rollback.

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

	// Flags
	rootCmd.Flags().BoolVarP(&autoExecute, "auto", "a", false, "Auto-execute safe commands")
	rootCmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Simulate without executing")
	rootCmd.Flags().BoolVarP(&explainOnly, "explain", "e", false, "Show explanation only")
	rootCmd.Flags().BoolVarP(&silent, "silent", "s", false, "Minimal output")
	rootCmd.Flags().StringVarP(&profileName, "profile", "p", "", "Configuration profile to use")
	rootCmd.Flags().BoolVar(&forceExecute, "force", false, "Override safety blocks (dangerous!)")

	// Subcommands
	rootCmd.AddCommand(chatCmd())
	rootCmd.AddCommand(llmCmd())
	rootCmd.AddCommand(askCmd())
	rootCmd.AddCommand(configCmd())
	rootCmd.AddCommand(historyCmd())
	rootCmd.AddCommand(undoCmd())
	rootCmd.AddCommand(modelsCmd())
	rootCmd.AddCommand(profileCmd())
	rootCmd.AddCommand(initCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func initializeApp() error {
	// Check for first run
	if config.IsFirstRun() && profileName == "" {
		// First run without profile - run wizard if interactive
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

	// Initialize backup manager
	if cfg.Backup.Enabled {
		var err error
		backupManager, err = undo.NewManager(
			cfg.Backup.Dir,
			cfg.Backup.MaxSizeMB,
			cfg.Backup.RetentionDays,
			cfg.Backup.Exclude,
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not initialize backup: %v\n", err)
		}
	}

	return nil
}

func runMain(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		// No prompt provided, show help or enter interactive mode
		return cmd.Help()
	}

	prompt := strings.Join(args, " ")
	return processPrompt(prompt)
}

func processPrompt(prompt string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.Get().Model.TimeoutSeconds)*time.Second)
	defer cancel()

	// Get system context
	sysCtx := shell.GetSystemContext()

	// Create AI provider
	aiProvider, err := ai.NewProviderFromConfig()
	if err != nil {
		return fmt.Errorf("failed to create AI provider: %w", err)
	}

	// Show spinner while generating
	if !silent {
		fmt.Print("🔮 Generating command...")
	}

	// Generate command
	response, err := aiProvider.GenerateCommand(ctx, prompt, sysCtx)
	if err != nil {
		fmt.Println() // Clear spinner line
		return fmt.Errorf("failed to generate command: %w", err)
	}

	fmt.Print("\r                        \r") // Clear spinner

	if response.Command == "" {
		ui.PrintError("Could not generate a command for this request")
		if response.Explanation != "" {
			ui.PrintInfo(response.Explanation)
		}
		return nil
	}

	// Display command
	if !silent {
		ui.PrintCommand(response.Command)
	}

	// Analyze command safety
	cfg := config.Get()
	analyzer := safety.NewAnalyzer(cfg.Safety.BlockedCommands, cfg.Safety.AllowedPaths)
	analysis, err := analyzer.Analyze(response.Command)
	if err != nil {
		ui.PrintWarning(fmt.Sprintf("Could not analyze command: %v", err))
	}

	// Merge AI risk assessment with pattern analysis
	if response.RiskLevel > analysis.RiskLevel {
		analysis.RiskLevel = response.RiskLevel
	}

	// Display analysis
	if !silent {
		if config.Get().UI.ShowExplanations && response.Explanation != "" {
			ui.PrintExplanation(response.Explanation)
		}
		ui.PrintRiskLevel(analysis.RiskLevel, analysis.RiskReasons)
	}

	// Handle warnings from AI
	if len(response.Warnings) > 0 {
		for _, warning := range response.Warnings {
			ui.PrintWarning(warning)
		}
	}

	// Check if blocked
	if analysis.RiskLevel == types.RiskCritical && !forceExecute {
		ui.PrintError("This command is blocked due to critical risk level")
		ui.PrintInfo("Use --force to override (not recommended)")
		return nil
	}

	// Handle execution mode
	if explainOnly {
		return nil
	}

	if dryRun {
		return executeDryRun(response.Command, analysis)
	}

	// Interactive confirmation
	if autoExecute && analysis.RiskLevel == types.RiskSafe {
		return executeCommand(response.Command, prompt, analysis)
	}

	// Check for auto-execute safe setting from config
	if config.Get().Safety.AutoExecuteSafe && analysis.RiskLevel == types.RiskSafe {
		return executeCommand(response.Command, prompt, analysis)
	}

	// Interactive confirmation
	return interactiveConfirm(response, analysis, prompt)
}

func interactiveConfirm(response *types.CommandResponse, analysis *types.CommandAnalysis, prompt string) error {
	reader := bufio.NewReader(os.Stdin)

	for {
		ui.PrintConfirmPrompt()
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		switch input {
		case "y", "yes":
			return executeCommand(response.Command, prompt, analysis)
		case "n", "no", "":
			ui.PrintInfo("Command cancelled")
			return nil
		case "d", "dry-run":
			return executeDryRun(response.Command, analysis)
		case "e", "explain":
			ui.PrintAnalysis(analysis)
		case "m", "modify":
			fmt.Print("\n  Enter modified command: ")
			newCmd, _ := reader.ReadString('\n')
			newCmd = strings.TrimSpace(newCmd)
			if newCmd != "" {
				response.Command = newCmd
				// Re-analyze
				cfg := config.Get()
				analyzer := safety.NewAnalyzer(cfg.Safety.BlockedCommands, cfg.Safety.AllowedPaths)
				newAnalysis, _ := analyzer.Analyze(newCmd)
				*analysis = *newAnalysis
				ui.PrintCommand(newCmd)
				ui.PrintRiskLevel(analysis.RiskLevel, analysis.RiskReasons)
			}
		default:
			fmt.Println("  Invalid option. Please enter y, n, d, e, or m")
		}
	}
}

func executeCommand(command, prompt string, analysis *types.CommandAnalysis) error {
	cfg := config.Get()

	// Create backup if enabled and command is risky
	var backupEntry *types.BackupEntry
	if backupManager != nil && analysis.RiskLevel >= types.RiskCaution && len(analysis.AffectedPaths) > 0 {
		cwd, _ := os.Getwd()
		var err error
		backupEntry, err = backupManager.CreateBackup(command, cwd, analysis.AffectedPaths)
		if err != nil {
			ui.PrintWarning(fmt.Sprintf("Could not create backup: %v", err))
		}
	}

	// Execute command
	start := time.Now()
	result, err := shell.Execute(command, false)
	duration := time.Since(start).Milliseconds()

	if err != nil && result == nil {
		return fmt.Errorf("execution failed: %w", err)
	}

	// Display result
	if !silent {
		ui.PrintExecutionResult(result.Stdout, result.Stderr, result.ExitCode, duration)
	} else {
		if result.Stdout != "" {
			fmt.Print(result.Stdout)
		}
	}

	// Save to history
	if historyStore != nil {
		cwd, _ := os.Getwd()
		entry := &types.HistoryEntry{
			Prompt:       prompt,
			GeneratedCmd: command,
			RiskLevel:    analysis.RiskLevel,
			Executed:     true,
			ExitCode:     result.ExitCode,
			DurationMs:   duration,
			WorkingDir:   cwd,
			Provider:     cfg.Provider.Name,
			Model:        cfg.Model.Name,
		}
		historyStore.AddCommand(entry)
	}

	// Show backup info if created
	if backupEntry != nil && !silent {
		ui.PrintBackupInfo(backupEntry)
	}

	// Offer retry option if not in auto mode and not silent
	if !autoExecute && !silent {
		return offerRetry(prompt, command, result)
	}

	return nil
}

// offerRetry gives the user a chance to refine the command after execution
func offerRetry(originalPrompt, executedCmd string, result *shell.ExecuteResult) error {
	reader := bufio.NewReader(os.Stdin)

	for {
		ui.PrintRetryPrompt()
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		switch input {
		case "r", "retry":
			return retryWithFeedback(originalPrompt, executedCmd, result)
		case "n", "no", "done", "":
			return nil
		default:
			fmt.Println("  Invalid option. Please enter r or n")
		}
	}
}

// retryWithFeedback asks for user feedback and regenerates the command
func retryWithFeedback(originalPrompt, executedCmd string, result *shell.ExecuteResult) error {
	reader := bufio.NewReader(os.Stdin)

	ui.PrintFeedbackPrompt()
	feedback, _ := reader.ReadString('\n')
	feedback = strings.TrimSpace(feedback)

	if feedback == "" {
		ui.PrintInfo("No feedback provided, keeping original command")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.Get().Model.TimeoutSeconds)*time.Second)
	defer cancel()

	// Get system context
	sysCtx := shell.GetSystemContext()

	// Create AI provider
	aiProvider, err := ai.NewProviderFromConfig()
	if err != nil {
		return fmt.Errorf("failed to create AI provider: %w", err)
	}

	// Show spinner while refining
	if !silent {
		fmt.Print("\n🔄 Refining command based on feedback...")
	}

	// Build refine request
	refineReq := ai.RefineRequest{
		OriginalPrompt: originalPrompt,
		GeneratedCmd:   executedCmd,
		Feedback:       feedback,
		WasExecuted:    true,
		ExitCode:       result.ExitCode,
		CommandOutput:  result.Stdout,
		CommandError:   result.Stderr,
	}

	// Refine command
	response, err := aiProvider.RefineCommand(ctx, refineReq, sysCtx)
	if err != nil {
		fmt.Println() // Clear spinner
		return fmt.Errorf("failed to refine command: %w", err)
	}

	fmt.Print("\r                                        \r") // Clear spinner

	if response.Command == "" {
		ui.PrintError("Could not generate a refined command")
		if response.Explanation != "" {
			ui.PrintInfo(response.Explanation)
		}
		return nil
	}

	// Display refined command
	if !silent {
		fmt.Println()
		ui.PrintSuccess("Refined command:")
		ui.PrintCommand(response.Command)
		if response.Explanation != "" {
			ui.PrintExplanation(response.Explanation)
		}
	}

	// Analyze the new command
	cfg := config.Get()
	analyzer := safety.NewAnalyzer(cfg.Safety.BlockedCommands, cfg.Safety.AllowedPaths)
	analysis, err := analyzer.Analyze(response.Command)
	if err != nil {
		ui.PrintWarning(fmt.Sprintf("Could not analyze command: %v", err))
	}

	if response.RiskLevel > analysis.RiskLevel {
		analysis.RiskLevel = response.RiskLevel
	}

	if !silent {
		ui.PrintRiskLevel(analysis.RiskLevel, analysis.RiskReasons)
	}

	// Interactive confirmation for the refined command
	return interactiveConfirm(response, analysis, originalPrompt)
}

func executeDryRun(command string, analysis *types.CommandAnalysis) error {
	ui.PrintInfo("DRY RUN - Command will not be executed")

	// Show detailed analysis
	ui.PrintAnalysis(analysis)

	// Try to show what files would be affected
	cfg := config.Get()
	analyzer := safety.NewAnalyzer(cfg.Safety.BlockedCommands, cfg.Safety.AllowedPaths)
	files, _ := analyzer.GetAffectedFiles(analysis)

	if len(files) > 0 {
		fmt.Println("\n  📁 Files that would be affected:")
		for _, f := range files {
			if f.IsDir {
				fmt.Printf("     📂 %s (%d files)\n", f.Path, f.FileCount)
			} else {
				fmt.Printf("     📄 %s (%s)\n", f.Path, formatSize(f.Size))
			}
		}
	}

	return nil
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// chatCmd returns the chat subcommand with session support
func chatCmd() *cobra.Command {
	var continueSession string

	cmd := &cobra.Command{
		Use:   "chat [session-name]",
		Short: "Start interactive shell chat mode with session persistence",
		Long: `Start an interactive shell chat session with AI-powered command generation.
Sessions are persisted and can be continued later.

The AI sees command outputs and can refine based on results.

Examples:
  sosomi chat                          # Start new session
  sosomi chat "Docker Setup"           # Start with name
  sosomi chat -c abc123                # Continue existing session
  sosomi chat list                     # List sessions
  sosomi chat pick                     # Interactive session picker
  sosomi chat delete <id>              # Delete session`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var sessName string
			if len(args) > 0 {
				sessName = args[0]
			}
			return runChat(sessName, continueSession)
		},
	}

	cmd.Flags().StringVarP(&continueSession, "continue", "c", "", "Continue an existing session (ID or name)")

	// Add subcommands
	cmd.AddCommand(chatListCmd())
	cmd.AddCommand(chatPickCmd())
	cmd.AddCommand(chatDeleteCmd())
	cmd.AddCommand(chatStatsCmd())
	cmd.AddCommand(chatExportCmd())
	cmd.AddCommand(chatImportCmd())

	return cmd
}

func chatListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all chat sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()
			store, err := session.NewStore(cfg.Chat.DBPath)
			if err != nil {
				return err
			}
			defer store.Close()

			sessions, err := store.ListSessions(50, 0)
			if err != nil {
				return err
			}

			if len(sessions) == 0 {
				fmt.Println("No sessions yet. Start one with: sosomi chat")
				return nil
			}

			fmt.Println("\n📂 Chat Sessions:")
			fmt.Println(ui.Dim("──────────────────────────────────────────────────────────────────"))
			fmt.Printf("  %-10s %-30s %-6s %-6s %s\n",
				ui.Dim("ID"), ui.Dim("Name"), ui.Dim("Cmds"), ui.Dim("Msgs"), ui.Dim("Updated"))
			fmt.Println(ui.Dim("──────────────────────────────────────────────────────────────────"))

			for _, s := range sessions {
				shortID := s.ID[:8]
				name := s.Name
				if len(name) > 28 {
					name = name[:25] + "..."
				}
				ago := ui.FormatDurationShort(time.Since(s.UpdatedAt))
				fmt.Printf("  %-10s %-30s %-6d %-6d %s\n",
					ui.Cyan(shortID), name, s.CommandCount, s.MessageCount, ui.Dim(ago))
			}
			fmt.Println()
			return nil
		},
	}
}

func chatPickCmd() *cobra.Command {
	var pageSize int

	cmd := &cobra.Command{
		Use:   "pick",
		Short: "Interactive session picker",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()
			store, err := session.NewStore(cfg.Chat.DBPath)
			if err != nil {
				return err
			}
			defer store.Close()

			sessions, err := store.ListSessions(100, 0)
			if err != nil {
				return err
			}

			selected, createNew, err := ui.SessionPicker(sessions, pageSize)
			if err != nil {
				return err
			}

			// Clear screen after picker
			fmt.Print("\033[2J\033[H")

			if createNew {
				return runChat("", "")
			}
			if selected != nil {
				return runChat("", selected.ID)
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&pageSize, "page-size", "p", 10, "Number of sessions per page")
	return cmd
}

func chatDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <session-id-or-name>",
		Short: "Delete a chat session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()
			store, err := session.NewStore(cfg.Chat.DBPath)
			if err != nil {
				return err
			}
			defer store.Close()

			if err := store.DeleteSession(args[0]); err != nil {
				return err
			}
			fmt.Printf("✓ Session deleted: %s\n", args[0])
			return nil
		},
	}
}

func chatStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show chat session statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()
			store, err := session.NewStore(cfg.Chat.DBPath)
			if err != nil {
				return err
			}
			defer store.Close()

			totalSessions, totalMessages, totalCommands, totalTokens, err := store.GetStats()
			if err != nil {
				return err
			}

			fmt.Println("\n📊 Chat Session Statistics:")
			fmt.Printf("   Sessions:  %d\n", totalSessions)
			fmt.Printf("   Messages:  %d\n", totalMessages)
			fmt.Printf("   Commands:  %d\n", totalCommands)
			fmt.Printf("   Tokens:    %d\n", totalTokens)
			fmt.Println()
			return nil
		},
	}
}

func chatExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export <session-id-or-name> [output-file]",
		Short: "Export a chat session to JSON",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()
			store, err := session.NewStore(cfg.Chat.DBPath)
			if err != nil {
				return err
			}
			defer store.Close()

			// Find session
			sess, err := store.GetSession(args[0])
			if err != nil {
				sess, err = store.GetSessionByName(args[0])
				if err != nil {
					return fmt.Errorf("session not found: %s", args[0])
				}
			}

			data, err := store.ExportSession(sess.ID)
			if err != nil {
				return err
			}

			// Output to file or stdout
			if len(args) > 1 {
				if err := os.WriteFile(args[1], data, 0644); err != nil {
					return err
				}
				fmt.Printf("✓ Exported to %s\n", args[1])
			} else {
				fmt.Println(string(data))
			}
			return nil
		},
	}
}

func chatImportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import <file>",
		Short: "Import a chat session from JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()
			store, err := session.NewStore(cfg.Chat.DBPath)
			if err != nil {
				return err
			}
			defer store.Close()

			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}

			sess, err := store.ImportSession(data)
			if err != nil {
				return err
			}

			fmt.Printf("✓ Imported session: %s (%s)\n", sess.Name, sess.ID[:8])
			return nil
		},
	}
}

func runChat(sessName, continueID string) error {
	cfg := config.Get()

	// Ensure data directory exists
	if err := os.MkdirAll(filepath.Dir(cfg.Chat.DBPath), 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Open session store
	sessStore, err := session.NewStore(cfg.Chat.DBPath)
	if err != nil {
		return fmt.Errorf("failed to open session store: %w", err)
	}
	defer sessStore.Close()

	// Create AI provider
	aiProvider, err := ai.NewProviderFromConfig()
	if err != nil {
		return fmt.Errorf("failed to create AI provider: %w", err)
	}

	// Get current directory
	cwd, _ := os.Getwd()

	// Load or create session
	var sess *types.Session
	var contextMsgs []ai.Message
	var storedMsgs []*types.SessionMessage

	if continueID != "" {
		// Continue existing session
		sess, err = sessStore.GetSession(continueID)
		if err != nil {
			sess, err = sessStore.GetSessionByName(continueID)
			if err != nil {
				return fmt.Errorf("session not found: %s", continueID)
			}
		}
		// Load existing messages into context
		storedMsgs, err = sessStore.GetMessages(sess.ID)
		if err != nil {
			return err
		}
		contextMsgs = buildChatContext(storedMsgs, cfg.Chat.OutputMaxLines)
		fmt.Printf("🖥️  Continuing session: %s\n", ui.Cyan(sess.Name))
		fmt.Printf("📊 Commands: %d, Messages: %d\n", sess.CommandCount, sess.MessageCount)
		// Show recent history
		printSessionHistory(storedMsgs, 5)
	} else {
		// Create new session
		if sessName == "" {
			sessName = "New Session"
		}
		sess, err = sessStore.CreateSession(sessName, cfg.Provider.Name, cfg.Model.Name, cwd)
		if err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}
		fmt.Printf("🖥️  New shell session: %s\n", ui.Cyan(sess.Name))
	}

	fmt.Printf("🤖 Model: %s (%s)\n", cfg.Model.Name, cfg.Provider.Name)
	fmt.Println("Type /help for commands, /quit to exit")
	fmt.Println()

	// Setup liner for readline with history
	line := liner.NewLiner()
	defer func() {
		if line != nil {
			line.Close()
		}
	}()

	line.SetCtrlCAborts(true)

	// Set terminal mode to support editing
	line.SetCompleter(nil) // No tab completion for now

	// Load history from session messages
	if len(storedMsgs) > 0 {
		for _, msg := range storedMsgs {
			if msg.Role == "user" && msg.Content != "" && !strings.HasPrefix(msg.Content, "/") {
				line.AppendHistory(msg.Content)
			}
		}
	}

	isFirstExchange := sess.MessageCount == 0

	// Build system prompt with shell context
	sysContext := shell.GetSystemContext()
	systemPrompt := buildChatSystemPrompt(sysContext)
	contextMsgs = append([]ai.Message{{Role: "system", Content: systemPrompt}}, contextMsgs...)

	for {
		// Show current directory in prompt
		currentDir, _ := os.Getwd()
		shortDir := shortenPath(currentDir)
		// Don't use ANSI colors in liner prompt - liner doesn't support it
		prompt := fmt.Sprintf("%s> ", shortDir)

		input, err := line.Prompt(prompt)
		if err != nil {
			// Check error type
			if err == liner.ErrPromptAborted {
				// Ctrl+C
				fmt.Println("\nGoodbye! 👋")
				return nil
			}
			// EOF or other error
			if err.Error() == "EOF" {
				fmt.Println("\nGoodbye! 👋")
				return nil
			}
			// Unexpected error
			return fmt.Errorf("input error: %w", err)
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Add to history (liner handles duplicates)
		line.AppendHistory(input)

		// Handle special commands
		switch {
		case input == "/quit" || input == "/exit" || input == "/q":
			fmt.Println("Goodbye! 👋")
			return nil
		case input == "/help" || input == "/h":
			printChatHelp()
			continue
		case input == "/info":
			printSessionInfo(sess, sessStore)
			continue
		case input == "/history":
			storedMsgs, _ := sessStore.GetMessages(sess.ID)
			printSessionHistory(storedMsgs, 20)
			continue
		case input == "/clear":
			fmt.Print("\033[2J\033[H")
			continue
		case input == "/pick":
			// Switch to another session
			sessions, _ := sessStore.ListSessions(100, 0)
			// Close liner before SessionPicker (which uses bubbletea and takes over terminal)
			line.Close()
			selected, createNew, _ := ui.SessionPicker(sessions, 10)
			fmt.Print("\033[2J\033[H")
			if createNew {
				return runChat("", "")
			}
			if selected != nil {
				return runChat("", selected.ID)
			}
			// Recreate liner after picker
			line = liner.NewLiner()
			line.SetCtrlCAborts(true)
			line.SetCompleter(nil)
			// Reload history
			storedMsgs, _ = sessStore.GetMessages(sess.ID)
			for _, msg := range storedMsgs {
				if msg.Role == "user" && msg.Content != "" && !strings.HasPrefix(msg.Content, "/") {
					line.AppendHistory(msg.Content)
				}
			}
			continue
		case input == "/new":
			return runChat("", "")
		case input == "/tokens":
			sess, _ = sessStore.GetSession(sess.ID)
			fmt.Printf("💰 Tokens used: %d\n", sess.TotalTokens)
			continue
		case input == "/auto":
			// Show current auto-execute state
			var err error
			sess, err = sessStore.GetSession(sess.ID)
			if err != nil {
				ui.PrintError("Failed to get session info")
				continue
			}
			status := "OFF"
			if sess.AutoExecute {
				status = "ON"
			}
			fmt.Printf("🤖 Auto-execute: %s\n", ui.Bold(status))
			continue
		case input == "/auto on":
			// Enable auto-execute for this session
			if err := sessStore.UpdateAutoExecute(sess.ID, true); err != nil {
				ui.PrintError("Failed to enable auto-execute")
				continue
			}
			sess.AutoExecute = true
			fmt.Printf("✓ Auto-execute %s for this session\n", ui.Success("enabled"))
			continue
		case input == "/auto off":
			// Disable auto-execute for this session
			if err := sessStore.UpdateAutoExecute(sess.ID, false); err != nil {
				ui.PrintError("Failed to disable auto-execute")
				continue
			}
			sess.AutoExecute = false
			fmt.Printf("✓ Auto-execute %s for this session\n", ui.Dim("disabled"))
			continue
		case strings.HasPrefix(input, "/"):
			fmt.Printf("Unknown command: %s. Type /help for available commands.\n", input)
			continue
		}

		// Add user message to context
		contextMsgs = append(contextMsgs, ai.Message{Role: "user", Content: input})

		// Generate command with AI (streaming)
		fmt.Print("🔮 ")
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Model.TimeoutSeconds)*time.Second)

		stream, err := aiProvider.ChatStream(ctx, contextMsgs)
		if err != nil {
			cancel()
			ui.PrintError(err.Error())
			contextMsgs = contextMsgs[:len(contextMsgs)-1]
			continue
		}

		var response strings.Builder
		var tokenUsage *ai.TokenUsage
		for chunk := range stream {
			if chunk.Error != nil {
				cancel()
				fmt.Println()
				ui.PrintError(chunk.Error.Error())
				contextMsgs = contextMsgs[:len(contextMsgs)-1]
				break
			}
			if chunk.Done {
				tokenUsage = chunk.Usage
				break
			}
			fmt.Print(chunk.Content)
			response.WriteString(chunk.Content)
		}
		fmt.Println()
		cancel()

		if response.Len() == 0 {
			continue
		}

		// Ensure output is flushed
		os.Stdout.Sync()

		assistantContent := response.String()
		contextMsgs = append(contextMsgs, ai.Message{Role: "assistant", Content: assistantContent})

		// Calculate token counts
		userTokens := 0
		assistantTokens := 0
		if tokenUsage != nil {
			assistantTokens = tokenUsage.CompletionTokens
			userTokens = tokenUsage.PromptTokens / max(1, len(contextMsgs)-1)
		}

		// Try to extract command from response
		command := extractCommand(assistantContent)
		if command == "" {
			// No command found - just a conversational response
			sessStore.AddMessage(sess.ID, "user", input, userTokens)
			sessStore.AddMessage(sess.ID, "assistant", assistantContent, assistantTokens)

			// Generate title after first exchange
			if isFirstExchange && cfg.Chat.GenerateTitles && sess.Name == "New Session" {
				go generateSessionTitle(aiProvider, sessStore, sess.ID, input, assistantContent)
				isFirstExchange = false
			}
			fmt.Println()
			continue
		}

		// Analyze command safety
		analyzer := safety.NewAnalyzer(cfg.Safety.BlockedCommands, cfg.Safety.AllowedPaths)
		analysis, _ := analyzer.Analyze(command)

		// Display command and risk
		fmt.Printf("\n%s %s\n", ui.Bold("Command:"), ui.Cyan(command))
		fmt.Printf("%s %s\n", analysis.RiskLevel.Emoji(), analysis.RiskLevel.String())

		if len(analysis.RiskReasons) > 0 {
			for _, reason := range analysis.RiskReasons {
				fmt.Printf("   %s\n", ui.Dim(reason))
			}
		}

		// Check if blocked
		if analysis.RiskLevel == types.RiskCritical {
			fmt.Println(ui.Error("⛔ Command blocked due to critical risk"))
			sessStore.AddExecutionMessage(sess.ID, input, command, "", 0, analysis.RiskLevel, 0, false, userTokens+assistantTokens)
			continue
		}

		// Auto-execute safe commands based on session setting or global config
		autoExec := (sess.AutoExecute || cfg.Safety.AutoExecuteSafe) && analysis.RiskLevel == types.RiskSafe

		var confirmed bool
		if autoExec {
			fmt.Println(ui.Dim("[auto-executing - SAFE]"))
			confirmed = true
		} else {
			// Prompt for confirmation (use simple reader for one-off prompts)
			confirmReader := bufio.NewReader(os.Stdin)
			fmt.Print("[y] run  [n] cancel  [e] edit > ")
			confirmInput, _ := confirmReader.ReadString('\n')
			confirmInput = strings.TrimSpace(strings.ToLower(confirmInput))

			switch confirmInput {
			case "y", "yes":
				confirmed = true
			case "e", "edit":
				fmt.Print("  Enter modified command: ")
				newCmd, _ := confirmReader.ReadString('\n')
				newCmd = strings.TrimSpace(newCmd)
				if newCmd != "" {
					command = newCmd
					analysis, _ = analyzer.Analyze(command)
					confirmed = true
				}
			default:
				fmt.Println(ui.Dim("Cancelled"))
				sessStore.AddExecutionMessage(sess.ID, input, command, "", 0, analysis.RiskLevel, 0, false, userTokens+assistantTokens)
				continue
			}
		}

		if confirmed {
			// Create backup if risky
			if backupManager != nil && analysis.RiskLevel >= types.RiskDangerous && len(analysis.AffectedPaths) > 0 {
				backup, err := backupManager.CreateBackup(command, cwd, analysis.AffectedPaths)
				if err == nil && backup != nil {
					fmt.Printf("📦 Backup created: %s\n", ui.Dim(backup.ID[:8]))
				}
			}

			// Execute command
			start := time.Now()
			result, execErr := shell.Execute(command, false)
			duration := time.Since(start).Milliseconds()

			var output string
			var exitCode int

			if result != nil {
				exitCode = result.ExitCode
				output = result.Stdout
				if result.Stderr != "" {
					if output != "" {
						output += "\n"
					}
					output += result.Stderr
				}

				// Truncate output for display
				displayOutput := truncateOutput(output, cfg.Chat.OutputMaxLines)
				if displayOutput != "" {
					fmt.Println()
					fmt.Println(ui.Dim("─── Output ───"))
					fmt.Print(displayOutput)
					if !strings.HasSuffix(displayOutput, "\n") {
						fmt.Println()
					}
					fmt.Println(ui.Dim("──────────────"))
				}

				if exitCode == 0 {
					fmt.Printf("%s (took %dms)\n", ui.Success("✓"), duration)
				} else {
					fmt.Printf("%s Exit code: %d\n", ui.Error("✗"), exitCode)
				}
			} else if execErr != nil {
				fmt.Printf("%s %s\n", ui.Error("✗"), execErr.Error())
				output = execErr.Error()
				exitCode = 1
			}

			// Save execution to session
			sessStore.AddExecutionMessage(sess.ID, input, command, output, exitCode, analysis.RiskLevel, duration, true, userTokens+assistantTokens)

			// Add execution result to context for AI to see
			execContext := fmt.Sprintf("[Command executed: %s]\n[Exit code: %d]\n[Output: %s]",
				command, exitCode, truncateOutput(output, 20))
			contextMsgs = append(contextMsgs, ai.Message{Role: "user", Content: execContext})

			// Update cwd in session if it might have changed
			if strings.HasPrefix(command, "cd ") {
				newCwd, _ := os.Getwd()
				sessStore.UpdateLastCwd(sess.ID, newCwd)
			}

			// Save to global history too
			if historyStore != nil {
				entry := &types.HistoryEntry{
					Prompt:       input,
					GeneratedCmd: command,
					RiskLevel:    analysis.RiskLevel,
					Executed:     true,
					ExitCode:     exitCode,
					DurationMs:   duration,
					WorkingDir:   cwd,
					Provider:     cfg.Provider.Name,
					Model:        cfg.Model.Name,
				}
				historyStore.AddCommand(entry)
			}
		}

		// Generate title after first exchange
		if isFirstExchange && cfg.Chat.GenerateTitles && sess.Name == "New Session" {
			go generateSessionTitle(aiProvider, sessStore, sess.ID, input, command)
			isFirstExchange = false
		}

		fmt.Println()
	}
}

func buildChatSystemPrompt(sysCtx types.SystemContext) string {
	return fmt.Sprintf(`You are an expert shell assistant running on %s with %s shell.
Current directory: %s
User: %s

Your role:
1. Convert natural language requests into shell commands
2. Explain what commands do when asked
3. Help troubleshoot command errors based on output
4. Suggest improvements or alternatives

When providing commands:
- Output the command on a single line, optionally prefixed with $ or >
- Provide brief explanation if helpful
- Consider the current directory and OS
- Use appropriate flags for the platform

Available tools: %s
Git: %s (%s)

Be concise. Focus on practical, working commands.`,
		sysCtx.OS,
		sysCtx.Shell,
		sysCtx.CurrentDir,
		sysCtx.Username,
		strings.Join(sysCtx.InstalledPkgMgrs, ", "),
		sysCtx.GitBranch,
		sysCtx.GitStatus,
	)
}

func buildChatContext(msgs []*types.SessionMessage, maxOutputLines int) []ai.Message {
	var context []ai.Message
	for _, msg := range msgs {
		if msg.Role == "execution" {
			// Format execution as user message showing what happened
			if msg.Executed {
				execMsg := fmt.Sprintf("I ran: %s\nExit code: %d", msg.Command, msg.ExitCode)
				if msg.Output != "" {
					execMsg += fmt.Sprintf("\nOutput:\n%s", truncateOutput(msg.Output, maxOutputLines))
				}
				context = append(context, ai.Message{Role: "user", Content: execMsg})
			} else {
				context = append(context, ai.Message{Role: "user", Content: msg.Content})
			}
		} else {
			context = append(context, ai.Message{Role: msg.Role, Content: msg.Content})
		}
	}
	return context
}

func extractCommand(response string) string {
	lines := strings.Split(response, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines and pure text
		if line == "" {
			continue
		}
		// Check for command indicators
		if strings.HasPrefix(line, "$ ") {
			return strings.TrimPrefix(line, "$ ")
		}
		if strings.HasPrefix(line, "> ") {
			return strings.TrimPrefix(line, "> ")
		}
		// Check for code block with bash/sh/zsh
		if strings.HasPrefix(line, "```") {
			continue
		}
		// Look for lines that look like commands (start with common command names)
		cmdPrefixes := []string{"ls", "cd", "cat", "grep", "find", "rm", "mv", "cp", "mkdir", "touch",
			"echo", "pwd", "chmod", "chown", "curl", "wget", "git", "docker", "npm", "yarn",
			"pip", "python", "go", "make", "brew", "apt", "yum", "sudo", "ssh", "scp", "tar",
			"zip", "unzip", "head", "tail", "sort", "wc", "awk", "sed", "xargs", "kill", "ps",
			"top", "df", "du", "mount", "umount", "ping", "netstat", "ifconfig", "ip", "./", "/"}
		for _, prefix := range cmdPrefixes {
			if strings.HasPrefix(line, prefix) {
				return line
			}
		}
	}
	return ""
}

func truncateOutput(output string, maxLines int) string {
	if output == "" || maxLines <= 0 {
		return output
	}
	lines := strings.Split(output, "\n")
	if len(lines) <= maxLines {
		return output
	}
	truncated := strings.Join(lines[:maxLines], "\n")
	return truncated + fmt.Sprintf("\n... (%d more lines)", len(lines)-maxLines)
}

func shortenPath(path string) string {
	home, _ := os.UserHomeDir()
	if strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

func printSessionHistory(msgs []*types.SessionMessage, limit int) {
	if len(msgs) == 0 {
		fmt.Println(ui.Dim("No messages yet"))
		return
	}

	fmt.Println()
	start := 0
	if len(msgs) > limit {
		start = len(msgs) - limit
		fmt.Printf(ui.Dim("... %d earlier messages ...\n"), start)
	}

	for _, msg := range msgs[start:] {
		switch msg.Role {
		case "user":
			fmt.Printf("%s %s\n", ui.Cyan("you>"), msg.Content)
		case "assistant":
			content := msg.Content
			if len(content) > 100 {
				content = content[:97] + "..."
			}
			fmt.Printf("%s %s\n", ui.Magenta("ai>"), content)
		case "execution":
			status := ui.Success("✓")
			if msg.ExitCode != 0 {
				status = ui.Error("✗")
			}
			if msg.Executed {
				fmt.Printf("%s %s\n", status, ui.Cyan(msg.Command))
			} else {
				fmt.Printf("%s %s %s\n", ui.Dim("⏸"), msg.Command, ui.Dim("(cancelled)"))
			}
		}
	}
	fmt.Println()
}

func printSessionInfo(sess *types.Session, store *session.Store) {
	sess, _ = store.GetSession(sess.ID)
	fmt.Println()
	fmt.Println(ui.Bold("📋 Session Info:"))
	fmt.Printf("   ID:       %s\n", sess.ID[:8])
	fmt.Printf("   Name:     %s\n", sess.Name)
	fmt.Printf("   Provider: %s\n", sess.Provider)
	fmt.Printf("   Model:    %s\n", sess.Model)
	fmt.Printf("   Commands: %d\n", sess.CommandCount)
	fmt.Printf("   Messages: %d\n", sess.MessageCount)
	fmt.Printf("   Tokens:   %d\n", sess.TotalTokens)
	fmt.Printf("   Created:  %s\n", sess.CreatedAt.Format("2006-01-02 15:04"))
	fmt.Printf("   Updated:  %s\n", sess.UpdatedAt.Format("2006-01-02 15:04"))
	fmt.Println()
}

func generateSessionTitle(provider ai.Provider, store *session.Store, sessID, userMsg, command string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	prompt := fmt.Sprintf(`Generate a short title (3-5 words) for this shell session. Respond with ONLY the title, no quotes.

User asked: %s
Command: %s`, truncate(userMsg, 100), truncate(command, 100))

	messages := []ai.Message{
		{Role: "system", Content: "Generate concise session titles."},
		{Role: "user", Content: prompt},
	}

	response, err := provider.Chat(ctx, messages)
	if err != nil {
		return
	}

	title := strings.TrimSpace(response)
	title = strings.Trim(title, "\"'")
	if len(title) > 50 {
		title = title[:47] + "..."
	}
	if title != "" {
		store.UpdateName(sessID, title)
	}
}

func printChatHelp() {
	fmt.Println(`
🖥️  Shell Chat Commands:
  /help, /h      Show this help
  /info          Show session info
  /history       Show session history
  /tokens        Show token usage
  /auto          Show auto-execute status
  /auto on       Enable auto-execute for safe commands
  /auto off      Disable auto-execute
  /pick          Switch to another session
  /new           Start a new session
  /clear         Clear screen
  /quit, /q      Exit

Just type your request in natural language to generate and run commands.
The AI sees command outputs and can help troubleshoot errors.`)
}

func showRecentHistory() {
	if historyStore == nil {
		ui.PrintWarning("History is not enabled")
		return
	}

	entries, err := historyStore.ListCommands(10, 0, "")
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to get history: %v", err))
		return
	}

	if len(entries) == 0 {
		ui.PrintInfo("No history entries")
		return
	}

	fmt.Println("\nRecent commands:")
	for _, entry := range entries {
		status := "⏸"
		if entry.Executed {
			if entry.ExitCode == 0 {
				status = "✓"
			} else {
				status = "✗"
			}
		}
		fmt.Printf("  %s %s: %s\n", status, entry.Timestamp.Format("15:04"), entry.GeneratedCmd)
	}
	fmt.Println()
}

// llmCmd returns the LLM client subcommand
func llmCmd() *cobra.Command {
	var systemPrompt string
	var continueConv string

	cmd := &cobra.Command{
		Use:   "llm [conversation-name]",
		Short: "Start LLM client mode (general-purpose chat)",
		Long: `Start an LLM client mode for general-purpose conversations.
Unlike 'sosomi chat', this mode is not focused on shell commands.

Examples:
  sosomi llm                           # Start new conversation
  sosomi llm "Python Help"             # Start with name
  sosomi llm -s "You are a poet"       # With system prompt
  sosomi llm -c abc123                 # Continue existing conversation
  sosomi llm list                      # List conversations
  sosomi llm delete <id>               # Delete conversation`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var convName string
			if len(args) > 0 {
				convName = args[0]
			}
			return runLLM(convName, systemPrompt, continueConv)
		},
	}

	cmd.Flags().StringVarP(&systemPrompt, "system", "s", "", "System prompt for the conversation")
	cmd.Flags().StringVarP(&continueConv, "continue", "c", "", "Continue an existing conversation (ID or name)")

	// Subcommands
	cmd.AddCommand(llmListCmd())
	cmd.AddCommand(llmPickCmd())
	cmd.AddCommand(llmDeleteCmd())
	cmd.AddCommand(llmExportCmd())
	cmd.AddCommand(llmImportCmd())
	cmd.AddCommand(llmStatsCmd())

	return cmd
}

func llmListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all conversations",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()
			store, err := conversation.NewStore(cfg.LLM.DBPath)
			if err != nil {
				return fmt.Errorf("failed to open conversation store: %w", err)
			}
			defer store.Close()

			convs, err := store.ListConversations(50, 0)
			if err != nil {
				return err
			}

			if len(convs) == 0 {
				fmt.Println("No conversations yet. Start one with: sosomi llm")
				return nil
			}

			fmt.Println("\n💬 Conversations:")
			fmt.Println(strings.Repeat("─", 70))
			for _, c := range convs {
				age := time.Since(c.UpdatedAt)
				ageStr := formatDuration(age)
				fmt.Printf("  %s  %-30s  %d msgs  %d tokens  %s ago\n",
					c.ID[:8], truncate(c.Name, 30), c.MessageCount, c.TotalTokens, ageStr)
			}
			fmt.Println()
			return nil
		},
	}
}

func llmDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id-or-name>",
		Short: "Delete a conversation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()
			store, err := conversation.NewStore(cfg.LLM.DBPath)
			if err != nil {
				return fmt.Errorf("failed to open conversation store: %w", err)
			}
			defer store.Close()

			// Try to find by ID first, then by name
			conv, err := store.GetConversation(args[0])
			if err != nil {
				conv, err = store.GetConversationByName(args[0])
				if err != nil {
					return fmt.Errorf("conversation not found: %s", args[0])
				}
			}

			if err := store.DeleteConversation(conv.ID); err != nil {
				return err
			}

			fmt.Printf("✓ Deleted conversation: %s\n", conv.Name)
			return nil
		},
	}
}

func llmExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export <id-or-name> [output-file]",
		Short: "Export a conversation to JSON",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()
			store, err := conversation.NewStore(cfg.LLM.DBPath)
			if err != nil {
				return fmt.Errorf("failed to open conversation store: %w", err)
			}
			defer store.Close()

			// Find conversation
			conv, err := store.GetConversation(args[0])
			if err != nil {
				conv, err = store.GetConversationByName(args[0])
				if err != nil {
					return fmt.Errorf("conversation not found: %s", args[0])
				}
			}

			export, err := store.ExportConversation(conv.ID)
			if err != nil {
				return err
			}

			data, err := json.MarshalIndent(export, "", "  ")
			if err != nil {
				return err
			}

			if len(args) > 1 {
				if err := os.WriteFile(args[1], data, 0644); err != nil {
					return err
				}
				fmt.Printf("✓ Exported to: %s\n", args[1])
			} else {
				fmt.Println(string(data))
			}
			return nil
		},
	}
}

func llmImportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import <file>",
		Short: "Import a conversation from JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()
			store, err := conversation.NewStore(cfg.LLM.DBPath)
			if err != nil {
				return fmt.Errorf("failed to open conversation store: %w", err)
			}
			defer store.Close()

			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}

			conv, err := store.ImportConversation(data)
			if err != nil {
				return err
			}

			fmt.Printf("✓ Imported conversation: %s (ID: %s)\n", conv.Name, conv.ID[:8])
			return nil
		},
	}
}

func llmStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show conversation statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()
			store, err := conversation.NewStore(cfg.LLM.DBPath)
			if err != nil {
				return fmt.Errorf("failed to open conversation store: %w", err)
			}
			defer store.Close()

			stats, err := store.GetStats()
			if err != nil {
				return err
			}

			fmt.Println("\n📊 LLM Client Statistics")
			fmt.Println(strings.Repeat("─", 40))
			fmt.Printf("  Conversations: %d\n", stats["total_conversations"])
			fmt.Printf("  Messages:      %d\n", stats["total_messages"])
			fmt.Printf("  Total Tokens:  %d\n", stats["total_tokens"])

			if byRole, ok := stats["by_role"].(map[string]int); ok && len(byRole) > 0 {
				fmt.Println("\n  By Role:")
				for role, count := range byRole {
					fmt.Printf("    %s: %d\n", role, count)
				}
			}

			if byProvider, ok := stats["by_provider"].(map[string]int); ok && len(byProvider) > 0 {
				fmt.Println("\n  By Provider:")
				for provider, count := range byProvider {
					fmt.Printf("    %s: %d\n", provider, count)
				}
			}
			fmt.Println()
			return nil
		},
	}
}

func llmPickCmd() *cobra.Command {
	var pageSize int

	cmd := &cobra.Command{
		Use:   "pick",
		Short: "Interactive conversation picker",
		Long: `Open an interactive UI to browse and select conversations.

Features:
  - Paginated list of conversations
  - Search by name or system prompt
  - Quick selection by number
  - Create new conversation

Controls:
  [1-9]  Select conversation by number
  [c]    Create new conversation
  [s]    Search conversations
  [n/p]  Next/Previous page
  [q]    Quit`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()
			store, err := conversation.NewStore(cfg.LLM.DBPath)
			if err != nil {
				return fmt.Errorf("failed to open conversation store: %w", err)
			}
			defer store.Close()

			convs, err := store.ListConversations(100, 0)
			if err != nil {
				return err
			}

			selected, isNew, err := ui.ConversationPicker(convs, pageSize)
			if err != nil {
				return err
			}

			if selected == nil && !isNew {
				// User quit
				return nil
			}

			if isNew {
				return runLLM("", "", "")
			}

			return runLLM("", "", selected.ID)
		},
	}

	cmd.Flags().IntVarP(&pageSize, "page-size", "p", 10, "Number of conversations per page")

	return cmd
}

func runLLM(convName, systemPrompt, continueID string) error {
	cfg := config.Get()

	// Ensure data directory exists
	if err := os.MkdirAll(filepath.Dir(cfg.LLM.DBPath), 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Open conversation store
	store, err := conversation.NewStore(cfg.LLM.DBPath)
	if err != nil {
		return fmt.Errorf("failed to open conversation store: %w", err)
	}
	defer store.Close()

	// Create AI provider
	provider, err := ai.NewProviderFromConfig()
	if err != nil {
		return fmt.Errorf("failed to create AI provider: %w", err)
	}

	// Initialize MCP manager if enabled
	var mcpManager *mcp.Manager
	if cfg.MCP.Enabled {
		mcpManager = mcp.NewManager()
		defer mcpManager.Shutdown()
	}

	// Load or create conversation
	var conv *types.Conversation
	var messages []ai.Message
	var storedMsgs []*types.ConversationMessage

	if continueID != "" {
		// Continue existing conversation
		conv, err = store.GetConversation(continueID)
		if err != nil {
			conv, err = store.GetConversationByName(continueID)
			if err != nil {
				return fmt.Errorf("conversation not found: %s", continueID)
			}
		}
		// Load existing messages
		storedMsgs, err = store.GetMessages(conv.ID)
		if err != nil {
			return err
		}
		for _, msg := range storedMsgs {
			messages = append(messages, ai.Message{Role: msg.Role, Content: msg.Content})
		}
		fmt.Printf("📝 Continuing conversation: %s\n", conv.Name)
		if conv.SystemPrompt != "" {
			fmt.Printf("📋 System: %s\n", ui.Dim(truncate(conv.SystemPrompt, 60)))
		}
		// Display conversation history
		printConversationHistory(storedMsgs)
	} else {
		// Create new conversation
		if convName == "" {
			convName = "New Conversation"
		}
		if systemPrompt == "" {
			systemPrompt = cfg.LLM.DefaultSystemPrompt
		}
		conv, err = store.CreateConversation(convName, systemPrompt, cfg.Provider.Name, cfg.Model.Name)
		if err != nil {
			return fmt.Errorf("failed to create conversation: %w", err)
		}
		if systemPrompt != "" {
			messages = append(messages, ai.Message{Role: "system", Content: systemPrompt})
		}
		fmt.Printf("📝 New conversation: %s\n", conv.Name)
	}

	fmt.Printf("🤖 Model: %s (%s)\n", cfg.Model.Name, cfg.Provider.Name)
	fmt.Println("Type /help for commands, /quit to exit")
	fmt.Println()

	// Setup liner for readline with history
	line := liner.NewLiner()
	defer line.Close()

	line.SetCtrlCAborts(true)

	// Load history from conversation messages
	if len(storedMsgs) > 0 {
		for _, msg := range storedMsgs {
			if msg.Role == "user" && msg.Content != "" && !strings.HasPrefix(msg.Content, "/") {
				line.AppendHistory(msg.Content)
			}
		}
	}

	isFirstExchange := conv.MessageCount <= 1 // Only system prompt or empty

	for {
		input, err := line.Prompt("you> ")
		if err != nil {
			// Ctrl+C or Ctrl+D
			fmt.Println("\nGoodbye! 👋")
			return nil
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Add to history
		line.AppendHistory(input)

		// Handle special commands
		switch {
		case input == "/quit" || input == "/exit" || input == "/q":
			fmt.Println("Goodbye! 👋")
			return nil
		case input == "/help" || input == "/h":
			printLLMHelp()
			continue
		case input == "/info":
			printConvInfo(conv, store)
			continue
		case input == "/clear":
			// Clear screen
			fmt.Print("\033[2J\033[H")
			continue
		case input == "/tokens":
			conv, _ = store.GetConversation(conv.ID)
			fmt.Printf("💰 Tokens used: %d\n", conv.TotalTokens)
			continue
		case input == "/history":
			storedMsgs, _ := store.GetMessages(conv.ID)
			printConversationHistory(storedMsgs)
			continue
		case input == "/system":
			conv, _ = store.GetConversation(conv.ID)
			fmt.Printf("\n📋 Current system prompt:\n%s\n\n", ui.Dim(conv.SystemPrompt))
			newPrompt, err := line.Prompt("Enter new system prompt (empty to keep, 'clear' to remove): ")
			if err != nil {
				continue
			}
			newPrompt = strings.TrimSpace(newPrompt)
			if newPrompt == "clear" {
				store.UpdateSystemPrompt(conv.ID, "")
				// Update messages context - remove old system message
				if len(messages) > 0 && messages[0].Role == "system" {
					messages = messages[1:]
				}
				fmt.Println(ui.Success("✓"), "System prompt cleared")
			} else if newPrompt != "" {
				store.UpdateSystemPrompt(conv.ID, newPrompt)
				// Update messages context
				if len(messages) > 0 && messages[0].Role == "system" {
					messages[0].Content = newPrompt
				} else {
					messages = append([]ai.Message{{Role: "system", Content: newPrompt}}, messages...)
				}
				fmt.Println(ui.Success("✓"), "System prompt updated")
			}
			continue
		case strings.HasPrefix(input, "/"):
			fmt.Printf("Unknown command: %s. Type /help for available commands.\n", input)
			continue
		}

		// Add user message
		messages = append(messages, ai.Message{Role: "user", Content: input})

		// Stream the response
		fmt.Print("assistant> ")
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Model.TimeoutSeconds)*time.Second)

		stream, err := provider.ChatStream(ctx, messages)
		if err != nil {
			cancel()
			ui.PrintError(err.Error())
			// Remove failed message from context
			messages = messages[:len(messages)-1]
			continue
		}

		var response strings.Builder
		var tokenUsage *ai.TokenUsage

		for chunk := range stream {
			if chunk.Error != nil {
				cancel()
				fmt.Println()
				ui.PrintError(chunk.Error.Error())
				// Remove failed message from context
				messages = messages[:len(messages)-1]
				break
			}
			if chunk.Done {
				tokenUsage = chunk.Usage
				break
			}
			fmt.Print(chunk.Content)
			response.WriteString(chunk.Content)
		}
		fmt.Println()
		cancel()

		if response.Len() == 0 {
			continue
		}

		// Add assistant response to context
		assistantContent := response.String()
		messages = append(messages, ai.Message{Role: "assistant", Content: assistantContent})

		// Save messages to store
		userTokens := 0
		assistantTokens := 0
		if tokenUsage != nil {
			// Rough split: estimate user tokens as prompt tokens minus previous context
			assistantTokens = tokenUsage.CompletionTokens
			userTokens = tokenUsage.PromptTokens / max(1, len(messages)-1)
		}
		store.AddMessage(conv.ID, "user", input, userTokens)
		store.AddMessage(conv.ID, "assistant", assistantContent, assistantTokens)

		// Generate title after first exchange if enabled
		if isFirstExchange && cfg.LLM.GenerateTitles && conv.Name == "New Conversation" {
			go generateConversationTitle(provider, store, conv.ID, input, assistantContent)
			isFirstExchange = false
		}

		fmt.Println()
	}
}

func generateConversationTitle(provider ai.Provider, store *conversation.Store, convID, userMsg, assistantMsg string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	prompt := fmt.Sprintf(`Generate a short title (3-5 words) for this conversation. Respond with ONLY the title, no quotes or punctuation.

User: %s
Assistant: %s`, truncate(userMsg, 200), truncate(assistantMsg, 200))

	messages := []ai.Message{
		{Role: "system", Content: "You are a helpful assistant that generates concise conversation titles."},
		{Role: "user", Content: prompt},
	}

	response, err := provider.Chat(ctx, messages)
	if err != nil {
		return // Silently fail - title generation is not critical
	}

	title := strings.TrimSpace(response)
	title = strings.Trim(title, "\"'")
	if len(title) > 50 {
		title = title[:47] + "..."
	}
	if title != "" {
		store.UpdateName(convID, title)
	}
}

func printLLMHelp() {
	fmt.Println(`
💬 LLM Client Mode Commands:
  /help, /h      Show this help
  /info          Show conversation info
  /history       Show conversation history
  /system        View/edit system prompt
  /tokens        Show token usage
  /clear         Clear screen
  /quit, /q      Exit

Just type your message to chat with the AI.`)
}

// printConversationHistory displays the conversation history
func printConversationHistory(messages []*types.ConversationMessage) {
	if len(messages) == 0 {
		fmt.Println(ui.Dim("\n  No messages yet.\n"))
		return
	}

	fmt.Println()
	fmt.Println(ui.Bold("📜 Conversation History:"))
	fmt.Println(strings.Repeat("─", 60))

	for _, msg := range messages {
		if msg.Role == "system" {
			continue // Skip system messages in history display
		}

		var prefix string
		var content string
		maxLen := 200 // Truncate long messages

		switch msg.Role {
		case "user":
			prefix = ui.Cyan("you> ")
			content = msg.Content
		case "assistant":
			prefix = ui.Magenta("assistant> ")
			content = msg.Content
		default:
			prefix = ui.Dim(msg.Role + "> ")
			content = msg.Content
		}

		if len(content) > maxLen {
			content = content[:maxLen-3] + "..."
		}

		// Handle multi-line content
		lines := strings.Split(content, "\n")
		if len(lines) > 3 {
			content = strings.Join(lines[:3], "\n") + "\n..."
		}

		fmt.Printf("%s%s\n", prefix, content)
	}

	fmt.Println(strings.Repeat("─", 60))
	fmt.Println()
}

func printConvInfo(conv *types.Conversation, store *conversation.Store) {
	// Refresh conv from store
	conv, _ = store.GetConversation(conv.ID)
	fmt.Printf("\n📝 Conversation Info:\n")
	fmt.Printf("  ID:        %s\n", conv.ID)
	fmt.Printf("  Name:      %s\n", conv.Name)
	fmt.Printf("  Provider:  %s\n", conv.Provider)
	fmt.Printf("  Model:     %s\n", conv.Model)
	fmt.Printf("  Messages:  %d\n", conv.MessageCount)
	fmt.Printf("  Tokens:    %d\n", conv.TotalTokens)
	fmt.Printf("  Created:   %s\n", conv.CreatedAt.Format("2006-01-02 15:04"))
	fmt.Printf("  Updated:   %s\n", conv.UpdatedAt.Format("2006-01-02 15:04"))
	if conv.SystemPrompt != "" {
		fmt.Printf("\n  📋 System Prompt:\n")
		// Display full system prompt with indentation
		lines := strings.Split(conv.SystemPrompt, "\n")
		for _, line := range lines {
			fmt.Printf("     %s\n", ui.Dim(line))
		}
	}
	fmt.Println()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

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
			fmt.Printf("Backup Enabled: %v\n", cfg.Backup.Enabled)
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

// runEditor opens a file in the user's editor
func runEditor(editor, file string) error {
	cmd := exec.Command(editor, file)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

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

// undoCmd returns the undo subcommand
func undoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "undo [backup-id]",
		Short: "Undo the last command or a specific backup",
		RunE: func(cmd *cobra.Command, args []string) error {
			if backupManager == nil {
				return fmt.Errorf("backup is not enabled")
			}

			var backup *types.BackupEntry
			var err error

			if len(args) > 0 {
				backup, err = backupManager.GetBackup(args[0])
			} else {
				backup, err = backupManager.GetLastBackup()
			}

			if err != nil {
				return fmt.Errorf("no backup found: %w", err)
			}

			fmt.Printf("Restore backup from: %s\n", backup.Timestamp.Format("2006-01-02 15:04:05"))
			fmt.Printf("Command: %s\n", backup.Command)
			fmt.Printf("Files: %d\n", len(backup.Files))

			ui.PrintSimpleConfirm("Restore these files?")

			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))

			if input != "y" && input != "yes" {
				ui.PrintInfo("Restore cancelled")
				return nil
			}

			if err := backupManager.Restore(backup.ID); err != nil {
				return fmt.Errorf("restore failed: %w", err)
			}

			ui.PrintSuccess("Files restored successfully!")
			return nil
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List available backups",
		RunE: func(cmd *cobra.Command, args []string) error {
			if backupManager == nil {
				return fmt.Errorf("backup is not enabled")
			}

			backups, err := backupManager.ListBackups()
			if err != nil {
				return err
			}

			if len(backups) == 0 {
				ui.PrintInfo("No backups available")
				return nil
			}

			for _, b := range backups {
				restored := ""
				if b.Restored {
					restored = " (restored)"
				}
				fmt.Printf("%s  %s  %d files  %s%s\n",
					b.ID[:8],
					b.Timestamp.Format("2006-01-02 15:04"),
					len(b.Files),
					formatSize(b.TotalSize),
					restored,
				)
				fmt.Printf("  └─ %s\n\n", b.Command)
			}
			return nil
		},
	})

	return cmd
}

// modelsCmd returns the models subcommand
func modelsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "List available models",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			aiProvider, err := ai.NewProviderFromConfig()
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
				ui.PrintInfo("Cancelled")
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

// askCmd returns the intelligent help command
func askCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ask [question]",
		Short: "Ask AI for help with sosomi commands",
		Long: `Ask the AI assistant for help with sosomi commands, options, and usage.

The AI will provide ready-to-use commands based on your question.

Examples:
  sosomi ask "how do I continue an existing conversation?"
  sosomi ask "what providers are supported?"
  sosomi ask "how to undo the last dangerous command?"
  sosomi ask "show me how to use local models with ollama"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			question := strings.Join(args, " ")
			return runAskCommand(question)
		},
	}
}

const sosomiHelpContext = `You are an expert assistant for Sosomi, a CLI tool that converts natural language to shell commands with safety features.

## SOSOMI COMMANDS REFERENCE

### Main Command
sosomi [prompt] - Convert natural language to shell command
Flags:
  -a, --auto        Auto-execute safe commands
  -d, --dry-run     Simulate without executing
  -e, --explain     Show explanation only  
  -s, --silent      Minimal output
  -p, --profile     Use specific profile
  --force           Override safety blocks (dangerous!)

### Subcommands

#### sosomi chat
Interactive shell command assistant (focused on generating shell commands)

#### sosomi llm [name]
General-purpose LLM chat client (not shell-focused)
Flags:
  -s, --system      Set system prompt
  -c, --continue    Continue existing conversation (by ID or name)

LLM Subcommands:
  sosomi llm list          List all conversations
  sosomi llm pick          Interactive conversation picker UI
  sosomi llm delete <id>   Delete a conversation
  sosomi llm export <id>   Export conversation to JSON
  sosomi llm import <file> Import conversation from JSON
  sosomi llm stats         Show usage statistics

In-conversation commands:
  /help      Show help
  /info      Show conversation info
  /history   Show conversation history
  /system    View/edit system prompt
  /tokens    Show token usage
  /clear     Clear screen
  /quit      Exit

#### sosomi config
  sosomi config show           Show current configuration
  sosomi config set <k> <v>    Set a config value

#### sosomi history
  sosomi history               Show recent command history
  sosomi history stats         Show history statistics
  sosomi history search <q>    Search history

#### sosomi undo
  sosomi undo                  Undo last risky operation
  sosomi undo list             List available backups
  sosomi undo <id>             Restore specific backup

#### sosomi models
  sosomi models                List available models for current provider

#### sosomi profile
  sosomi profile list          List profiles
  sosomi profile create        Create new profile interactively
  sosomi profile use <n>       Set profile as default
  sosomi profile show <n>      Show profile details
  sosomi profile test <n>      Test profile connectivity
  sosomi profile delete <n>    Delete profile
  sosomi profile export <n>    Export profile (without secrets)
  sosomi profile import <f>    Import profile from file

#### sosomi init
Interactive setup wizard for first-time configuration

#### sosomi ask [question]
This command - get AI help about sosomi usage

## PROVIDERS
- openai: OpenAI API (GPT-4, GPT-3.5, etc.)
- ollama: Local Ollama server
- lmstudio: LM Studio local server
- llamacpp: llama.cpp server
- generic: Any OpenAI-compatible endpoint

## CONFIGURATION
Config file: ~/.config/sosomi/config.yaml
Environment variables:
  OPENAI_API_KEY or SOSOMI_API_KEY: API key
  SOSOMI_PROVIDER: Default provider
  SOSOMI_MODEL: Default model

## RISK LEVELS
- SAFE: Read-only operations
- CAUTION: Modifying operations
- DANGEROUS: High-risk, backup created
- CRITICAL: Blocked by default

## COMMON TASKS

### Using local models
sosomi -p ollama "your prompt"
sosomi config set provider ollama
sosomi config set model llama3.2

### Conversation management
sosomi llm                    # New conversation
sosomi llm "Topic Name"       # New with name
sosomi llm -c "Topic Name"    # Continue by name
sosomi llm pick               # Interactive picker

### Safety features
sosomi "command" --dry-run    # Preview without executing
sosomi undo                   # Undo last risky operation
sosomi undo list              # See available backups

---

IMPORTANT: This is a CLI tool, so your response will be displayed in a terminal.
- Do NOT use markdown code blocks (no triple backticks)
- Just show commands directly, indented with 2 spaces
- Use plain text formatting only
- Keep answers concise and scannable

Based on this documentation, answer the user's question. Provide ready-to-use commands when applicable.`

func runAskCommand(question string) error {
	// Try to create AI provider
	provider, err := ai.NewProviderFromConfig()
	if err != nil {
		// Fallback to showing static help if no provider configured
		fmt.Println(ui.Warning("⚠"), "AI provider not configured. Showing basic help.")
		fmt.Println()
		fmt.Println("To enable intelligent help, configure an AI provider:")
		fmt.Println("  sosomi init                    # Interactive setup")
		fmt.Println("  export OPENAI_API_KEY=...      # Or set API key")
		fmt.Println()
		fmt.Println("Available commands:")
		fmt.Println("  sosomi [prompt]     Generate shell command")
		fmt.Println("  sosomi chat         Interactive shell assistant")
		fmt.Println("  sosomi llm          General LLM chat client")
		fmt.Println("  sosomi llm pick     Interactive conversation picker")
		fmt.Println("  sosomi config       Manage configuration")
		fmt.Println("  sosomi history      View command history")
		fmt.Println("  sosomi undo         Undo risky operations")
		fmt.Println("  sosomi models       List available models")
		fmt.Println("  sosomi profile      Manage profiles")
		fmt.Println("  sosomi init         Setup wizard")
		fmt.Println()
		fmt.Println("Use --help on any command for more details.")
		return nil
	}

	cfg := config.Get()

	fmt.Printf("%s Thinking...\n", ui.Cyan("🤔"))

	messages := []ai.Message{
		{Role: "system", Content: sosomiHelpContext},
		{Role: "user", Content: question},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Model.TimeoutSeconds)*time.Second)
	defer cancel()

	// Use streaming for better UX
	stream, err := provider.ChatStream(ctx, messages)
	if err != nil {
		return fmt.Errorf("failed to get response: %w", err)
	}

	fmt.Println()
	fmt.Println(ui.Bold("💡 Answer:"))
	fmt.Println(strings.Repeat("─", 60))

	for chunk := range stream {
		if chunk.Error != nil {
			return chunk.Error
		}
		if chunk.Done {
			break
		}
		fmt.Print(chunk.Content)
	}

	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println()

	return nil
}
