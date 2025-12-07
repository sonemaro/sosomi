// Chat command and related functionality for sosomi CLI
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/peterh/liner"
	"github.com/spf13/cobra"

	"github.com/soroush/sosomi/internal/ai"
	"github.com/soroush/sosomi/internal/config"
	"github.com/soroush/sosomi/internal/safety"
	"github.com/soroush/sosomi/internal/session"
	"github.com/soroush/sosomi/internal/shell"
	"github.com/soroush/sosomi/internal/types"
	"github.com/soroush/sosomi/internal/ui"
)

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
	aiProvider, err := getAIProvider()
	if err != nil {
		return err
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
			// Prompt for confirmation using liner
			confirmInput, err := line.Prompt("[y] run  [n] cancel  [e] edit > ")
			if err != nil {
				fmt.Println(ui.Dim("Cancelled"))
				sessStore.AddExecutionMessage(sess.ID, input, command, "", 0, analysis.RiskLevel, 0, false, userTokens+assistantTokens)
				continue
			}
			confirmInput = strings.TrimSpace(strings.ToLower(confirmInput))

			switch confirmInput {
			case "y", "yes":
				confirmed = true
			case "e", "edit":
				newCmd, err := line.Prompt("  Enter modified command: ")
				if err != nil {
					continue
				}
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
