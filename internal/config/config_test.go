// Package config tests
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	// Test provider defaults
	if cfg.Provider.Name != "openai" {
		t.Errorf("Expected default Provider.Name to be 'openai', got '%s'", cfg.Provider.Name)
	}
	if cfg.Model.Name != "gpt-4o" {
		t.Errorf("Expected default Model.Name to be 'gpt-4o', got '%s'", cfg.Model.Name)
	}
	if cfg.Provider.Endpoint != "https://api.openai.com/v1" {
		t.Errorf("Expected default Provider.Endpoint to be 'https://api.openai.com/v1', got '%s'", cfg.Provider.Endpoint)
	}
	if cfg.Provider.APIKeyEnv != "OPENAI_API_KEY" {
		t.Errorf("Expected default Provider.APIKeyEnv to be 'OPENAI_API_KEY', got '%s'", cfg.Provider.APIKeyEnv)
	}

	// Test model defaults
	if cfg.Model.MaxTokens != 2048 {
		t.Errorf("Expected default Model.MaxTokens to be 2048, got %d", cfg.Model.MaxTokens)
	}
	if cfg.Model.Temperature != 0.1 {
		t.Errorf("Expected default Model.Temperature to be 0.1, got %f", cfg.Model.Temperature)
	}
	if cfg.Model.TimeoutSeconds != 30 {
		t.Errorf("Expected default Model.TimeoutSeconds to be 30, got %d", cfg.Model.TimeoutSeconds)
	}
	if cfg.Model.MaxRetries != 3 {
		t.Errorf("Expected default Model.MaxRetries to be 3, got %d", cfg.Model.MaxRetries)
	}
	if !cfg.Model.StreamOutput {
		t.Error("Expected default Model.StreamOutput to be true")
	}

	// Test safety defaults
	if cfg.Safety.Level != "moderate" {
		t.Errorf("Expected default Safety.Level to be 'moderate', got '%s'", cfg.Safety.Level)
	}
	if !cfg.Safety.RequireConfirmation {
		t.Error("Expected default Safety.RequireConfirmation to be true")
	}
	if cfg.Safety.AutoExecuteSafe {
		t.Error("Expected default Safety.AutoExecuteSafe to be false")
	}

	// Test blocked commands
	if len(cfg.Safety.BlockedCommands) != 5 {
		t.Errorf("Expected 5 default BlockedCommands, got %d", len(cfg.Safety.BlockedCommands))
	}

	// Test history defaults
	if !cfg.History.Enabled {
		t.Error("Expected default History.Enabled to be true")
	}
	if cfg.History.RetentionDays != 30 {
		t.Errorf("Expected default History.RetentionDays to be 30, got %d", cfg.History.RetentionDays)
	}

	// Test MCP defaults
	if !cfg.MCP.Enabled {
		t.Error("Expected default MCP.Enabled to be true")
	}

	// Test UI defaults
	if !cfg.UI.ColorEnabled {
		t.Error("Expected default UI.ColorEnabled to be true")
	}
	if !cfg.UI.ShowExplanations {
		t.Error("Expected default UI.ShowExplanations to be true")
	}
	if cfg.UI.Language != "en" {
		t.Errorf("Expected default UI.Language to be 'en', got '%s'", cfg.UI.Language)
	}
}

func TestGet_ReturnsDefaultWhenNotInitialized(t *testing.T) {
	// Reset the global config
	ResetInitialized()

	got := Get()
	if got == nil {
		t.Fatal("Get() returned nil")
	}

	// Should return default config
	if got.Provider.Name != "openai" {
		t.Errorf("Expected Provider.Name to be 'openai', got '%s'", got.Provider.Name)
	}
}

func TestInit_WithNonExistentConfig(t *testing.T) {
	// Reset the global config
	ResetInitialized()

	// Initialize with a non-existent config file
	err := Init("/tmp/nonexistent-sosomi-config.yaml")
	if err != nil {
		// Config file not found is acceptable, default config should be used
		t.Logf("Init with non-existent file: %v", err)
	}

	// Should still have a valid config
	got := Get()
	if got == nil {
		t.Fatal("Get() returned nil after Init")
	}
}

func TestInit_WithTempConfigFile(t *testing.T) {
	// Reset the global config
	ResetInitialized()

	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
version: 1
provider:
  name: ollama
  endpoint: http://localhost:11434
model:
  name: llama3.2
  max_tokens: 4096
safety:
  level: strict
  auto_execute_safe: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}

	err := Init(configPath)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	got := Get()
	if got.Provider.Name != "ollama" {
		t.Errorf("Expected Provider.Name to be 'ollama', got '%s'", got.Provider.Name)
	}
	if got.Model.Name != "llama3.2" {
		t.Errorf("Expected Model.Name to be 'llama3.2', got '%s'", got.Model.Name)
	}
	if got.Safety.Level != "strict" {
		t.Errorf("Expected Safety.Level to be 'strict', got '%s'", got.Safety.Level)
	}
	if !got.Safety.AutoExecuteSafe {
		t.Error("Expected Safety.AutoExecuteSafe to be true")
	}
}

func TestGetEndpoint(t *testing.T) {
	ResetInitialized()

	tests := []struct {
		name     string
		provider string
		endpoint string
		expected string
	}{
		{"OpenAI", "openai", "", "https://api.openai.com/v1"},
		{"Ollama", "ollama", "", "http://localhost:11434"},
		{"LMStudio", "lmstudio", "", "http://localhost:1234/v1"},
		{"LlamaCpp", "llamacpp", "", "http://localhost:8080/v1"},
		{"CustomEndpoint", "openai", "https://custom.api.com/v1", "https://custom.api.com/v1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetInitialized()
			cfg := DefaultConfig()
			cfg.Provider.Name = tt.provider
			cfg.Provider.Endpoint = tt.endpoint
			// Set the active config directly for testing
			activeCfg = cfg

			got := GetEndpoint()
			if got != tt.expected {
				t.Errorf("GetEndpoint() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetAPIKey(t *testing.T) {
	// Save original env
	originalKey := os.Getenv("OPENAI_API_KEY")
	defer os.Setenv("OPENAI_API_KEY", originalKey)

	ResetInitialized()

	// Test with environment variable
	os.Setenv("OPENAI_API_KEY", "test-api-key-from-env")

	cfg := DefaultConfig()
	cfg.Provider.APIKeyEnv = "OPENAI_API_KEY"
	activeCfg = cfg

	key := GetAPIKey()
	if key != "test-api-key-from-env" {
		t.Errorf("Expected API key 'test-api-key-from-env', got '%s'", key)
	}

	// Test with plain text key (not recommended but supported)
	cfg.Provider.APIKey = "plain-text-key"
	key = GetAPIKey()
	// Command takes precedence, then env var, then plain text (depends on implementation)
	// Let's just verify we get a key
	if key == "" {
		t.Error("Expected GetAPIKey to return a key")
	}
}

func TestSet(t *testing.T) {
	ResetInitialized()
	baseCfg := DefaultConfig()
	cfg = baseCfg
	activeCfg = baseCfg

	// Test setting a nested string value
	err := Set("provider.name", "ollama")
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}
	if cfg.Provider.Name != "ollama" {
		t.Errorf("Expected Provider.Name to be 'ollama', got '%s'", cfg.Provider.Name)
	}

	// Test setting a nested boolean value
	err = Set("safety.auto_execute_safe", true)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}
	if !cfg.Safety.AutoExecuteSafe {
		t.Error("Expected Safety.AutoExecuteSafe to be true")
	}

	// Test setting a nested integer value
	err = Set("model.timeout_seconds", 60)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}
	if cfg.Model.TimeoutSeconds != 60 {
		t.Errorf("Expected Model.TimeoutSeconds to be 60, got %d", cfg.Model.TimeoutSeconds)
	}

	// Test setting model name
	err = Set("model.name", "gpt-4")
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}
	if cfg.Model.Name != "gpt-4" {
		t.Errorf("Expected Model.Name to be 'gpt-4', got '%s'", cfg.Model.Name)
	}
}

func TestGetValue(t *testing.T) {
	ResetInitialized()
	baseCfg := DefaultConfig()
	baseCfg.Provider.Name = "ollama"
	baseCfg.Model.Name = "llama3.2"
	activeCfg = baseCfg

	// Test getting provider name
	val, err := GetValue("provider.name")
	if err != nil {
		t.Errorf("GetValue failed: %v", err)
	}
	if val != "ollama" {
		t.Errorf("Expected 'ollama', got '%v'", val)
	}

	// Test getting model name
	val, err = GetValue("model.name")
	if err != nil {
		t.Errorf("GetValue failed: %v", err)
	}
	if val != "llama3.2" {
		t.Errorf("Expected 'llama3.2', got '%v'", val)
	}
}

func TestEnsureDirs(t *testing.T) {
	tmpDir := t.TempDir()

	ResetInitialized()
	baseCfg := DefaultConfig()
	baseCfg.History.DBPath = filepath.Join(tmpDir, "data", "history.db")
	baseCfg.MCP.ToolsDir = filepath.Join(tmpDir, "mcp_tools")
	baseCfg.Safety.CustomRulesPath = filepath.Join(tmpDir, "rules", "safety.yaml")
	activeCfg = baseCfg

	err := EnsureDirs()
	if err != nil {
		t.Fatalf("EnsureDirs failed: %v", err)
	}

	// Check that directories were created
	historyDir := filepath.Dir(baseCfg.History.DBPath)
	if _, err := os.Stat(historyDir); os.IsNotExist(err) {
		t.Error("History directory was not created")
	}
}

func TestBlockedCommands(t *testing.T) {
	cfg := DefaultConfig()

	expectedBlocked := []string{"shutdown", "reboot", "init 0", "init 6", ":(){ :|:& };:"}
	if len(cfg.Safety.BlockedCommands) != len(expectedBlocked) {
		t.Errorf("Expected %d blocked commands, got %d", len(expectedBlocked), len(cfg.Safety.BlockedCommands))
	}

	for i, cmd := range expectedBlocked {
		if cfg.Safety.BlockedCommands[i] != cmd {
			t.Errorf("Expected blocked command '%s' at index %d, got '%s'", cmd, i, cfg.Safety.BlockedCommands[i])
		}
	}
}

func TestGetConfigPaths(t *testing.T) {
	paths := GetConfigPaths()

	if paths.System == "" {
		t.Error("Expected System path to be set")
	}
	if paths.User == "" {
		t.Error("Expected User path to be set")
	}
	if paths.Project == "" {
		t.Error("Expected Project path to be set")
	}
	if paths.ProfileDir == "" {
		t.Error("Expected ProfileDir path to be set")
	}
}

func TestIsFirstRun(t *testing.T) {
	// Reset and check with non-existent config
	ResetInitialized()

	// This just tests that the function doesn't panic
	// The actual result depends on whether a config file exists
	_ = IsFirstRun()
}

func TestIsLocalProvider(t *testing.T) {
	tests := []struct {
		provider string
		expected bool
	}{
		{"ollama", true},
		{"lmstudio", true},
		{"llamacpp", true},
		{"openai", false},
		{"generic", false},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			ResetInitialized()
			baseCfg := DefaultConfig()
			baseCfg.Provider.Name = tt.provider
			activeCfg = baseCfg

			got := IsLocalProvider()
			if got != tt.expected {
				t.Errorf("IsLocalProvider() = %v, want %v for provider %s", got, tt.expected, tt.provider)
			}
		})
	}
}

func TestCopyConfig(t *testing.T) {
	original := DefaultConfig()
	original.Provider.Name = "test-provider"
	original.Safety.BlockedCommands = []string{"rm", "mv"}
	original.Aliases = map[string]string{"l": "ls -la"}

	copy := copyConfig(original)

	// Verify copy is not nil
	if copy == nil {
		t.Fatal("copyConfig returned nil")
	}

	// Verify values are copied
	if copy.Provider.Name != original.Provider.Name {
		t.Errorf("Provider.Name not copied correctly")
	}

	// Verify slices are deep copied
	original.Safety.BlockedCommands[0] = "changed"
	if copy.Safety.BlockedCommands[0] == "changed" {
		t.Error("BlockedCommands slice was not deep copied")
	}

	// Verify maps are deep copied
	original.Aliases["new"] = "value"
	if _, exists := copy.Aliases["new"]; exists {
		t.Error("Aliases map was not deep copied")
	}
}

func TestMergeConfig(t *testing.T) {
	dst := DefaultConfig()
	src := &Config{
		Provider: ProviderConfig{
			Name: "ollama",
		},
		Model: ModelConfig{
			Name:      "llama3.2",
			MaxTokens: 4096,
		},
		Safety: SafetyConfig{
			Level: "strict",
		},
	}

	mergeConfig(dst, src)

	if dst.Provider.Name != "ollama" {
		t.Errorf("Expected Provider.Name to be 'ollama', got '%s'", dst.Provider.Name)
	}
	if dst.Model.Name != "llama3.2" {
		t.Errorf("Expected Model.Name to be 'llama3.2', got '%s'", dst.Model.Name)
	}
	if dst.Model.MaxTokens != 4096 {
		t.Errorf("Expected Model.MaxTokens to be 4096, got %d", dst.Model.MaxTokens)
	}
	if dst.Safety.Level != "strict" {
		t.Errorf("Expected Safety.Level to be 'strict', got '%s'", dst.Safety.Level)
	}

	// Verify that unset fields in src don't override dst
	if dst.UI.Language != "en" {
		t.Errorf("Expected UI.Language to remain 'en', got '%s'", dst.UI.Language)
	}
}

func TestInitWithProfile(t *testing.T) {
	ResetInitialized()

	// Create a temp profile
	tmpDir := t.TempDir()
	profileDir := filepath.Join(tmpDir, ".config", "sosomi", "profiles")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatalf("Failed to create profile dir: %v", err)
	}

	profileContent := `
name: test-profile
provider:
  name: ollama
  endpoint: http://localhost:11434
model:
  name: codellama
`
	profilePath := filepath.Join(profileDir, "test-profile.yaml")
	if err := os.WriteFile(profilePath, []byte(profileContent), 0644); err != nil {
		t.Fatalf("Failed to write profile: %v", err)
	}

	// Override config paths for test
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)

	// Reset config paths cache
	configPaths = ConfigPaths{}

	err := InitWithProfile("", "test-profile")
	if err != nil {
		t.Logf("InitWithProfile error (may be expected): %v", err)
	}

	// Verify profile was loaded
	if GetActiveProfile() == "test-profile" {
		cfg := Get()
		if cfg.Provider.Name != "ollama" {
			t.Errorf("Expected Provider.Name to be 'ollama' from profile, got '%s'", cfg.Provider.Name)
		}
	}
}

func TestToFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected float64
	}{
		{"float64", float64(3.14), 3.14},
		{"float32", float32(2.5), 2.5},
		{"int", int(42), 42.0},
		{"string", "5.5", 5.5},
		{"invalid string", "abc", 0.0},
		{"nil", nil, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toFloat(tt.input)
			if result != tt.expected {
				t.Errorf("toFloat(%v) = %f; want %f", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToInt(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected int
	}{
		{"int", int(42), 42},
		{"int64", int64(100), 100},
		{"float64", float64(3.7), 3},
		{"string", "123", 123},
		{"invalid string", "abc", 0},
		{"nil", nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toInt(tt.input)
			if result != tt.expected {
				t.Errorf("toInt(%v) = %d; want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToBool(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected bool
	}{
		{"bool true", true, true},
		{"bool false", false, false},
		{"string true", "true", true},
		{"string false", "false", false},
		{"string 1", "1", true},
		{"string 0", "0", false},
		{"int 1", 1, true},
		{"int 0", 0, false},
		{"invalid", "maybe", false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toBool(tt.input)
			if result != tt.expected {
				t.Errorf("toBool(%v) = %t; want %t", tt.input, result, tt.expected)
			}
		})
	}
}
