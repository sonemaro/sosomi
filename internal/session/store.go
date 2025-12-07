// Package session provides session storage for the shell chat mode
package session

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/soroush/sosomi/internal/types"
)

// Store manages session storage
type Store struct {
	db *sql.DB
}

// NewStore creates a new session store
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

func (s *Store) initialize() error {
	schema := `
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		provider TEXT,
		model TEXT,
		total_tokens INTEGER DEFAULT 0,
		message_count INTEGER DEFAULT 0,
		command_count INTEGER DEFAULT 0,
		last_cwd TEXT,
		auto_execute INTEGER DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS session_messages (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		tokens INTEGER DEFAULT 0,
		command TEXT,
		output TEXT,
		exit_code INTEGER DEFAULT 0,
		risk_level INTEGER DEFAULT 0,
		duration_ms INTEGER DEFAULT 0,
		executed INTEGER DEFAULT 0
	);

	CREATE INDEX IF NOT EXISTS idx_sessions_updated ON sessions(updated_at);
	CREATE INDEX IF NOT EXISTS idx_session_messages_session ON session_messages(session_id);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	// Migration: Add auto_execute column if it doesn't exist
	_, err := s.db.Exec(`ALTER TABLE sessions ADD COLUMN auto_execute INTEGER DEFAULT 0`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		// Ignore error if column already exists, but log others
	}

	return nil
}

// CreateSession creates a new session
func (s *Store) CreateSession(name, provider, model, cwd string) (*types.Session, error) {
	sess := &types.Session{
		ID:        uuid.New().String(),
		Name:      name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Provider:  provider,
		Model:     model,
		LastCwd:   cwd,
	}

	_, err := s.db.Exec(`
		INSERT INTO sessions (id, name, created_at, updated_at, provider, model, last_cwd)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, sess.ID, sess.Name, sess.CreatedAt, sess.UpdatedAt, sess.Provider, sess.Model, sess.LastCwd)

	if err != nil {
		return nil, err
	}
	return sess, nil
}

// GetSession retrieves a session by ID (supports partial ID match)
func (s *Store) GetSession(id string) (*types.Session, error) {
	row := s.db.QueryRow(`
		SELECT id, name, created_at, updated_at, provider, model, total_tokens, message_count, command_count, last_cwd, auto_execute
		FROM sessions WHERE id = ? OR id LIKE ?
	`, id, id+"%")

	sess := &types.Session{}
	var lastCwd sql.NullString
	err := row.Scan(&sess.ID, &sess.Name, &sess.CreatedAt, &sess.UpdatedAt, &sess.Provider, &sess.Model,
		&sess.TotalTokens, &sess.MessageCount, &sess.CommandCount, &lastCwd, &sess.AutoExecute)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found: %s", id)
		}
		return nil, err
	}
	if lastCwd.Valid {
		sess.LastCwd = lastCwd.String
	}
	return sess, nil
}

// GetSessionByName retrieves a session by name (partial match)
func (s *Store) GetSessionByName(name string) (*types.Session, error) {
	row := s.db.QueryRow(`
		SELECT id, name, created_at, updated_at, provider, model, total_tokens, message_count, command_count, last_cwd, auto_execute
		FROM sessions WHERE name LIKE ? ORDER BY updated_at DESC LIMIT 1
	`, "%"+name+"%")

	sess := &types.Session{}
	var lastCwd sql.NullString
	err := row.Scan(&sess.ID, &sess.Name, &sess.CreatedAt, &sess.UpdatedAt, &sess.Provider, &sess.Model,
		&sess.TotalTokens, &sess.MessageCount, &sess.CommandCount, &lastCwd, &sess.AutoExecute)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found: %s", name)
		}
		return nil, err
	}
	if lastCwd.Valid {
		sess.LastCwd = lastCwd.String
	}
	return sess, nil
}

// ListSessions lists all sessions ordered by updated_at
func (s *Store) ListSessions(limit, offset int) ([]*types.Session, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`
		SELECT id, name, created_at, updated_at, provider, model, total_tokens, message_count, command_count, last_cwd, auto_execute
		FROM sessions ORDER BY updated_at DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*types.Session
	for rows.Next() {
		sess := &types.Session{}
		var lastCwd sql.NullString
		if err := rows.Scan(&sess.ID, &sess.Name, &sess.CreatedAt, &sess.UpdatedAt, &sess.Provider, &sess.Model,
			&sess.TotalTokens, &sess.MessageCount, &sess.CommandCount, &lastCwd, &sess.AutoExecute); err != nil {
			return nil, err
		}
		if lastCwd.Valid {
			sess.LastCwd = lastCwd.String
		}
		sessions = append(sessions, sess)
	}
	return sessions, nil
}

// AddMessage adds a regular message (user/assistant) to the session
func (s *Store) AddMessage(sessionID, role, content string, tokens int) (*types.SessionMessage, error) {
	msg := &types.SessionMessage{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Role:      role,
		Content:   content,
		CreatedAt: time.Now(),
		Tokens:    tokens,
	}

	_, err := s.db.Exec(`
		INSERT INTO session_messages (id, session_id, role, content, created_at, tokens)
		VALUES (?, ?, ?, ?, ?, ?)
	`, msg.ID, msg.SessionID, msg.Role, msg.Content, msg.CreatedAt, msg.Tokens)
	if err != nil {
		return nil, err
	}

	// Update session stats
	s.db.Exec(`UPDATE sessions SET message_count = message_count + 1, total_tokens = total_tokens + ?, updated_at = ? WHERE id = ?`,
		tokens, time.Now(), sessionID)

	return msg, nil
}

// AddExecutionMessage adds an execution message (command + output) to the session
func (s *Store) AddExecutionMessage(sessionID, userPrompt, command, output string, exitCode int, riskLevel types.RiskLevel, durationMs int64, executed bool, tokens int) error {
	msg := &types.SessionMessage{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Role:      "execution",
		Content:   userPrompt,
		CreatedAt: time.Now(),
		Tokens:    tokens,
		Command:   command,
		Output:    output,
		ExitCode:  exitCode,
		RiskLevel: riskLevel,
		Duration:  durationMs,
		Executed:  executed,
	}

	executedInt := 0
	if executed {
		executedInt = 1
	}

	_, err := s.db.Exec(`
		INSERT INTO session_messages (id, session_id, role, content, created_at, tokens, command, output, exit_code, risk_level, duration_ms, executed)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, msg.ID, msg.SessionID, msg.Role, msg.Content, msg.CreatedAt, msg.Tokens, msg.Command, msg.Output, msg.ExitCode, int(msg.RiskLevel), msg.Duration, executedInt)
	if err != nil {
		return err
	}

	// Update session stats
	if executed {
		s.db.Exec(`UPDATE sessions SET message_count = message_count + 1, command_count = command_count + 1, total_tokens = total_tokens + ?, updated_at = ? WHERE id = ?`,
			tokens, time.Now(), sessionID)
	} else {
		s.db.Exec(`UPDATE sessions SET message_count = message_count + 1, total_tokens = total_tokens + ?, updated_at = ? WHERE id = ?`,
			tokens, time.Now(), sessionID)
	}

	return nil
}

// GetMessages retrieves all messages for a session
func (s *Store) GetMessages(sessionID string) ([]*types.SessionMessage, error) {
	rows, err := s.db.Query(`
		SELECT id, session_id, role, content, created_at, tokens, command, output, exit_code, risk_level, duration_ms, executed
		FROM session_messages WHERE session_id = ? ORDER BY created_at ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*types.SessionMessage
	for rows.Next() {
		msg := &types.SessionMessage{}
		var command, output sql.NullString
		var riskLevel int
		var executed int
		if err := rows.Scan(&msg.ID, &msg.SessionID, &msg.Role, &msg.Content, &msg.CreatedAt, &msg.Tokens,
			&command, &output, &msg.ExitCode, &riskLevel, &msg.Duration, &executed); err != nil {
			return nil, err
		}
		if command.Valid {
			msg.Command = command.String
		}
		if output.Valid {
			msg.Output = output.String
		}
		msg.RiskLevel = types.RiskLevel(riskLevel)
		msg.Executed = executed == 1
		messages = append(messages, msg)
	}
	return messages, nil
}

// UpdateName updates the session name
func (s *Store) UpdateName(sessionID, name string) error {
	_, err := s.db.Exec(`UPDATE sessions SET name = ?, updated_at = ? WHERE id = ?`, name, time.Now(), sessionID)
	return err
}

// UpdateLastCwd updates the last working directory
func (s *Store) UpdateLastCwd(sessionID, cwd string) error {
	_, err := s.db.Exec(`UPDATE sessions SET last_cwd = ?, updated_at = ? WHERE id = ?`, cwd, time.Now(), sessionID)
	return err
}

// UpdateAutoExecute updates the auto-execute setting for a session
func (s *Store) UpdateAutoExecute(sessionID string, autoExecute bool) error {
	_, err := s.db.Exec(`UPDATE sessions SET auto_execute = ?, updated_at = ? WHERE id = ?`, autoExecute, time.Now(), sessionID)
	return err
}

// DeleteSession deletes a session and all its messages
func (s *Store) DeleteSession(id string) error {
	// First try by ID
	result, err := s.db.Exec(`DELETE FROM session_messages WHERE session_id = ?`, id)
	if err != nil {
		return err
	}

	result, err = s.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		// Try by name
		sess, err := s.GetSessionByName(id)
		if err != nil {
			return fmt.Errorf("session not found: %s", id)
		}
		s.db.Exec(`DELETE FROM session_messages WHERE session_id = ?`, sess.ID)
		s.db.Exec(`DELETE FROM sessions WHERE id = ?`, sess.ID)
	}

	return nil
}

// SearchSessions searches sessions by name
func (s *Store) SearchSessions(query string) ([]*types.Session, error) {
	rows, err := s.db.Query(`
		SELECT id, name, created_at, updated_at, provider, model, total_tokens, message_count, command_count, last_cwd, auto_execute
		FROM sessions WHERE name LIKE ? ORDER BY updated_at DESC LIMIT 50
	`, "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*types.Session
	for rows.Next() {
		sess := &types.Session{}
		var lastCwd sql.NullString
		if err := rows.Scan(&sess.ID, &sess.Name, &sess.CreatedAt, &sess.UpdatedAt, &sess.Provider, &sess.Model,
			&sess.TotalTokens, &sess.MessageCount, &sess.CommandCount, &lastCwd, &sess.AutoExecute); err != nil {
			return nil, err
		}
		if lastCwd.Valid {
			sess.LastCwd = lastCwd.String
		}
		sessions = append(sessions, sess)
	}
	return sessions, nil
}

// GetStats returns usage statistics
func (s *Store) GetStats() (totalSessions, totalMessages, totalCommands, totalTokens int, err error) {
	row := s.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(message_count), 0), COALESCE(SUM(command_count), 0), COALESCE(SUM(total_tokens), 0) FROM sessions`)
	err = row.Scan(&totalSessions, &totalMessages, &totalCommands, &totalTokens)
	return
}

// ExportSession exports a session to JSON
func (s *Store) ExportSession(sessionID string) ([]byte, error) {
	sess, err := s.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	msgs, err := s.GetMessages(sessionID)
	if err != nil {
		return nil, err
	}

	export := types.SessionExport{
		Version:    1,
		ExportedAt: time.Now(),
		Session:    *sess,
		Messages:   make([]types.SessionMessage, len(msgs)),
	}
	for i, m := range msgs {
		export.Messages[i] = *m
	}

	return json.MarshalIndent(export, "", "  ")
}

// ImportSession imports a session from JSON
func (s *Store) ImportSession(data []byte) (*types.Session, error) {
	var export types.SessionExport
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, fmt.Errorf("invalid session format: %w", err)
	}

	// Create new session with new ID
	sess := &types.Session{
		ID:           uuid.New().String(),
		Name:         export.Session.Name + " (imported)",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Provider:     export.Session.Provider,
		Model:        export.Session.Model,
		TotalTokens:  export.Session.TotalTokens,
		MessageCount: export.Session.MessageCount,
		CommandCount: export.Session.CommandCount,
		LastCwd:      export.Session.LastCwd,
	}

	_, err := s.db.Exec(`
		INSERT INTO sessions (id, name, created_at, updated_at, provider, model, total_tokens, message_count, command_count, last_cwd)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, sess.ID, sess.Name, sess.CreatedAt, sess.UpdatedAt, sess.Provider, sess.Model, sess.TotalTokens, sess.MessageCount, sess.CommandCount, sess.LastCwd)
	if err != nil {
		return nil, err
	}

	// Import messages
	for _, msg := range export.Messages {
		executedInt := 0
		if msg.Executed {
			executedInt = 1
		}
		_, err := s.db.Exec(`
			INSERT INTO session_messages (id, session_id, role, content, created_at, tokens, command, output, exit_code, risk_level, duration_ms, executed)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, uuid.New().String(), sess.ID, msg.Role, msg.Content, msg.CreatedAt, msg.Tokens, msg.Command, msg.Output, msg.ExitCode, int(msg.RiskLevel), msg.Duration, executedInt)
		if err != nil {
			return nil, err
		}
	}

	return sess, nil
}

// Cleanup removes old sessions
func (s *Store) Cleanup(retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	// Get IDs of sessions to delete
	rows, err := s.db.Query(`SELECT id FROM sessions WHERE updated_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}

	// Delete messages first
	for _, id := range ids {
		s.db.Exec(`DELETE FROM session_messages WHERE session_id = ?`, id)
	}

	// Delete sessions
	result, err := s.db.Exec(`DELETE FROM sessions WHERE updated_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}
