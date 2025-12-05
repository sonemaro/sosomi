// Package history tests
package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/soroush/sosomi/internal/types"
)

func TestNewStore(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_history.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	if store == nil {
		t.Fatal("NewStore returned nil")
	}

	// Check that database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}
}

func TestNewStore_InvalidPath(t *testing.T) {
	// Try to create store in non-existent directory without permission
	_, err := NewStore("/nonexistent/path/test.db")
	if err == nil {
		t.Error("Expected error for invalid path")
	}
}

func TestStore_AddCommand(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	entry := &types.HistoryEntry{
		Prompt:       "list files",
		GeneratedCmd: "ls -la",
		RiskLevel:    types.RiskSafe,
		Executed:     false,
		WorkingDir:   "/tmp",
		Provider:     "openai",
		Model:        "gpt-4o",
	}

	err = store.AddCommand(entry)
	if err != nil {
		t.Fatalf("AddCommand failed: %v", err)
	}

	// ID should be set
	if entry.ID == "" {
		t.Error("Expected ID to be set")
	}

	// Timestamp should be set
	if entry.Timestamp.IsZero() {
		t.Error("Expected Timestamp to be set")
	}
}

func TestStore_AddCommand_WithExistingID(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	entry := &types.HistoryEntry{
		ID:           "custom-id-123",
		Prompt:       "test prompt",
		GeneratedCmd: "echo test",
		RiskLevel:    types.RiskSafe,
	}

	err = store.AddCommand(entry)
	if err != nil {
		t.Fatalf("AddCommand failed: %v", err)
	}

	if entry.ID != "custom-id-123" {
		t.Errorf("Expected ID to remain 'custom-id-123', got '%s'", entry.ID)
	}
}

func TestStore_GetCommand(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	entry := &types.HistoryEntry{
		Prompt:       "get files",
		GeneratedCmd: "find . -type f",
		RiskLevel:    types.RiskSafe,
		Executed:     true,
		ExitCode:     0,
		DurationMs:   100,
	}

	store.AddCommand(entry)

	retrieved, err := store.GetCommand(entry.ID)
	if err != nil {
		t.Fatalf("GetCommand failed: %v", err)
	}

	if retrieved.Prompt != entry.Prompt {
		t.Errorf("Expected Prompt '%s', got '%s'", entry.Prompt, retrieved.Prompt)
	}
	if retrieved.GeneratedCmd != entry.GeneratedCmd {
		t.Errorf("Expected GeneratedCmd '%s', got '%s'", entry.GeneratedCmd, retrieved.GeneratedCmd)
	}
	if retrieved.RiskLevel != entry.RiskLevel {
		t.Errorf("Expected RiskLevel %v, got %v", entry.RiskLevel, retrieved.RiskLevel)
	}
}

func TestStore_GetCommand_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	_, err = store.GetCommand("nonexistent-id")
	if err == nil {
		t.Error("Expected error for non-existent command")
	}
}

func TestStore_ListCommands(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	// Add multiple commands
	for i := 0; i < 5; i++ {
		entry := &types.HistoryEntry{
			Prompt:       "test prompt",
			GeneratedCmd: "echo test",
			RiskLevel:    types.RiskSafe,
		}
		store.AddCommand(entry)
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// List all
	entries, err := store.ListCommands(10, 0, "")
	if err != nil {
		t.Fatalf("ListCommands failed: %v", err)
	}

	if len(entries) != 5 {
		t.Errorf("Expected 5 entries, got %d", len(entries))
	}

	// List with limit
	entries, err = store.ListCommands(3, 0, "")
	if err != nil {
		t.Fatalf("ListCommands failed: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(entries))
	}
}

func TestStore_ListCommands_WithOffset(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	// Add commands
	for i := 0; i < 5; i++ {
		entry := &types.HistoryEntry{
			Prompt:       "test prompt",
			GeneratedCmd: "echo test",
			RiskLevel:    types.RiskSafe,
		}
		store.AddCommand(entry)
	}

	// List with offset
	entries, err := store.ListCommands(10, 3, "")
	if err != nil {
		t.Fatalf("ListCommands failed: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("Expected 2 entries with offset 3, got %d", len(entries))
	}
}

func TestStore_ListCommands_WithRiskFilter(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	// Add commands with different risk levels
	entries := []*types.HistoryEntry{
		{Prompt: "safe", GeneratedCmd: "echo safe", RiskLevel: types.RiskSafe},
		{Prompt: "caution", GeneratedCmd: "rm file", RiskLevel: types.RiskCaution},
		{Prompt: "dangerous", GeneratedCmd: "rm -rf", RiskLevel: types.RiskDangerous},
	}

	for _, e := range entries {
		store.AddCommand(e)
	}

	// Filter by risk
	results, err := store.ListCommands(10, 0, "SAFE")
	if err != nil {
		t.Fatalf("ListCommands failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 SAFE entry, got %d", len(results))
	}
}

func TestStore_SearchCommands(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	entries := []*types.HistoryEntry{
		{Prompt: "list files in directory", GeneratedCmd: "ls -la", RiskLevel: types.RiskSafe},
		{Prompt: "count lines in file", GeneratedCmd: "wc -l file.txt", RiskLevel: types.RiskSafe},
		{Prompt: "find large files", GeneratedCmd: "find . -size +100M", RiskLevel: types.RiskSafe},
	}

	for _, e := range entries {
		store.AddCommand(e)
	}

	// Search by prompt
	results, err := store.SearchCommands("files", 10)
	if err != nil {
		t.Fatalf("SearchCommands failed: %v", err)
	}

	if len(results) < 2 {
		t.Errorf("Expected at least 2 results for 'files', got %d", len(results))
	}

	// Search by command
	results, err = store.SearchCommands("wc", 10)
	if err != nil {
		t.Fatalf("SearchCommands failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result for 'wc', got %d", len(results))
	}
}

func TestStore_UpdateExecuted(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	entry := &types.HistoryEntry{
		Prompt:       "test",
		GeneratedCmd: "echo test",
		RiskLevel:    types.RiskSafe,
		Executed:     false,
	}
	store.AddCommand(entry)

	// Update execution status
	err = store.UpdateExecuted(entry.ID, true, 0, 150)
	if err != nil {
		t.Fatalf("UpdateExecuted failed: %v", err)
	}

	// Verify update
	retrieved, _ := store.GetCommand(entry.ID)
	if !retrieved.Executed {
		t.Error("Expected Executed to be true")
	}
	if retrieved.ExitCode != 0 {
		t.Errorf("Expected ExitCode 0, got %d", retrieved.ExitCode)
	}
	if retrieved.DurationMs != 150 {
		t.Errorf("Expected DurationMs 150, got %d", retrieved.DurationMs)
	}
}

func TestStore_GetStats(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	// Add some commands
	entries := []*types.HistoryEntry{
		{Prompt: "p1", GeneratedCmd: "c1", RiskLevel: types.RiskSafe, Executed: true},
		{Prompt: "p2", GeneratedCmd: "c2", RiskLevel: types.RiskSafe, Executed: true},
		{Prompt: "p3", GeneratedCmd: "c3", RiskLevel: types.RiskCaution, Executed: false},
	}

	for _, e := range entries {
		store.AddCommand(e)
	}

	stats, err := store.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	totalCommands, ok := stats["total_commands"].(int)
	if !ok || totalCommands != 3 {
		t.Errorf("Expected total_commands 3, got %v", stats["total_commands"])
	}

	executedCommands, ok := stats["executed_commands"].(int)
	if !ok || executedCommands != 2 {
		t.Errorf("Expected executed_commands 2, got %v", stats["executed_commands"])
	}
}

func TestParseRiskLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected types.RiskLevel
	}{
		{"SAFE", types.RiskSafe},
		{"CAUTION", types.RiskCaution},
		{"DANGEROUS", types.RiskDangerous},
		{"CRITICAL", types.RiskCritical},
		{"unknown", types.RiskCaution},  // Default to caution
		{"", types.RiskCaution},         // Default to caution
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseRiskLevel(tt.input)
			if result != tt.expected {
				t.Errorf("parseRiskLevel(%s) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
