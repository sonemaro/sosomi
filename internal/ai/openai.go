// Package ai provides OpenAI provider implementation
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/soroush/sosomi/internal/config"
	"github.com/soroush/sosomi/internal/types"
)

// OpenAIProvider implements the Provider interface for OpenAI
type OpenAIProvider struct {
	client *openai.Client
	model  string
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(apiKey, endpoint, model string) (*OpenAIProvider, error) {
	if apiKey == "" {
		apiKey = config.GetAPIKey()
	}
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	cfg := openai.DefaultConfig(apiKey)
	if endpoint != "" && endpoint != "https://api.openai.com/v1" {
		cfg.BaseURL = endpoint
	}

	if model == "" {
		model = "gpt-4o"
	}

	return &OpenAIProvider{
		client: openai.NewClientWithConfig(cfg),
		model:  model,
	}, nil
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

func (p *OpenAIProvider) GenerateCommand(ctx context.Context, prompt string, sysCtx types.SystemContext) (*types.CommandResponse, error) {
	systemMessage := SystemPrompt + "\n\n" + BuildSystemContext(sysCtx)

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: p.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemMessage},
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		Temperature: 0.1,
		MaxTokens:   1024,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate command: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	return parseCommandResponse(resp.Choices[0].Message.Content)
}

func (p *OpenAIProvider) RefineCommand(ctx context.Context, req RefineRequest, sysCtx types.SystemContext) (*types.CommandResponse, error) {
	systemMessage := RefinePrompt + "\n\n" + BuildSystemContext(sysCtx)

	// Build the user message with all context
	var userMessage strings.Builder
	userMessage.WriteString("ORIGINAL REQUEST: " + req.OriginalPrompt + "\n\n")
	userMessage.WriteString("GENERATED COMMAND: " + req.GeneratedCmd + "\n\n")
	
	if req.WasExecuted {
		userMessage.WriteString("COMMAND WAS EXECUTED\n")
		userMessage.WriteString(fmt.Sprintf("EXIT CODE: %d\n", req.ExitCode))
		if req.CommandOutput != "" {
			// Truncate output if too long
			output := req.CommandOutput
			if len(output) > 1000 {
				output = output[:1000] + "\n... (output truncated)"
			}
			userMessage.WriteString("OUTPUT:\n" + output + "\n\n")
		}
		if req.CommandError != "" {
			userMessage.WriteString("ERROR OUTPUT:\n" + req.CommandError + "\n\n")
		}
	} else {
		userMessage.WriteString("COMMAND WAS NOT EXECUTED\n\n")
	}
	
	userMessage.WriteString("USER FEEDBACK: " + req.Feedback)

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: p.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemMessage},
			{Role: openai.ChatMessageRoleUser, Content: userMessage.String()},
		},
		Temperature: 0.1,
		MaxTokens:   1024,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to refine command: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from AI")
	}

	return parseCommandResponse(resp.Choices[0].Message.Content)
}

func (p *OpenAIProvider) GenerateCommandStream(ctx context.Context, prompt string, sysCtx types.SystemContext) (<-chan StreamChunk, error) {
	systemMessage := SystemPrompt + "\n\n" + BuildSystemContext(sysCtx)

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
			if err == io.EOF {
				ch <- StreamChunk{Done: true, Usage: usage}
				return
			}
			if err != nil {
				ch <- StreamChunk{Error: err}
				return
			}
			// Capture usage from the final chunk if available
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

func (p *OpenAIProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	resp, err := p.ChatWithUsage(ctx, messages)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// ChatWithUsage sends a chat message and returns the response with token usage
func (p *OpenAIProvider) ChatWithUsage(ctx context.Context, messages []Message) (*ChatResponse, error) {
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

func (p *OpenAIProvider) ChatStream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
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
			if err == io.EOF {
				ch <- StreamChunk{Done: true, Usage: usage}
				return
			}
			if err != nil {
				ch <- StreamChunk{Error: err}
				return
			}
			// Capture usage from the final chunk if available
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

func (p *OpenAIProvider) ListModels(ctx context.Context) ([]string, error) {
	resp, err := p.client.ListModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	models := make([]string, 0)
	for _, model := range resp.Models {
		// Filter to relevant models
		if strings.Contains(model.ID, "gpt") {
			models = append(models, model.ID)
		}
	}
	return models, nil
}

func (p *OpenAIProvider) SupportsTools() bool {
	return true
}

func (p *OpenAIProvider) CallTool(ctx context.Context, tool types.MCPToolCall) (*types.MCPToolResult, error) {
	// Tool calling implementation for OpenAI
	// This would use function calling API
	return nil, fmt.Errorf("tool calling not yet implemented for OpenAI")
}

// parseCommandResponse parses the JSON response from the AI
func parseCommandResponse(content string) (*types.CommandResponse, error) {
	// Try to extract JSON from the response
	content = strings.TrimSpace(content)
	
	// Handle markdown code blocks
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	var rawResp struct {
		Command      string   `json:"command"`
		Explanation  string   `json:"explanation"`
		RiskLevel    string   `json:"risk_level"`
		Confidence   float64  `json:"confidence"`
		Warnings     []string `json:"warnings"`
		Alternatives []string `json:"alternatives"`
	}

	if err := json.Unmarshal([]byte(content), &rawResp); err != nil {
		// If JSON parsing fails, treat the entire response as a command
		return &types.CommandResponse{
			Command:     content,
			Explanation: "Generated command",
			RiskLevel:   types.RiskCaution, // Default to caution when we can't parse
			Confidence:  0.5,
		}, nil
	}

	// Convert risk level string to RiskLevel
	var riskLevel types.RiskLevel
	switch strings.ToLower(rawResp.RiskLevel) {
	case "safe":
		riskLevel = types.RiskSafe
	case "caution":
		riskLevel = types.RiskCaution
	case "dangerous":
		riskLevel = types.RiskDangerous
	case "critical":
		riskLevel = types.RiskCritical
	default:
		riskLevel = types.RiskCaution
	}

	return &types.CommandResponse{
		Command:      rawResp.Command,
		Explanation:  rawResp.Explanation,
		RiskLevel:    riskLevel,
		Confidence:   rawResp.Confidence,
		Warnings:     rawResp.Warnings,
		Alternatives: rawResp.Alternatives,
	}, nil
}
