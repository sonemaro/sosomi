// LLM command and related functionality for sosomi CLI
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/peterh/liner"
	"github.com/spf13/cobra"

	"github.com/sonemaro/sosomi/internal/ai"
	"github.com/sonemaro/sosomi/internal/config"
	"github.com/sonemaro/sosomi/internal/conversation"
	"github.com/sonemaro/sosomi/internal/mcp"
	"github.com/sonemaro/sosomi/internal/types"
	"github.com/sonemaro/sosomi/internal/ui"
)

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
	provider, err := getAIProvider()
	if err != nil {
		return err
	}

	// Initialize MCP manager if enabled
	var mcpManager *mcp.Manager
	if cfg.MCP.Enabled {
		mcpManager = mcp.NewManager()
		defer mcpManager.Shutdown()
	}
	_ = mcpManager // Silence unused variable warning for now

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
