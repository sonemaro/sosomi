// Package config - Profile management
package config

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Profile represents a named configuration profile
type Profile struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`

	// Embedded config fields (not full config, just overrides)
	Provider ProviderConfig    `yaml:"provider,omitempty"`
	Model    ModelConfig       `yaml:"model,omitempty"`
	Safety   SafetyConfig      `yaml:"safety,omitempty"`
	UI       UIConfig          `yaml:"ui,omitempty"`
	Aliases  map[string]string `yaml:"aliases,omitempty"`
}

// ListProfiles returns all available profile names
func ListProfiles() ([]string, error) {
	paths := GetConfigPaths()

	entries, err := os.ReadDir(paths.ProfileDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var profiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
			name := strings.TrimSuffix(entry.Name(), ".yaml")
			profiles = append(profiles, name)
		}
	}
	sort.Strings(profiles)
	return profiles, nil
}

// GetProfile loads a profile by name
func GetProfile(name string) (*Profile, error) {
	paths := GetConfigPaths()
	profilePath := filepath.Join(paths.ProfileDir, name+".yaml")

	data, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, err
	}

	profile := &Profile{Name: name}
	if err := yaml.Unmarshal(data, profile); err != nil {
		return nil, fmt.Errorf("failed to parse profile: %w", err)
	}

	return profile, nil
}

// SaveProfile saves a profile
func SaveProfile(profile *Profile) error {
	paths := GetConfigPaths()

	if err := os.MkdirAll(paths.ProfileDir, 0755); err != nil {
		return err
	}

	profilePath := filepath.Join(paths.ProfileDir, profile.Name+".yaml")

	data, err := yaml.Marshal(profile)
	if err != nil {
		return err
	}

	return os.WriteFile(profilePath, data, 0644)
}

// DeleteProfile removes a profile
func DeleteProfile(name string) error {
	paths := GetConfigPaths()
	profilePath := filepath.Join(paths.ProfileDir, name+".yaml")

	// Check if this is the default profile
	if GetBase().DefaultProfile == name {
		GetBase().DefaultProfile = ""
		if err := Save(); err != nil {
			return fmt.Errorf("failed to clear default profile: %w", err)
		}
	}

	return os.Remove(profilePath)
}

// SetDefaultProfile sets the default profile in config
func SetDefaultProfile(name string) error {
	// Verify profile exists (unless clearing)
	if name != "" {
		if _, err := GetProfile(name); err != nil {
			return fmt.Errorf("profile '%s' not found", name)
		}
	}

	GetBase().DefaultProfile = name
	return Save()
}

// ProfileExists checks if a profile exists
func ProfileExists(name string) bool {
	paths := GetConfigPaths()
	profilePath := filepath.Join(paths.ProfileDir, name+".yaml")
	_, err := os.Stat(profilePath)
	return err == nil
}

// TestProfile tests if a profile's provider is reachable
func TestProfile(name string) error {
	profile, err := GetProfile(name)
	if err != nil {
		return fmt.Errorf("failed to load profile: %w", err)
	}

	endpoint := profile.Provider.Endpoint
	if endpoint == "" {
		// Use default for provider type
		switch profile.Provider.Name {
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
		return fmt.Errorf("cannot reach %s: %w", endpoint, err)
	}
	resp.Body.Close()

	return nil
}

// CreateProfile creates a new profile from current config
func CreateProfile(name, description string) (*Profile, error) {
	if ProfileExists(name) {
		return nil, fmt.Errorf("profile '%s' already exists", name)
	}

	c := Get()
	profile := &Profile{
		Name:        name,
		Description: description,
		Provider:    c.Provider,
		Model:       c.Model,
		Safety: SafetyConfig{
			Level: c.Safety.Level,
		},
		UI: UIConfig{
			ColorEnabled:     c.UI.ColorEnabled,
			ShowExplanations: c.UI.ShowExplanations,
		},
	}

	if err := SaveProfile(profile); err != nil {
		return nil, err
	}

	return profile, nil
}

// DuplicateProfile creates a copy of an existing profile
func DuplicateProfile(srcName, dstName string) (*Profile, error) {
	if ProfileExists(dstName) {
		return nil, fmt.Errorf("profile '%s' already exists", dstName)
	}

	src, err := GetProfile(srcName)
	if err != nil {
		return nil, err
	}

	dst := *src
	dst.Name = dstName
	dst.Description = fmt.Sprintf("Copy of %s", srcName)

	if err := SaveProfile(&dst); err != nil {
		return nil, err
	}

	return &dst, nil
}

// GetProfileInfo returns a formatted string with profile information
func GetProfileInfo(name string) (string, error) {
	profile, err := GetProfile(name)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Profile: %s\n", profile.Name))
	if profile.Description != "" {
		sb.WriteString(fmt.Sprintf("Description: %s\n", profile.Description))
	}
	sb.WriteString(fmt.Sprintf("Provider: %s\n", profile.Provider.Name))
	if profile.Provider.Endpoint != "" {
		sb.WriteString(fmt.Sprintf("Endpoint: %s\n", profile.Provider.Endpoint))
	}
	sb.WriteString(fmt.Sprintf("Model: %s\n", profile.Model.Name))
	if profile.Model.MaxTokens > 0 {
		sb.WriteString(fmt.Sprintf("Max Tokens: %d\n", profile.Model.MaxTokens))
	}
	if profile.Model.Temperature > 0 {
		sb.WriteString(fmt.Sprintf("Temperature: %.2f\n", profile.Model.Temperature))
	}
	if profile.Safety.Level != "" {
		sb.WriteString(fmt.Sprintf("Safety Level: %s\n", profile.Safety.Level))
	}

	return sb.String(), nil
}

// ExportProfile exports a profile without sensitive data
func ExportProfile(name string) ([]byte, error) {
	profile, err := GetProfile(name)
	if err != nil {
		return nil, err
	}

	// Clear any sensitive fields
	profile.Provider.APIKeyCmd = ""
	// Keep APIKeyEnv as it's just a reference

	return yaml.Marshal(profile)
}

// ImportProfile imports a profile from a file path
func ImportProfile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("cannot read profile file: %w", err)
	}

	_, err = ImportProfileData(data, false)
	return err
}

// ImportProfileData imports a profile from YAML data
func ImportProfileData(data []byte, overwrite bool) (*Profile, error) {
	var profile Profile
	if err := yaml.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("invalid profile data: %w", err)
	}

	if profile.Name == "" {
		return nil, fmt.Errorf("profile must have a name")
	}

	if ProfileExists(profile.Name) && !overwrite {
		return nil, fmt.Errorf("profile '%s' already exists (use overwrite to replace)", profile.Name)
	}

	if err := SaveProfile(&profile); err != nil {
		return nil, err
	}

	return &profile, nil
}
