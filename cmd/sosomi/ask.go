// Ask command for sosomi CLI - intelligent help with AI
package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sonemaro/sosomi/internal/ai"
	"github.com/sonemaro/sosomi/internal/config"
	"github.com/sonemaro/sosomi/internal/ui"
)

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
- DANGEROUS: High-risk operations
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

---

IMPORTANT: This is a CLI tool, so your response will be displayed in a terminal.
- Do NOT use markdown code blocks (no triple backticks)
- Just show commands directly, indented with 2 spaces
- Use plain text formatting only
- Keep answers concise and scannable

Based on this documentation, answer the user's question. Provide ready-to-use commands when applicable.`

func runAskCommand(question string) error {
	// Try to create AI provider
	provider, err := getAIProvider()
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
