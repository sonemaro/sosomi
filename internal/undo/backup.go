// Package undo provides backup and rollback functionality
package undo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/soroush/sosomi/internal/types"
)

// Manager handles backup and restore operations
type Manager struct {
	backupDir     string
	maxSize       int64 // in bytes
	retentionDays int
	exclude       []string
}

// NewManager creates a new backup manager
func NewManager(backupDir string, maxSizeMB, retentionDays int, exclude []string) (*Manager, error) {
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	return &Manager{
		backupDir:     backupDir,
		maxSize:       int64(maxSizeMB) * 1024 * 1024,
		retentionDays: retentionDays,
		exclude:       exclude,
	}, nil
}

// CreateBackup creates a backup of the specified files before command execution
func (m *Manager) CreateBackup(command, workingDir string, paths []string) (*types.BackupEntry, error) {
	entry := &types.BackupEntry{
		ID:         uuid.New().String(),
		Timestamp:  time.Now(),
		Command:    command,
		WorkingDir: workingDir,
	}

	backupPath := filepath.Join(m.backupDir, entry.ID)
	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	var totalSize int64

	for _, path := range paths {
		// Expand path
		if strings.HasPrefix(path, "~") {
			home, _ := os.UserHomeDir()
			path = filepath.Join(home, path[1:])
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(workingDir, path)
		}

		// Check if path should be excluded
		if m.shouldExclude(path) {
			continue
		}

		info, err := os.Stat(path)
		if err != nil {
			continue // Skip non-existent paths
		}

		if info.IsDir() {
			// Backup directory recursively
			files, size, err := m.backupDirectory(path, backupPath)
			if err != nil {
				continue
			}
			entry.Files = append(entry.Files, files...)
			totalSize += size
		} else {
			// Backup single file
			file, size, err := m.backupFile(path, backupPath)
			if err != nil {
				continue
			}
			entry.Files = append(entry.Files, *file)
			totalSize += size
		}

		// Check size limit
		if totalSize > m.maxSize {
			// Remove partial backup
			os.RemoveAll(backupPath)
			return nil, fmt.Errorf("backup exceeds maximum size limit (%d MB)", m.maxSize/(1024*1024))
		}
	}

	entry.TotalSize = totalSize

	// Save metadata
	metadataPath := filepath.Join(backupPath, "metadata.json")
	metadataData, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		os.RemoveAll(backupPath)
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if err := os.WriteFile(metadataPath, metadataData, 0644); err != nil {
		os.RemoveAll(backupPath)
		return nil, fmt.Errorf("failed to write metadata: %w", err)
	}

	return entry, nil
}

// backupFile backs up a single file
func (m *Manager) backupFile(srcPath, backupPath string) (*types.BackedUpFile, int64, error) {
	src, err := os.Open(srcPath)
	if err != nil {
		return nil, 0, err
	}
	defer src.Close()

	info, err := src.Stat()
	if err != nil {
		return nil, 0, err
	}

	// Create hash of the file
	hasher := sha256.New()
	if _, err := io.Copy(hasher, src); err != nil {
		return nil, 0, err
	}
	hash := hex.EncodeToString(hasher.Sum(nil))

	// Reset file position
	src.Seek(0, 0)

	// Create backup file
	relPath := strings.TrimPrefix(srcPath, "/")
	dstPath := filepath.Join(backupPath, "files", relPath)
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return nil, 0, err
	}

	dst, err := os.Create(dstPath)
	if err != nil {
		return nil, 0, err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return nil, 0, err
	}

	return &types.BackedUpFile{
		OriginalPath: srcPath,
		BackupPath:   dstPath,
		Hash:         hash,
		Size:         info.Size(),
		IsDir:        false,
	}, info.Size(), nil
}

// backupDirectory backs up a directory recursively
func (m *Manager) backupDirectory(srcPath, backupPath string) ([]types.BackedUpFile, int64, error) {
	var files []types.BackedUpFile
	var totalSize int64

	err := filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible files
		}

		if m.shouldExclude(path) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			return nil
		}

		file, size, err := m.backupFile(path, backupPath)
		if err != nil {
			return nil // Skip files that can't be backed up
		}

		files = append(files, *file)
		totalSize += size

		return nil
	})

	return files, totalSize, err
}

// shouldExclude checks if a path should be excluded from backup
func (m *Manager) shouldExclude(path string) bool {
	name := filepath.Base(path)
	for _, pattern := range m.exclude {
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

// Restore restores files from a backup
func (m *Manager) Restore(backupID string) error {
	backupPath := filepath.Join(m.backupDir, backupID)
	
	// Read metadata
	metadataPath := filepath.Join(backupPath, "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to read backup metadata: %w", err)
	}

	var entry types.BackupEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return fmt.Errorf("failed to parse backup metadata: %w", err)
	}

	// Restore each file
	for _, file := range entry.Files {
		if file.IsDir {
			continue
		}

		// Ensure target directory exists
		if err := os.MkdirAll(filepath.Dir(file.OriginalPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", file.OriginalPath, err)
		}

		// Copy file back
		src, err := os.Open(file.BackupPath)
		if err != nil {
			return fmt.Errorf("failed to open backup file %s: %w", file.BackupPath, err)
		}

		dst, err := os.Create(file.OriginalPath)
		if err != nil {
			src.Close()
			return fmt.Errorf("failed to create restore file %s: %w", file.OriginalPath, err)
		}

		_, err = io.Copy(dst, src)
		src.Close()
		dst.Close()

		if err != nil {
			return fmt.Errorf("failed to restore file %s: %w", file.OriginalPath, err)
		}
	}

	// Update metadata to mark as restored
	now := time.Now()
	entry.Restored = true
	entry.RestoredAt = &now

	data, _ = json.MarshalIndent(entry, "", "  ")
	os.WriteFile(metadataPath, data, 0644)

	return nil
}

// ListBackups returns all available backups
func (m *Manager) ListBackups() ([]*types.BackupEntry, error) {
	entries, err := os.ReadDir(m.backupDir)
	if err != nil {
		return nil, err
	}

	var backups []*types.BackupEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		metadataPath := filepath.Join(m.backupDir, entry.Name(), "metadata.json")
		data, err := os.ReadFile(metadataPath)
		if err != nil {
			continue
		}

		var backup types.BackupEntry
		if err := json.Unmarshal(data, &backup); err != nil {
			continue
		}

		backups = append(backups, &backup)
	}

	// Sort by timestamp (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp.After(backups[j].Timestamp)
	})

	return backups, nil
}

// GetBackup retrieves a specific backup
func (m *Manager) GetBackup(id string) (*types.BackupEntry, error) {
	metadataPath := filepath.Join(m.backupDir, id, "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("backup not found: %w", err)
	}

	var backup types.BackupEntry
	if err := json.Unmarshal(data, &backup); err != nil {
		return nil, fmt.Errorf("failed to parse backup metadata: %w", err)
	}

	return &backup, nil
}

// GetLastBackup returns the most recent backup
func (m *Manager) GetLastBackup() (*types.BackupEntry, error) {
	backups, err := m.ListBackups()
	if err != nil {
		return nil, err
	}

	if len(backups) == 0 {
		return nil, fmt.Errorf("no backups available")
	}

	return backups[0], nil
}

// DeleteBackup removes a backup
func (m *Manager) DeleteBackup(id string) error {
	backupPath := filepath.Join(m.backupDir, id)
	return os.RemoveAll(backupPath)
}

// Cleanup removes old backups
func (m *Manager) Cleanup() error {
	cutoff := time.Now().AddDate(0, 0, -m.retentionDays)

	backups, err := m.ListBackups()
	if err != nil {
		return err
	}

	for _, backup := range backups {
		if backup.Timestamp.Before(cutoff) {
			m.DeleteBackup(backup.ID)
		}
	}

	return nil
}

// GetTotalSize returns the total size of all backups
func (m *Manager) GetTotalSize() (int64, error) {
	var total int64

	err := filepath.Walk(m.backupDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})

	return total, err
}
