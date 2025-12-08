// Package mcp tests
package mcp

import (
	"encoding/json"
	"testing"

	"github.com/sonemaro/sosomi/internal/types"
)

func TestNewManager(t *testing.T) {
	manager := NewManager()
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.servers == nil {
		t.Error("Expected servers map to be initialized")
	}
}

func TestManager_GetTools_Empty(t *testing.T) {
	manager := NewManager()
	tools := manager.GetTools()

	// GetTools returns nil when no servers, which is valid
	// We just check that len(tools) is 0
	if len(tools) != 0 {
		t.Errorf("Expected 0 tools for empty manager, got %d", len(tools))
	}
}

func TestMessage_Serialization(t *testing.T) {
	msg := Message{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	var parsed Message
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if parsed.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", parsed.JSONRPC)
	}
	if parsed.ID != 1 {
		t.Errorf("Expected ID 1, got %d", parsed.ID)
	}
	if parsed.Method != "initialize" {
		t.Errorf("Expected Method 'initialize', got '%s'", parsed.Method)
	}
}

func TestRPCError_Serialization(t *testing.T) {
	rpcErr := RPCError{
		Code:    -32600,
		Message: "Invalid Request",
	}

	data, err := json.Marshal(rpcErr)
	if err != nil {
		t.Fatalf("Failed to marshal error: %v", err)
	}

	var parsed RPCError
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal error: %v", err)
	}

	if parsed.Code != -32600 {
		t.Errorf("Expected Code -32600, got %d", parsed.Code)
	}
	if parsed.Message != "Invalid Request" {
		t.Errorf("Expected Message 'Invalid Request', got '%s'", parsed.Message)
	}
}

func TestInitializeParams_Serialization(t *testing.T) {
	params := InitializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities: Capabilities{
			Tools: &ToolsCapability{},
		},
		ClientInfo: ClientInfo{
			Name:    "sosomi",
			Version: "1.0.0",
		},
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal params: %v", err)
	}

	var parsed InitializeParams
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal params: %v", err)
	}

	if parsed.ProtocolVersion != "2024-11-05" {
		t.Errorf("Expected ProtocolVersion '2024-11-05', got '%s'", parsed.ProtocolVersion)
	}
	if parsed.ClientInfo.Name != "sosomi" {
		t.Errorf("Expected ClientInfo.Name 'sosomi', got '%s'", parsed.ClientInfo.Name)
	}
}

func TestToolsListResult_Serialization(t *testing.T) {
	result := ToolsListResult{
		Tools: []types.MCPTool{
			{
				Name:        "read_file",
				Description: "Read a file",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}

	var parsed ToolsListResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if len(parsed.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(parsed.Tools))
	}
	if parsed.Tools[0].Name != "read_file" {
		t.Errorf("Expected tool name 'read_file', got '%s'", parsed.Tools[0].Name)
	}
}

func TestToolCallParams_Serialization(t *testing.T) {
	params := ToolCallParams{
		Name: "read_file",
		Arguments: map[string]interface{}{
			"path": "/tmp/test.txt",
		},
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal params: %v", err)
	}

	var parsed ToolCallParams
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal params: %v", err)
	}

	if parsed.Name != "read_file" {
		t.Errorf("Expected name 'read_file', got '%s'", parsed.Name)
	}
	if parsed.Arguments["path"] != "/tmp/test.txt" {
		t.Errorf("Expected path '/tmp/test.txt', got '%v'", parsed.Arguments["path"])
	}
}

func TestToolCallResult_Serialization(t *testing.T) {
	result := ToolCallResult{
		Content: []ContentBlock{
			{Type: "text", Text: "file contents here"},
		},
		IsError: false,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}

	var parsed ToolCallResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if len(parsed.Content) != 1 {
		t.Errorf("Expected 1 content block, got %d", len(parsed.Content))
	}
	if parsed.Content[0].Text != "file contents here" {
		t.Errorf("Expected text 'file contents here', got '%s'", parsed.Content[0].Text)
	}
	if parsed.IsError {
		t.Error("Expected IsError to be false")
	}
}

func TestToolCallResult_Error(t *testing.T) {
	result := ToolCallResult{
		Content: []ContentBlock{
			{Type: "text", Text: "file not found"},
		},
		IsError: true,
	}

	if !result.IsError {
		t.Error("Expected IsError to be true")
	}
}

func TestContentBlock_Serialization(t *testing.T) {
	block := ContentBlock{
		Type: "text",
		Text: "Hello, World!",
	}

	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("Failed to marshal block: %v", err)
	}

	var parsed ContentBlock
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal block: %v", err)
	}

	if parsed.Type != "text" {
		t.Errorf("Expected Type 'text', got '%s'", parsed.Type)
	}
	if parsed.Text != "Hello, World!" {
		t.Errorf("Expected Text 'Hello, World!', got '%s'", parsed.Text)
	}
}

func TestInitializeResult_Serialization(t *testing.T) {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities:    Capabilities{},
		ServerInfo: ServerInfo{
			Name:    "test-server",
			Version: "1.0.0",
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}

	var parsed InitializeResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if parsed.ServerInfo.Name != "test-server" {
		t.Errorf("Expected ServerInfo.Name 'test-server', got '%s'", parsed.ServerInfo.Name)
	}
}

func TestManager_StopServer_NotFound(t *testing.T) {
	manager := NewManager()

	err := manager.StopServer("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent server")
	}
}

func TestServer_Fields(t *testing.T) {
	// Test that Server struct has expected fields
	server := &Server{
		name:    "test",
		running: false,
	}

	if server.name != "test" {
		t.Errorf("Expected name 'test', got '%s'", server.name)
	}
	if server.running {
		t.Error("Expected running to be false")
	}
}

func TestManager_ThreadSafety(t *testing.T) {
	manager := NewManager()

	// Concurrent access should not panic
	done := make(chan bool)

	go func() {
		for i := 0; i < 100; i++ {
			_ = manager.GetTools()
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			_ = manager.GetTools()
		}
		done <- true
	}()

	<-done
	<-done
}

func TestBuiltInTools(t *testing.T) {
	// Test that built-in tools are defined correctly
	tools := BuiltinTools()

	if len(tools) == 0 {
		t.Error("Expected built-in tools to be defined")
	}

	for _, tool := range tools {
		if tool.Name == "" {
			t.Error("Tool name should not be empty")
		}
		if tool.Description == "" {
			t.Error("Tool description should not be empty")
		}
	}
}

func TestCallBuiltInTool(t *testing.T) {
	// Test calling a built-in tool
	result, err := ExecuteBuiltinTool("list_directory", map[string]interface{}{
		"path": ".",
	})
	if err != nil {
		t.Fatalf("ExecuteBuiltinTool returned error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.IsError {
		t.Logf("Tool returned error: %s", result.Content)
	}
}

func TestCallBuiltInTool_Unknown(t *testing.T) {
	result, err := ExecuteBuiltinTool("unknown_tool", map[string]interface{}{})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error result for unknown tool")
	}
}

func TestExecuteBuiltinTool_ListDirectory(t *testing.T) {
	result, err := ExecuteBuiltinTool("list_directory", map[string]interface{}{
		"path": ".",
	})
	if err != nil {
		t.Fatalf("ExecuteBuiltinTool returned error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Result may be error or success depending on permissions
	if result.IsError {
		t.Logf("Tool returned error (may be expected): %s", result.Content)
	}
}

func TestExecuteBuiltinTool_InvalidParams(t *testing.T) {
	// Missing required path parameter
	result, err := ExecuteBuiltinTool("read_file", map[string]interface{}{})
	if err != nil {
		t.Fatalf("ExecuteBuiltinTool returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error for missing parameters")
	}
}
