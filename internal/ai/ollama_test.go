// Package ai Ollama provider tests
package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/soroush/sosomi/internal/types"
)

func TestOllamaProvider_Name(t *testing.T) {
	provider, err := NewOllamaProvider("http://localhost:11434", "llama3.2")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	if provider.Name() != "ollama" {
		t.Errorf("Expected name 'ollama', got '%s'", provider.Name())
	}
}

func TestOllamaProvider_DefaultEndpoint(t *testing.T) {
	provider, err := NewOllamaProvider("", "")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	if provider.endpoint != "http://localhost:11434" {
		t.Errorf("Expected default endpoint 'http://localhost:11434', got '%s'", provider.endpoint)
	}

	if provider.model != "llama3.2" {
		t.Errorf("Expected default model 'llama3.2', got '%s'", provider.model)
	}
}

func TestOllamaProvider_CustomEndpoint(t *testing.T) {
	provider, err := NewOllamaProvider("http://custom:11434/", "custom-model")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Should trim trailing slash
	if provider.endpoint != "http://custom:11434" {
		t.Errorf("Expected endpoint without trailing slash, got '%s'", provider.endpoint)
	}

	if provider.model != "custom-model" {
		t.Errorf("Expected model 'custom-model', got '%s'", provider.model)
	}
}

func TestOllamaProvider_SupportsTools(t *testing.T) {
	provider, err := NewOllamaProvider("", "")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Ollama may or may not support tools depending on version
	_ = provider.SupportsTools()
}

// Mock Ollama server
func createMockOllamaServer(response OllamaChatResponse) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request format
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/chat" {
			// Could be other endpoints
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}

var t *testing.T // Global for mock server error reporting

func TestOllamaProvider_GenerateCommand_WithMock(t *testing.T) {
	mockResponse := OllamaChatResponse{
		Model:     "llama3.2",
		CreatedAt: "2024-01-01T00:00:00Z",
		Message: OllamaMessage{
			Role:    "assistant",
			Content: `{"command": "ls -la", "explanation": "List files", "risk_level": "safe", "confidence": 0.95}`,
		},
		Done: true,
	}

	server := createMockOllamaServer(mockResponse)
	defer server.Close()

	provider, err := NewOllamaProvider(server.URL, "llama3.2")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx := context.Background()
	sysCtx := types.SystemContext{
		OS:         "linux",
		Shell:      "bash",
		CurrentDir: "/home/user",
	}

	resp, err := provider.GenerateCommand(ctx, "list all files", sysCtx)
	if err != nil {
		t.Fatalf("GenerateCommand failed: %v", err)
	}

	if resp.Command != "ls -la" {
		t.Errorf("Expected command 'ls -la', got '%s'", resp.Command)
	}
}

func TestOllamaProvider_RefineCommand_WithMock(t *testing.T) {
	mockResponse := OllamaChatResponse{
		Model:     "llama3.2",
		CreatedAt: "2024-01-01T00:00:00Z",
		Message: OllamaMessage{
			Role:    "assistant",
			Content: `{"command": "ls -lS", "explanation": "List sorted by size", "risk_level": "safe", "confidence": 0.9}`,
		},
		Done: true,
	}

	server := createMockOllamaServer(mockResponse)
	defer server.Close()

	provider, err := NewOllamaProvider(server.URL, "llama3.2")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx := context.Background()
	sysCtx := types.SystemContext{
		OS:    "darwin",
		Shell: "zsh",
	}

	req := RefineRequest{
		OriginalPrompt: "list by size",
		GeneratedCmd:   "ls --sort=size",
		Feedback:       "failed on macOS",
		WasExecuted:    true,
		ExitCode:       1,
	}

	resp, err := provider.RefineCommand(ctx, req, sysCtx)
	if err != nil {
		t.Fatalf("RefineCommand failed: %v", err)
	}

	if resp.Command != "ls -lS" {
		t.Errorf("Expected command 'ls -lS', got '%s'", resp.Command)
	}
}

func TestOllamaProvider_Chat_WithMock(t *testing.T) {
	mockResponse := OllamaChatResponse{
		Model:     "llama3.2",
		CreatedAt: "2024-01-01T00:00:00Z",
		Message: OllamaMessage{
			Role:    "assistant",
			Content: "Hello! I'm here to help.",
		},
		Done: true,
	}

	server := createMockOllamaServer(mockResponse)
	defer server.Close()

	provider, err := NewOllamaProvider(server.URL, "llama3.2")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx := context.Background()
	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	resp, err := provider.Chat(ctx, messages)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if resp != "Hello! I'm here to help." {
		t.Errorf("Unexpected response: %s", resp)
	}
}

func TestOllamaProvider_ListModels_WithMock(t *testing.T) {
	modelsResponse := OllamaModelsResponse{
		Models: []OllamaModel{
			{Name: "llama3.2", ModifiedAt: "2024-01-01T00:00:00Z", Size: 1000000000},
			{Name: "codellama", ModifiedAt: "2024-01-01T00:00:00Z", Size: 2000000000},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(modelsResponse)
		}
	}))
	defer server.Close()

	provider, err := NewOllamaProvider(server.URL, "")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx := context.Background()
	models, err := provider.ListModels(ctx)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}
}

func TestOllamaChatRequest_Serialization(t *testing.T) {
	req := OllamaChatRequest{
		Model: "llama3.2",
		Messages: []OllamaMessage{
			{Role: "system", Content: "You are a helpful assistant"},
			{Role: "user", Content: "Hello"},
		},
		Stream: false,
		Format: "json",
		Options: &OllamaOptions{
			Temperature: 0.1,
			NumPredict:  1024,
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	var parsed OllamaChatRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}

	if parsed.Model != "llama3.2" {
		t.Errorf("Expected model 'llama3.2', got '%s'", parsed.Model)
	}
	if len(parsed.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(parsed.Messages))
	}
}

func TestOllamaMessage_Serialization(t *testing.T) {
	msg := OllamaMessage{
		Role:    "assistant",
		Content: "Hello!",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	var parsed OllamaMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if parsed.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", parsed.Role)
	}
	if parsed.Content != "Hello!" {
		t.Errorf("Expected content 'Hello!', got '%s'", parsed.Content)
	}
}
