// Package shell tests
package shell

import (
	"os"
	"runtime"
	"testing"
)

func TestGetSystemContext(t *testing.T) {
	ctx := GetSystemContext()

	// Test OS detection
	if ctx.OS != runtime.GOOS {
		t.Errorf("Expected OS to be '%s', got '%s'", runtime.GOOS, ctx.OS)
	}

	// Test shell detection
	if ctx.Shell == "" {
		t.Error("Expected Shell to not be empty")
	}

	// Test current directory
	wd, _ := os.Getwd()
	if ctx.CurrentDir != wd {
		t.Errorf("Expected CurrentDir to be '%s', got '%s'", wd, ctx.CurrentDir)
	}

	// Test home directory
	home, _ := os.UserHomeDir()
	if ctx.HomeDir != home {
		t.Errorf("Expected HomeDir to be '%s', got '%s'", home, ctx.HomeDir)
	}

	// Test username
	if ctx.Username == "" {
		t.Error("Expected Username to not be empty")
	}
}

func TestGetShell(t *testing.T) {
	// Save original SHELL env
	originalShell := os.Getenv("SHELL")
	defer os.Setenv("SHELL", originalShell)

	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{"zsh", "/bin/zsh", "zsh"},
		{"bash", "/bin/bash", "bash"},
		{"fish", "/usr/local/bin/fish", "fish"},
		{"sh default", "", "sh"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("SHELL", tt.envValue)
			got := getShell()
			if got != tt.expected {
				t.Errorf("getShell() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestExecute_DryRun(t *testing.T) {
	result, err := Execute("echo hello", true)
	if err != nil {
		t.Fatalf("Execute dry run failed: %v", err)
	}

	if result == nil {
		t.Fatal("Execute returned nil result")
	}

	if result.Command != "echo hello" {
		t.Errorf("Expected Command to be 'echo hello', got '%s'", result.Command)
	}

	if result.Stdout != "[DRY RUN] Would execute: echo hello" {
		t.Errorf("Expected dry run message, got '%s'", result.Stdout)
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected ExitCode to be 0 for dry run, got %d", result.ExitCode)
	}
}

func TestExecute_SimpleCommand(t *testing.T) {
	result, err := Execute("echo 'test output'", false)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result == nil {
		t.Fatal("Execute returned nil result")
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected ExitCode to be 0, got %d", result.ExitCode)
	}

	if result.Stdout == "" {
		t.Error("Expected Stdout to contain output")
	}

	if result.DurationMs < 0 {
		t.Error("Expected DurationMs to be non-negative")
	}
}

func TestExecute_FailingCommand(t *testing.T) {
	result, err := Execute("exit 1", false)
	if err != nil {
		t.Fatalf("Execute failed unexpectedly: %v", err)
	}

	if result.ExitCode != 1 {
		t.Errorf("Expected ExitCode to be 1, got %d", result.ExitCode)
	}

	if result.Error == nil {
		t.Error("Expected Error to be set for failing command")
	}
}

func TestExecute_CommandWithStderr(t *testing.T) {
	result, err := Execute("echo error >&2", false)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Stderr == "" {
		t.Error("Expected Stderr to contain output")
	}
}

func TestExecute_InvalidCommand(t *testing.T) {
	result, err := Execute("nonexistent_command_12345", false)
	if err != nil {
		t.Fatalf("Execute failed unexpectedly: %v", err)
	}

	if result.ExitCode == 0 {
		t.Error("Expected non-zero exit code for invalid command")
	}
}

func TestExecuteResult_Fields(t *testing.T) {
	result := &ExecuteResult{
		Command:    "test command",
		ExitCode:   0,
		Stdout:     "output",
		Stderr:     "",
		DurationMs: 100,
		Error:      nil,
	}

	if result.Command != "test command" {
		t.Errorf("Expected Command to be 'test command', got '%s'", result.Command)
	}
	if result.ExitCode != 0 {
		t.Errorf("Expected ExitCode to be 0, got %d", result.ExitCode)
	}
	if result.DurationMs != 100 {
		t.Errorf("Expected DurationMs to be 100, got %d", result.DurationMs)
	}
}

func TestDetectPackageManagers(t *testing.T) {
	managers := detectPackageManagers()

	// On any Unix system, at least one package manager should be present
	// This is a sanity check - we won't fail if none are found
	t.Logf("Detected package managers: %v", managers)

	// Verify the function returns a slice
	if managers == nil {
		t.Error("Expected managers to be a slice, got nil")
	}
}

func TestGetGitBranch(t *testing.T) {
	// This test depends on whether we're in a git repo
	branch := getGitBranch()
	t.Logf("Git branch: %s", branch)

	// We can't reliably test the branch name, but we can verify
	// the function doesn't panic and returns a string
}

func TestGetGitStatus(t *testing.T) {
	status := getGitStatus()
	t.Logf("Git status: %s", status)

	// Status should be empty, "clean", or "dirty"
	validStatuses := []string{"", "clean", "dirty"}
	valid := false
	for _, v := range validStatuses {
		if status == v {
			valid = true
			break
		}
	}
	if !valid {
		t.Errorf("Unexpected git status: %s", status)
	}
}

func TestGetGitRemote(t *testing.T) {
	remote := getGitRemote()
	t.Logf("Git remote: %s", remote)

	// We can't reliably test the remote, but we verify it doesn't panic
}

func TestGetRecentCommands(t *testing.T) {
	cmds := getRecentCommands(5)
	t.Logf("Recent commands: %v", cmds)

	// Verify we get at most 5 commands
	if len(cmds) > 5 {
		t.Errorf("Expected at most 5 commands, got %d", len(cmds))
	}

	// Verify the function returns a slice
	if cmds == nil {
		// nil is acceptable if no history file exists
		t.Log("No recent commands found (history file may not exist)")
	}
}

func TestNow(t *testing.T) {
	n1 := now()
	n2 := now()

	if n2 < n1 {
		t.Error("now() should return non-decreasing values")
	}

	// Verify it returns milliseconds (should be a large number)
	if n1 < 1000000000000 { // After year 2001 in ms
		t.Error("now() should return milliseconds since epoch")
	}
}

func TestExecute_WithPipeline(t *testing.T) {
	result, err := Execute("echo hello | cat", false)
	if err != nil {
		t.Fatalf("Execute with pipeline failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected ExitCode to be 0, got %d", result.ExitCode)
	}

	if result.Stdout == "" {
		t.Error("Expected Stdout to contain output from pipeline")
	}
}

func TestExecute_WithVariables(t *testing.T) {
	result, err := Execute("VAR=hello; echo $VAR", false)
	if err != nil {
		t.Fatalf("Execute with variables failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected ExitCode to be 0, got %d", result.ExitCode)
	}
}

func TestExecute_WorkingDirectory(t *testing.T) {
	// Execute should use current working directory
	result, err := Execute("pwd", false)
	if err != nil {
		t.Fatalf("Execute pwd failed: %v", err)
	}

	wd, _ := os.Getwd()
	if result.Stdout == "" {
		t.Error("Expected pwd output")
	}
	t.Logf("pwd output: %s, working dir: %s", result.Stdout, wd)
}
