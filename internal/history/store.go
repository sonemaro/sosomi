// Package history provides command history and audit logging
package history

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/soroush/sosomi/internal/types"
)

// Store manages command history storage
type Store struct {
	db *sql.DB
}

// NewStore creates a new history store
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &Store{db: db}
	if err := store.initialize(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return store, nil
}

// initialize creates the database schema
func (s *Store) initialize() error {
	schema := `
	CREATE TABLE IF NOT EXISTS commands (
		id TEXT PRIMARY KEY,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		prompt TEXT NOT NULL,
		generated_cmd TEXT NOT NULL,
		risk_level TEXT,
		executed INTEGER DEFAULT 0,
		exit_code INTEGER,
		duration_ms INTEGER,
		working_dir TEXT,
		provider TEXT,
		model TEXT,
		prompt_tokens INTEGER DEFAULT 0,
		completion_tokens INTEGER DEFAULT 0,
		total_tokens INTEGER DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS backups (
		id TEXT PRIMARY KEY,
		command_id TEXT REFERENCES commands(id),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		restored_at DATETIME,
		command TEXT,
		working_dir TEXT,
		files_json TEXT,
		total_size INTEGER
	);

	CREATE INDEX IF NOT EXISTS idx_commands_timestamp ON commands(timestamp);
	CREATE INDEX IF NOT EXISTS idx_commands_risk ON commands(risk_level);
	CREATE INDEX IF NOT EXISTS idx_backups_command ON backups(command_id);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return err
	}

	// Run migration to add token columns if they don't exist
	return s.migrateTokenColumns()
}

// migrateTokenColumns adds token columns to existing databases
func (s *Store) migrateTokenColumns() error {
	// Check if columns exist by trying to query them
	_, err := s.db.Query("SELECT prompt_tokens FROM commands LIMIT 1")
	if err != nil {
		// Columns don't exist, add them
		migrations := []string{
			"ALTER TABLE commands ADD COLUMN prompt_tokens INTEGER DEFAULT 0",
			"ALTER TABLE commands ADD COLUMN completion_tokens INTEGER DEFAULT 0",
			"ALTER TABLE commands ADD COLUMN total_tokens INTEGER DEFAULT 0",
		}
		for _, m := range migrations {
			if _, err := s.db.Exec(m); err != nil {
				// Ignore errors if column already exists
				continue
			}
		}
	}
	return nil
}

// AddCommand adds a command to history
func (s *Store) AddCommand(entry *types.HistoryEntry) error {
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	_, err := s.db.Exec(`
		INSERT INTO commands (id, timestamp, prompt, generated_cmd, risk_level, executed, exit_code, duration_ms, working_dir, provider, model, prompt_tokens, completion_tokens, total_tokens)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		entry.ID,
		entry.Timestamp,
		entry.Prompt,
		entry.GeneratedCmd,
		entry.RiskLevel.String(),
		entry.Executed,
		entry.ExitCode,
		entry.DurationMs,
		entry.WorkingDir,
		entry.Provider,
		entry.Model,
		entry.PromptTokens,
		entry.CompletionTokens,
		entry.TotalTokens,
	)
	return err
}

// UpdateExecuted updates the execution status of a command
func (s *Store) UpdateExecuted(id string, executed bool, exitCode int, durationMs int64) error {
	_, err := s.db.Exec(`
		UPDATE commands SET executed = ?, exit_code = ?, duration_ms = ? WHERE id = ?
	`, executed, exitCode, durationMs, id)
	return err
}

// GetCommand retrieves a command by ID
func (s *Store) GetCommand(id string) (*types.HistoryEntry, error) {
	row := s.db.QueryRow(`
		SELECT id, timestamp, prompt, generated_cmd, risk_level, executed, exit_code, duration_ms, working_dir, provider, model, COALESCE(prompt_tokens, 0), COALESCE(completion_tokens, 0), COALESCE(total_tokens, 0)
		FROM commands WHERE id = ?
	`, id)

	entry := &types.HistoryEntry{}
	var riskLevel string
	err := row.Scan(
		&entry.ID,
		&entry.Timestamp,
		&entry.Prompt,
		&entry.GeneratedCmd,
		&riskLevel,
		&entry.Executed,
		&entry.ExitCode,
		&entry.DurationMs,
		&entry.WorkingDir,
		&entry.Provider,
		&entry.Model,
		&entry.PromptTokens,
		&entry.CompletionTokens,
		&entry.TotalTokens,
	)
	if err != nil {
		return nil, err
	}

	entry.RiskLevel = parseRiskLevel(riskLevel)
	return entry, nil
}

// ListCommands lists commands with optional filters
func (s *Store) ListCommands(limit int, offset int, riskFilter string) ([]*types.HistoryEntry, error) {
	query := `
		SELECT id, timestamp, prompt, generated_cmd, risk_level, executed, exit_code, duration_ms, working_dir, provider, model, COALESCE(prompt_tokens, 0), COALESCE(completion_tokens, 0), COALESCE(total_tokens, 0)
		FROM commands
	`

	var args []interface{}
	if riskFilter != "" {
		query += " WHERE risk_level = ?"
		args = append(args, riskFilter)
	}

	query += " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*types.HistoryEntry
	for rows.Next() {
		entry := &types.HistoryEntry{}
		var riskLevel string
		if err := rows.Scan(
			&entry.ID,
			&entry.Timestamp,
			&entry.Prompt,
			&entry.GeneratedCmd,
			&riskLevel,
			&entry.Executed,
			&entry.ExitCode,
			&entry.DurationMs,
			&entry.WorkingDir,
			&entry.Provider,
			&entry.Model,
			&entry.PromptTokens,
			&entry.CompletionTokens,
			&entry.TotalTokens,
		); err != nil {
			return nil, err
		}
		entry.RiskLevel = parseRiskLevel(riskLevel)
		entries = append(entries, entry)
	}

	return entries, nil
}

// SearchCommands searches commands by prompt or generated command
func (s *Store) SearchCommands(query string, limit int) ([]*types.HistoryEntry, error) {
	rows, err := s.db.Query(`
		SELECT id, timestamp, prompt, generated_cmd, risk_level, executed, exit_code, duration_ms, working_dir, provider, model, COALESCE(prompt_tokens, 0), COALESCE(completion_tokens, 0), COALESCE(total_tokens, 0)
		FROM commands
		WHERE prompt LIKE ? OR generated_cmd LIKE ?
		ORDER BY timestamp DESC
		LIMIT ?
	`, "%"+query+"%", "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*types.HistoryEntry
	for rows.Next() {
		entry := &types.HistoryEntry{}
		var riskLevel string
		if err := rows.Scan(
			&entry.ID,
			&entry.Timestamp,
			&entry.Prompt,
			&entry.GeneratedCmd,
			&riskLevel,
			&entry.Executed,
			&entry.ExitCode,
			&entry.DurationMs,
			&entry.WorkingDir,
			&entry.Provider,
			&entry.Model,
			&entry.PromptTokens,
			&entry.CompletionTokens,
			&entry.TotalTokens,
		); err != nil {
			return nil, err
		}
		entry.RiskLevel = parseRiskLevel(riskLevel)
		entries = append(entries, entry)
	}

	return entries, nil
}

// GetStats returns statistics about command history
func (s *Store) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total commands
	var total int
	s.db.QueryRow("SELECT COUNT(*) FROM commands").Scan(&total)
	stats["total_commands"] = total

	// Executed commands
	var executed int
	s.db.QueryRow("SELECT COUNT(*) FROM commands WHERE executed = 1").Scan(&executed)
	stats["executed_commands"] = executed

	// Total tokens
	var totalTokens int
	s.db.QueryRow("SELECT COALESCE(SUM(total_tokens), 0) FROM commands").Scan(&totalTokens)
	stats["total_tokens"] = totalTokens

	// Token breakdown
	var promptTokens, completionTokens int
	s.db.QueryRow("SELECT COALESCE(SUM(prompt_tokens), 0), COALESCE(SUM(completion_tokens), 0) FROM commands").Scan(&promptTokens, &completionTokens)
	stats["prompt_tokens"] = promptTokens
	stats["completion_tokens"] = completionTokens

	// Commands by risk level
	riskStats := make(map[string]int)
	rows, _ := s.db.Query("SELECT risk_level, COUNT(*) FROM commands GROUP BY risk_level")
	for rows.Next() {
		var level string
		var count int
		rows.Scan(&level, &count)
		riskStats[level] = count
	}
	rows.Close()
	stats["by_risk_level"] = riskStats

	// Commands by provider
	providerStats := make(map[string]int)
	rows, _ = s.db.Query("SELECT provider, COUNT(*) FROM commands WHERE provider != '' GROUP BY provider")
	for rows.Next() {
		var provider string
		var count int
		rows.Scan(&provider, &count)
		providerStats[provider] = count
	}
	rows.Close()
	stats["by_provider"] = providerStats

	return stats, nil
}

// Cleanup removes old entries
func (s *Store) Cleanup(retentionDays int) error {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	_, err := s.db.Exec("DELETE FROM commands WHERE timestamp < ?", cutoff)
	return err
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// parseRiskLevel converts a string to RiskLevel
func parseRiskLevel(s string) types.RiskLevel {
	switch s {
	case "SAFE":
		return types.RiskSafe
	case "CAUTION":
		return types.RiskCaution
	case "DANGEROUS":
		return types.RiskDangerous
	case "CRITICAL":
		return types.RiskCritical
	default:
		return types.RiskCaution
	}
}
