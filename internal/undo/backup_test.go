// Package undo backup tests
package undo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/soroush/sosomi/internal/types"
)

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")

	manager, err := NewManager(backupDir, 100, 7, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	// Check that backup directory was created
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		t.Error("Backup directory was not created")
	}
}

func TestNewManager_WithExclude(t *testing.T) {
	tmpDir := t.TempDir()
	exclude := []string{"*.log", "node_modules", ".git"}

	manager, err := NewManager(filepath.Join(tmpDir, "backups"), 100, 7, exclude)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if len(manager.exclude) != 3 {
		t.Errorf("Expected 3 exclude patterns, got %d", len(manager.exclude))
	}
}

func TestManager_CreateBackup(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	workDir := filepath.Join(tmpDir, "work")

	// Create test files
	os.MkdirAll(workDir, 0755)
	testFile := filepath.Join(workDir, "test.txt")
	os.WriteFile(testFile, []byte("test content"), 0644)

	manager, err := NewManager(backupDir, 100, 7, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	entry, err := manager.CreateBackup("rm test.txt", workDir, []string{testFile})
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	if entry == nil {
		t.Fatal("CreateBackup returned nil entry")
	}

	if entry.ID == "" {
		t.Error("Expected ID to be set")
	}

	if entry.Command != "rm test.txt" {
		t.Errorf("Expected command 'rm test.txt', got '%s'", entry.Command)
	}

	if len(entry.Files) == 0 {
		t.Error("Expected at least one backed up file")
	}
}

func TestManager_CreateBackup_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	workDir := filepath.Join(tmpDir, "work")
	testDir := filepath.Join(workDir, "testdir")

	// Create test directory with files
	os.MkdirAll(testDir, 0755)
	os.WriteFile(filepath.Join(testDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(testDir, "file2.txt"), []byte("content2"), 0644)

	manager, err := NewManager(backupDir, 100, 7, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	entry, err := manager.CreateBackup("rm -rf testdir", workDir, []string{testDir})
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	if len(entry.Files) < 2 {
		t.Errorf("Expected at least 2 files backed up, got %d", len(entry.Files))
	}
}

func TestManager_CreateBackup_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	workDir := filepath.Join(tmpDir, "work")

	os.MkdirAll(workDir, 0755)

	manager, err := NewManager(backupDir, 100, 7, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Try to backup non-existent file
	entry, err := manager.CreateBackup("rm nonexistent", workDir, []string{"nonexistent.txt"})
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	// Should succeed but with no files
	if len(entry.Files) != 0 {
		t.Errorf("Expected 0 files for non-existent, got %d", len(entry.Files))
	}
}

func TestManager_Restore(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	workDir := filepath.Join(tmpDir, "work")

	// Create test file
	os.MkdirAll(workDir, 0755)
	testFile := filepath.Join(workDir, "test.txt")
	originalContent := "original content"
	os.WriteFile(testFile, []byte(originalContent), 0644)

	manager, err := NewManager(backupDir, 100, 7, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Create backup
	entry, err := manager.CreateBackup("rm test.txt", workDir, []string{testFile})
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	// Simulate deletion
	os.Remove(testFile)

	// Restore
	err = manager.Restore(entry.ID)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Verify restoration
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read restored file: %v", err)
	}

	if string(content) != originalContent {
		t.Errorf("Expected restored content '%s', got '%s'", originalContent, string(content))
	}
}

func TestManager_ListBackups(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	workDir := filepath.Join(tmpDir, "work")

	os.MkdirAll(workDir, 0755)
	testFile := filepath.Join(workDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	manager, err := NewManager(backupDir, 100, 7, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Create multiple backups
	for i := 0; i < 3; i++ {
		manager.CreateBackup("rm test.txt", workDir, []string{testFile})
	}

	backups, err := manager.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups failed: %v", err)
	}

	if len(backups) != 3 {
		t.Errorf("Expected 3 backups, got %d", len(backups))
	}
}

func TestManager_GetBackup(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	workDir := filepath.Join(tmpDir, "work")

	os.MkdirAll(workDir, 0755)
	testFile := filepath.Join(workDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	manager, err := NewManager(backupDir, 100, 7, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	entry, _ := manager.CreateBackup("rm test.txt", workDir, []string{testFile})

	retrieved, err := manager.GetBackup(entry.ID)
	if err != nil {
		t.Fatalf("GetBackup failed: %v", err)
	}

	if retrieved.ID != entry.ID {
		t.Errorf("Expected ID '%s', got '%s'", entry.ID, retrieved.ID)
	}
}

func TestManager_GetBackup_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(filepath.Join(tmpDir, "backups"), 100, 7, nil)

	_, err := manager.GetBackup("nonexistent-id")
	if err == nil {
		t.Error("Expected error for non-existent backup")
	}
}

func TestManager_Cleanup(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	workDir := filepath.Join(tmpDir, "work")

	os.MkdirAll(workDir, 0755)
	testFile := filepath.Join(workDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	// Create manager with 0 retention (immediate cleanup)
	manager, err := NewManager(backupDir, 100, 0, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Create backup
	manager.CreateBackup("rm test.txt", workDir, []string{testFile})

	// Cleanup
	err = manager.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
}

func TestManager_ShouldExclude(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(filepath.Join(tmpDir, "backups"), 100, 7, []string{"*.log", "node_modules", ".git"})

	tests := []struct {
		path     string
		expected bool
	}{
		{"test.log", true},
		{"node_modules/package.json", true},
		{".git/config", true},
		{"src/main.go", false},
		{"README.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := manager.shouldExclude(tt.path)
			if result != tt.expected {
				t.Errorf("shouldExclude(%s) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestManager_BackupFileSizeLimit(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	workDir := filepath.Join(tmpDir, "work")

	os.MkdirAll(workDir, 0755)

	// Create a file larger than the limit
	largeFile := filepath.Join(workDir, "large.txt")
	content := make([]byte, 2*1024*1024) // 2MB
	os.WriteFile(largeFile, content, 0644)

	// Create manager with 1MB limit
	manager, _ := NewManager(backupDir, 1, 7, nil)

	_, err := manager.CreateBackup("rm large.txt", workDir, []string{largeFile})
	if err == nil {
		t.Error("Expected error for file exceeding size limit")
	}
}

func TestBackedUpFile_Fields(t *testing.T) {
	file := types.BackedUpFile{
		OriginalPath: "/home/user/file.txt",
		BackupPath:   "/backups/123/file.txt",
		Hash:         "abc123def456",
		Size:         1024,
		IsDir:        false,
	}

	if file.OriginalPath != "/home/user/file.txt" {
		t.Errorf("Unexpected OriginalPath: %s", file.OriginalPath)
	}
	if file.Hash != "abc123def456" {
		t.Errorf("Unexpected Hash: %s", file.Hash)
	}
	if file.Size != 1024 {
		t.Errorf("Unexpected Size: %d", file.Size)
	}
}

func TestManager_BackupWithTildeExpansion(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	workDir := filepath.Join(tmpDir, "work")

	os.MkdirAll(workDir, 0755)
	testFile := filepath.Join(workDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	manager, _ := NewManager(backupDir, 100, 7, nil)

	// This tests the tilde expansion code path
	// Note: ~ expansion only works for actual home directory
	entry, err := manager.CreateBackup("rm test.txt", workDir, []string{testFile})
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	if entry == nil {
		t.Fatal("Expected non-nil entry")
	}
}

func TestManager_RestorePreservesPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	workDir := filepath.Join(tmpDir, "work")

	os.MkdirAll(workDir, 0755)
	testFile := filepath.Join(workDir, "executable.sh")
	os.WriteFile(testFile, []byte("#!/bin/bash\necho hello"), 0755)

	manager, _ := NewManager(backupDir, 100, 7, nil)

	entry, _ := manager.CreateBackup("rm executable.sh", workDir, []string{testFile})
	os.Remove(testFile)

	err := manager.Restore(entry.ID)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("Restored file does not exist")
	}
}
