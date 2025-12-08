// Package ai factory tests
package ai

import (
	"os"
	"testing"

	"github.com/sonemaro/sosomi/internal/config"
)

func TestAvailableProviders(t *testing.T) {
	providers := AvailableProviders()

	if len(providers) == 0 {
		t.Error("AvailableProviders should return at least one provider")
	}

	expectedProviders := []string{"openai", "ollama", "lmstudio", "llamacpp", "generic"}
	for _, expected := range expectedProviders {
		found := false
		for _, p := range providers {
			if p == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected provider '%s' in AvailableProviders", expected)
		}
	}
}

func TestRecommendedModels(t *testing.T) {
	models := RecommendedModels()

	if len(models) == 0 {
		t.Error("RecommendedModels should return at least one provider")
	}

	// Check OpenAI models
	openaiModels, ok := models["openai"]
	if !ok {
		t.Error("Expected openai in RecommendedModels")
	}
	if len(openaiModels) == 0 {
		t.Error("Expected at least one OpenAI model")
	}

	// Check that gpt-4o is recommended
	hasGpt4o := false
	for _, m := range openaiModels {
		if m == "gpt-4o" {
			hasGpt4o = true
			break
		}
	}
	if !hasGpt4o {
		t.Error("Expected gpt-4o in OpenAI recommended models")
	}

	// Check Ollama models
	ollamaModels, ok := models["ollama"]
	if !ok {
		t.Error("Expected ollama in RecommendedModels")
	}
	if len(ollamaModels) == 0 {
		t.Error("Expected at least one Ollama model")
	}
}

func TestNewProvider_OpenAI(t *testing.T) {
	// Skip if no API key (would fail validation)
	provider, err := NewProvider("openai", "test-key", "", "gpt-4o")
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}

	if provider == nil {
		t.Fatal("NewProvider returned nil")
	}

	if provider.Name() != "openai" {
		t.Errorf("Expected name 'openai', got '%s'", provider.Name())
	}
}

func TestNewProvider_OpenAI_NoKey(t *testing.T) {
	// Initialize config to avoid nil pointer dereference
	config.Init("")

	// Clear any existing API key in config
	origKey := os.Getenv("OPENAI_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("SOSOMI_API_KEY")
	defer func() {
		if origKey != "" {
			os.Setenv("OPENAI_API_KEY", origKey)
		}
	}()

	_, err := NewProvider("openai", "", "", "gpt-4o")
	if err == nil {
		t.Error("Expected error when creating OpenAI provider without API key")
	}
}

func TestNewProvider_Ollama(t *testing.T) {
	provider, err := NewProvider("ollama", "", "http://localhost:11434", "llama3.2")
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}

	if provider == nil {
		t.Fatal("NewProvider returned nil")
	}

	if provider.Name() != "ollama" {
		t.Errorf("Expected name 'ollama', got '%s'", provider.Name())
	}
}

func TestNewProvider_LMStudio(t *testing.T) {
	provider, err := NewProvider("lmstudio", "", "http://localhost:1234/v1", "local-model")
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}

	if provider == nil {
		t.Fatal("NewProvider returned nil")
	}

	if provider.Name() != "lmstudio" {
		t.Errorf("Expected name 'lmstudio', got '%s'", provider.Name())
	}
}

func TestNewProvider_LlamaCpp(t *testing.T) {
	provider, err := NewProvider("llamacpp", "", "http://localhost:8080/v1", "local-model")
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}

	if provider == nil {
		t.Fatal("NewProvider returned nil")
	}

	if provider.Name() != "llamacpp" {
		t.Errorf("Expected name 'llamacpp', got '%s'", provider.Name())
	}
}

func TestNewProvider_Generic(t *testing.T) {
	provider, err := NewProvider("generic", "api-key", "http://example.com/v1", "model")
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}

	if provider == nil {
		t.Fatal("NewProvider returned nil")
	}

	if provider.Name() != "generic" {
		t.Errorf("Expected name 'generic', got '%s'", provider.Name())
	}
}

func TestNewProvider_Unknown(t *testing.T) {
	_, err := NewProvider("unknown_provider", "", "", "")
	if err == nil {
		t.Error("Expected error for unknown provider")
	}
}

func TestNewOllamaProvider(t *testing.T) {
	provider, err := NewOllamaProvider("", "")
	if err != nil {
		t.Fatalf("NewOllamaProvider failed: %v", err)
	}

	if provider.Name() != "ollama" {
		t.Errorf("Expected name 'ollama', got '%s'", provider.Name())
	}
}

func TestNewOllamaProvider_WithEndpoint(t *testing.T) {
	provider, err := NewOllamaProvider("http://custom:11434", "custom-model")
	if err != nil {
		t.Fatalf("NewOllamaProvider failed: %v", err)
	}

	if provider.model != "custom-model" {
		t.Errorf("Expected model 'custom-model', got '%s'", provider.model)
	}
}

func TestNewLMStudioProvider(t *testing.T) {
	provider, err := NewLMStudioProvider("", "")
	if err != nil {
		t.Fatalf("NewLMStudioProvider failed: %v", err)
	}

	if provider.Name() != "lmstudio" {
		t.Errorf("Expected name 'lmstudio', got '%s'", provider.Name())
	}
}

func TestNewLlamaCppProvider(t *testing.T) {
	provider, err := NewLlamaCppProvider("", "")
	if err != nil {
		t.Fatalf("NewLlamaCppProvider failed: %v", err)
	}

	if provider.Name() != "llamacpp" {
		t.Errorf("Expected name 'llamacpp', got '%s'", provider.Name())
	}
}

func TestNewGenericOpenAIProvider(t *testing.T) {
	provider, err := NewGenericOpenAIProvider("key", "http://example.com/v1", "model")
	if err != nil {
		t.Fatalf("NewGenericOpenAIProvider failed: %v", err)
	}

	if provider.Name() != "generic" {
		t.Errorf("Expected name 'generic', got '%s'", provider.Name())
	}
}

func TestNewGenericOpenAIProvider_NoEndpoint(t *testing.T) {
	_, err := NewGenericOpenAIProvider("key", "", "model")
	if err == nil {
		t.Error("Expected error when creating generic provider without endpoint")
	}
}

func TestNewProviderFromConfig_OpenAI(t *testing.T) {
	// Initialize config first
	config.Init("")

	// Set test config values
	cfg := config.Get()
	cfg.Provider.Name = "openai"
	cfg.Provider.APIKey = "test-key"
	cfg.Provider.Endpoint = "https://api.openai.com/v1"
	cfg.Model.Name = "gpt-4o"

	provider, err := NewProviderFromConfig()
	if err != nil {
		t.Fatalf("NewProviderFromConfig failed: %v", err)
	}

	if provider == nil {
		t.Fatal("NewProviderFromConfig returned nil")
	}

	if provider.Name() != "openai" {
		t.Errorf("Expected provider name 'openai', got '%s'", provider.Name())
	}
}

func TestNewProviderFromConfig_Ollama(t *testing.T) {
	config.Init("")

	cfg := config.Get()
	cfg.Provider.Name = "ollama"
	cfg.Provider.Endpoint = "http://localhost:11434"
	cfg.Model.Name = "llama3.2"

	provider, err := NewProviderFromConfig()
	if err != nil {
		t.Fatalf("NewProviderFromConfig failed: %v", err)
	}

	if provider.Name() != "ollama" {
		t.Errorf("Expected provider name 'ollama', got '%s'", provider.Name())
	}
}

func TestNewProviderFromConfig_LMStudio(t *testing.T) {
	config.Init("")

	cfg := config.Get()
	cfg.Provider.Name = "lmstudio"
	cfg.Provider.Endpoint = "http://localhost:1234"
	cfg.Model.Name = "local-model"

	provider, err := NewProviderFromConfig()
	if err != nil {
		t.Fatalf("NewProviderFromConfig failed: %v", err)
	}

	if provider.Name() != "lmstudio" {
		t.Errorf("Expected provider name 'lmstudio', got '%s'", provider.Name())
	}
}

func TestNewProviderFromConfig_LlamaCpp(t *testing.T) {
	config.Init("")

	cfg := config.Get()
	cfg.Provider.Name = "llamacpp"
	cfg.Provider.Endpoint = "http://localhost:8080"
	cfg.Model.Name = "local-model"

	provider, err := NewProviderFromConfig()
	if err != nil {
		t.Fatalf("NewProviderFromConfig failed: %v", err)
	}

	if provider.Name() != "llamacpp" {
		t.Errorf("Expected provider name 'llamacpp', got '%s'", provider.Name())
	}
}

func TestNewProviderFromConfig_Generic(t *testing.T) {
	config.Init("")

	cfg := config.Get()
	cfg.Provider.Name = "generic"
	cfg.Provider.APIKey = "test-key"
	cfg.Provider.Endpoint = "http://custom-endpoint/v1"
	cfg.Model.Name = "custom-model"

	provider, err := NewProviderFromConfig()
	if err != nil {
		t.Fatalf("NewProviderFromConfig failed: %v", err)
	}

	if provider.Name() != "generic" {
		t.Errorf("Expected provider name 'generic', got '%s'", provider.Name())
	}
}

func TestNewProviderFromConfig_UnknownProvider(t *testing.T) {
	config.Init("")

	cfg := config.Get()
	cfg.Provider.Name = "unknown-provider"

	_, err := NewProviderFromConfig()
	if err == nil {
		t.Error("Expected error for unknown provider")
	}
}
