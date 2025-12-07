// Package ai OpenAI provider tests
package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/soroush/sosomi/internal/types"
)

func TestOpenAIProvider_Name(t *testing.T) {
	provider, err := NewOpenAIProvider("test-key", "", "gpt-4o")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	if provider.Name() != "openai" {
		t.Errorf("Expected name 'openai', got '%s'", provider.Name())
	}
}

func TestOpenAIProvider_SupportsTools(t *testing.T) {
	provider, err := NewOpenAIProvider("test-key", "", "gpt-4o")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// OpenAI should support tools
	if !provider.SupportsTools() {
		t.Error("Expected OpenAI to support tools")
	}
}

func TestParseCommandResponse(t *testing.T) {
	validJSON := `{
		"command": "ls -la",
		"explanation": "List all files",
		"risk_level": "safe",
		"confidence": 0.95,
		"warnings": [],
		"alternatives": ["ls -l"]
	}`

	resp, err := parseCommandResponse(validJSON)
	if err != nil {
		t.Fatalf("parseCommandResponse failed: %v", err)
	}

	if resp.Command != "ls -la" {
		t.Errorf("Expected command 'ls -la', got '%s'", resp.Command)
	}
	if resp.Explanation != "List all files" {
		t.Errorf("Expected explanation 'List all files', got '%s'", resp.Explanation)
	}
	if resp.RiskLevel != types.RiskSafe {
		t.Errorf("Expected RiskSafe, got %v", resp.RiskLevel)
	}
	if resp.Confidence != 0.95 {
		t.Errorf("Expected confidence 0.95, got %f", resp.Confidence)
	}
}

func TestParseCommandResponse_RiskLevels(t *testing.T) {
	tests := []struct {
		riskStr  string
		expected types.RiskLevel
	}{
		{"safe", types.RiskSafe},
		{"caution", types.RiskCaution},
		{"dangerous", types.RiskDangerous},
		{"critical", types.RiskCritical},
		{"SAFE", types.RiskSafe},
		{"DANGEROUS", types.RiskDangerous},
	}

	for _, tt := range tests {
		t.Run(tt.riskStr, func(t *testing.T) {
			jsonStr := `{"command": "test", "explanation": "", "risk_level": "` + tt.riskStr + `", "confidence": 1.0}`
			resp, err := parseCommandResponse(jsonStr)
			if err != nil {
				t.Fatalf("parseCommandResponse failed: %v", err)
			}
			if resp.RiskLevel != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, resp.RiskLevel)
			}
		})
	}
}

func TestParseCommandResponse_InvalidJSON(t *testing.T) {
	// When JSON parsing fails, the function treats the content as the command itself
	resp, err := parseCommandResponse("not valid json")
	if err != nil {
		t.Errorf("parseCommandResponse should not return error, got: %v", err)
	}
	if resp.Command != "not valid json" {
		t.Errorf("Expected command 'not valid json', got '%s'", resp.Command)
	}
	// Should default to CAUTION risk level for unparseable responses
	if resp.RiskLevel != types.RiskCaution {
		t.Errorf("Expected RiskCaution for unparseable response, got %v", resp.RiskLevel)
	}
}

func TestParseCommandResponse_WithCodeBlock(t *testing.T) {
	// Some models wrap response in code blocks
	jsonWithCodeBlock := "```json\n{\"command\": \"ls\", \"explanation\": \"list\", \"risk_level\": \"safe\", \"confidence\": 1.0}\n```"
	resp, err := parseCommandResponse(jsonWithCodeBlock)
	if err != nil {
		t.Fatalf("parseCommandResponse should handle code blocks: %v", err)
	}
	if resp.Command != "ls" {
		t.Errorf("Expected command 'ls', got '%s'", resp.Command)
	}
}

// MockOpenAI server for testing
func createMockOpenAIServer(response interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}

func TestOpenAIProvider_WithMockServer(t *testing.T) {
	mockResponse := map[string]interface{}{
		"id":     "test-id",
		"object": "chat.completion",
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": `{"command": "echo hello", "explanation": "Print hello", "risk_level": "safe", "confidence": 0.99}`,
				},
				"finish_reason": "stop",
			},
		},
	}

	server := createMockOpenAIServer(mockResponse)
	defer server.Close()

	provider, err := NewOpenAIProvider("test-key", server.URL, "gpt-4o")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx := context.Background()
	sysCtx := types.SystemContext{
		OS:         "darwin",
		Shell:      "zsh",
		CurrentDir: "/tmp",
		HomeDir:    "/Users/test",
		Username:   "test",
	}

	resp, err := provider.GenerateCommand(ctx, "print hello", sysCtx)
	if err != nil {
		t.Fatalf("GenerateCommand failed: %v", err)
	}

	if resp.Command != "echo hello" {
		t.Errorf("Expected command 'echo hello', got '%s'", resp.Command)
	}
}

func TestOpenAIProvider_Chat_WithMock(t *testing.T) {
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

	server := createMockOpenAIServer(mockResponse)
	defer server.Close()

	provider, err := NewOpenAIProvider("test-key", server.URL, "gpt-4o")
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

func TestOpenAIProvider_RefineCommand_WithMock(t *testing.T) {
	mockResponse := map[string]interface{}{
		"id":     "test-id",
		"object": "chat.completion",
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": `{"command": "ls -lS", "explanation": "List files sorted by size (macOS compatible)", "risk_level": "safe", "confidence": 0.99}`,
				},
				"finish_reason": "stop",
			},
		},
	}

	server := createMockOpenAIServer(mockResponse)
	defer server.Close()

	provider, err := NewOpenAIProvider("test-key", server.URL, "gpt-4o")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx := context.Background()
	sysCtx := types.SystemContext{
		OS:         "darwin",
		Shell:      "zsh",
		CurrentDir: "/tmp",
	}

	req := RefineRequest{
		OriginalPrompt: "list files by size",
		GeneratedCmd:   "ls --sort=size",
		Feedback:       "doesn't work on macOS",
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

func TestOpenAIProvider_GenerateCommand_FakeEndpoint(t *testing.T) {
	provider, err := NewOpenAIProvider("test-key", "http://fake-endpoint", "gpt-4o")
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
	_, err = provider.GenerateCommand(ctx, "test prompt", sysCtx)

	// Expect an error since endpoint is fake
	if err == nil {
		t.Error("Expected error for fake endpoint")
	}
}

func TestOpenAIProvider_Chat_FakeEndpoint(t *testing.T) {
	provider, err := NewOpenAIProvider("test-key", "http://fake-endpoint", "gpt-4o")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx := context.Background()
	messages := []Message{
		{Role: "user", Content: "test message"},
	}

	// This will fail to connect, but we're testing that the function exists
	_, err = provider.Chat(ctx, messages)

	// Expect an error since endpoint is fake
	if err == nil {
		t.Error("Expected error for fake endpoint")
	}
}

func TestOpenAIProvider_ListModels_FakeEndpoint(t *testing.T) {
	provider, err := NewOpenAIProvider("test-key", "http://fake-endpoint", "gpt-4o")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx := context.Background()

	// Test that ListModels method exists and returns error for fake endpoint
	_, err = provider.ListModels(ctx)
	if err == nil {
		t.Error("Expected error for fake endpoint")
	}
}
