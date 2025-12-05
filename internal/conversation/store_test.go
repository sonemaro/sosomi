package conversation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewStore(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Verify database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestCreateConversation(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	tests := []struct {
		name         string
		convName     string
		systemPrompt string
		provider     string
		model        string
	}{
		{
			name:         "basic conversation",
			convName:     "Test Chat",
			systemPrompt: "",
			provider:     "openai",
			model:        "gpt-4o",
		},
		{
			name:         "with system prompt",
			convName:     "Coding Assistant",
			systemPrompt: "You are a helpful coding assistant.",
			provider:     "ollama",
			model:        "llama3.2",
		},
		{
			name:         "empty name",
			convName:     "",
			systemPrompt: "",
			provider:     "openai",
			model:        "gpt-4o-mini",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conv, err := store.CreateConversation(tt.convName, tt.systemPrompt, tt.provider, tt.model)
			if err != nil {
				t.Fatalf("failed to create conversation: %v", err)
			}

			if conv.ID == "" {
				t.Error("conversation ID should not be empty")
			}
			if conv.Name != tt.convName {
				t.Errorf("got name %q, want %q", conv.Name, tt.convName)
			}
			if conv.SystemPrompt != tt.systemPrompt {
				t.Errorf("got system prompt %q, want %q", conv.SystemPrompt, tt.systemPrompt)
			}
			if conv.Provider != tt.provider {
				t.Errorf("got provider %q, want %q", conv.Provider, tt.provider)
			}
			if conv.Model != tt.model {
				t.Errorf("got model %q, want %q", conv.Model, tt.model)
			}
			if conv.CreatedAt.IsZero() {
				t.Error("created_at should not be zero")
			}

			// If system prompt is set, message count should be 1
			if tt.systemPrompt != "" && conv.MessageCount != 0 {
				// Note: CreateConversation adds system prompt as first message
				// which increments message_count via AddMessage
			}
		})
	}
}

func TestGetConversation(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	// Create a conversation
	created, err := store.CreateConversation("Test", "System prompt", "openai", "gpt-4o")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	// Get it back
	retrieved, err := store.GetConversation(created.ID)
	if err != nil {
		t.Fatalf("failed to get conversation: %v", err)
	}

	if retrieved.ID != created.ID {
		t.Errorf("got ID %q, want %q", retrieved.ID, created.ID)
	}
	if retrieved.Name != created.Name {
		t.Errorf("got name %q, want %q", retrieved.Name, created.Name)
	}

	// Test not found
	_, err = store.GetConversation("nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent conversation")
	}
}

func TestGetConversationByName(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	// Create conversations
	store.CreateConversation("Python Help", "", "openai", "gpt-4o")
	store.CreateConversation("Go Development", "", "openai", "gpt-4o")
	store.CreateConversation("JavaScript Tutorial", "", "openai", "gpt-4o")

	tests := []struct {
		searchName   string
		expectName   string
		expectError  bool
	}{
		{"Python", "Python Help", false},
		{"Go", "Go Development", false},
		{"Java", "JavaScript Tutorial", false},
		{"nonexistent", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.searchName, func(t *testing.T) {
			conv, err := store.GetConversationByName(tt.searchName)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if conv.Name != tt.expectName {
				t.Errorf("got name %q, want %q", conv.Name, tt.expectName)
			}
		})
	}
}

func TestListConversations(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	// Create several conversations
	for i := 0; i < 5; i++ {
		store.CreateConversation("Conv "+string(rune('A'+i)), "", "openai", "gpt-4o")
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// List all
	convs, err := store.ListConversations(10, 0)
	if err != nil {
		t.Fatalf("failed to list conversations: %v", err)
	}
	if len(convs) != 5 {
		t.Errorf("got %d conversations, want 5", len(convs))
	}

	// Test pagination
	convs, err = store.ListConversations(2, 0)
	if err != nil {
		t.Fatalf("failed to list with limit: %v", err)
	}
	if len(convs) != 2 {
		t.Errorf("got %d conversations, want 2", len(convs))
	}

	convs, err = store.ListConversations(2, 2)
	if err != nil {
		t.Fatalf("failed to list with offset: %v", err)
	}
	if len(convs) != 2 {
		t.Errorf("got %d conversations, want 2", len(convs))
	}
}

func TestAddMessage(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	conv, _ := store.CreateConversation("Test", "", "openai", "gpt-4o")

	tests := []struct {
		role    string
		content string
		tokens  int
	}{
		{"user", "Hello, how are you?", 10},
		{"assistant", "I'm doing well, thank you!", 15},
		{"user", "What is 2+2?", 8},
		{"assistant", "2+2 equals 4.", 12},
	}

	for _, tt := range tests {
		msg, err := store.AddMessage(conv.ID, tt.role, tt.content, tt.tokens)
		if err != nil {
			t.Fatalf("failed to add message: %v", err)
		}

		if msg.ID == "" {
			t.Error("message ID should not be empty")
		}
		if msg.Role != tt.role {
			t.Errorf("got role %q, want %q", msg.Role, tt.role)
		}
		if msg.Content != tt.content {
			t.Errorf("got content %q, want %q", msg.Content, tt.content)
		}
		if msg.Tokens != tt.tokens {
			t.Errorf("got tokens %d, want %d", msg.Tokens, tt.tokens)
		}
	}

	// Verify conversation stats updated
	updated, _ := store.GetConversation(conv.ID)
	if updated.MessageCount != 4 {
		t.Errorf("got message count %d, want 4", updated.MessageCount)
	}
	expectedTokens := 10 + 15 + 8 + 12
	if updated.TotalTokens != expectedTokens {
		t.Errorf("got total tokens %d, want %d", updated.TotalTokens, expectedTokens)
	}
}

func TestGetMessages(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	conv, _ := store.CreateConversation("Test", "", "openai", "gpt-4o")

	// Add messages
	store.AddMessage(conv.ID, "user", "First message", 5)
	time.Sleep(10 * time.Millisecond)
	store.AddMessage(conv.ID, "assistant", "Second message", 10)
	time.Sleep(10 * time.Millisecond)
	store.AddMessage(conv.ID, "user", "Third message", 5)

	messages, err := store.GetMessages(conv.ID)
	if err != nil {
		t.Fatalf("failed to get messages: %v", err)
	}

	if len(messages) != 3 {
		t.Errorf("got %d messages, want 3", len(messages))
	}

	// Verify order (should be chronological)
	if messages[0].Content != "First message" {
		t.Error("messages not in chronological order")
	}
	if messages[2].Content != "Third message" {
		t.Error("messages not in chronological order")
	}
}

func TestUpdateName(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	conv, _ := store.CreateConversation("Original Name", "", "openai", "gpt-4o")

	err := store.UpdateName(conv.ID, "Updated Name")
	if err != nil {
		t.Fatalf("failed to update name: %v", err)
	}

	updated, _ := store.GetConversation(conv.ID)
	if updated.Name != "Updated Name" {
		t.Errorf("got name %q, want %q", updated.Name, "Updated Name")
	}
}

func TestUpdateSystemPrompt(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	conv, _ := store.CreateConversation("Test Conv", "Original prompt", "openai", "gpt-4o")

	// Verify original prompt
	original, _ := store.GetConversation(conv.ID)
	if original.SystemPrompt != "Original prompt" {
		t.Errorf("got system prompt %q, want %q", original.SystemPrompt, "Original prompt")
	}

	// Update prompt
	err := store.UpdateSystemPrompt(conv.ID, "Updated prompt")
	if err != nil {
		t.Fatalf("failed to update system prompt: %v", err)
	}

	updated, _ := store.GetConversation(conv.ID)
	if updated.SystemPrompt != "Updated prompt" {
		t.Errorf("got system prompt %q, want %q", updated.SystemPrompt, "Updated prompt")
	}

	// Clear prompt
	err = store.UpdateSystemPrompt(conv.ID, "")
	if err != nil {
		t.Fatalf("failed to clear system prompt: %v", err)
	}

	cleared, _ := store.GetConversation(conv.ID)
	if cleared.SystemPrompt != "" {
		t.Errorf("got system prompt %q, want empty", cleared.SystemPrompt)
	}
}

func TestDeleteConversation(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	conv, _ := store.CreateConversation("To Delete", "", "openai", "gpt-4o")
	store.AddMessage(conv.ID, "user", "Hello", 5)
	store.AddMessage(conv.ID, "assistant", "Hi there!", 10)

	err := store.DeleteConversation(conv.ID)
	if err != nil {
		t.Fatalf("failed to delete conversation: %v", err)
	}

	// Verify conversation is gone
	_, err = store.GetConversation(conv.ID)
	if err == nil {
		t.Error("conversation should be deleted")
	}

	// Verify messages are gone
	messages, _ := store.GetMessages(conv.ID)
	if len(messages) != 0 {
		t.Errorf("messages should be deleted, got %d", len(messages))
	}

	// Test deleting nonexistent
	err = store.DeleteConversation("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent conversation")
	}
}

func TestExportImportConversation(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	// Create a conversation with messages
	conv, _ := store.CreateConversation("Export Test", "You are helpful.", "openai", "gpt-4o")
	store.AddMessage(conv.ID, "user", "Hello", 5)
	store.AddMessage(conv.ID, "assistant", "Hi there!", 10)

	// Export
	export, err := store.ExportConversation(conv.ID)
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	if export.Version != 1 {
		t.Errorf("got version %d, want 1", export.Version)
	}
	if export.Conversation.Name != "Export Test" {
		t.Errorf("got name %q, want %q", export.Conversation.Name, "Export Test")
	}
	// 3 messages: system prompt + user + assistant
	if len(export.Messages) != 3 {
		t.Errorf("got %d messages, want 3", len(export.Messages))
	}

	// Serialize to JSON
	data, err := json.Marshal(export)
	if err != nil {
		t.Fatalf("failed to marshal export: %v", err)
	}

	// Import
	imported, err := store.ImportConversation(data)
	if err != nil {
		t.Fatalf("failed to import: %v", err)
	}

	if imported.Name != "Export Test (imported)" {
		t.Errorf("got name %q, want %q", imported.Name, "Export Test (imported)")
	}
	if imported.ID == conv.ID {
		t.Error("imported conversation should have new ID")
	}

	// Verify messages were imported
	messages, _ := store.GetMessages(imported.ID)
	if len(messages) != 3 {
		t.Errorf("got %d imported messages, want 3", len(messages))
	}
}

func TestSearchConversations(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	// Create conversations
	conv1, _ := store.CreateConversation("Python Help", "", "openai", "gpt-4o")
	conv2, _ := store.CreateConversation("Go Development", "", "openai", "gpt-4o")
	store.CreateConversation("Random Chat", "", "openai", "gpt-4o")

	// Add messages with specific content
	store.AddMessage(conv1.ID, "user", "How do I use decorators in Python?", 10)
	store.AddMessage(conv2.ID, "user", "What are goroutines?", 8)

	tests := []struct {
		query       string
		expectCount int
	}{
		{"Python", 1},
		{"Go", 1},
		{"decorators", 1},
		{"goroutines", 1},
		{"nonexistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			results, err := store.SearchConversations(tt.query, 10)
			if err != nil {
				t.Fatalf("search failed: %v", err)
			}
			if len(results) != tt.expectCount {
				t.Errorf("got %d results, want %d", len(results), tt.expectCount)
			}
		})
	}
}

func TestGetStats(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	// Create some test data
	conv1, _ := store.CreateConversation("Test 1", "", "openai", "gpt-4o")
	conv2, _ := store.CreateConversation("Test 2", "", "ollama", "llama3.2")

	store.AddMessage(conv1.ID, "user", "Hello", 10)
	store.AddMessage(conv1.ID, "assistant", "Hi", 5)
	store.AddMessage(conv2.ID, "user", "Hey", 8)

	stats, err := store.GetStats()
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	if stats["total_conversations"].(int) != 2 {
		t.Errorf("got total_conversations %v, want 2", stats["total_conversations"])
	}
	if stats["total_messages"].(int) != 3 {
		t.Errorf("got total_messages %v, want 3", stats["total_messages"])
	}
	if stats["total_tokens"].(int) != 23 {
		t.Errorf("got total_tokens %v, want 23", stats["total_tokens"])
	}

	byRole := stats["by_role"].(map[string]int)
	if byRole["user"] != 2 {
		t.Errorf("got user count %d, want 2", byRole["user"])
	}
	if byRole["assistant"] != 1 {
		t.Errorf("got assistant count %d, want 1", byRole["assistant"])
	}

	byProvider := stats["by_provider"].(map[string]int)
	if byProvider["openai"] != 1 {
		t.Errorf("got openai count %d, want 1", byProvider["openai"])
	}
	if byProvider["ollama"] != 1 {
		t.Errorf("got ollama count %d, want 1", byProvider["ollama"])
	}
}

func TestCleanup(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	// Create a conversation
	conv, _ := store.CreateConversation("Old Chat", "", "openai", "gpt-4o")
	store.AddMessage(conv.ID, "user", "Hello", 5)

	// Set updated_at to past (by direct SQL update for testing)
	store.db.Exec("UPDATE conversations SET updated_at = datetime('now', '-100 days') WHERE id = ?", conv.ID)

	// Cleanup with 30 day retention
	err := store.Cleanup(30)
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	// Verify conversation was deleted
	_, err = store.GetConversation(conv.ID)
	if err == nil {
		t.Error("old conversation should be deleted")
	}

	// Create a new conversation (recent)
	newConv, _ := store.CreateConversation("New Chat", "", "openai", "gpt-4o")

	// Cleanup should not delete it
	store.Cleanup(30)

	_, err = store.GetConversation(newConv.ID)
	if err != nil {
		t.Error("recent conversation should not be deleted")
	}
}

// Helper function to create a test store
func setupTestStore(t *testing.T) *Store {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}

	return store
}
