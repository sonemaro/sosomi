// Package ai provides the AI provider interface and implementations
package ai

import (
	"context"

	"github.com/soroush/sosomi/internal/types"
)

// Provider defines the interface for AI providers
type Provider interface {
	// Name returns the provider name
	Name() string

	// GenerateCommand generates a shell command from a natural language prompt
	GenerateCommand(ctx context.Context, prompt string, sysCtx types.SystemContext) (*types.CommandResponse, error)

	// RefineCommand refines a previously generated command based on feedback
	RefineCommand(ctx context.Context, original RefineRequest, sysCtx types.SystemContext) (*types.CommandResponse, error)

	// GenerateCommandStream generates a command with streaming output
	GenerateCommandStream(ctx context.Context, prompt string, sysCtx types.SystemContext) (<-chan StreamChunk, error)

	// Chat sends a chat message and returns the response
	Chat(ctx context.Context, messages []Message) (string, error)

	// ChatStream sends a chat message with streaming response
	ChatStream(ctx context.Context, messages []Message) (<-chan StreamChunk, error)

	// ListModels returns available models
	ListModels(ctx context.Context) ([]string, error)

	// SupportsTools returns whether the provider supports function/tool calling
	SupportsTools() bool

	// CallTool executes a tool call
	CallTool(ctx context.Context, tool types.MCPToolCall) (*types.MCPToolResult, error)
}

// RefineRequest contains the context for refining a command
type RefineRequest struct {
	OriginalPrompt string // The original user request
	GeneratedCmd   string // The command that was generated
	Feedback       string // User's feedback on what was wrong
	CommandOutput  string // Output from running the command (if any)
	CommandError   string // Error from running the command (if any)
	ExitCode       int    // Exit code if command was run
	WasExecuted    bool   // Whether the command was actually run
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// TokenUsage represents token usage statistics from an API call
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatResponse represents a chat response with token usage
type ChatResponse struct {
	Content string     `json:"content"`
	Usage   TokenUsage `json:"usage"`
}

// StreamChunk represents a chunk of streamed response
type StreamChunk struct {
	Content string
	Done    bool
	Error   error
	Usage   *TokenUsage // Only populated on the final chunk (Done=true)
}

// ProviderType represents the type of AI provider
type ProviderType string

const (
	ProviderOpenAI    ProviderType = "openai"
	ProviderAnthropic ProviderType = "anthropic"
	ProviderOllama    ProviderType = "ollama"
	ProviderLMStudio  ProviderType = "lmstudio"
	ProviderLlamaCpp  ProviderType = "llamacpp"
	ProviderAzure     ProviderType = "azure"
	ProviderGeneric   ProviderType = "generic"
)

// SystemPrompt is the default system prompt for command generation
const SystemPrompt = `You are Sosomi, an expert shell command assistant for macOS/Linux. Your job is to convert natural language requests into safe, efficient shell commands.

RULES:
1. Generate ONLY the shell command, no explanations unless asked
2. Always consider safety - never generate commands that could cause data loss without explicit confirmation
3. Prefer safe alternatives when possible (e.g., 'rm -i' instead of 'rm -rf')
4. For dangerous operations, include appropriate flags for safety
5. Consider the user's current working directory and system context
6. Use portable commands that work across Unix-like systems when possible
7. If a request is ambiguous, ask for clarification
8. NEVER generate commands for: system destruction, unauthorized access, or malicious purposes

SAFETY GUIDELINES:
- Flag destructive commands (rm, dd, mkfs, etc.)
- Warn about commands requiring sudo
- Suggest safer alternatives when available
- Refuse clearly harmful requests

When generating commands, output in this JSON format:
{
  "command": "the shell command",
  "explanation": "brief explanation of what the command does",
  "risk_level": "safe|caution|dangerous|critical",
  "confidence": 0.0-1.0,
  "warnings": ["any warnings about the command"],
  "alternatives": ["safer alternative commands if applicable"]
}

If you cannot or should not generate a command, respond with:
{
  "command": "",
  "explanation": "reason why the command cannot be generated",
  "risk_level": "critical",
  "confidence": 0.0,
  "warnings": ["explanation of the issue"]
}`

// RefinePrompt is the system prompt for refining commands based on feedback
const RefinePrompt = `You are Sosomi, an expert shell command assistant. The user tried a command but it didn't work as expected. Your job is to fix or improve the command based on their feedback.

CONTEXT:
- You will receive the original request, the command that was generated, and feedback about what went wrong
- The feedback might include: command errors, wrong output, missing features, or general dissatisfaction
- Generate an improved command that addresses the user's concerns

IMPORTANT:
- Pay attention to OS-specific command syntax (e.g., macOS uses BSD tools, not GNU)
- macOS 'ps' uses different flags than Linux - avoid GNU-style long options like --sort
- Consider using alternative commands if the original approach doesn't work on this OS
- If the command had a syntax error, fix the syntax for the user's specific shell/OS

Output in the same JSON format as before:
{
  "command": "the improved shell command",
  "explanation": "what was wrong and how you fixed it",
  "risk_level": "safe|caution|dangerous|critical",
  "confidence": 0.0-1.0,
  "warnings": ["any warnings"],
  "alternatives": ["other approaches if available"]
}`

// BuildSystemContext creates a formatted system context string for the prompt
func BuildSystemContext(ctx types.SystemContext) string {
	result := "SYSTEM CONTEXT:\n"
	result += "- OS: " + ctx.OS + "\n"
	result += "- Shell: " + ctx.Shell + "\n"
	result += "- Current Directory: " + ctx.CurrentDir + "\n"
	result += "- Home Directory: " + ctx.HomeDir + "\n"
	result += "- User: " + ctx.Username + "\n"

	if ctx.GitBranch != "" {
		result += "- Git Branch: " + ctx.GitBranch + "\n"
		result += "- Git Status: " + ctx.GitStatus + "\n"
	}

	if len(ctx.InstalledPkgMgrs) > 0 {
		result += "- Package Managers: "
		for i, pm := range ctx.InstalledPkgMgrs {
			if i > 0 {
				result += ", "
			}
			result += pm
		}
		result += "\n"
	}

	return result
}
