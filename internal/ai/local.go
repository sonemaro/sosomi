// Package ai provides LM Studio/llama.cpp provider implementation
// These use OpenAI-compatible APIs
package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"

	"github.com/soroush/sosomi/internal/types"
)

// LocalOpenAIProvider implements the Provider interface for OpenAI-compatible local servers
// This works with LM Studio, llama.cpp, text-generation-webui, etc.
type LocalOpenAIProvider struct {
	client   *openai.Client
	model    string
	name     string
	endpoint string
}

// NewLMStudioProvider creates a new LM Studio provider
func NewLMStudioProvider(endpoint, model string) (*LocalOpenAIProvider, error) {
	if endpoint == "" {
		endpoint = "http://localhost:1234/v1"
	}
	if model == "" {
		model = "local-model" // LM Studio uses the loaded model
	}

	cfg := openai.DefaultConfig("lm-studio") // LM Studio doesn't require an API key
	cfg.BaseURL = endpoint

	return &LocalOpenAIProvider{
		client:   openai.NewClientWithConfig(cfg),
		model:    model,
		name:     "lmstudio",
		endpoint: endpoint,
	}, nil
}

// NewLlamaCppProvider creates a new llama.cpp provider
func NewLlamaCppProvider(endpoint, model string) (*LocalOpenAIProvider, error) {
	if endpoint == "" {
		endpoint = "http://localhost:8080/v1"
	}
	if model == "" {
		model = "local-model"
	}

	cfg := openai.DefaultConfig("llamacpp")
	cfg.BaseURL = endpoint

	return &LocalOpenAIProvider{
		client:   openai.NewClientWithConfig(cfg),
		model:    model,
		name:     "llamacpp",
		endpoint: endpoint,
	}, nil
}

// NewGenericOpenAIProvider creates a provider for any OpenAI-compatible API
func NewGenericOpenAIProvider(apiKey, endpoint, model string) (*LocalOpenAIProvider, error) {
	if endpoint == "" {
		return nil, fmt.Errorf("endpoint is required for generic provider")
	}
	if apiKey == "" {
		apiKey = "no-key"
	}
	if model == "" {
		model = "default"
	}

	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = endpoint

	return &LocalOpenAIProvider{
		client:   openai.NewClientWithConfig(cfg),
		model:    model,
		name:     "generic",
		endpoint: endpoint,
	}, nil
}

func (p *LocalOpenAIProvider) Name() string {
	return p.name
}

func (p *LocalOpenAIProvider) GenerateCommand(ctx context.Context, prompt string, sysCtx types.SystemContext) (*types.CommandResponse, error) {
	// For local models, use a simpler prompt format that works better
	systemMessage := buildLocalModelSystemPrompt(sysCtx)

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: p.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemMessage},
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		Temperature: 0.1,
		MaxTokens:   1024,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate command: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from model")
	}

	return parseLocalModelResponse(resp.Choices[0].Message.Content)
}

func (p *LocalOpenAIProvider) RefineCommand(ctx context.Context, req RefineRequest, sysCtx types.SystemContext) (*types.CommandResponse, error) {
	systemMessage := buildLocalModelRefinePrompt(sysCtx)

	// Build the user message with all context
	var userMessage strings.Builder
	userMessage.WriteString("ORIGINAL REQUEST: " + req.OriginalPrompt + "\n\n")
	userMessage.WriteString("GENERATED COMMAND: " + req.GeneratedCmd + "\n\n")

	if req.WasExecuted {
		userMessage.WriteString("COMMAND WAS EXECUTED\n")
		userMessage.WriteString(fmt.Sprintf("EXIT CODE: %d\n", req.ExitCode))
		if req.CommandOutput != "" {
			output := req.CommandOutput
			if len(output) > 500 {
				output = output[:500] + "\n... (truncated)"
			}
			userMessage.WriteString("OUTPUT:\n" + output + "\n\n")
		}
		if req.CommandError != "" {
			userMessage.WriteString("ERROR:\n" + req.CommandError + "\n\n")
		}
	}

	userMessage.WriteString("PROBLEM: " + req.Feedback + "\n\n")
	userMessage.WriteString("Please provide a corrected command.")

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: p.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemMessage},
			{Role: openai.ChatMessageRoleUser, Content: userMessage.String()},
		},
		Temperature: 0.1,
		MaxTokens:   1024,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to refine command: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from model")
	}

	return parseLocalModelResponse(resp.Choices[0].Message.Content)
}

func (p *LocalOpenAIProvider) GenerateCommandStream(ctx context.Context, prompt string, sysCtx types.SystemContext) (<-chan StreamChunk, error) {
	systemMessage := buildLocalModelSystemPrompt(sysCtx)

	stream, err := p.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model: p.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemMessage},
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		Temperature: 0.1,
		MaxTokens:   1024,
		Stream:      true,
		StreamOptions: &openai.StreamOptions{
			IncludeUsage: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}

	ch := make(chan StreamChunk)
	go func() {
		defer close(ch)
		defer stream.Close()

		var usage *TokenUsage
		for {
			resp, err := stream.Recv()
			if err != nil {
				if err.Error() == "EOF" {
					ch <- StreamChunk{Done: true, Usage: usage}
					return
				}
				ch <- StreamChunk{Error: err}
				return
			}
			// Capture usage from final chunk if available
			if resp.Usage != nil {
				usage = &TokenUsage{
					PromptTokens:     resp.Usage.PromptTokens,
					CompletionTokens: resp.Usage.CompletionTokens,
					TotalTokens:      resp.Usage.TotalTokens,
				}
			}
			if len(resp.Choices) > 0 {
				ch <- StreamChunk{Content: resp.Choices[0].Delta.Content}
			}
		}
	}()

	return ch, nil
}

func (p *LocalOpenAIProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	resp, err := p.ChatWithUsage(ctx, messages)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// ChatWithUsage sends a chat message and returns the response with token usage
func (p *LocalOpenAIProvider) ChatWithUsage(ctx context.Context, messages []Message) (*ChatResponse, error) {
	openaiMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, msg := range messages {
		openaiMessages[i] = openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       p.model,
		Messages:    openaiMessages,
		Temperature: 0.7,
		MaxTokens:   2048,
	})
	if err != nil {
		return nil, fmt.Errorf("chat failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response")
	}

	return &ChatResponse{
		Content: resp.Choices[0].Message.Content,
		Usage: TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}, nil
}

func (p *LocalOpenAIProvider) ChatStream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	openaiMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, msg := range messages {
		openaiMessages[i] = openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	stream, err := p.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:       p.model,
		Messages:    openaiMessages,
		Temperature: 0.7,
		MaxTokens:   2048,
		Stream:      true,
		StreamOptions: &openai.StreamOptions{
			IncludeUsage: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}

	ch := make(chan StreamChunk)
	go func() {
		defer close(ch)
		defer stream.Close()

		var usage *TokenUsage
		for {
			resp, err := stream.Recv()
			if err != nil {
				if err.Error() == "EOF" {
					ch <- StreamChunk{Done: true, Usage: usage}
					return
				}
				ch <- StreamChunk{Error: err}
				return
			}
			// Capture usage from final chunk if available
			if resp.Usage != nil {
				usage = &TokenUsage{
					PromptTokens:     resp.Usage.PromptTokens,
					CompletionTokens: resp.Usage.CompletionTokens,
					TotalTokens:      resp.Usage.TotalTokens,
				}
			}
			if len(resp.Choices) > 0 {
				ch <- StreamChunk{Content: resp.Choices[0].Delta.Content}
			}
		}
	}()

	return ch, nil
}

func (p *LocalOpenAIProvider) ListModels(ctx context.Context) ([]string, error) {
	resp, err := p.client.ListModels(ctx)
	if err != nil {
		// Local servers might not support listing models
		return []string{p.model}, nil
	}

	models := make([]string, len(resp.Models))
	for i, model := range resp.Models {
		models[i] = model.ID
	}
	return models, nil
}

func (p *LocalOpenAIProvider) SupportsTools() bool {
	return false // Local models generally don't support function calling
}

func (p *LocalOpenAIProvider) CallTool(ctx context.Context, tool types.MCPToolCall) (*types.MCPToolResult, error) {
	return nil, fmt.Errorf("tool calling not supported for local models")
}

// buildLocalModelSystemPrompt creates a simpler prompt for local models
func buildLocalModelSystemPrompt(sysCtx types.SystemContext) string {
	return fmt.Sprintf(`You are a shell command assistant. Convert natural language to shell commands.

SYSTEM INFO:
- OS: %s
- Shell: %s
- Current directory: %s
- User: %s

RULES:
1. Output ONLY the shell command, nothing else
2. If the command is dangerous, prefix with "WARNING:" on a separate line
3. Use the appropriate commands for the user's OS and shell
4. If you cannot generate a safe command, say "ERROR:" followed by the reason

Examples:
User: list files
Output: ls -la

User: delete everything
Output: ERROR: Cannot generate destructive commands without specific targets

User: disk usage
Output: df -h`, sysCtx.OS, sysCtx.Shell, sysCtx.CurrentDir, sysCtx.Username)
}

// buildLocalModelRefinePrompt creates a prompt for refining commands
func buildLocalModelRefinePrompt(sysCtx types.SystemContext) string {
	return fmt.Sprintf(`You are a shell command assistant. A previous command didn't work as expected. Fix it based on the user's feedback.

SYSTEM INFO:
- OS: %s (IMPORTANT: macOS uses BSD commands, not GNU. Flags differ!)
- Shell: %s
- Current directory: %s
- User: %s

IMPORTANT macOS NOTES:
- 'ps' does not support GNU-style long options like --sort
- Use 'ps aux' or 'ps -eo' with short flags
- 'sort' and 'awk' work differently than on Linux
- When in doubt, use simpler portable commands

RULES:
1. Read the error or feedback carefully
2. Output ONLY the corrected shell command
3. Make sure the command works on the user's specific OS
4. If the original approach won't work, suggest a different approach`, sysCtx.OS, sysCtx.Shell, sysCtx.CurrentDir, sysCtx.Username)
}

// parseLocalModelResponse parses the simpler response format from local models
func parseLocalModelResponse(content string) (*types.CommandResponse, error) {
	content = strings.TrimSpace(content)

	resp := &types.CommandResponse{
		Confidence: 0.8,
		RiskLevel:  types.RiskSafe,
	}

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "WARNING:") {
			resp.Warnings = append(resp.Warnings, strings.TrimPrefix(line, "WARNING:"))
			resp.RiskLevel = types.RiskCaution
		} else if strings.HasPrefix(line, "ERROR:") {
			resp.Explanation = strings.TrimPrefix(line, "ERROR:")
			resp.RiskLevel = types.RiskCritical
			resp.Command = ""
			return resp, nil
		} else if line != "" && resp.Command == "" {
			// First non-warning/error line is the command
			resp.Command = line
			if i+1 < len(lines) {
				resp.Explanation = strings.Join(lines[i+1:], " ")
			}
		}
	}

	if resp.Command == "" {
		resp.Command = content
	}

	return resp, nil
}
