// Package config - Configuration validation
package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
	Hint    string
}

func (e ValidationError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s: %s\n  Hint: %s", e.Field, e.Message, e.Hint)
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationResult contains all validation errors
type ValidationResult struct {
	Errors   []ValidationError
	Warnings []ValidationError
}

// IsValid returns true if there are no errors
func (r *ValidationResult) IsValid() bool {
	return len(r.Errors) == 0
}

// HasWarnings returns true if there are warnings
func (r *ValidationResult) HasWarnings() bool {
	return len(r.Warnings) > 0
}

// String returns a formatted string of all errors and warnings
func (r *ValidationResult) String() string {
	var sb strings.Builder

	if len(r.Errors) > 0 {
		sb.WriteString("Errors:\n")
		for _, e := range r.Errors {
			sb.WriteString(fmt.Sprintf("  ✗ %s\n", e.Error()))
		}
	}

	if len(r.Warnings) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("Warnings:\n")
		for _, w := range r.Warnings {
			sb.WriteString(fmt.Sprintf("  ⚠ %s\n", w.Error()))
		}
	}

	return sb.String()
}

// Validate validates the configuration and returns all errors and warnings
func Validate(cfg *Config) *ValidationResult {
	result := &ValidationResult{
		Errors:   []ValidationError{},
		Warnings: []ValidationError{},
	}

	validateProvider(cfg, result)
	validateModel(cfg, result)
	validateSafety(cfg, result)
	validateBackup(cfg, result)
	validateHistory(cfg, result)
	validateMCP(cfg, result)

	return result
}

// ValidateCurrent validates the current configuration
func ValidateCurrent() *ValidationResult {
	return Validate(Get())
}

func validateProvider(cfg *Config, result *ValidationResult) {
	// Validate provider name
	validProviders := []string{"openai", "ollama", "lmstudio", "llamacpp", "local", "generic"}
	providerName := strings.ToLower(cfg.Provider.Name)
	
	if providerName == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "provider.name",
			Message: "provider name is required",
			Hint:    "Set provider.name to one of: openai, ollama, lmstudio, llamacpp, local",
		})
	} else if !containsString(validProviders, providerName) {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "provider.name",
			Message: fmt.Sprintf("unknown provider '%s'", providerName),
			Hint:    "Known providers: openai, ollama, lmstudio, llamacpp, local",
		})
	}

	// Validate endpoint for remote providers
	if providerName != "local" && providerName != "" {
		endpoint := cfg.Provider.Endpoint
		if endpoint == "" {
			// Use defaults
			switch providerName {
			case "openai":
				// OpenAI has a default
			case "ollama":
				// Ollama has a default
			case "lmstudio":
				// LM Studio has a default
			case "llamacpp":
				// llamacpp has a default
			default:
				result.Errors = append(result.Errors, ValidationError{
					Field:   "provider.endpoint",
					Message: "endpoint URL is required for this provider",
					Hint:    "Set provider.endpoint to your API endpoint URL",
				})
			}
		} else {
			// Validate URL format
			if _, err := url.ParseRequestURI(endpoint); err != nil {
				result.Errors = append(result.Errors, ValidationError{
					Field:   "provider.endpoint",
					Message: fmt.Sprintf("invalid endpoint URL: %s", endpoint),
					Hint:    "Endpoint should be a valid URL like https://api.openai.com/v1",
				})
			}
		}
	}

	// Validate API key for OpenAI
	if providerName == "openai" {
		apiKey := cfg.Provider.APIKey
		apiKeyEnv := cfg.Provider.APIKeyEnv
		apiKeyCmd := cfg.Provider.APIKeyCmd

		hasKey := false

		if apiKey != "" {
			hasKey = true
			result.Warnings = append(result.Warnings, ValidationError{
				Field:   "provider.api_key",
				Message: "API key stored in plain text",
				Hint:    "Use api_key_env or api_key_cmd instead for better security",
			})
		}

		if apiKeyEnv != "" {
			if os.Getenv(apiKeyEnv) != "" {
				hasKey = true
			} else {
				result.Errors = append(result.Errors, ValidationError{
					Field:   "provider.api_key_env",
					Message: fmt.Sprintf("environment variable %s is not set", apiKeyEnv),
					Hint:    fmt.Sprintf("Export the variable: export %s=your_api_key", apiKeyEnv),
				})
			}
		}

		if apiKeyCmd != "" {
			hasKey = true // Assume command will work
		}

		if !hasKey && apiKeyEnv == "" && apiKeyCmd == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "provider.api_key",
				Message: "OpenAI requires an API key",
				Hint:    "Set api_key_env: OPENAI_API_KEY and export the variable",
			})
		}
	}
}

func validateModel(cfg *Config, result *ValidationResult) {
	if cfg.Model.Name == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "model.name",
			Message: "model name is required",
			Hint:    "Set model.name to the model you want to use (e.g., gpt-4o, llama3.2)",
		})
	}

	// Validate temperature
	if cfg.Model.Temperature < 0 || cfg.Model.Temperature > 2 {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "model.temperature",
			Message: fmt.Sprintf("unusual temperature value: %.2f", cfg.Model.Temperature),
			Hint:    "Temperature typically ranges from 0 (deterministic) to 1 (creative)",
		})
	}

	// Validate max_tokens
	if cfg.Model.MaxTokens < 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "model.max_tokens",
			Message: "max_tokens cannot be negative",
		})
	} else if cfg.Model.MaxTokens > 128000 {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "model.max_tokens",
			Message: fmt.Sprintf("very high max_tokens: %d", cfg.Model.MaxTokens),
			Hint:    "Most models have a context limit around 4096-128000 tokens",
		})
	}
}

func validateSafety(cfg *Config, result *ValidationResult) {
	validLevels := []string{"strict", "cautious", "moderate", "normal", "relaxed", "dangerous"}
	level := strings.ToLower(cfg.Safety.Level)

	if level == "" {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "safety.level",
			Message: "safety level not set, using 'cautious'",
		})
	} else if !containsString(validLevels, level) {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "safety.level",
			Message: fmt.Sprintf("invalid safety level: %s", level),
			Hint:    "Valid levels: strict, cautious, normal, relaxed, dangerous",
		})
	}

	if level == "dangerous" {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "safety.level",
			Message: "safety level is set to 'dangerous'",
			Hint:    "Commands will be executed with minimal safety checks!",
		})
	}
}

func validateBackup(cfg *Config, result *ValidationResult) {
	if !cfg.Backup.Enabled {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "backup.enabled",
			Message: "backup is disabled",
			Hint:    "Enable backups to recover from mistakes with 'sosomi undo'",
		})
	}

	if cfg.Backup.RetentionDays < 1 {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "backup.retention_days",
			Message: "retention_days is very low or zero",
			Hint:    "Set to at least 7 to keep recent backups",
		})
	}
}

func validateHistory(cfg *Config, result *ValidationResult) {
	if !cfg.History.Enabled {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "history.enabled",
			Message: "history is disabled",
			Hint:    "Enable history for better context and command recall",
		})
	}

	if cfg.History.RetentionDays < 1 {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "history.retention_days",
			Message: "retention_days is very low",
			Hint:    "Consider setting to at least 7 for useful history",
		})
	}
}

func validateMCP(cfg *Config, result *ValidationResult) {
	if cfg.MCP.Enabled {
		if len(cfg.MCP.Servers) == 0 {
			result.Warnings = append(result.Warnings, ValidationError{
				Field:   "mcp.servers",
				Message: "MCP is enabled but no servers configured",
				Hint:    "Add MCP servers to extend sosomi's capabilities",
			})
		}
	}
}

// ValidateConfigFile validates a configuration file at the given path
func ValidateConfigFile(path string) (*ValidationResult, error) {
	// Read and parse the file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read config file: %w", err)
	}

	// Parse YAML
	cfg := DefaultConfig()
	if err := parseYAML(data, cfg); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}

	return Validate(cfg), nil
}

func containsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// parseYAML is a placeholder - in real implementation use yaml.Unmarshal
func parseYAML(data []byte, cfg *Config) error {
	// This would use yaml.Unmarshal in real implementation
	// For now, we just return nil to indicate success
	_ = data
	_ = cfg
	return nil
}
