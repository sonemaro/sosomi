// Package conversation provides conversation storage for the LLM client mode
package conversation

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"

	"github.com/soroush/sosomi/internal/types"
)

// Store manages conversation storage
type Store struct {
	db *sql.DB
}

// NewStore creates a new conversation store
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
	CREATE TABLE IF NOT EXISTS conversations (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		system_prompt TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		provider TEXT,
		model TEXT,
		total_tokens INTEGER DEFAULT 0,
		message_count INTEGER DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		tokens INTEGER DEFAULT 0
	);

	CREATE INDEX IF NOT EXISTS idx_conversations_updated ON conversations(updated_at);
	CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_messages_created ON messages(created_at);
	`

	_, err := s.db.Exec(schema)
	return err
}

// CreateConversation creates a new conversation
func (s *Store) CreateConversation(name, systemPrompt, provider, model string) (*types.Conversation, error) {
	conv := &types.Conversation{
		ID:           uuid.New().String(),
		Name:         name,
		SystemPrompt: systemPrompt,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Provider:     provider,
		Model:        model,
		TotalTokens:  0,
		MessageCount: 0,
	}

	_, err := s.db.Exec(`
		INSERT INTO conversations (id, name, system_prompt, created_at, updated_at, provider, model, total_tokens, message_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		conv.ID,
		conv.Name,
		conv.SystemPrompt,
		conv.CreatedAt,
		conv.UpdatedAt,
		conv.Provider,
		conv.Model,
		conv.TotalTokens,
		conv.MessageCount,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	// If there's a system prompt, add it as the first message
	if systemPrompt != "" {
		_, err = s.AddMessage(conv.ID, "system", systemPrompt, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to add system prompt message: %w", err)
		}
	}

	return conv, nil
}

// GetConversation retrieves a conversation by ID
func (s *Store) GetConversation(id string) (*types.Conversation, error) {
	row := s.db.QueryRow(`
		SELECT id, name, system_prompt, created_at, updated_at, provider, model, total_tokens, message_count
		FROM conversations WHERE id = ?
	`, id)

	conv := &types.Conversation{}
	var systemPrompt sql.NullString
	err := row.Scan(
		&conv.ID,
		&conv.Name,
		&systemPrompt,
		&conv.CreatedAt,
		&conv.UpdatedAt,
		&conv.Provider,
		&conv.Model,
		&conv.TotalTokens,
		&conv.MessageCount,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("conversation not found: %s", id)
		}
		return nil, err
	}

	if systemPrompt.Valid {
		conv.SystemPrompt = systemPrompt.String
	}

	return conv, nil
}

// GetConversationByName retrieves a conversation by name (partial match)
func (s *Store) GetConversationByName(name string) (*types.Conversation, error) {
	row := s.db.QueryRow(`
		SELECT id, name, system_prompt, created_at, updated_at, provider, model, total_tokens, message_count
		FROM conversations WHERE name LIKE ? ORDER BY updated_at DESC LIMIT 1
	`, "%"+name+"%")

	conv := &types.Conversation{}
	var systemPrompt sql.NullString
	err := row.Scan(
		&conv.ID,
		&conv.Name,
		&systemPrompt,
		&conv.CreatedAt,
		&conv.UpdatedAt,
		&conv.Provider,
		&conv.Model,
		&conv.TotalTokens,
		&conv.MessageCount,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("conversation not found: %s", name)
		}
		return nil, err
	}

	if systemPrompt.Valid {
		conv.SystemPrompt = systemPrompt.String
	}

	return conv, nil
}

// ListConversations lists all conversations ordered by updated_at
func (s *Store) ListConversations(limit, offset int) ([]*types.Conversation, error) {
	rows, err := s.db.Query(`
		SELECT id, name, system_prompt, created_at, updated_at, provider, model, total_tokens, message_count
		FROM conversations
		ORDER BY updated_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conversations []*types.Conversation
	for rows.Next() {
		conv := &types.Conversation{}
		var systemPrompt sql.NullString
		if err := rows.Scan(
			&conv.ID,
			&conv.Name,
			&systemPrompt,
			&conv.CreatedAt,
			&conv.UpdatedAt,
			&conv.Provider,
			&conv.Model,
			&conv.TotalTokens,
			&conv.MessageCount,
		); err != nil {
			return nil, err
		}
		if systemPrompt.Valid {
			conv.SystemPrompt = systemPrompt.String
		}
		conversations = append(conversations, conv)
	}

	return conversations, nil
}

// AddMessage adds a message to a conversation
func (s *Store) AddMessage(conversationID, role, content string, tokens int) (*types.ConversationMessage, error) {
	msg := &types.ConversationMessage{
		ID:             uuid.New().String(),
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		CreatedAt:      time.Now(),
		Tokens:         tokens,
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO messages (id, conversation_id, role, content, created_at, tokens)
		VALUES (?, ?, ?, ?, ?, ?)
	`,
		msg.ID,
		msg.ConversationID,
		msg.Role,
		msg.Content,
		msg.CreatedAt,
		msg.Tokens,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert message: %w", err)
	}

	// Update conversation stats
	_, err = tx.Exec(`
		UPDATE conversations 
		SET updated_at = ?, 
		    total_tokens = total_tokens + ?,
		    message_count = message_count + 1
		WHERE id = ?
	`, time.Now(), tokens, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to update conversation: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return msg, nil
}

// GetMessages retrieves all messages for a conversation
func (s *Store) GetMessages(conversationID string) ([]*types.ConversationMessage, error) {
	rows, err := s.db.Query(`
		SELECT id, conversation_id, role, content, created_at, tokens
		FROM messages
		WHERE conversation_id = ?
		ORDER BY created_at ASC
	`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*types.ConversationMessage
	for rows.Next() {
		msg := &types.ConversationMessage{}
		if err := rows.Scan(
			&msg.ID,
			&msg.ConversationID,
			&msg.Role,
			&msg.Content,
			&msg.CreatedAt,
			&msg.Tokens,
		); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// UpdateName updates the conversation name
func (s *Store) UpdateName(id, name string) error {
	_, err := s.db.Exec(`
		UPDATE conversations SET name = ?, updated_at = ? WHERE id = ?
	`, name, time.Now(), id)
	return err
}

// UpdateSystemPrompt updates the conversation system prompt
func (s *Store) UpdateSystemPrompt(id, systemPrompt string) error {
	_, err := s.db.Exec(`
		UPDATE conversations SET system_prompt = ?, updated_at = ? WHERE id = ?
	`, systemPrompt, time.Now(), id)
	return err
}

// DeleteConversation deletes a conversation and all its messages
func (s *Store) DeleteConversation(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete messages first
	_, err = tx.Exec("DELETE FROM messages WHERE conversation_id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete messages: %w", err)
	}

	// Delete conversation
	result, err := tx.Exec("DELETE FROM conversations WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("conversation not found: %s", id)
	}

	return tx.Commit()
}

// ExportConversation exports a conversation with all messages as JSON
func (s *Store) ExportConversation(id string) (*types.ConversationExport, error) {
	conv, err := s.GetConversation(id)
	if err != nil {
		return nil, err
	}

	messages, err := s.GetMessages(id)
	if err != nil {
		return nil, err
	}

	export := &types.ConversationExport{
		Version:      1,
		ExportedAt:   time.Now(),
		Conversation: *conv,
		Messages:     make([]types.ConversationMessage, len(messages)),
	}

	for i, msg := range messages {
		export.Messages[i] = *msg
	}

	return export, nil
}

// ImportConversation imports a conversation from JSON export
func (s *Store) ImportConversation(data []byte) (*types.Conversation, error) {
	var export types.ConversationExport
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, fmt.Errorf("failed to parse import data: %w", err)
	}

	// Create new IDs to avoid conflicts
	newConvID := uuid.New().String()

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Insert conversation
	now := time.Now()
	_, err = tx.Exec(`
		INSERT INTO conversations (id, name, system_prompt, created_at, updated_at, provider, model, total_tokens, message_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		newConvID,
		export.Conversation.Name+" (imported)",
		export.Conversation.SystemPrompt,
		now,
		now,
		export.Conversation.Provider,
		export.Conversation.Model,
		export.Conversation.TotalTokens,
		len(export.Messages),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to import conversation: %w", err)
	}

	// Insert messages
	for _, msg := range export.Messages {
		newMsgID := uuid.New().String()
		_, err = tx.Exec(`
			INSERT INTO messages (id, conversation_id, role, content, created_at, tokens)
			VALUES (?, ?, ?, ?, ?, ?)
		`,
			newMsgID,
			newConvID,
			msg.Role,
			msg.Content,
			msg.CreatedAt,
			msg.Tokens,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to import message: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.GetConversation(newConvID)
}

// SearchConversations searches conversations by name or message content
func (s *Store) SearchConversations(query string, limit int) ([]*types.Conversation, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT c.id, c.name, c.system_prompt, c.created_at, c.updated_at, c.provider, c.model, c.total_tokens, c.message_count
		FROM conversations c
		LEFT JOIN messages m ON c.id = m.conversation_id
		WHERE c.name LIKE ? OR m.content LIKE ?
		ORDER BY c.updated_at DESC
		LIMIT ?
	`, "%"+query+"%", "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conversations []*types.Conversation
	for rows.Next() {
		conv := &types.Conversation{}
		var systemPrompt sql.NullString
		if err := rows.Scan(
			&conv.ID,
			&conv.Name,
			&systemPrompt,
			&conv.CreatedAt,
			&conv.UpdatedAt,
			&conv.Provider,
			&conv.Model,
			&conv.TotalTokens,
			&conv.MessageCount,
		); err != nil {
			return nil, err
		}
		if systemPrompt.Valid {
			conv.SystemPrompt = systemPrompt.String
		}
		conversations = append(conversations, conv)
	}

	return conversations, nil
}

// GetStats returns statistics about conversations
func (s *Store) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total conversations
	var total int
	s.db.QueryRow("SELECT COUNT(*) FROM conversations").Scan(&total)
	stats["total_conversations"] = total

	// Total messages
	var messages int
	s.db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&messages)
	stats["total_messages"] = messages

	// Total tokens
	var tokens int
	s.db.QueryRow("SELECT COALESCE(SUM(total_tokens), 0) FROM conversations").Scan(&tokens)
	stats["total_tokens"] = tokens

	// Messages by role
	roleStats := make(map[string]int)
	rows, _ := s.db.Query("SELECT role, COUNT(*) FROM messages GROUP BY role")
	for rows.Next() {
		var role string
		var count int
		rows.Scan(&role, &count)
		roleStats[role] = count
	}
	rows.Close()
	stats["by_role"] = roleStats

	// Conversations by provider
	providerStats := make(map[string]int)
	rows, _ = s.db.Query("SELECT provider, COUNT(*) FROM conversations WHERE provider != '' GROUP BY provider")
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

// Cleanup removes old conversations
func (s *Store) Cleanup(retentionDays int) error {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get IDs of old conversations
	rows, err := tx.Query("SELECT id FROM conversations WHERE updated_at < ?", cutoff)
	if err != nil {
		return err
	}

	var ids []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()

	// Delete messages for old conversations
	for _, id := range ids {
		_, err = tx.Exec("DELETE FROM messages WHERE conversation_id = ?", id)
		if err != nil {
			return err
		}
	}

	// Delete old conversations
	_, err = tx.Exec("DELETE FROM conversations WHERE updated_at < ?", cutoff)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}
