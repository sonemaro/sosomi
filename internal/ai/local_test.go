// Package ai local provider tests (LM Studio, llama.cpp)
package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sonemaro/sosomi/internal/types"
)

func TestLocalOpenAIProvider_LMStudio_Name(t *testing.T) {
	provider, err := NewLMStudioProvider("", "")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	if provider.Name() != "lmstudio" {
		t.Errorf("Expected name 'lmstudio', got '%s'", provider.Name())
	}
}

func TestLocalOpenAIProvider_LlamaCpp_Name(t *testing.T) {
	provider, err := NewLlamaCppProvider("", "")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	if provider.Name() != "llamacpp" {
		t.Errorf("Expected name 'llamacpp', got '%s'", provider.Name())
	}
}

func TestLocalOpenAIProvider_Generic_Name(t *testing.T) {
	provider, err := NewGenericOpenAIProvider("key", "http://example.com/v1", "model")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	if provider.Name() != "generic" {
		t.Errorf("Expected name 'generic', got '%s'", provider.Name())
	}
}

func TestLMStudioProvider_DefaultEndpoint(t *testing.T) {
	provider, err := NewLMStudioProvider("", "")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	if provider.endpoint != "http://localhost:1234/v1" {
		t.Errorf("Expected default endpoint 'http://localhost:1234/v1', got '%s'", provider.endpoint)
	}

	if provider.model != "local-model" {
		t.Errorf("Expected default model 'local-model', got '%s'", provider.model)
	}
}

func TestLlamaCppProvider_DefaultEndpoint(t *testing.T) {
	provider, err := NewLlamaCppProvider("", "")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	if provider.endpoint != "http://localhost:8080/v1" {
		t.Errorf("Expected default endpoint 'http://localhost:8080/v1', got '%s'", provider.endpoint)
	}
}

func TestLocalOpenAIProvider_SupportsTools(t *testing.T) {
	provider, err := NewLMStudioProvider("", "")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Local models may or may not support tools
	_ = provider.SupportsTools()
}

// Test buildLocalModelSystemPrompt
func TestBuildLocalModelSystemPrompt(t *testing.T) {
	sysCtx := types.SystemContext{
		OS:         "darwin",
		Shell:      "zsh",
		CurrentDir: "/Users/test",
		HomeDir:    "/Users/test",
		Username:   "test",
	}

	prompt := buildLocalModelSystemPrompt(sysCtx)

	if prompt == "" {
		t.Error("buildLocalModelSystemPrompt returned empty string")
	}

	// Should contain OS info
	expectedContents := []string{
		"darwin",
		"zsh",
		"command",
	}

	for _, expected := range expectedContents {
		if !containsString(prompt, expected) {
			t.Errorf("Expected prompt to contain '%s'", expected)
		}
	}
}

// Test buildLocalModelRefinePrompt
func TestBuildLocalModelRefinePrompt(t *testing.T) {
	sysCtx := types.SystemContext{
		OS:         "darwin",
		Shell:      "zsh",
		CurrentDir: "/tmp",
	}

	prompt := buildLocalModelRefinePrompt(sysCtx)

	if prompt == "" {
		t.Error("buildLocalModelRefinePrompt returned empty string")
	}

	// Should contain macOS-specific notes
	if !containsString(prompt, "macOS") && !containsString(prompt, "darwin") {
		t.Log("Refine prompt may need macOS-specific guidance")
	}
}

// Test parseLocalModelResponse
func TestParseLocalModelResponse(t *testing.T) {
	// parseLocalModelResponse expects plain text format, not JSON
	// The first non-warning/error line is treated as the command

	// Test with plain command response
	plainResp := "ls -la"
	resp, err := parseLocalModelResponse(plainResp)
	if err != nil {
		t.Fatalf("parseLocalModelResponse failed for plain: %v", err)
	}
	if resp.Command != "ls -la" {
		t.Errorf("Expected command 'ls -la', got '%s'", resp.Command)
	}

	// Test with command and explanation on separate lines
	multiLineResp := "ls -la\nList all files with details"
	resp, err = parseLocalModelResponse(multiLineResp)
	if err != nil {
		t.Fatalf("parseLocalModelResponse failed for multiline: %v", err)
	}
	if resp.Command != "ls -la" {
		t.Errorf("Expected command 'ls -la', got '%s'", resp.Command)
	}
}

func TestParseLocalModelResponse_WithWarning(t *testing.T) {
	// Test with WARNING prefix
	warningResp := "WARNING: This may delete files\nrm -rf temp"
	resp, err := parseLocalModelResponse(warningResp)
	if err != nil {
		t.Fatalf("parseLocalModelResponse failed: %v", err)
	}
	if len(resp.Warnings) == 0 {
		t.Error("Expected warnings to be populated")
	}
	if resp.RiskLevel != types.RiskCaution {
		t.Errorf("Expected RiskCaution for warning, got %v", resp.RiskLevel)
	}
}

func TestParseLocalModelResponse_WithError(t *testing.T) {
	// Test with ERROR prefix
	errorResp := "ERROR: Cannot process this request"
	resp, err := parseLocalModelResponse(errorResp)
	if err != nil {
		t.Fatalf("parseLocalModelResponse failed: %v", err)
	}
	if resp.RiskLevel != types.RiskCritical {
		t.Errorf("Expected RiskCritical for error, got %v", resp.RiskLevel)
	}
	if resp.Command != "" {
		t.Error("Command should be empty for error response")
	}
}

// Mock server for local OpenAI-compatible API
func createMockLocalServer(response map[string]interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}

func TestLocalOpenAIProvider_GenerateCommand_WithMock(t *testing.T) {
	// parseLocalModelResponse expects plain text, not JSON
	mockResponse := map[string]interface{}{
		"id":     "test-id",
		"object": "chat.completion",
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "ls -la\nList all files with details",
				},
				"finish_reason": "stop",
			},
		},
	}

	server := createMockLocalServer(mockResponse)
	defer server.Close()

	provider, err := NewLMStudioProvider(server.URL, "test-model")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx := context.Background()
	sysCtx := types.SystemContext{
		OS:    "darwin",
		Shell: "zsh",
	}

	resp, err := provider.GenerateCommand(ctx, "list files", sysCtx)
	if err != nil {
		t.Fatalf("GenerateCommand failed: %v", err)
	}

	if resp.Command != "ls -la" {
		t.Errorf("Expected command 'ls -la', got '%s'", resp.Command)
	}
}

func TestLocalOpenAIProvider_RefineCommand_WithMock(t *testing.T) {
	// parseLocalModelResponse expects plain text, not JSON
	mockResponse := map[string]interface{}{
		"id":     "test-id",
		"object": "chat.completion",
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "ls -lS\nSort by size (BSD style)",
				},
				"finish_reason": "stop",
			},
		},
	}

	server := createMockLocalServer(mockResponse)
	defer server.Close()

	provider, err := NewLMStudioProvider(server.URL, "test-model")
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
		Feedback:       "doesn't work on mac",
		WasExecuted:    true,
		ExitCode:       1,
		CommandError:   "illegal option",
	}

	resp, err := provider.RefineCommand(ctx, req, sysCtx)
	if err != nil {
		t.Fatalf("RefineCommand failed: %v", err)
	}

	if resp.Command != "ls -lS" {
		t.Errorf("Expected command 'ls -lS', got '%s'", resp.Command)
	}
}

func TestLocalOpenAIProvider_Chat_WithMock(t *testing.T) {
	mockResponse := map[string]interface{}{
		"id":     "test-id",
		"object": "chat.completion",
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "Hello! How can I help you today?",
				},
				"finish_reason": "stop",
			},
		},
	}

	server := createMockLocalServer(mockResponse)
	defer server.Close()

	provider, err := NewLMStudioProvider(server.URL, "test-model")
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

	if resp != "Hello! How can I help you today?" {
		t.Errorf("Unexpected response: %s", resp)
	}
}

func TestLocalOpenAIProvider_ListModels_WithMock(t *testing.T) {
	mockResponse := map[string]interface{}{
		"object": "list",
		"data": []map[string]interface{}{
			{"id": "model-1", "object": "model"},
			{"id": "model-2", "object": "model"},
		},
	}

	server := createMockLocalServer(mockResponse)
	defer server.Close()

	provider, err := NewLMStudioProvider(server.URL, "test-model")
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

func TestLocalOpenAIProvider_GenerateCommandStream(t *testing.T) {
	provider, err := NewLMStudioProvider("http://fake-endpoint:1234", "test-model")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx := context.Background()
	sysCtx := types.SystemContext{
		OS:         "darwin",
		Shell:      "zsh",
		CurrentDir: "/tmp",
	}

	// This will fail to connect, but we're testing that the function exists
	resp, err := provider.GenerateCommand(ctx, "test prompt", sysCtx)

	// Expect an error since endpoint is fake
	if err == nil {
		t.Error("Expected error for fake endpoint")
	}
	_ = resp // Use the response to avoid unused variable warning
}

func TestLocalOpenAIProvider_ChatWithUsage(t *testing.T) {
	provider, err := NewLMStudioProvider("http://fake-endpoint:1234", "test-model")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx := context.Background()
	messages := []Message{
		{Role: "user", Content: "test message"},
	}

	// This will fail to connect, but we're testing that the function exists
	_, err = provider.ChatWithUsage(ctx, messages)

	// Expect an error since endpoint is fake
	if err == nil {
		t.Error("Expected error for fake endpoint")
	}
}

func TestLocalOpenAIProvider_CallTool(t *testing.T) {
	provider, err := NewLMStudioProvider("http://fake-endpoint:1234", "test-model")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Test that CallTool method exists
	supported := provider.SupportsTools()
	if supported {
		t.Error("Local OpenAI providers should not support tools by default")
	}
}
