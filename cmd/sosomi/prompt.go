// Prompt processing logic for sosomi CLI
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sonemaro/sosomi/internal/ai"
	"github.com/sonemaro/sosomi/internal/config"
	"github.com/sonemaro/sosomi/internal/safety"
	"github.com/sonemaro/sosomi/internal/shell"
	"github.com/sonemaro/sosomi/internal/types"
	"github.com/sonemaro/sosomi/internal/ui"
)

// processPrompt handles a single natural language prompt
func processPrompt(prompt string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.Get().Model.TimeoutSeconds)*time.Second)
	defer cancel()

	// Get system context
	sysCtx := shell.GetSystemContext()

	// Create AI provider
	aiProvider, err := getAIProvider()
	if err != nil {
		return err
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
	if analysis.RiskLevel == types.RiskCritical {
		ui.PrintError("This command is blocked due to critical risk level")
		return nil
	}

	// Handle execution mode
	if explainOnly {
		return nil
	}

	if dryRun {
		return executeDryRun(response.Command, analysis)
	}

	// Auto-execute safe commands if enabled
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

// interactiveConfirm prompts the user to confirm, modify, or explain the command
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
			ui.PrintInfo("Command canceled")
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

// executeCommand runs the command and logs to history
func executeCommand(command, prompt string, analysis *types.CommandAnalysis) error {
	cfg := config.Get()

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
	aiProvider, err := getAIProvider()
	if err != nil {
		return err
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

// executeDryRun shows what would happen without executing
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
