// Package config - Interactive setup wizard
package config

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// WizardResult contains the result of running the setup wizard
type WizardResult struct {
	Config      *Config
	ProfileName string
	Success     bool
}

// RunWizard runs the interactive setup wizard
func RunWizard() (*WizardResult, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("╭──────────────────────────────────────────────────────────────╮")
	fmt.Println("│  🚀 Welcome to sosomi!                                        │")
	fmt.Println("│     Your AI-powered shell assistant                          │")
	fmt.Println("╰──────────────────────────────────────────────────────────────╯")
	fmt.Println()
	fmt.Println("Let's configure your AI provider.")
	fmt.Println()

	// Provider selection
	fmt.Println("Select your AI provider:")
	fmt.Println()
	fmt.Println("  1) OpenAI        - GPT-4o, GPT-4, etc. (requires API key)")
	fmt.Println("  2) Ollama        - Local models, free (llama3, mistral, etc.)")
	fmt.Println("  3) LM Studio     - Local models with GUI")
	fmt.Println("  4) llama.cpp     - Local llama.cpp server")
	fmt.Println("  5) Other         - Any OpenAI-compatible API")
	fmt.Println()
	fmt.Print("Choice [1-5]: ")

	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	cfg := DefaultConfig()

	switch choice {
	case "1", "":
		if err := configureOpenAI(reader, cfg); err != nil {
			return nil, err
		}
	case "2":
		if err := configureOllama(reader, cfg); err != nil {
			return nil, err
		}
	case "3":
		if err := configureLMStudio(reader, cfg); err != nil {
			return nil, err
		}
	case "4":
		if err := configureLlamaCpp(reader, cfg); err != nil {
			return nil, err
		}
	case "5":
		if err := configureGeneric(reader, cfg); err != nil {
			return nil, err
		}
	default:
		fmt.Println("Invalid choice, using OpenAI defaults.")
		configureOpenAI(reader, cfg)
	}

	// Test connection
	fmt.Println()
	fmt.Print("🔍 Testing connection...")

	if testConnection(cfg) {
		fmt.Println(" ✓ Connected!")
	} else {
		fmt.Println(" ✗ Could not connect")
		fmt.Println("   (Configuration will be saved anyway, you can fix it later)")
	}

	// Save profile option
	result := &WizardResult{Config: cfg, Success: true}
	
	fmt.Println()
	fmt.Print("Save this configuration as a profile? (y/n) [n]: ")
	saveProfile, _ := reader.ReadString('\n')
	saveProfile = strings.TrimSpace(strings.ToLower(saveProfile))

	if saveProfile == "y" || saveProfile == "yes" {
		fmt.Print("Profile name: ")
		profileName, _ := reader.ReadString('\n')
		profileName = strings.TrimSpace(profileName)

		if profileName != "" {
			profile := &Profile{
				Name:     profileName,
				Provider: cfg.Provider,
				Model:    cfg.Model,
				Safety:   SafetyConfig{Level: cfg.Safety.Level},
			}
			if err := SaveProfile(profile); err != nil {
				fmt.Printf("⚠ Could not save profile: %v\n", err)
			} else {
				cfg.DefaultProfile = profileName
				result.ProfileName = profileName
				fmt.Printf("✓ Profile '%s' saved!\n", profileName)
			}
		}
	}

	// Save main config
	*GetBase() = *cfg
	if err := Save(); err != nil {
		return result, fmt.Errorf("failed to save config: %w", err)
	}

	// Ensure directories exist
	if err := EnsureDirs(); err != nil {
		fmt.Printf("⚠ Could not create directories: %v\n", err)
	}

	printWizardComplete()
	return result, nil
}

func configureOpenAI(reader *bufio.Reader, cfg *Config) error {
	cfg.Provider.Name = "openai"
	cfg.Provider.Endpoint = "https://api.openai.com/v1"
	cfg.Provider.APIKeyEnv = "OPENAI_API_KEY"
	cfg.Model.Name = "gpt-4o"

	fmt.Println()
	fmt.Println("OpenAI requires an API key from https://platform.openai.com")
	fmt.Println()
	
	// Check if key already exists
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		fmt.Println("✓ Found OPENAI_API_KEY environment variable")
	} else {
		fmt.Println("Set the OPENAI_API_KEY environment variable with your API key.")
		fmt.Print("Or enter a custom env var name [OPENAI_API_KEY]: ")
		envVar, _ := reader.ReadString('\n')
		envVar = strings.TrimSpace(envVar)
		if envVar != "" {
			cfg.Provider.APIKeyEnv = envVar
		}
	}

	fmt.Println()
	fmt.Println("Available models: gpt-4o, gpt-4o-mini, gpt-4-turbo, gpt-3.5-turbo")
	fmt.Print("Model [gpt-4o]: ")
	model, _ := reader.ReadString('\n')
	model = strings.TrimSpace(model)
	if model != "" {
		cfg.Model.Name = model
	}

	return nil
}

func configureOllama(reader *bufio.Reader, cfg *Config) error {
	cfg.Provider.Name = "ollama"
	cfg.Model.Name = "llama3.2"
	cfg.Provider.APIKeyEnv = "" // Ollama doesn't need API key

	fmt.Println()
	fmt.Print("Ollama server endpoint [http://localhost:11434]: ")
	endpoint, _ := reader.ReadString('\n')
	endpoint = strings.TrimSpace(endpoint)
	if endpoint != "" {
		cfg.Provider.Endpoint = endpoint
	} else {
		cfg.Provider.Endpoint = "http://localhost:11434"
	}

	fmt.Println()
	fmt.Println("Popular models: llama3.2, codellama, mistral, deepseek-coder")
	fmt.Print("Model [llama3.2]: ")
	model, _ := reader.ReadString('\n')
	model = strings.TrimSpace(model)
	if model != "" {
		cfg.Model.Name = model
	}

	return nil
}

func configureLMStudio(reader *bufio.Reader, cfg *Config) error {
	cfg.Provider.Name = "lmstudio"
	cfg.Model.Name = "local-model"
	cfg.Provider.APIKeyEnv = ""

	fmt.Println()
	fmt.Print("LM Studio endpoint [http://localhost:1234/v1]: ")
	endpoint, _ := reader.ReadString('\n')
	endpoint = strings.TrimSpace(endpoint)
	if endpoint != "" {
		cfg.Provider.Endpoint = endpoint
	} else {
		cfg.Provider.Endpoint = "http://localhost:1234/v1"
	}

	fmt.Println()
	fmt.Print("Model name (or leave empty for auto-detect): ")
	model, _ := reader.ReadString('\n')
	model = strings.TrimSpace(model)
	if model != "" {
		cfg.Model.Name = model
	}

	return nil
}

func configureLlamaCpp(reader *bufio.Reader, cfg *Config) error {
	cfg.Provider.Name = "llamacpp"
	cfg.Model.Name = "local-model"
	cfg.Provider.APIKeyEnv = ""

	fmt.Println()
	fmt.Print("llama.cpp server endpoint [http://localhost:8080/v1]: ")
	endpoint, _ := reader.ReadString('\n')
	endpoint = strings.TrimSpace(endpoint)
	if endpoint != "" {
		cfg.Provider.Endpoint = endpoint
	} else {
		cfg.Provider.Endpoint = "http://localhost:8080/v1"
	}

	return nil
}

func configureGeneric(reader *bufio.Reader, cfg *Config) error {
	cfg.Provider.Name = "generic"

	fmt.Println()
	fmt.Print("API endpoint URL: ")
	endpoint, _ := reader.ReadString('\n')
	cfg.Provider.Endpoint = strings.TrimSpace(endpoint)

	fmt.Print("Model name: ")
	model, _ := reader.ReadString('\n')
	cfg.Model.Name = strings.TrimSpace(model)

	fmt.Print("API key environment variable (leave empty if none): ")
	envVar, _ := reader.ReadString('\n')
	cfg.Provider.APIKeyEnv = strings.TrimSpace(envVar)

	return nil
}

func testConnection(cfg *Config) bool {
	endpoint := cfg.Provider.Endpoint
	if endpoint == "" {
		switch cfg.Provider.Name {
		case "ollama":
			endpoint = "http://localhost:11434"
		case "lmstudio":
			endpoint = "http://localhost:1234/v1"
		case "llamacpp":
			endpoint = "http://localhost:8080/v1"
		default:
			endpoint = "https://api.openai.com/v1"
		}
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

func printWizardComplete() {
	fmt.Println()
	fmt.Println("╭──────────────────────────────────────────────────────────────╮")
	fmt.Println("│  ✅ Configuration complete!                                   │")
	fmt.Println("╰──────────────────────────────────────────────────────────────╯")
	fmt.Println()
	fmt.Println("Quick start:")
	fmt.Println()
	fmt.Println("  sosomi \"list all files larger than 100MB\"")
	fmt.Println("  sosomi \"find duplicate files in current directory\"")
	fmt.Println("  sosomi \"show disk usage sorted by size\"")
	fmt.Println()
	fmt.Println("Profile management:")
	fmt.Println()
	fmt.Println("  sosomi profile list              List all profiles")
	fmt.Println("  sosomi profile create <name>     Create a new profile")
	fmt.Println("  sosomi profile use <name>        Set default profile")
	fmt.Println("  sosomi -p <profile> \"query\"      Use specific profile")
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Println()
	fmt.Println("  sosomi config list               Show all settings")
	fmt.Println("  sosomi config set <key> <value>  Change a setting")
	fmt.Println("  sosomi config edit               Edit config file")
	fmt.Println()
}

// QuickSetup performs a quick non-interactive setup
func QuickSetup(provider, endpoint, model string) error {
	cfg := DefaultConfig()

	switch provider {
	case "openai":
		cfg.Provider.Name = "openai"
		cfg.Provider.Endpoint = "https://api.openai.com/v1"
		cfg.Provider.APIKeyEnv = "OPENAI_API_KEY"
		if model != "" {
			cfg.Model.Name = model
		} else {
			cfg.Model.Name = "gpt-4o"
		}
	case "ollama":
		cfg.Provider.Name = "ollama"
		if endpoint != "" {
			cfg.Provider.Endpoint = endpoint
		} else {
			cfg.Provider.Endpoint = "http://localhost:11434"
		}
		if model != "" {
			cfg.Model.Name = model
		} else {
			cfg.Model.Name = "llama3.2"
		}
	case "lmstudio":
		cfg.Provider.Name = "lmstudio"
		if endpoint != "" {
			cfg.Provider.Endpoint = endpoint
		} else {
			cfg.Provider.Endpoint = "http://localhost:1234/v1"
		}
		if model != "" {
			cfg.Model.Name = model
		}
	case "llamacpp":
		cfg.Provider.Name = "llamacpp"
		if endpoint != "" {
			cfg.Provider.Endpoint = endpoint
		} else {
			cfg.Provider.Endpoint = "http://localhost:8080/v1"
		}
		if model != "" {
			cfg.Model.Name = model
		}
	default:
		return fmt.Errorf("unknown provider: %s", provider)
	}

	*GetBase() = *cfg
	if err := Save(); err != nil {
		return err
	}
	return EnsureDirs()
}
