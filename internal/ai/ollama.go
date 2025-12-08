// Package ai provides Ollama provider implementation for local models
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sonemaro/sosomi/internal/types"
)

// OllamaProvider implements the Provider interface for Ollama
type OllamaProvider struct {
	endpoint string
	model    string
	client   *http.Client
}

// OllamaChatRequest represents an Ollama chat API request
type OllamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Format   string          `json:"format,omitempty"`
	Options  *OllamaOptions  `json:"options,omitempty"`
}

// OllamaMessage represents a message in Ollama format
type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OllamaOptions represents model options
type OllamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	TopK        int     `json:"top_k,omitempty"`
}

// OllamaChatResponse represents an Ollama chat API response
type OllamaChatResponse struct {
	Model           string        `json:"model"`
	CreatedAt       string        `json:"created_at"`
	Message         OllamaMessage `json:"message"`
	Done            bool          `json:"done"`
	PromptEvalCount int           `json:"prompt_eval_count,omitempty"`
	EvalCount       int           `json:"eval_count,omitempty"`
}

// OllamaModelsResponse represents the response from listing models
type OllamaModelsResponse struct {
	Models []OllamaModel `json:"models"`
}

// OllamaModel represents a model in Ollama
type OllamaModel struct {
	Name       string `json:"name"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
}

// NewOllamaProvider creates a new Ollama provider
func NewOllamaProvider(endpoint, model string) (*OllamaProvider, error) {
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}
	if model == "" {
		model = "llama3.2" // Default to latest llama
	}

	return &OllamaProvider{
		endpoint: strings.TrimSuffix(endpoint, "/"),
		model:    model,
		client:   &http.Client{},
	}, nil
}

func (p *OllamaProvider) Name() string {
	return "ollama"
}

func (p *OllamaProvider) GenerateCommand(ctx context.Context, prompt string, sysCtx types.SystemContext) (*types.CommandResponse, error) {
	systemMessage := SystemPrompt + "\n\n" + BuildSystemContext(sysCtx)

	reqBody := OllamaChatRequest{
		Model: p.model,
		Messages: []OllamaMessage{
			{Role: "system", Content: systemMessage},
			{Role: "user", Content: prompt},
		},
		Stream: false,
		Format: "json",
		Options: &OllamaOptions{
			Temperature: 0.1,
			NumPredict:  1024,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.endpoint+"/api/chat", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama API error: %s - %s", resp.Status, string(body))
	}

	var ollamaResp OllamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return parseCommandResponse(ollamaResp.Message.Content)
}

func (p *OllamaProvider) RefineCommand(ctx context.Context, req RefineRequest, sysCtx types.SystemContext) (*types.CommandResponse, error) {
	systemMessage := RefinePrompt + "\n\n" + BuildSystemContext(sysCtx)

	// Build the user message with context
	var userMessage strings.Builder
	userMessage.WriteString("ORIGINAL REQUEST: " + req.OriginalPrompt + "\n\n")
	userMessage.WriteString("GENERATED COMMAND: " + req.GeneratedCmd + "\n\n")

	if req.WasExecuted {
		userMessage.WriteString("COMMAND WAS EXECUTED\n")
		userMessage.WriteString(fmt.Sprintf("EXIT CODE: %d\n", req.ExitCode))
		if req.CommandOutput != "" {
			output := req.CommandOutput
			if len(output) > 1000 {
				output = output[:1000] + "\n... (truncated)"
			}
			userMessage.WriteString("OUTPUT:\n" + output + "\n\n")
		}
		if req.CommandError != "" {
			userMessage.WriteString("ERROR OUTPUT:\n" + req.CommandError + "\n\n")
		}
	}

	userMessage.WriteString("USER FEEDBACK: " + req.Feedback)

	reqBody := OllamaChatRequest{
		Model: p.model,
		Messages: []OllamaMessage{
			{Role: "system", Content: systemMessage},
			{Role: "user", Content: userMessage.String()},
		},
		Stream: false,
		Format: "json",
		Options: &OllamaOptions{
			Temperature: 0.1,
			NumPredict:  1024,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.endpoint+"/api/chat", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama API error: %s - %s", resp.Status, string(body))
	}

	var ollamaResp OllamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return parseCommandResponse(ollamaResp.Message.Content)
}

func (p *OllamaProvider) GenerateCommandStream(ctx context.Context, prompt string, sysCtx types.SystemContext) (<-chan StreamChunk, error) {
	systemMessage := SystemPrompt + "\n\n" + BuildSystemContext(sysCtx)

	reqBody := OllamaChatRequest{
		Model: p.model,
		Messages: []OllamaMessage{
			{Role: "system", Content: systemMessage},
			{Role: "user", Content: prompt},
		},
		Stream: true,
		Options: &OllamaOptions{
			Temperature: 0.1,
			NumPredict:  1024,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.endpoint+"/api/chat", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("ollama API error: %s", resp.Status)
	}

	ch := make(chan StreamChunk)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		decoder := json.NewDecoder(resp.Body)
		for {
			var ollamaResp OllamaChatResponse
			if err := decoder.Decode(&ollamaResp); err != nil {
				if err == io.EOF {
					ch <- StreamChunk{Done: true}
					return
				}
				ch <- StreamChunk{Error: err}
				return
			}

			if ollamaResp.Done {
				// Final chunk includes token usage
				ch <- StreamChunk{
					Content: ollamaResp.Message.Content,
					Done:    true,
					Usage: &TokenUsage{
						PromptTokens:     ollamaResp.PromptEvalCount,
						CompletionTokens: ollamaResp.EvalCount,
						TotalTokens:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
					},
				}
				return
			}

			ch <- StreamChunk{
				Content: ollamaResp.Message.Content,
				Done:    false,
			}
		}
	}()

	return ch, nil
}

func (p *OllamaProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	resp, err := p.ChatWithUsage(ctx, messages)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// ChatWithUsage sends a chat message and returns the response with token usage
func (p *OllamaProvider) ChatWithUsage(ctx context.Context, messages []Message) (*ChatResponse, error) {
	ollamaMessages := make([]OllamaMessage, len(messages))
	for i, msg := range messages {
		ollamaMessages[i] = OllamaMessage(msg)
	}

	reqBody := OllamaChatRequest{
		Model:    p.model,
		Messages: ollamaMessages,
		Stream:   false,
		Options: &OllamaOptions{
			Temperature: 0.7,
			NumPredict:  2048,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.endpoint+"/api/chat", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama API error: %s - %s", resp.Status, string(body))
	}

	var ollamaResp OllamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &ChatResponse{
		Content: ollamaResp.Message.Content,
		Usage: TokenUsage{
			PromptTokens:     ollamaResp.PromptEvalCount,
			CompletionTokens: ollamaResp.EvalCount,
			TotalTokens:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
		},
	}, nil
}

func (p *OllamaProvider) ChatStream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	ollamaMessages := make([]OllamaMessage, len(messages))
	for i, msg := range messages {
		ollamaMessages[i] = OllamaMessage(msg)
	}

	reqBody := OllamaChatRequest{
		Model:    p.model,
		Messages: ollamaMessages,
		Stream:   true,
		Options: &OllamaOptions{
			Temperature: 0.7,
			NumPredict:  2048,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.endpoint+"/api/chat", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("ollama API error: %s", resp.Status)
	}

	ch := make(chan StreamChunk)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		decoder := json.NewDecoder(resp.Body)
		for {
			var ollamaResp OllamaChatResponse
			if err := decoder.Decode(&ollamaResp); err != nil {
				if err == io.EOF {
					ch <- StreamChunk{Done: true}
					return
				}
				ch <- StreamChunk{Error: err}
				return
			}

			if ollamaResp.Done {
				// Final chunk includes token usage
				ch <- StreamChunk{
					Content: ollamaResp.Message.Content,
					Done:    true,
					Usage: &TokenUsage{
						PromptTokens:     ollamaResp.PromptEvalCount,
						CompletionTokens: ollamaResp.EvalCount,
						TotalTokens:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
					},
				}
				return
			}

			ch <- StreamChunk{
				Content: ollamaResp.Message.Content,
				Done:    false,
			}
		}
	}()

	return ch, nil
}

func (p *OllamaProvider) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.endpoint+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama API error: %s", resp.Status)
	}

	var modelsResp OllamaModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]string, len(modelsResp.Models))
	for i, model := range modelsResp.Models {
		models[i] = model.Name
	}
	return models, nil
}

func (p *OllamaProvider) SupportsTools() bool {
	// Some Ollama models support tools, but not all
	return false
}

func (p *OllamaProvider) CallTool(ctx context.Context, tool types.MCPToolCall) (*types.MCPToolResult, error) {
	return nil, fmt.Errorf("tool calling not supported for Ollama")
}
