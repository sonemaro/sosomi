// Package config tests for validation
package config

import (
	"os"
	"testing"
)

func TestValidate_ValidConfig(t *testing.T) {
	// Set API key env
	originalKey := os.Getenv("OPENAI_API_KEY")
	defer os.Setenv("OPENAI_API_KEY", originalKey)
	os.Setenv("OPENAI_API_KEY", "test-key")

	cfg := DefaultConfig()
	result := Validate(cfg)

	if !result.IsValid() {
		t.Errorf("Expected valid config, got errors: %s", result.String())
	}
}

func TestValidate_MissingProvider(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Provider.Name = ""

	result := Validate(cfg)

	if result.IsValid() {
		t.Error("Expected invalid config for missing provider")
	}

	// Check for provider error
	found := false
	for _, e := range result.Errors {
		if e.Field == "provider.name" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected error for provider.name field")
	}
}

func TestValidate_UnknownProvider(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Provider.Name = "unknown-provider"

	result := Validate(cfg)

	// Unknown provider should be a warning, not error
	found := false
	for _, w := range result.Warnings {
		if w.Field == "provider.name" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected warning for unknown provider")
	}
}

func TestValidate_MissingModel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Model.Name = ""

	result := Validate(cfg)

	if result.IsValid() {
		t.Error("Expected invalid config for missing model")
	}

	// Check for model error
	found := false
	for _, e := range result.Errors {
		if e.Field == "model.name" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected error for model.name field")
	}
}

func TestValidate_UnusualTemperature(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Model.Temperature = 2.5 // Too high

	result := Validate(cfg)

	// Should be a warning
	found := false
	for _, w := range result.Warnings {
		if w.Field == "model.temperature" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected warning for unusual temperature")
	}
}

func TestValidate_NegativeMaxTokens(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Model.MaxTokens = -1

	result := Validate(cfg)

	if result.IsValid() {
		t.Error("Expected invalid config for negative max_tokens")
	}
}

func TestValidate_InvalidSafetyLevel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Safety.Level = "invalid-level"

	result := Validate(cfg)

	if result.IsValid() {
		t.Error("Expected invalid config for invalid safety level")
	}
}

func TestValidate_DangerousSafetyLevel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Safety.Level = "dangerous"

	result := Validate(cfg)

	// Should have a warning
	found := false
	for _, w := range result.Warnings {
		if w.Field == "safety.level" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected warning for dangerous safety level")
	}
}

func TestValidate_DisabledHistory(t *testing.T) {
	cfg := DefaultConfig()
	cfg.History.Enabled = false

	result := Validate(cfg)

	// Should have a warning
	found := false
	for _, w := range result.Warnings {
		if w.Field == "history.enabled" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected warning for disabled history")
	}
}

func TestValidate_MCPEnabledNoServers(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MCP.Enabled = true
	cfg.MCP.Servers = []string{}

	result := Validate(cfg)

	// Should have a warning
	found := false
	for _, w := range result.Warnings {
		if w.Field == "mcp.servers" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected warning for MCP enabled with no servers")
	}
}

func TestValidate_OpenAI_MissingAPIKey(t *testing.T) {
	// Clear API key env
	originalKey := os.Getenv("OPENAI_API_KEY")
	defer os.Setenv("OPENAI_API_KEY", originalKey)
	os.Unsetenv("OPENAI_API_KEY")

	cfg := DefaultConfig()
	cfg.Provider.Name = "openai"
	cfg.Provider.APIKey = ""
	cfg.Provider.APIKeyEnv = ""
	cfg.Provider.APIKeyCmd = ""

	result := Validate(cfg)

	if result.IsValid() {
		t.Error("Expected invalid config for OpenAI without API key")
	}
}

func TestValidate_OpenAI_PlainTextAPIKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Provider.Name = "openai"
	cfg.Provider.APIKey = "sk-plain-text-key"
	cfg.Provider.APIKeyEnv = ""

	result := Validate(cfg)

	// Should have a warning about plain text key
	found := false
	for _, w := range result.Warnings {
		if w.Field == "provider.api_key" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected warning for plain text API key")
	}
}

func TestValidate_OpenAI_MissingEnvVar(t *testing.T) {
	// Clear the env var
	originalKey := os.Getenv("CUSTOM_API_KEY")
	defer os.Setenv("CUSTOM_API_KEY", originalKey)
	os.Unsetenv("CUSTOM_API_KEY")

	cfg := DefaultConfig()
	cfg.Provider.Name = "openai"
	cfg.Provider.APIKeyEnv = "CUSTOM_API_KEY"

	result := Validate(cfg)

	// Should have an error
	found := false
	for _, e := range result.Errors {
		if e.Field == "provider.api_key_env" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected error for missing env var")
	}
}

func TestValidate_InvalidEndpointURL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Provider.Endpoint = "not-a-valid-url"

	result := Validate(cfg)

	// Should have an error
	found := false
	for _, e := range result.Errors {
		if e.Field == "provider.endpoint" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected error for invalid endpoint URL")
	}
}

func TestValidateCurrent(t *testing.T) {
	// Set API key env
	originalKey := os.Getenv("OPENAI_API_KEY")
	defer os.Setenv("OPENAI_API_KEY", originalKey)
	os.Setenv("OPENAI_API_KEY", "test-key")

	ResetInitialized()
	cfg := DefaultConfig()
	activeCfg = cfg

	result := ValidateCurrent()

	if result == nil {
		t.Fatal("ValidateCurrent returned nil")
	}
}

func TestValidationResult_String(t *testing.T) {
	result := &ValidationResult{
		Errors: []ValidationError{
			{Field: "field1", Message: "error message", Hint: "error hint"},
		},
		Warnings: []ValidationError{
			{Field: "field2", Message: "warning message"},
		},
	}

	str := result.String()

	if str == "" {
		t.Error("Expected non-empty string")
	}

	// Check that it contains error and warning sections
	if !containsStr(str, "Errors:") {
		t.Error("Expected 'Errors:' in output")
	}
	if !containsStr(str, "Warnings:") {
		t.Error("Expected 'Warnings:' in output")
	}
}

func TestValidationResult_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		errors   []ValidationError
		expected bool
	}{
		{"No errors", nil, true},
		{"Empty errors", []ValidationError{}, true},
		{"Has errors", []ValidationError{{Field: "f", Message: "m"}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ValidationResult{Errors: tt.errors}
			if result.IsValid() != tt.expected {
				t.Errorf("Expected IsValid() = %v, got %v", tt.expected, result.IsValid())
			}
		})
	}
}

func TestValidationResult_HasWarnings(t *testing.T) {
	tests := []struct {
		name     string
		warnings []ValidationError
		expected bool
	}{
		{"No warnings", nil, false},
		{"Empty warnings", []ValidationError{}, false},
		{"Has warnings", []ValidationError{{Field: "f", Message: "m"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ValidationResult{Warnings: tt.warnings}
			if result.HasWarnings() != tt.expected {
				t.Errorf("Expected HasWarnings() = %v, got %v", tt.expected, result.HasWarnings())
			}
		})
	}
}

func TestValidationError_String(t *testing.T) {
	tests := []struct {
		name     string
		err      ValidationError
		contains []string
	}{
		{
			name:     "With hint",
			err:      ValidationError{Field: "field", Message: "message", Hint: "hint"},
			contains: []string{"field", "message", "Hint", "hint"},
		},
		{
			name:     "Without hint",
			err:      ValidationError{Field: "field", Message: "message"},
			contains: []string{"field", "message"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			str := tt.err.Error()
			for _, s := range tt.contains {
				if !containsStr(str, s) {
					t.Errorf("Expected '%s' in error string", s)
				}
			}
		})
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
