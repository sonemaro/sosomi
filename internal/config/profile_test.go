// Package config tests for profile management
package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestListProfiles(t *testing.T) {
	tmpDir := t.TempDir()
	profileDir := filepath.Join(tmpDir, ".config", "sosomi", "profiles")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatalf("Failed to create profile dir: %v", err)
	}

	// Create test profiles
	profiles := []string{"work", "home", "testing"}
	for _, name := range profiles {
		profilePath := filepath.Join(profileDir, name+".yaml")
		content := "name: " + name + "\n"
		if err := os.WriteFile(profilePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write profile: %v", err)
		}
	}

	// Override HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)

	// Reset config paths cache
	configPaths = ConfigPaths{}

	list, err := ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles failed: %v", err)
	}

	if len(list) != 3 {
		t.Errorf("Expected 3 profiles, got %d", len(list))
	}

	// Profiles should be sorted
	expectedOrder := []string{"home", "testing", "work"}
	for i, expected := range expectedOrder {
		if i < len(list) && list[i] != expected {
			t.Errorf("Expected profile '%s' at index %d, got '%s'", expected, i, list[i])
		}
	}
}

func TestListProfiles_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	profileDir := filepath.Join(tmpDir, ".config", "sosomi", "profiles")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatalf("Failed to create profile dir: %v", err)
	}

	// Override HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)
	configPaths = ConfigPaths{}

	list, err := ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles failed: %v", err)
	}

	if len(list) != 0 {
		t.Errorf("Expected 0 profiles, got %d", len(list))
	}
}

func TestGetProfile(t *testing.T) {
	tmpDir := t.TempDir()
	profileDir := filepath.Join(tmpDir, ".config", "sosomi", "profiles")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatalf("Failed to create profile dir: %v", err)
	}

	profileContent := `
name: work
description: Work profile
provider:
  name: openai
  endpoint: https://api.openai.com/v1
model:
  name: gpt-4o
  max_tokens: 4096
safety:
  level: strict
`
	profilePath := filepath.Join(profileDir, "work.yaml")
	if err := os.WriteFile(profilePath, []byte(profileContent), 0644); err != nil {
		t.Fatalf("Failed to write profile: %v", err)
	}

	// Override HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)
	configPaths = ConfigPaths{}

	profile, err := GetProfile("work")
	if err != nil {
		t.Fatalf("GetProfile failed: %v", err)
	}

	if profile.Name != "work" {
		t.Errorf("Expected Name 'work', got '%s'", profile.Name)
	}
	if profile.Description != "Work profile" {
		t.Errorf("Expected Description 'Work profile', got '%s'", profile.Description)
	}
	if profile.Provider.Name != "openai" {
		t.Errorf("Expected Provider.Name 'openai', got '%s'", profile.Provider.Name)
	}
	if profile.Model.Name != "gpt-4o" {
		t.Errorf("Expected Model.Name 'gpt-4o', got '%s'", profile.Model.Name)
	}
	if profile.Safety.Level != "strict" {
		t.Errorf("Expected Safety.Level 'strict', got '%s'", profile.Safety.Level)
	}
}

func TestGetProfile_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	profileDir := filepath.Join(tmpDir, ".config", "sosomi", "profiles")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatalf("Failed to create profile dir: %v", err)
	}

	// Override HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)
	configPaths = ConfigPaths{}

	_, err := GetProfile("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent profile")
	}
}

func TestSaveProfile(t *testing.T) {
	tmpDir := t.TempDir()
	profileDir := filepath.Join(tmpDir, ".config", "sosomi", "profiles")

	// Override HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)
	configPaths = ConfigPaths{}

	profile := &Profile{
		Name:        "test-save",
		Description: "Test profile",
		Provider: ProviderConfig{
			Name:     "ollama",
			Endpoint: "http://localhost:11434",
		},
		Model: ModelConfig{
			Name: "llama3.2",
		},
	}

	err := SaveProfile(profile)
	if err != nil {
		t.Fatalf("SaveProfile failed: %v", err)
	}

	// Verify file was created
	profilePath := filepath.Join(profileDir, "test-save.yaml")
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		t.Error("Profile file was not created")
	}

	// Verify contents
	loaded, err := GetProfile("test-save")
	if err != nil {
		t.Fatalf("Failed to load saved profile: %v", err)
	}
	if loaded.Name != "test-save" {
		t.Errorf("Expected Name 'test-save', got '%s'", loaded.Name)
	}
	if loaded.Provider.Name != "ollama" {
		t.Errorf("Expected Provider.Name 'ollama', got '%s'", loaded.Provider.Name)
	}
}

func TestDeleteProfile(t *testing.T) {
	tmpDir := t.TempDir()
	profileDir := filepath.Join(tmpDir, ".config", "sosomi", "profiles")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatalf("Failed to create profile dir: %v", err)
	}

	// Create a profile to delete
	profilePath := filepath.Join(profileDir, "to-delete.yaml")
	if err := os.WriteFile(profilePath, []byte("name: to-delete\n"), 0644); err != nil {
		t.Fatalf("Failed to write profile: %v", err)
	}

	// Override HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)
	configPaths = ConfigPaths{}

	// Reset config to avoid issues with default profile
	ResetInitialized()
	cfg = DefaultConfig()
	activeCfg = cfg

	err := DeleteProfile("to-delete")
	if err != nil {
		t.Fatalf("DeleteProfile failed: %v", err)
	}

	// Verify file was deleted
	if _, err := os.Stat(profilePath); !os.IsNotExist(err) {
		t.Error("Profile file was not deleted")
	}
}

func TestProfileExists(t *testing.T) {
	tmpDir := t.TempDir()
	profileDir := filepath.Join(tmpDir, ".config", "sosomi", "profiles")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatalf("Failed to create profile dir: %v", err)
	}

	// Create a profile
	profilePath := filepath.Join(profileDir, "exists.yaml")
	if err := os.WriteFile(profilePath, []byte("name: exists\n"), 0644); err != nil {
		t.Fatalf("Failed to write profile: %v", err)
	}

	// Override HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)
	configPaths = ConfigPaths{}

	if !ProfileExists("exists") {
		t.Error("Expected ProfileExists to return true for existing profile")
	}

	if ProfileExists("nonexistent") {
		t.Error("Expected ProfileExists to return false for nonexistent profile")
	}
}

func TestSetDefaultProfile(t *testing.T) {
	tmpDir := t.TempDir()
	profileDir := filepath.Join(tmpDir, ".config", "sosomi", "profiles")
	configDir := filepath.Join(tmpDir, ".config", "sosomi")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatalf("Failed to create profile dir: %v", err)
	}

	// Create a profile
	profilePath := filepath.Join(profileDir, "default-test.yaml")
	if err := os.WriteFile(profilePath, []byte("name: default-test\n"), 0644); err != nil {
		t.Fatalf("Failed to write profile: %v", err)
	}

	// Override HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)
	configPaths = ConfigPaths{}

	// Initialize config
	ResetInitialized()
	cfg = DefaultConfig()
	activeCfg = cfg

	// Create config file location
	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("version: 1\n"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	err := SetDefaultProfile("default-test")
	if err != nil {
		t.Fatalf("SetDefaultProfile failed: %v", err)
	}

	if cfg.DefaultProfile != "default-test" {
		t.Errorf("Expected DefaultProfile 'default-test', got '%s'", cfg.DefaultProfile)
	}
}

func TestSetDefaultProfile_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	profileDir := filepath.Join(tmpDir, ".config", "sosomi", "profiles")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatalf("Failed to create profile dir: %v", err)
	}

	// Override HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)
	configPaths = ConfigPaths{}

	err := SetDefaultProfile("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent profile")
	}
}

func TestCreateProfile(t *testing.T) {
	tmpDir := t.TempDir()
	profileDir := filepath.Join(tmpDir, ".config", "sosomi", "profiles")

	// Override HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)
	configPaths = ConfigPaths{}

	// Initialize config
	ResetInitialized()
	cfg = DefaultConfig()
	cfg.Provider.Name = "ollama"
	cfg.Model.Name = "llama3.2"
	activeCfg = cfg

	profile, err := CreateProfile("new-profile", "A new profile")
	if err != nil {
		t.Fatalf("CreateProfile failed: %v", err)
	}

	if profile.Name != "new-profile" {
		t.Errorf("Expected Name 'new-profile', got '%s'", profile.Name)
	}
	if profile.Description != "A new profile" {
		t.Errorf("Expected Description 'A new profile', got '%s'", profile.Description)
	}
	if profile.Provider.Name != "ollama" {
		t.Errorf("Expected Provider.Name 'ollama', got '%s'", profile.Provider.Name)
	}

	// Verify file exists
	profilePath := filepath.Join(profileDir, "new-profile.yaml")
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		t.Error("Profile file was not created")
	}
}

func TestCreateProfile_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	profileDir := filepath.Join(tmpDir, ".config", "sosomi", "profiles")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatalf("Failed to create profile dir: %v", err)
	}

	// Create existing profile
	profilePath := filepath.Join(profileDir, "existing.yaml")
	if err := os.WriteFile(profilePath, []byte("name: existing\n"), 0644); err != nil {
		t.Fatalf("Failed to write profile: %v", err)
	}

	// Override HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)
	configPaths = ConfigPaths{}

	ResetInitialized()
	cfg = DefaultConfig()
	activeCfg = cfg

	_, err := CreateProfile("existing", "Duplicate")
	if err == nil {
		t.Error("Expected error for existing profile")
	}
}

func TestDuplicateProfile(t *testing.T) {
	tmpDir := t.TempDir()
	profileDir := filepath.Join(tmpDir, ".config", "sosomi", "profiles")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatalf("Failed to create profile dir: %v", err)
	}

	// Create source profile
	srcContent := `
name: source
description: Source profile
provider:
  name: openai
model:
  name: gpt-4o
`
	srcPath := filepath.Join(profileDir, "source.yaml")
	if err := os.WriteFile(srcPath, []byte(srcContent), 0644); err != nil {
		t.Fatalf("Failed to write source profile: %v", err)
	}

	// Override HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)
	configPaths = ConfigPaths{}

	dst, err := DuplicateProfile("source", "destination")
	if err != nil {
		t.Fatalf("DuplicateProfile failed: %v", err)
	}

	if dst.Name != "destination" {
		t.Errorf("Expected Name 'destination', got '%s'", dst.Name)
	}
	if dst.Provider.Name != "openai" {
		t.Errorf("Expected Provider.Name 'openai', got '%s'", dst.Provider.Name)
	}

	// Verify file exists
	dstPath := filepath.Join(profileDir, "destination.yaml")
	if _, err := os.Stat(dstPath); os.IsNotExist(err) {
		t.Error("Destination profile file was not created")
	}
}

func TestExportProfile(t *testing.T) {
	tmpDir := t.TempDir()
	profileDir := filepath.Join(tmpDir, ".config", "sosomi", "profiles")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatalf("Failed to create profile dir: %v", err)
	}

	// Create profile with sensitive data
	profileContent := `
name: export-test
provider:
  name: openai
  api_key_env: OPENAI_API_KEY
  api_key_cmd: op read 'OpenAI API Key'
model:
  name: gpt-4o
`
	profilePath := filepath.Join(profileDir, "export-test.yaml")
	if err := os.WriteFile(profilePath, []byte(profileContent), 0644); err != nil {
		t.Fatalf("Failed to write profile: %v", err)
	}

	// Override HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)
	configPaths = ConfigPaths{}

	data, err := ExportProfile("export-test")
	if err != nil {
		t.Fatalf("ExportProfile failed: %v", err)
	}

	// Verify that api_key_cmd is cleared (security)
	if string(data) == "" {
		t.Error("Export returned empty data")
	}

	// Parse and check
	exported := &Profile{}
	if err := yaml.Unmarshal(data, exported); err != nil {
		t.Fatalf("Failed to parse exported profile: %v", err)
	}

	if exported.Provider.APIKeyCmd != "" {
		t.Error("api_key_cmd should be cleared in export")
	}
	// api_key_env is kept as it's just a reference, not the actual key
	if exported.Provider.APIKeyEnv != "OPENAI_API_KEY" {
		t.Errorf("api_key_env should be preserved, got '%s'", exported.Provider.APIKeyEnv)
	}
}

func TestImportProfile(t *testing.T) {
	tmpDir := t.TempDir()
	profileDir := filepath.Join(tmpDir, ".config", "sosomi", "profiles")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatalf("Failed to create profile dir: %v", err)
	}

	// Create import file
	importContent := `
name: imported
provider:
  name: ollama
model:
  name: llama3.2
`
	importPath := filepath.Join(tmpDir, "import.yaml")
	if err := os.WriteFile(importPath, []byte(importContent), 0644); err != nil {
		t.Fatalf("Failed to write import file: %v", err)
	}

	// Override HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)
	configPaths = ConfigPaths{}

	err := ImportProfile(importPath)
	if err != nil {
		t.Fatalf("ImportProfile failed: %v", err)
	}

	// Verify profile was created
	if !ProfileExists("imported") {
		t.Error("Imported profile not found")
	}

	// Load and verify
	profile, err := GetProfile("imported")
	if err != nil {
		t.Fatalf("Failed to get imported profile: %v", err)
	}
	if profile.Provider.Name != "ollama" {
		t.Errorf("Expected Provider.Name 'ollama', got '%s'", profile.Provider.Name)
	}
}

func TestImportProfileData(t *testing.T) {
	tmpDir := t.TempDir()
	profileDir := filepath.Join(tmpDir, ".config", "sosomi", "profiles")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatalf("Failed to create profile dir: %v", err)
	}

	// Override HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)
	configPaths = ConfigPaths{}

	data := []byte(`
name: data-import
provider:
  name: openai
model:
  name: gpt-4o
`)

	profile, err := ImportProfileData(data, false)
	if err != nil {
		t.Fatalf("ImportProfileData failed: %v", err)
	}

	if profile.Name != "data-import" {
		t.Errorf("Expected Name 'data-import', got '%s'", profile.Name)
	}
}

func TestImportProfileData_NoOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	profileDir := filepath.Join(tmpDir, ".config", "sosomi", "profiles")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatalf("Failed to create profile dir: %v", err)
	}

	// Create existing profile
	profilePath := filepath.Join(profileDir, "existing.yaml")
	if err := os.WriteFile(profilePath, []byte("name: existing\n"), 0644); err != nil {
		t.Fatalf("Failed to write profile: %v", err)
	}

	// Override HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)
	configPaths = ConfigPaths{}

	data := []byte("name: existing\nprovider:\n  name: new\n")

	_, err := ImportProfileData(data, false)
	if err == nil {
		t.Error("Expected error when importing existing profile without overwrite")
	}
}

func TestImportProfileData_WithOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	profileDir := filepath.Join(tmpDir, ".config", "sosomi", "profiles")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatalf("Failed to create profile dir: %v", err)
	}

	// Create existing profile
	profilePath := filepath.Join(profileDir, "existing.yaml")
	if err := os.WriteFile(profilePath, []byte("name: existing\nprovider:\n  name: old\n"), 0644); err != nil {
		t.Fatalf("Failed to write profile: %v", err)
	}

	// Override HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)
	configPaths = ConfigPaths{}

	data := []byte("name: existing\nprovider:\n  name: new\n")

	profile, err := ImportProfileData(data, true)
	if err != nil {
		t.Fatalf("ImportProfileData with overwrite failed: %v", err)
	}

	if profile.Provider.Name != "new" {
		t.Errorf("Expected Provider.Name 'new', got '%s'", profile.Provider.Name)
	}
}

func TestGetProfileInfo(t *testing.T) {
	tmpDir := t.TempDir()
	profileDir := filepath.Join(tmpDir, ".config", "sosomi", "profiles")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatalf("Failed to create profile dir: %v", err)
	}

	profileContent := `
name: info-test
description: Test profile for info
provider:
  name: openai
  endpoint: https://api.openai.com/v1
model:
  name: gpt-4o
  max_tokens: 4096
  temperature: 0.7
safety:
  level: strict
`
	profilePath := filepath.Join(profileDir, "info-test.yaml")
	if err := os.WriteFile(profilePath, []byte(profileContent), 0644); err != nil {
		t.Fatalf("Failed to write profile: %v", err)
	}

	// Override HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)
	configPaths = ConfigPaths{}

	info, err := GetProfileInfo("info-test")
	if err != nil {
		t.Fatalf("GetProfileInfo failed: %v", err)
	}

	if info == "" {
		t.Error("Expected non-empty info string")
	}

	// Check that info contains expected fields
	expected := []string{
		"Profile: info-test",
		"Description: Test profile for info",
		"Provider: openai",
		"Model: gpt-4o",
	}
	for _, s := range expected {
		if !contains(info, s) {
			t.Errorf("Expected info to contain '%s'", s)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
