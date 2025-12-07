// Package types provides shared type definitions for sosomi
package types

import "testing"

func TestRiskLevel_String(t *testing.T) {
	tests := []struct {
		name     string
		level    RiskLevel
		expected string
	}{
		{"Safe", RiskSafe, "SAFE"},
		{"Caution", RiskCaution, "CAUTION"},
		{"Dangerous", RiskDangerous, "DANGEROUS"},
		{"Critical", RiskCritical, "CRITICAL"},
		{"Unknown", RiskLevel(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("RiskLevel.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRiskLevel_Color(t *testing.T) {
	tests := []struct {
		name     string
		level    RiskLevel
		expected string
	}{
		{"Safe", RiskSafe, "green"},
		{"Caution", RiskCaution, "yellow"},
		{"Dangerous", RiskDangerous, "orange"},
		{"Critical", RiskCritical, "red"},
		{"Unknown", RiskLevel(99), "white"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.level.Color(); got != tt.expected {
				t.Errorf("RiskLevel.Color() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRiskLevel_Emoji(t *testing.T) {
	tests := []struct {
		name     string
		level    RiskLevel
		expected string
	}{
		{"Safe", RiskSafe, "🟢"},
		{"Caution", RiskCaution, "🟡"},
		{"Dangerous", RiskDangerous, "🟠"},
		{"Critical", RiskCritical, "🔴"},
		{"Unknown", RiskLevel(99), "⚪"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.level.Emoji(); got != tt.expected {
				t.Errorf("RiskLevel.Emoji() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRiskLevelOrdering(t *testing.T) {
	// Verify that risk levels are ordered correctly for comparisons
	if RiskSafe >= RiskCaution {
		t.Error("RiskSafe should be less than RiskCaution")
	}
	if RiskCaution >= RiskDangerous {
		t.Error("RiskCaution should be less than RiskDangerous")
	}
	if RiskDangerous >= RiskCritical {
		t.Error("RiskDangerous should be less than RiskCritical")
	}
}

func TestSystemContext_Fields(t *testing.T) {
	ctx := SystemContext{
		OS:               "darwin",
		Shell:            "zsh",
		CurrentDir:       "/Users/test/projects",
		HomeDir:          "/Users/test",
		Username:         "testuser",
		GitBranch:        "main",
		GitStatus:        "clean",
		GitRemote:        "git@github.com:test/repo.git",
		EnvVars:          []string{"PATH=/usr/bin", "HOME=/Users/test"},
		RecentCmds:       []string{"ls", "cd projects"},
		InstalledPkgMgrs: []string{"brew", "npm"},
	}

	if ctx.OS != "darwin" {
		t.Errorf("Expected OS to be 'darwin', got '%s'", ctx.OS)
	}
	if ctx.Shell != "zsh" {
		t.Errorf("Expected Shell to be 'zsh', got '%s'", ctx.Shell)
	}
	if ctx.Username != "testuser" {
		t.Errorf("Expected Username to be 'testuser', got '%s'", ctx.Username)
	}
	if len(ctx.InstalledPkgMgrs) != 2 {
		t.Errorf("Expected 2 package managers, got %d", len(ctx.InstalledPkgMgrs))
	}
}

func TestCommandResponse_Fields(t *testing.T) {
	resp := CommandResponse{
		Command:      "ls -la",
		Explanation:  "List all files with details",
		RiskLevel:    RiskSafe,
		Confidence:   0.95,
		Alternatives: []string{"ls -l", "ls -a"},
		Warnings:     []string{},
	}

	if resp.Command != "ls -la" {
		t.Errorf("Expected Command to be 'ls -la', got '%s'", resp.Command)
	}
	if resp.RiskLevel != RiskSafe {
		t.Errorf("Expected RiskLevel to be RiskSafe, got %v", resp.RiskLevel)
	}
	if resp.Confidence != 0.95 {
		t.Errorf("Expected Confidence to be 0.95, got %f", resp.Confidence)
	}
	if len(resp.Alternatives) != 2 {
		t.Errorf("Expected 2 alternatives, got %d", len(resp.Alternatives))
	}
}

func TestCommandAnalysis_Fields(t *testing.T) {
	analysis := CommandAnalysis{
		Command:       "rm -rf /tmp/test",
		RiskLevel:     RiskDangerous,
		RiskReasons:   []string{"Recursive force deletion"},
		AffectedPaths: []string{"/tmp/test"},
		Actions:       []string{"DELETE files/directories"},
		Reversible:    false,
		RequiresSudo:  false,
		Patterns: []MatchedPattern{
			{Pattern: "rm -rf", Description: "Force delete", RiskLevel: RiskDangerous},
		},
	}

	if analysis.Command != "rm -rf /tmp/test" {
		t.Errorf("Expected Command to be 'rm -rf /tmp/test', got '%s'", analysis.Command)
	}
	if analysis.Reversible {
		t.Error("Expected Reversible to be false")
	}
	if len(analysis.Patterns) != 1 {
		t.Errorf("Expected 1 pattern, got %d", len(analysis.Patterns))
	}
}

func TestFileInfo_Fields(t *testing.T) {
	file := FileInfo{
		Path:      "/Users/test/file.txt",
		Size:      1024,
		IsDir:     false,
		FileCount: 0,
	}

	if file.Path != "/Users/test/file.txt" {
		t.Errorf("Expected Path to be '/Users/test/file.txt', got '%s'", file.Path)
	}
	if file.Size != 1024 {
		t.Errorf("Expected Size to be 1024, got %d", file.Size)
	}
	if file.IsDir {
		t.Error("Expected IsDir to be false")
	}

	dir := FileInfo{
		Path:      "/Users/test/folder",
		Size:      4096,
		IsDir:     true,
		FileCount: 10,
	}

	if !dir.IsDir {
		t.Error("Expected IsDir to be true for directory")
	}
	if dir.FileCount != 10 {
		t.Errorf("Expected FileCount to be 10, got %d", dir.FileCount)
	}
}

func TestHistoryEntry_Fields(t *testing.T) {
	entry := HistoryEntry{
		ID:           "test-id-123",
		Prompt:       "list files",
		GeneratedCmd: "ls -la",
		RiskLevel:    RiskSafe,
		Executed:     true,
		ExitCode:     0,
		DurationMs:   150,
		WorkingDir:   "/Users/test",
		Provider:     "openai",
		Model:        "gpt-4o",
	}

	if entry.ID != "test-id-123" {
		t.Errorf("Expected ID to be 'test-id-123', got '%s'", entry.ID)
	}
	if entry.ExitCode != 0 {
		t.Errorf("Expected ExitCode to be 0, got %d", entry.ExitCode)
	}
	if !entry.Executed {
		t.Error("Expected Executed to be true")
	}
}

func TestMCPTool_Fields(t *testing.T) {
	tool := MCPTool{
		Name:        "read_file",
		Description: "Read contents of a file",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file",
				},
			},
		},
	}

	if tool.Name != "read_file" {
		t.Errorf("Expected Name to be 'read_file', got '%s'", tool.Name)
	}
	if tool.Description == "" {
		t.Error("Expected Description to not be empty")
	}
	if tool.InputSchema == nil {
		t.Error("Expected InputSchema to not be nil")
	}
}

func TestMCPToolCall_Fields(t *testing.T) {
	call := MCPToolCall{
		Name: "read_file",
		Arguments: map[string]interface{}{
			"path": "/tmp/test.txt",
		},
	}

	if call.Name != "read_file" {
		t.Errorf("Expected Name to be 'read_file', got '%s'", call.Name)
	}
	if call.Arguments["path"] != "/tmp/test.txt" {
		t.Errorf("Expected path argument to be '/tmp/test.txt', got '%v'", call.Arguments["path"])
	}
}

func TestMCPToolResult_Fields(t *testing.T) {
	result := MCPToolResult{
		Content: "file contents here",
		IsError: false,
	}

	if result.Content != "file contents here" {
		t.Errorf("Expected Content to be 'file contents here', got '%s'", result.Content)
	}
	if result.IsError {
		t.Error("Expected IsError to be false")
	}

	errorResult := MCPToolResult{
		Content: "file not found",
		IsError: true,
	}

	if !errorResult.IsError {
		t.Error("Expected IsError to be true for error result")
	}
}

func TestExecutionMode_Values(t *testing.T) {
	// Verify all execution modes have distinct values
	modes := []ExecutionMode{
		ModeInteractive,
		ModeAuto,
		ModeDryRun,
		ModeExplainOnly,
		ModeSilent,
	}

	seen := make(map[ExecutionMode]bool)
	for _, mode := range modes {
		if seen[mode] {
			t.Errorf("Duplicate execution mode value: %d", mode)
		}
		seen[mode] = true
	}

	// Verify ModeInteractive is the default (0)
	if ModeInteractive != 0 {
		t.Errorf("Expected ModeInteractive to be 0, got %d", ModeInteractive)
	}
}
