// Package config handles sosomi configuration management with profile support
package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the complete application configuration
type Config struct {
	// Version for config migration
	Version int `yaml:"version" mapstructure:"version"`

	// Profile settings
	DefaultProfile string `yaml:"default_profile,omitempty" mapstructure:"default_profile"`
	ActiveProfile  string `yaml:"-" mapstructure:"-"` // Runtime only, not saved

	// Provider configuration
	Provider ProviderConfig `yaml:"provider" mapstructure:"provider"`

	// Model parameters
	Model ModelConfig `yaml:"model" mapstructure:"model"`

	// Safety settings
	Safety SafetyConfig `yaml:"safety" mapstructure:"safety"`

	// History settings
	History HistoryConfig `yaml:"history" mapstructure:"history"`

	// LLM client settings
	LLM LLMConfig `yaml:"llm" mapstructure:"llm"`

	// Chat mode settings (shell sessions)
	Chat ChatConfig `yaml:"chat" mapstructure:"chat"`

	// MCP settings
	MCP MCPConfig `yaml:"mcp" mapstructure:"mcp"`

	// UI settings
	UI UIConfig `yaml:"ui" mapstructure:"ui"`

	// Shell settings
	Shell ShellConfig `yaml:"shell" mapstructure:"shell"`

	// Aliases for common commands
	Aliases map[string]string `yaml:"aliases,omitempty" mapstructure:"aliases"`
}

// ProviderConfig holds AI provider settings
type ProviderConfig struct {
	Name     string `yaml:"name" mapstructure:"name"`         // openai, ollama, lmstudio, llamacpp, generic
	Endpoint string `yaml:"endpoint" mapstructure:"endpoint"` // API endpoint

	// Credential references (prefer api_key_env over api_key for security)
	APIKey    string `yaml:"api_key,omitempty" mapstructure:"api_key"`         // Plain text key (not recommended)
	APIKeyEnv string `yaml:"api_key_env,omitempty" mapstructure:"api_key_env"` // Environment variable name
	APIKeyCmd string `yaml:"api_key_cmd,omitempty" mapstructure:"api_key_cmd"` // Command to get key (e.g., "op read 'OpenAI'")
}

// ModelConfig holds model-specific parameters
type ModelConfig struct {
	Name             string   `yaml:"name" mapstructure:"name"`
	MaxTokens        int      `yaml:"max_tokens" mapstructure:"max_tokens"`
	Temperature      float64  `yaml:"temperature" mapstructure:"temperature"`
	TopP             float64  `yaml:"top_p" mapstructure:"top_p"`
	FrequencyPenalty float64  `yaml:"frequency_penalty,omitempty" mapstructure:"frequency_penalty"`
	PresencePenalty  float64  `yaml:"presence_penalty,omitempty" mapstructure:"presence_penalty"`
	StopSequences    []string `yaml:"stop_sequences,omitempty" mapstructure:"stop_sequences"`
	TimeoutSeconds   int      `yaml:"timeout_seconds" mapstructure:"timeout_seconds"`
	MaxRetries       int      `yaml:"max_retries" mapstructure:"max_retries"`
	StreamOutput     bool     `yaml:"stream_output" mapstructure:"stream_output"`
}

// SafetyConfig holds safety-related settings
type SafetyConfig struct {
	Level               string   `yaml:"level" mapstructure:"level"` // strict, moderate, permissive
	RequireConfirmation bool     `yaml:"require_confirmation" mapstructure:"require_confirmation"`
	AutoExecuteSafe     bool     `yaml:"auto_execute_safe" mapstructure:"auto_execute_safe"`
	ConfirmThreshold    string   `yaml:"confirm_threshold,omitempty" mapstructure:"confirm_threshold"` // safe, caution, dangerous
	DryRunDefault       bool     `yaml:"dry_run_default,omitempty" mapstructure:"dry_run_default"`
	MaxAffectedFiles    int      `yaml:"max_affected_files,omitempty" mapstructure:"max_affected_files"`
	BlockedCommands     []string `yaml:"blocked_commands,omitempty" mapstructure:"blocked_commands"`
	ProtectedPaths      []string `yaml:"protected_paths,omitempty" mapstructure:"protected_paths"`
	AllowedPaths        []string `yaml:"allowed_paths,omitempty" mapstructure:"allowed_paths"`
	CustomRulesPath     string   `yaml:"custom_rules_path,omitempty" mapstructure:"custom_rules_path"`
}

// HistoryConfig holds history settings
type HistoryConfig struct {
	Enabled       bool   `yaml:"enabled" mapstructure:"enabled"`
	DBPath        string `yaml:"db_path" mapstructure:"db_path"`
	RetentionDays int    `yaml:"retention_days" mapstructure:"retention_days"`
}

// LLMConfig holds LLM client mode settings
type LLMConfig struct {
	Enabled             bool   `yaml:"enabled" mapstructure:"enabled"`
	DBPath              string `yaml:"db_path" mapstructure:"db_path"`
	DefaultSystemPrompt string `yaml:"default_system_prompt,omitempty" mapstructure:"default_system_prompt"`
	GenerateTitles      bool   `yaml:"generate_titles" mapstructure:"generate_titles"`
	RetentionDays       int    `yaml:"retention_days" mapstructure:"retention_days"`
}

// ChatConfig holds chat mode (shell session) settings
type ChatConfig struct {
	Enabled        bool   `yaml:"enabled" mapstructure:"enabled"`
	DBPath         string `yaml:"db_path" mapstructure:"db_path"`
	GenerateTitles bool   `yaml:"generate_titles" mapstructure:"generate_titles"`
	RetentionDays  int    `yaml:"retention_days" mapstructure:"retention_days"`
	OutputMaxLines int    `yaml:"output_max_lines" mapstructure:"output_max_lines"`
}

// MCPConfig holds MCP (Model Context Protocol) settings
type MCPConfig struct {
	Enabled  bool     `yaml:"enabled" mapstructure:"enabled"`
	Servers  []string `yaml:"servers,omitempty" mapstructure:"servers"`
	ToolsDir string   `yaml:"tools_dir,omitempty" mapstructure:"tools_dir"`
}

// UIConfig holds UI settings
type UIConfig struct {
	ColorEnabled     bool   `yaml:"color_enabled" mapstructure:"color_enabled"`
	ShowExplanations bool   `yaml:"show_explanations" mapstructure:"show_explanations"`
	ShowTimings      bool   `yaml:"show_timings,omitempty" mapstructure:"show_timings"`
	CompactMode      bool   `yaml:"compact_mode,omitempty" mapstructure:"compact_mode"`
	Language         string `yaml:"language,omitempty" mapstructure:"language"`
}

// ShellConfig holds shell settings
type ShellConfig struct {
	DefaultShell   string   `yaml:"default_shell,omitempty" mapstructure:"default_shell"`
	ShellArgs      []string `yaml:"shell_args,omitempty" mapstructure:"shell_args"`
	CaptureOutput  bool     `yaml:"capture_output,omitempty" mapstructure:"capture_output"`
	OutputMaxLines int      `yaml:"output_max_lines,omitempty" mapstructure:"output_max_lines"`
}

// ConfigPaths holds the various config file locations
type ConfigPaths struct {
	System     string // /etc/sosomi/config.yaml
	User       string // ~/.config/sosomi/config.yaml
	Project    string // ./.sosomi/config.yaml
	ProfileDir string // ~/.config/sosomi/profiles/
}

var (
	cfg           *Config
	activeCfg     *Config // Merged configuration
	configPaths   ConfigPaths
	activeProfile string
	initialized   bool
)

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".config", "sosomi")
	dataDir := filepath.Join(homeDir, ".local", "share", "sosomi")

	return &Config{
		Version:        1,
		DefaultProfile: "",

		Provider: ProviderConfig{
			Name:      "openai",
			Endpoint:  "https://api.openai.com/v1",
			APIKeyEnv: "OPENAI_API_KEY",
		},

		Model: ModelConfig{
			Name:           "gpt-4o",
			MaxTokens:      2048,
			Temperature:    0.1,
			TopP:           1.0,
			TimeoutSeconds: 30,
			MaxRetries:     3,
			StreamOutput:   true,
		},

		Safety: SafetyConfig{
			Level:               "moderate",
			RequireConfirmation: true,
			AutoExecuteSafe:     false,
			ConfirmThreshold:    "caution",
			DryRunDefault:       false,
			MaxAffectedFiles:    100,
			BlockedCommands:     []string{"shutdown", "reboot", "init 0", "init 6", ":(){ :|:& };:"},
			ProtectedPaths:      []string{"/", "/etc", "/usr", "/bin", "/sbin", "/boot"},
			CustomRulesPath:     filepath.Join(configDir, "safety_rules.yaml"),
		},

		History: HistoryConfig{
			Enabled:       true,
			DBPath:        filepath.Join(dataDir, "history.db"),
			RetentionDays: 30,
		},

		LLM: LLMConfig{
			Enabled:             true,
			DBPath:              filepath.Join(dataDir, "conversations.db"),
			DefaultSystemPrompt: "",
			GenerateTitles:      true,
			RetentionDays:       90,
		},

		Chat: ChatConfig{
			Enabled:        true,
			DBPath:         filepath.Join(dataDir, "sessions.db"),
			GenerateTitles: true,
			RetentionDays:  90,
			OutputMaxLines: 50,
		},

		MCP: MCPConfig{
			Enabled:  true,
			Servers:  []string{},
			ToolsDir: filepath.Join(configDir, "mcp_tools"),
		},

		UI: UIConfig{
			ColorEnabled:     true,
			ShowExplanations: true,
			ShowTimings:      true,
			CompactMode:      false,
			Language:         "en",
		},

		Shell: ShellConfig{
			CaptureOutput:  true,
			OutputMaxLines: 100,
		},

		Aliases: map[string]string{},
	}
}

// GetConfigPaths returns the configuration file paths
func GetConfigPaths() ConfigPaths {
	if configPaths.User == "" {
		homeDir, _ := os.UserHomeDir()
		configPaths = ConfigPaths{
			System:     "/etc/sosomi/config.yaml",
			User:       filepath.Join(homeDir, ".config", "sosomi", "config.yaml"),
			Project:    filepath.Join(".sosomi", "config.yaml"),
			ProfileDir: filepath.Join(homeDir, ".config", "sosomi", "profiles"),
		}
	}
	return configPaths
}

// Init initializes the configuration system
func Init(configPath string) error {
	if initialized && configPath == "" {
		return nil
	}

	paths := GetConfigPaths()

	// Start with defaults
	cfg = DefaultConfig()
	activeCfg = copyConfig(cfg)

	// Load in order of precedence (lowest to highest)
	// 1. System config
	if data, err := os.ReadFile(paths.System); err == nil {
		if err := loadConfigData(data, cfg); err != nil {
			return fmt.Errorf("failed to parse system config: %w", err)
		}
	}

	// 2. User config (or custom path)
	userConfigPath := paths.User
	if configPath != "" {
		userConfigPath = configPath
	}
	if data, err := os.ReadFile(userConfigPath); err == nil {
		if err := loadConfigData(data, cfg); err != nil {
			return fmt.Errorf("failed to parse user config: %w", err)
		}
	}

	// 3. Project config (if exists)
	if data, err := os.ReadFile(paths.Project); err == nil {
		projectCfg := &Config{}
		if err := loadConfigData(data, projectCfg); err != nil {
			return fmt.Errorf("failed to parse project config: %w", err)
		}
		mergeConfig(cfg, projectCfg)
	}

	// 4. Environment variable overrides
	applyEnvOverrides(cfg)

	// Create a copy for the active config
	activeCfg = copyConfig(cfg)

	initialized = true
	return nil
}

// InitWithProfile initializes config and loads a specific profile
func InitWithProfile(configPath, profileName string) error {
	if err := Init(configPath); err != nil {
		return err
	}

	// Determine which profile to load
	profile := profileName
	if profile == "" {
		profile = os.Getenv("SOSOMI_PROFILE")
	}
	if profile == "" {
		profile = cfg.DefaultProfile
	}

	// Load profile if specified
	if profile != "" {
		if err := LoadProfile(profile); err != nil {
			// Profile not found is not fatal, continue with base config
			if !os.IsNotExist(err) {
				return fmt.Errorf("failed to load profile %s: %w", profile, err)
			}
		}
	}

	activeProfile = profile
	if activeCfg != nil {
		activeCfg.ActiveProfile = profile
	}

	return nil
}

// LoadProfile loads a profile and merges it with the current config
func LoadProfile(name string) error {
	paths := GetConfigPaths()
	profilePath := filepath.Join(paths.ProfileDir, name+".yaml")

	data, err := os.ReadFile(profilePath)
	if err != nil {
		return err
	}

	profileCfg := &Config{}
	if err := yaml.Unmarshal(data, profileCfg); err != nil {
		return fmt.Errorf("failed to parse profile: %w", err)
	}

	// Start fresh from base config
	activeCfg = copyConfig(cfg)

	// Merge profile into active config
	mergeConfig(activeCfg, profileCfg)
	activeProfile = name
	activeCfg.ActiveProfile = name

	return nil
}

// copyConfig creates a deep copy of a config
func copyConfig(src *Config) *Config {
	if src == nil {
		return nil
	}
	dst := *src
	// Deep copy slices
	if src.Safety.BlockedCommands != nil {
		dst.Safety.BlockedCommands = make([]string, len(src.Safety.BlockedCommands))
		copy(dst.Safety.BlockedCommands, src.Safety.BlockedCommands)
	}
	if src.Safety.ProtectedPaths != nil {
		dst.Safety.ProtectedPaths = make([]string, len(src.Safety.ProtectedPaths))
		copy(dst.Safety.ProtectedPaths, src.Safety.ProtectedPaths)
	}
	if src.Safety.AllowedPaths != nil {
		dst.Safety.AllowedPaths = make([]string, len(src.Safety.AllowedPaths))
		copy(dst.Safety.AllowedPaths, src.Safety.AllowedPaths)
	}
	if src.MCP.Servers != nil {
		dst.MCP.Servers = make([]string, len(src.MCP.Servers))
		copy(dst.MCP.Servers, src.MCP.Servers)
	}
	if src.Aliases != nil {
		dst.Aliases = make(map[string]string)
		for k, v := range src.Aliases {
			dst.Aliases[k] = v
		}
	}
	return &dst
}

// mergeConfig merges src into dst (non-zero values from src override dst)
func mergeConfig(dst, src *Config) {
	if src.Provider.Name != "" {
		dst.Provider.Name = src.Provider.Name
	}
	if src.Provider.Endpoint != "" {
		dst.Provider.Endpoint = src.Provider.Endpoint
	}
	if src.Provider.APIKey != "" {
		dst.Provider.APIKey = src.Provider.APIKey
	}
	if src.Provider.APIKeyEnv != "" {
		dst.Provider.APIKeyEnv = src.Provider.APIKeyEnv
	}
	if src.Provider.APIKeyCmd != "" {
		dst.Provider.APIKeyCmd = src.Provider.APIKeyCmd
	}

	if src.Model.Name != "" {
		dst.Model.Name = src.Model.Name
	}
	if src.Model.MaxTokens != 0 {
		dst.Model.MaxTokens = src.Model.MaxTokens
	}
	if src.Model.Temperature != 0 {
		dst.Model.Temperature = src.Model.Temperature
	}
	if src.Model.TopP != 0 {
		dst.Model.TopP = src.Model.TopP
	}
	if src.Model.TimeoutSeconds != 0 {
		dst.Model.TimeoutSeconds = src.Model.TimeoutSeconds
	}
	if src.Model.MaxRetries != 0 {
		dst.Model.MaxRetries = src.Model.MaxRetries
	}

	if src.Safety.Level != "" {
		dst.Safety.Level = src.Safety.Level
	}
	if src.Safety.ConfirmThreshold != "" {
		dst.Safety.ConfirmThreshold = src.Safety.ConfirmThreshold
	}
	if len(src.Safety.BlockedCommands) > 0 {
		dst.Safety.BlockedCommands = src.Safety.BlockedCommands
	}
	if len(src.Safety.ProtectedPaths) > 0 {
		dst.Safety.ProtectedPaths = src.Safety.ProtectedPaths
	}
	if len(src.Safety.AllowedPaths) > 0 {
		dst.Safety.AllowedPaths = src.Safety.AllowedPaths
	}

	if src.History.DBPath != "" {
		dst.History.DBPath = src.History.DBPath
	}
	if src.History.RetentionDays != 0 {
		dst.History.RetentionDays = src.History.RetentionDays
	}

	if src.UI.Language != "" {
		dst.UI.Language = src.UI.Language
	}

	// Merge aliases
	if src.Aliases != nil {
		if dst.Aliases == nil {
			dst.Aliases = make(map[string]string)
		}
		for k, v := range src.Aliases {
			dst.Aliases[k] = v
		}
	}
}

// applyEnvOverrides applies environment variable overrides
func applyEnvOverrides(c *Config) {
	if provider := os.Getenv("SOSOMI_PROVIDER"); provider != "" {
		c.Provider.Name = provider
	}
	if model := os.Getenv("SOSOMI_MODEL"); model != "" {
		c.Model.Name = model
	}
	if endpoint := os.Getenv("SOSOMI_ENDPOINT"); endpoint != "" {
		c.Provider.Endpoint = endpoint
	}
}

// Get returns the active (merged) configuration
func Get() *Config {
	if activeCfg == nil {
		activeCfg = DefaultConfig()
	}
	return activeCfg
}

// GetBase returns the base configuration (before profile merge)
func GetBase() *Config {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return cfg
}

// Save writes the current base configuration to the user config file
func Save() error {
	paths := GetConfigPaths()

	// Ensure directory exists
	configDir := filepath.Dir(paths.User)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(paths.User, data, 0644)
}

// Set sets a configuration value by dot-notation key
func Set(key string, value interface{}) error {
	parts := strings.Split(key, ".")
	return setNestedValue(cfg, parts, value)
}

// GetValue gets a configuration value by dot-notation key
func GetValue(key string) (interface{}, error) {
	parts := strings.Split(key, ".")
	return getNestedValue(activeCfg, parts)
}

// setNestedValue sets a value in the config by path
func setNestedValue(c *Config, path []string, value interface{}) error {
	if len(path) == 0 {
		return fmt.Errorf("empty path")
	}

	strVal := fmt.Sprintf("%v", value)

	// Handle top-level and nested settings
	switch path[0] {
	case "provider":
		if len(path) >= 2 {
			switch path[1] {
			case "name":
				c.Provider.Name = strVal
			case "endpoint":
				c.Provider.Endpoint = strVal
			case "api_key_env":
				c.Provider.APIKeyEnv = strVal
			case "api_key_cmd":
				c.Provider.APIKeyCmd = strVal
			default:
				return fmt.Errorf("unknown key: %s", strings.Join(path, "."))
			}
			return nil
		}
	case "model":
		if len(path) >= 2 {
			switch path[1] {
			case "name":
				c.Model.Name = strVal
			case "max_tokens":
				c.Model.MaxTokens = toInt(value)
			case "temperature":
				c.Model.Temperature = toFloat(value)
			case "top_p":
				c.Model.TopP = toFloat(value)
			case "timeout_seconds":
				c.Model.TimeoutSeconds = toInt(value)
			case "max_retries":
				c.Model.MaxRetries = toInt(value)
			case "stream_output":
				c.Model.StreamOutput = toBool(value)
			default:
				return fmt.Errorf("unknown key: %s", strings.Join(path, "."))
			}
			return nil
		}
	case "safety":
		if len(path) >= 2 {
			switch path[1] {
			case "level":
				c.Safety.Level = strVal
			case "require_confirmation":
				c.Safety.RequireConfirmation = toBool(value)
			case "auto_execute_safe":
				c.Safety.AutoExecuteSafe = toBool(value)
			case "confirm_threshold":
				c.Safety.ConfirmThreshold = strVal
			case "dry_run_default":
				c.Safety.DryRunDefault = toBool(value)
			case "max_affected_files":
				c.Safety.MaxAffectedFiles = toInt(value)
			default:
				return fmt.Errorf("unknown key: %s", strings.Join(path, "."))
			}
			return nil
		}
	case "history":
		if len(path) >= 2 {
			switch path[1] {
			case "enabled":
				c.History.Enabled = toBool(value)
			case "db_path":
				c.History.DBPath = strVal
			case "retention_days":
				c.History.RetentionDays = toInt(value)
			default:
				return fmt.Errorf("unknown key: %s", strings.Join(path, "."))
			}
			return nil
		}
	case "ui":
		if len(path) >= 2 {
			switch path[1] {
			case "color_enabled":
				c.UI.ColorEnabled = toBool(value)
			case "show_explanations":
				c.UI.ShowExplanations = toBool(value)
			case "show_timings":
				c.UI.ShowTimings = toBool(value)
			case "compact_mode":
				c.UI.CompactMode = toBool(value)
			case "language":
				c.UI.Language = strVal
			default:
				return fmt.Errorf("unknown key: %s", strings.Join(path, "."))
			}
			return nil
		}
	case "shell":
		if len(path) >= 2 {
			switch path[1] {
			case "default_shell":
				c.Shell.DefaultShell = strVal
			case "capture_output":
				c.Shell.CaptureOutput = toBool(value)
			case "output_max_lines":
				c.Shell.OutputMaxLines = toInt(value)
			default:
				return fmt.Errorf("unknown key: %s", strings.Join(path, "."))
			}
			return nil
		}
	case "mcp":
		if len(path) >= 2 {
			switch path[1] {
			case "enabled":
				c.MCP.Enabled = toBool(value)
			case "tools_dir":
				c.MCP.ToolsDir = strVal
			default:
				return fmt.Errorf("unknown key: %s", strings.Join(path, "."))
			}
			return nil
		}
	case "default_profile":
		c.DefaultProfile = strVal
		return nil
	}

	return fmt.Errorf("unknown key: %s", path[0])
}

// getNestedValue gets a value from the config by path
func getNestedValue(c *Config, path []string) (interface{}, error) {
	if len(path) == 0 {
		return nil, fmt.Errorf("empty path")
	}

	switch path[0] {
	case "provider":
		if len(path) == 1 {
			return c.Provider, nil
		}
		switch path[1] {
		case "name":
			return c.Provider.Name, nil
		case "endpoint":
			return c.Provider.Endpoint, nil
		case "api_key_env":
			return c.Provider.APIKeyEnv, nil
		case "api_key_cmd":
			return c.Provider.APIKeyCmd, nil
		}
	case "model":
		if len(path) == 1 {
			return c.Model, nil
		}
		switch path[1] {
		case "name":
			return c.Model.Name, nil
		case "max_tokens":
			return c.Model.MaxTokens, nil
		case "temperature":
			return c.Model.Temperature, nil
		case "top_p":
			return c.Model.TopP, nil
		case "timeout_seconds":
			return c.Model.TimeoutSeconds, nil
		case "max_retries":
			return c.Model.MaxRetries, nil
		case "stream_output":
			return c.Model.StreamOutput, nil
		}
	case "safety":
		if len(path) == 1 {
			return c.Safety, nil
		}
		switch path[1] {
		case "level":
			return c.Safety.Level, nil
		case "require_confirmation":
			return c.Safety.RequireConfirmation, nil
		case "auto_execute_safe":
			return c.Safety.AutoExecuteSafe, nil
		}
	case "history":
		if len(path) == 1 {
			return c.History, nil
		}
		switch path[1] {
		case "enabled":
			return c.History.Enabled, nil
		case "db_path":
			return c.History.DBPath, nil
		}
	case "ui":
		if len(path) == 1 {
			return c.UI, nil
		}
		switch path[1] {
		case "color_enabled":
			return c.UI.ColorEnabled, nil
		case "show_explanations":
			return c.UI.ShowExplanations, nil
		}
	case "default_profile":
		return c.DefaultProfile, nil
	case "active_profile":
		return c.ActiveProfile, nil
	}

	return nil, fmt.Errorf("unknown key: %s", strings.Join(path, "."))
}

// Helper conversion functions
func toInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case string:
		var i int
		fmt.Sscanf(val, "%d", &i)
		return i
	}
	return 0
}

func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	}
	return 0
}

func toBool(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val == "true" || val == "1" || val == "yes"
	case int:
		return val != 0
	}
	return false
}

// GetAPIKey returns the API key for the current provider
func GetAPIKey() string {
	c := Get()

	// 1. Check command (e.g., 1Password CLI)
	if c.Provider.APIKeyCmd != "" {
		parts := strings.Fields(c.Provider.APIKeyCmd)
		if len(parts) > 0 {
			cmd := exec.Command(parts[0], parts[1:]...)
			output, err := cmd.Output()
			if err == nil {
				return strings.TrimSpace(string(output))
			}
		}
	}

	// 2. Check configured environment variable
	if c.Provider.APIKeyEnv != "" {
		if key := os.Getenv(c.Provider.APIKeyEnv); key != "" {
			return key
		}
	}

	// 3. Check direct API key in config
	if c.Provider.APIKey != "" {
		return c.Provider.APIKey
	}

	// 4. Check common environment variables
	if key := os.Getenv("SOSOMI_API_KEY"); key != "" {
		return key
	}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return key
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return key
	}

	return ""
}

// GetEndpoint returns the API endpoint for the current provider
func GetEndpoint() string {
	c := Get()

	// Check if endpoint is explicitly set
	if c.Provider.Endpoint != "" {
		return c.Provider.Endpoint
	}

	// Return defaults based on provider
	switch c.Provider.Name {
	case "ollama":
		return "http://localhost:11434"
	case "lmstudio":
		return "http://localhost:1234/v1"
	case "llamacpp":
		return "http://localhost:8080/v1"
	default:
		return "https://api.openai.com/v1"
	}
}

// IsLocalProvider returns true if the current provider is a local model
func IsLocalProvider() bool {
	c := Get()
	switch c.Provider.Name {
	case "ollama", "lmstudio", "llamacpp":
		return true
	default:
		return false
	}
}

// EnsureDirs creates necessary directories
func EnsureDirs() error {
	c := Get()
	paths := GetConfigPaths()

	dirs := []string{
		filepath.Dir(c.History.DBPath),
		c.MCP.ToolsDir,
		filepath.Dir(c.Safety.CustomRulesPath),
		paths.ProfileDir,
	}

	for _, dir := range dirs {
		if dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		}
	}
	return nil
}

// GetActiveProfile returns the name of the active profile
func GetActiveProfile() string {
	return activeProfile
}

// IsFirstRun returns true if this appears to be the first run
func IsFirstRun() bool {
	paths := GetConfigPaths()
	_, err := os.Stat(paths.User)
	return os.IsNotExist(err)
}

// ResetInitialized resets the initialization state (for testing)
func ResetInitialized() {
	initialized = false
	cfg = nil
	activeCfg = nil
	activeProfile = ""
}

// LegacyConfig represents the old flat configuration format
type LegacyConfig struct {
	Provider             string   `yaml:"provider"`
	Model                string   `yaml:"model"`
	APIEndpoint          string   `yaml:"api_endpoint"`
	APIKey               string   `yaml:"api_key"`
	OllamaEndpoint       string   `yaml:"ollama_endpoint"`
	LMStudioEndpoint     string   `yaml:"lmstudio_endpoint"`
	LlamaCppEndpoint     string   `yaml:"llamacpp_endpoint"`
	SafetyProfile        string   `yaml:"safety_profile"`
	AutoExecuteSafe      bool     `yaml:"auto_execute_safe"`
	RequireConfirmation  bool     `yaml:"require_confirmation"`
	BlockedCommands      []string `yaml:"blocked_commands"`
	AllowedPaths         []string `yaml:"allowed_paths"`
	HistoryEnabled       bool     `yaml:"history_enabled"`
	HistoryDBPath        string   `yaml:"history_db_path"`
	HistoryRetentionDays int      `yaml:"history_retention_days"`
	MCPEnabled           bool     `yaml:"mcp_enabled"`
	MCPServers           []string `yaml:"mcp_servers"`
	MCPToolsDir          string   `yaml:"mcp_tools_dir"`
	ColorEnabled         bool     `yaml:"color_enabled"`
	ShowExplanations     bool     `yaml:"show_explanations"`
	Language             string   `yaml:"language"`
	TimeoutSeconds       int      `yaml:"timeout_seconds"`
	MaxRetries           int      `yaml:"max_retries"`
	StreamOutput         bool     `yaml:"stream_output"`
	CustomRulesPath      string   `yaml:"custom_rules_path"`
}

// loadConfigData attempts to load config data, handling both new and legacy formats
func loadConfigData(data []byte, c *Config) error {
	// First, try to load as new format
	if err := yaml.Unmarshal(data, c); err == nil {
		// Check if it looks like new format (has version or nested structures)
		if c.Version > 0 || c.Provider.Name != "" {
			return nil
		}
	}

	// Try legacy format
	legacy := &LegacyConfig{}
	if err := yaml.Unmarshal(data, legacy); err != nil {
		return err
	}

	// Migrate legacy to new format
	migrateLegacy(legacy, c)
	return nil
}

// migrateLegacy converts legacy config to new format
func migrateLegacy(legacy *LegacyConfig, c *Config) {
	// Provider
	if legacy.Provider != "" {
		c.Provider.Name = legacy.Provider
	}
	if legacy.APIEndpoint != "" {
		c.Provider.Endpoint = legacy.APIEndpoint
	}
	if legacy.APIKey != "" {
		c.Provider.APIKey = legacy.APIKey
	}
	// Set endpoint based on provider if not set
	switch c.Provider.Name {
	case "ollama":
		if legacy.OllamaEndpoint != "" {
			c.Provider.Endpoint = legacy.OllamaEndpoint
		}
	case "lmstudio":
		if legacy.LMStudioEndpoint != "" {
			c.Provider.Endpoint = legacy.LMStudioEndpoint
		}
	case "llamacpp":
		if legacy.LlamaCppEndpoint != "" {
			c.Provider.Endpoint = legacy.LlamaCppEndpoint
		}
	}

	// Model
	if legacy.Model != "" {
		c.Model.Name = legacy.Model
	}
	if legacy.TimeoutSeconds > 0 {
		c.Model.TimeoutSeconds = legacy.TimeoutSeconds
	}
	if legacy.MaxRetries > 0 {
		c.Model.MaxRetries = legacy.MaxRetries
	}
	c.Model.StreamOutput = legacy.StreamOutput

	// Safety
	if legacy.SafetyProfile != "" {
		c.Safety.Level = legacy.SafetyProfile
	}
	c.Safety.AutoExecuteSafe = legacy.AutoExecuteSafe
	c.Safety.RequireConfirmation = legacy.RequireConfirmation
	if len(legacy.BlockedCommands) > 0 {
		c.Safety.BlockedCommands = legacy.BlockedCommands
	}
	if len(legacy.AllowedPaths) > 0 {
		c.Safety.AllowedPaths = legacy.AllowedPaths
	}
	if legacy.CustomRulesPath != "" {
		c.Safety.CustomRulesPath = legacy.CustomRulesPath
	}

	// History
	c.History.Enabled = legacy.HistoryEnabled
	if legacy.HistoryDBPath != "" {
		c.History.DBPath = legacy.HistoryDBPath
	}
	if legacy.HistoryRetentionDays > 0 {
		c.History.RetentionDays = legacy.HistoryRetentionDays
	}

	// MCP
	c.MCP.Enabled = legacy.MCPEnabled
	if len(legacy.MCPServers) > 0 {
		c.MCP.Servers = legacy.MCPServers
	}
	if legacy.MCPToolsDir != "" {
		c.MCP.ToolsDir = legacy.MCPToolsDir
	}

	// UI
	c.UI.ColorEnabled = legacy.ColorEnabled
	c.UI.ShowExplanations = legacy.ShowExplanations
	if legacy.Language != "" {
		c.UI.Language = legacy.Language
	}
}
