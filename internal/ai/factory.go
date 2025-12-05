// Package ai provides factory functions for creating AI providers
package ai

import (
	"fmt"

	"github.com/soroush/sosomi/internal/config"
)

// NewProvider creates a new AI provider based on configuration
func NewProvider(providerType, apiKey, endpoint, model string) (Provider, error) {
	switch providerType {
	case "openai":
		return NewOpenAIProvider(apiKey, endpoint, model)
	case "ollama":
		return NewOllamaProvider(endpoint, model)
	case "lmstudio":
		return NewLMStudioProvider(endpoint, model)
	case "llamacpp":
		return NewLlamaCppProvider(endpoint, model)
	case "generic":
		return NewGenericOpenAIProvider(apiKey, endpoint, model)
	default:
		return nil, fmt.Errorf("unknown provider: %s", providerType)
	}
}

// NewProviderFromConfig creates a provider using the current configuration
func NewProviderFromConfig() (Provider, error) {
	cfg := config.Get()
	endpoint := config.GetEndpoint()
	apiKey := config.GetAPIKey()

	return NewProvider(cfg.Provider.Name, apiKey, endpoint, cfg.Model.Name)
}

// AvailableProviders returns a list of available provider types
func AvailableProviders() []string {
	return []string{
		"openai",
		"ollama",
		"lmstudio",
		"llamacpp",
		"generic",
	}
}

// RecommendedModels returns recommended models for each provider
func RecommendedModels() map[string][]string {
	return map[string][]string{
		"openai": {
			"gpt-4o",
			"gpt-4o-mini",
			"gpt-4-turbo",
			"gpt-4",
			"gpt-3.5-turbo",
		},
		"ollama": {
			"llama3.2",
			"llama3.1",
			"codellama",
			"mistral",
			"mixtral",
			"qwen2.5-coder",
			"deepseek-coder-v2",
		},
		"lmstudio": {
			"local-model", // Use whatever is loaded
		},
		"llamacpp": {
			"local-model",
		},
	}
}
