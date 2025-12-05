// Package ui provides terminal user interface components
package ui

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/soroush/sosomi/internal/types"
)

var (
	// Colors
	Green   = color.New(color.FgGreen).SprintFunc()
	Yellow  = color.New(color.FgYellow).SprintFunc()
	Red     = color.New(color.FgRed).SprintFunc()
	Cyan    = color.New(color.FgCyan).SprintFunc()
	Magenta = color.New(color.FgMagenta).SprintFunc()
	Bold    = color.New(color.Bold).SprintFunc()
	Dim     = color.New(color.Faint).SprintFunc()

	// Styled
	Success = color.New(color.FgGreen, color.Bold).SprintFunc()
	Warning = color.New(color.FgYellow, color.Bold).SprintFunc()
	Error   = color.New(color.FgRed, color.Bold).SprintFunc()
	Info    = color.New(color.FgCyan, color.Bold).SprintFunc()
)

// Box characters
const (
	BoxTopLeft     = "┌"
	BoxTopRight    = "┐"
	BoxBottomLeft  = "└"
	BoxBottomRight = "┘"
	BoxHorizontal  = "─"
	BoxVertical    = "│"
	BoxTeeRight    = "├"
	BoxTeeLeft     = "┤"
)

// PrintCommand displays the generated command
func PrintCommand(cmd string) {
	fmt.Println()
	fmt.Printf("✨ %s\n", Bold("Generated command:"))
	fmt.Printf("   %s\n", Cyan(cmd))
}

// PrintExplanation displays the command explanation
func PrintExplanation(explanation string) {
	if explanation == "" {
		return
	}
	fmt.Println()
	fmt.Printf("📋 %s\n", Bold("Explanation:"))
	for _, line := range strings.Split(explanation, "\n") {
		fmt.Printf("   %s\n", line)
	}
}

// PrintRiskLevel displays the risk level with appropriate styling
func PrintRiskLevel(level types.RiskLevel, reasons []string) {
	fmt.Println()
	
	var icon, levelStr string
	var colorFn func(a ...interface{}) string

	switch level {
	case types.RiskSafe:
		icon = "🟢"
		levelStr = "SAFE"
		colorFn = Green
	case types.RiskCaution:
		icon = "🟡"
		levelStr = "CAUTION"
		colorFn = Yellow
	case types.RiskDangerous:
		icon = "🟠"
		levelStr = "DANGEROUS"
		colorFn = func(a ...interface{}) string {
			return color.New(color.FgHiRed).Sprint(a...)
		}
	case types.RiskCritical:
		icon = "🔴"
		levelStr = "CRITICAL"
		colorFn = Red
	}

	fmt.Printf("%s %s: %s\n", icon, Bold("Risk Level"), colorFn(levelStr))

	if len(reasons) > 0 {
		for _, reason := range reasons {
			fmt.Printf("   • %s\n", reason)
		}
	}
}

// PrintAnalysis displays the full command analysis
func PrintAnalysis(analysis *types.CommandAnalysis) {
	width := 60

	// Top border
	fmt.Println()
	fmt.Print(BoxTopLeft)
	fmt.Print(strings.Repeat(BoxHorizontal, width))
	fmt.Println(BoxTopRight)

	// Title
	title := "  🔍 Command Analysis"
	fmt.Printf("%s%s%s%s\n", BoxVertical, title, strings.Repeat(" ", width-len(title)+2), BoxVertical)

	// Separator
	fmt.Print(BoxTeeRight)
	fmt.Print(strings.Repeat(BoxHorizontal, width))
	fmt.Println(BoxTeeLeft)

	// Command
	cmdLine := fmt.Sprintf("  Command:     %s", truncate(analysis.Command, 40))
	fmt.Printf("%s%s%s%s\n", BoxVertical, cmdLine, strings.Repeat(" ", width-len(cmdLine)+2), BoxVertical)

	// Risk Level
	riskColor := getRiskColor(analysis.RiskLevel)
	riskLine := fmt.Sprintf("  Risk Level:  %s %s", analysis.RiskLevel.Emoji(), riskColor(analysis.RiskLevel.String()))
	// Account for ANSI codes in length calculation
	padding := width - 26 + 2
	fmt.Printf("%s%s%s%s\n", BoxVertical, riskLine, strings.Repeat(" ", padding), BoxVertical)

	// Empty line
	fmt.Printf("%s%s%s\n", BoxVertical, strings.Repeat(" ", width), BoxVertical)

	// Affected files
	if len(analysis.AffectedPaths) > 0 {
		fmt.Printf("%s  📁 Affected Files:%s%s\n", BoxVertical, strings.Repeat(" ", width-19), BoxVertical)
		for _, path := range analysis.AffectedPaths {
			pathLine := fmt.Sprintf("     • %s", truncate(path, 50))
			fmt.Printf("%s%s%s%s\n", BoxVertical, pathLine, strings.Repeat(" ", width-len(pathLine)+2), BoxVertical)
		}
		fmt.Printf("%s%s%s\n", BoxVertical, strings.Repeat(" ", width), BoxVertical)
	}

	// Actions
	if len(analysis.Actions) > 0 {
		fmt.Printf("%s  ⚡ Actions:%s%s\n", BoxVertical, strings.Repeat(" ", width-12), BoxVertical)
		for _, action := range analysis.Actions {
			actionLine := fmt.Sprintf("     • %s", action)
			fmt.Printf("%s%s%s%s\n", BoxVertical, actionLine, strings.Repeat(" ", width-len(actionLine)+2), BoxVertical)
		}
		fmt.Printf("%s%s%s\n", BoxVertical, strings.Repeat(" ", width), BoxVertical)
	}

	// Reversible
	reversibleIcon := "✅"
	reversibleText := "Yes (backup will be created)"
	if !analysis.Reversible {
		reversibleIcon = "❌"
		reversibleText = "No (cannot be undone)"
	}
	reverseLine := fmt.Sprintf("  ↩️  Reversible: %s %s", reversibleIcon, reversibleText)
	fmt.Printf("%s%s%s%s\n", BoxVertical, reverseLine, strings.Repeat(" ", width-len(reverseLine)+4), BoxVertical)

	// Bottom border
	fmt.Print(BoxBottomLeft)
	fmt.Print(strings.Repeat(BoxHorizontal, width))
	fmt.Println(BoxBottomRight)
}

// PrintConfirmPrompt displays the confirmation prompt
func PrintConfirmPrompt() {
	fmt.Println()
	fmt.Println("  [y] Execute  [n] Cancel  [d] Dry-run  [e] Explain  [m] Modify")
	fmt.Print("\n  Choice: ")
}

// PrintRetryPrompt displays the post-execution retry prompt
func PrintRetryPrompt() {
	fmt.Println()
	fmt.Println("  [r] Retry with feedback  [n] Done")
	fmt.Print("\n  Choice: ")
}

// PrintFeedbackPrompt asks for user feedback to refine the command
func PrintFeedbackPrompt() {
	fmt.Print("\n  💬 What was wrong? (describe the issue): ")
}

// PrintSimpleConfirm displays a simple yes/no prompt
func PrintSimpleConfirm(message string) {
	fmt.Printf("\n%s [y/N]: ", message)
}

// PrintSuccess displays a success message
func PrintSuccess(message string) {
	fmt.Printf("\n%s %s\n", Success("✓"), message)
}

// PrintError displays an error message
func PrintError(message string) {
	fmt.Printf("\n%s %s\n", Error("✗"), message)
}

// PrintWarning displays a warning message
func PrintWarning(message string) {
	fmt.Printf("\n%s %s\n", Warning("⚠"), message)
}

// PrintInfo displays an info message
func PrintInfo(message string) {
	fmt.Printf("\n%s %s\n", Info("ℹ"), message)
}

// PrintExecutionResult displays the result of command execution
func PrintExecutionResult(stdout, stderr string, exitCode int, durationMs int64) {
	fmt.Println()
	
	if stdout != "" {
		fmt.Println(stdout)
	}

	if stderr != "" {
		fmt.Println(Yellow(stderr))
	}

	fmt.Println()
	if exitCode == 0 {
		fmt.Printf("%s Command completed successfully (%.2fs)\n", Success("✓"), float64(durationMs)/1000)
	} else {
		fmt.Printf("%s Command failed with exit code %d (%.2fs)\n", Error("✗"), exitCode, float64(durationMs)/1000)
	}
}

// PrintBackupInfo displays backup information
func PrintBackupInfo(backup *types.BackupEntry) {
	fmt.Println()
	fmt.Printf("📦 %s\n", Bold("Backup Created"))
	fmt.Printf("   ID: %s\n", Dim(backup.ID[:8]+"..."))
	fmt.Printf("   Files: %d\n", len(backup.Files))
	fmt.Printf("   Size: %s\n", formatSize(backup.TotalSize))
	fmt.Printf("   Use '%s' to undo\n", Cyan("sosomi undo"))
}

// PrintHeader displays the sosomi header
func PrintHeader() {
	fmt.Println()
	fmt.Println(Magenta("  🐚 Sosomi - Safe AI Shell Assistant"))
	fmt.Println(Dim("  Type your request in natural language"))
	fmt.Println()
}

// Spinner represents a loading spinner
type Spinner struct {
	frames  []string
	current int
	message string
}

// NewSpinner creates a new spinner
func NewSpinner(message string) *Spinner {
	return &Spinner{
		frames:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		message: message,
	}
}

// Frame returns the next spinner frame
func (s *Spinner) Frame() string {
	frame := s.frames[s.current]
	s.current = (s.current + 1) % len(s.frames)
	return fmt.Sprintf("\r%s %s", Cyan(frame), s.message)
}

// Helper functions

func getRiskColor(level types.RiskLevel) func(a ...interface{}) string {
	switch level {
	case types.RiskSafe:
		return Green
	case types.RiskCaution:
		return Yellow
	case types.RiskDangerous:
		return func(a ...interface{}) string {
			return color.New(color.FgHiRed).Sprint(a...)
		}
	case types.RiskCritical:
		return Red
	default:
		return fmt.Sprint
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
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

// ConversationPicker displays an interactive conversation picker
// Returns the selected conversation, whether user chose to create new, and any error
func ConversationPicker(conversations []*types.Conversation, pageSize int) (*types.Conversation, bool, error) {
	if pageSize <= 0 {
		pageSize = 10
	}

	reader := bufio.NewReader(os.Stdin)
	page := 0
	totalPages := (len(conversations) + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	for {
		// Clear screen
		fmt.Print("\033[2J\033[H")

		// Header
		fmt.Println()
		fmt.Println(Magenta("  💬 Conversation Picker"))
		fmt.Println(Dim("  Select a conversation to continue or start a new one"))
		fmt.Println()

		// Calculate page bounds
		start := page * pageSize
		end := start + pageSize
		if end > len(conversations) {
			end = len(conversations)
		}

		// Display conversations for current page
		fmt.Println(strings.Repeat("─", 78))
		fmt.Printf("  %s  %-8s  %-32s  %-8s  %-8s  %s\n",
			Dim("#"), Dim("ID"), Dim("Name"), Dim("Msgs"), Dim("Tokens"), Dim("Updated"))
		fmt.Println(strings.Repeat("─", 78))

		if len(conversations) == 0 {
			fmt.Println(Dim("  No conversations yet."))
		} else {
			for i := start; i < end; i++ {
				c := conversations[i]
				num := i - start + 1
				age := FormatDurationShort(time.Since(c.UpdatedAt))
				name := truncate(c.Name, 30)

				fmt.Printf("  %s  %s  %-32s  %-8d  %-8d  %s\n",
					Cyan(fmt.Sprintf("%-2d", num)),
					Dim(c.ID[:8]),
					name,
					c.MessageCount,
					c.TotalTokens,
					Dim(age))
			}
		}

		fmt.Println(strings.Repeat("─", 78))

		// Pagination info
		if totalPages > 1 {
			fmt.Printf("  Page %d/%d", page+1, totalPages)
			if page > 0 {
				fmt.Print("  [p] Previous")
			}
			if page < totalPages-1 {
				fmt.Print("  [n] Next")
			}
			fmt.Println()
		}

		// Options
		fmt.Println()
		fmt.Println("  [1-9] Select  [c] Create new  [s] Search  [q] Quit")
		fmt.Print("\n  Choice: ")

		input, err := reader.ReadString('\n')
		if err != nil {
			return nil, false, err
		}
		input = strings.TrimSpace(strings.ToLower(input))

		switch {
		case input == "q" || input == "quit":
			return nil, false, nil
		case input == "c" || input == "create" || input == "new":
			return nil, true, nil
		case input == "p" || input == "prev":
			if page > 0 {
				page--
			}
		case input == "n" || input == "next":
			if page < totalPages-1 {
				page++
			}
		case input == "s" || input == "search":
			fmt.Print("  🔍 Search: ")
			query, _ := reader.ReadString('\n')
			query = strings.TrimSpace(query)
			if query != "" {
				filtered := FilterConversations(conversations, query)
				if len(filtered) > 0 {
					result, isNew, err := ConversationPicker(filtered, pageSize)
					if result != nil || isNew || err != nil {
						return result, isNew, err
					}
				} else {
					fmt.Println(Yellow("  No matches found. Press Enter to continue..."))
					reader.ReadString('\n')
				}
			}
		default:
			// Try to parse as number
			if num, err := strconv.Atoi(input); err == nil && num >= 1 && num <= pageSize {
				idx := start + num - 1
				if idx < len(conversations) {
					return conversations[idx], false, nil
				}
			}
		}
	}
}

// FilterConversations filters conversations by name or system prompt (case-insensitive)
func FilterConversations(conversations []*types.Conversation, query string) []*types.Conversation {
	query = strings.ToLower(query)
	var result []*types.Conversation
	for _, c := range conversations {
		if strings.Contains(strings.ToLower(c.Name), query) ||
			strings.Contains(strings.ToLower(c.SystemPrompt), query) {
			result = append(result, c)
		}
	}
	return result
}

// FormatDurationShort formats a duration in a short human-readable format
func FormatDurationShort(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days < 30 {
		return fmt.Sprintf("%dd ago", days)
	}
	return fmt.Sprintf("%dmo ago", days/30)
}
