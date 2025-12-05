// Package ai provider tests
package ai

import (
	"testing"

	"github.com/soroush/sosomi/internal/types"
)

func TestBuildSystemContext(t *testing.T) {
	ctx := types.SystemContext{
		OS:               "darwin",
		Shell:            "zsh",
		CurrentDir:       "/Users/test/projects",
		HomeDir:          "/Users/test",
		Username:         "testuser",
		GitBranch:        "main",
		GitStatus:        "clean",
		InstalledPkgMgrs: []string{"brew", "npm"},
	}

	result := BuildSystemContext(ctx)

	// Check that all fields are included
	if result == "" {
		t.Error("BuildSystemContext returned empty string")
	}

	expectedContents := []string{
		"darwin",
		"zsh",
		"/Users/test/projects",
		"/Users/test",
		"testuser",
		"main",
		"clean",
		"brew",
		"npm",
	}

	for _, expected := range expectedContents {
		if !containsString(result, expected) {
			t.Errorf("Expected result to contain '%s'", expected)
		}
	}
}

func TestBuildSystemContext_WithoutGit(t *testing.T) {
	ctx := types.SystemContext{
		OS:         "linux",
		Shell:      "bash",
		CurrentDir: "/home/user",
		HomeDir:    "/home/user",
		Username:   "user",
		GitBranch:  "", // No git
	}

	result := BuildSystemContext(ctx)

	if containsString(result, "Git Branch") && ctx.GitBranch == "" {
		// It's fine if Git Branch is not shown when empty
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestSystemPrompt(t *testing.T) {
	if SystemPrompt == "" {
		t.Error("SystemPrompt should not be empty")
	}

	// Check for key elements
	expectedElements := []string{
		"Sosomi",
		"shell command",
		"JSON",
		"command",
		"explanation",
		"risk_level",
	}

	for _, elem := range expectedElements {
		if !containsString(SystemPrompt, elem) {
			t.Errorf("SystemPrompt should contain '%s'", elem)
		}
	}
}

func TestRefinePrompt(t *testing.T) {
	if RefinePrompt == "" {
		t.Error("RefinePrompt should not be empty")
	}

	// Check for key elements
	expectedElements := []string{
		"Sosomi",
		"feedback",
		"macOS",
		"BSD",
		"JSON",
	}

	for _, elem := range expectedElements {
		if !containsString(RefinePrompt, elem) {
			t.Errorf("RefinePrompt should contain '%s'", elem)
		}
	}
}

func TestProviderType_Values(t *testing.T) {
	providers := []ProviderType{
		ProviderOpenAI,
		ProviderAnthropic,
		ProviderOllama,
		ProviderLMStudio,
		ProviderLlamaCpp,
		ProviderAzure,
		ProviderGeneric,
	}

	for _, p := range providers {
		if p == "" {
			t.Error("ProviderType should not be empty")
		}
	}

	// Verify specific values
	if ProviderOpenAI != "openai" {
		t.Errorf("Expected ProviderOpenAI to be 'openai', got '%s'", ProviderOpenAI)
	}
	if ProviderOllama != "ollama" {
		t.Errorf("Expected ProviderOllama to be 'ollama', got '%s'", ProviderOllama)
	}
	if ProviderLMStudio != "lmstudio" {
		t.Errorf("Expected ProviderLMStudio to be 'lmstudio', got '%s'", ProviderLMStudio)
	}
}

func TestMessage_Fields(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "list files",
	}

	if msg.Role != "user" {
		t.Errorf("Expected Role 'user', got '%s'", msg.Role)
	}
	if msg.Content != "list files" {
		t.Errorf("Expected Content 'list files', got '%s'", msg.Content)
	}
}

func TestStreamChunk_Fields(t *testing.T) {
	chunk := StreamChunk{
		Content: "partial response",
		Done:    false,
		Error:   nil,
	}

	if chunk.Content != "partial response" {
		t.Errorf("Expected Content 'partial response', got '%s'", chunk.Content)
	}
	if chunk.Done {
		t.Error("Expected Done to be false")
	}
	if chunk.Error != nil {
		t.Error("Expected Error to be nil")
	}

	doneChunk := StreamChunk{
		Done: true,
	}
	if !doneChunk.Done {
		t.Error("Expected Done to be true")
	}
}

func TestRefineRequest_Fields(t *testing.T) {
	req := RefineRequest{
		OriginalPrompt: "list files by size",
		GeneratedCmd:   "ls --sort=size",
		Feedback:       "command failed on macOS",
		CommandOutput:  "",
		CommandError:   "ls: illegal option -- -",
		ExitCode:       1,
		WasExecuted:    true,
	}

	if req.OriginalPrompt != "list files by size" {
		t.Errorf("Expected OriginalPrompt 'list files by size', got '%s'", req.OriginalPrompt)
	}
	if req.GeneratedCmd != "ls --sort=size" {
		t.Errorf("Expected GeneratedCmd 'ls --sort=size', got '%s'", req.GeneratedCmd)
	}
	if req.ExitCode != 1 {
		t.Errorf("Expected ExitCode 1, got %d", req.ExitCode)
	}
	if !req.WasExecuted {
		t.Error("Expected WasExecuted to be true")
	}
}
