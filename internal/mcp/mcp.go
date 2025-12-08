// Package mcp provides Model Context Protocol support
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/sonemaro/sosomi/internal/types"
)

// Server represents an MCP server connection
type Server struct {
	name    string
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Reader
	stderr  io.ReadCloser
	tools   []types.MCPTool
	mu      sync.Mutex
	running bool
}

// Message represents an MCP JSON-RPC message
type Message struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// InitializeParams are sent when initializing an MCP server
type InitializeParams struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ClientInfo      ClientInfo   `json:"clientInfo"`
}

// Capabilities describes client capabilities
type Capabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

// ToolsCapability describes tool-related capabilities
type ToolsCapability struct{}

// ClientInfo provides information about the client
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult is the response from initialization
type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
}

// ServerInfo provides information about the server
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ToolsListResult contains the list of available tools
type ToolsListResult struct {
	Tools []types.MCPTool `json:"tools"`
}

// ToolCallParams are the parameters for calling a tool
type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolCallResult is the result of a tool call
type ToolCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents content in a tool result
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Manager manages multiple MCP servers
type Manager struct {
	servers map[string]*Server
	mu      sync.RWMutex
}

// NewManager creates a new MCP manager
func NewManager() *Manager {
	return &Manager{
		servers: make(map[string]*Server),
	}
}

// StartServer starts an MCP server
func (m *Manager) StartServer(ctx context.Context, name string, command string, args ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.servers[name]; exists {
		return fmt.Errorf("server %s already running", name)
	}

	cmd := exec.CommandContext(ctx, command, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	server := &Server{
		name:    name,
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReader(stdout),
		stderr:  stderr,
		running: true,
	}

	// Initialize the server
	if err := server.initialize(); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	// Get available tools
	if err := server.listTools(); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("failed to list tools: %w", err)
	}

	m.servers[name] = server
	return nil
}

// StopServer stops an MCP server
func (m *Manager) StopServer(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	server, exists := m.servers[name]
	if !exists {
		return fmt.Errorf("server %s not found", name)
	}

	server.stdin.Close()
	server.cmd.Process.Kill()
	server.running = false
	delete(m.servers, name)

	return nil
}

// GetTools returns all tools from all servers
func (m *Manager) GetTools() []types.MCPTool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tools []types.MCPTool
	for _, server := range m.servers {
		tools = append(tools, server.tools...)
	}
	return tools
}

// CallTool calls a tool on the appropriate server
func (m *Manager) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*types.MCPToolResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, server := range m.servers {
		for _, tool := range server.tools {
			if tool.Name == name {
				return server.callTool(name, arguments)
			}
		}
	}

	return nil, fmt.Errorf("tool %s not found", name)
}

// Shutdown stops all servers
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, server := range m.servers {
		server.stdin.Close()
		server.cmd.Process.Kill()
		delete(m.servers, name)
	}
}

var messageID = 0

func nextID() int {
	messageID++
	return messageID
}

func (s *Server) send(msg *Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(s.stdin, "%s\n", data)
	return err
}

func (s *Server) receive() (*Message, error) {
	line, err := s.stdout.ReadString('\n')
	if err != nil {
		return nil, err
	}

	var msg Message
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

func (s *Server) initialize() error {
	msg := &Message{
		JSONRPC: "2.0",
		ID:      nextID(),
		Method:  "initialize",
		Params: InitializeParams{
			ProtocolVersion: "2024-11-05",
			Capabilities: Capabilities{
				Tools: &ToolsCapability{},
			},
			ClientInfo: ClientInfo{
				Name:    "sosomi",
				Version: "1.0.0",
			},
		},
	}

	if err := s.send(msg); err != nil {
		return err
	}

	resp, err := s.receive()
	if err != nil {
		return err
	}

	if resp.Error != nil {
		return fmt.Errorf("initialization failed: %s", resp.Error.Message)
	}

	// Send initialized notification
	notif := &Message{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	return s.send(notif)
}

func (s *Server) listTools() error {
	msg := &Message{
		JSONRPC: "2.0",
		ID:      nextID(),
		Method:  "tools/list",
	}

	if err := s.send(msg); err != nil {
		return err
	}

	resp, err := s.receive()
	if err != nil {
		return err
	}

	if resp.Error != nil {
		return fmt.Errorf("failed to list tools: %s", resp.Error.Message)
	}

	// Parse tools from result
	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return err
	}

	var result ToolsListResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return err
	}

	s.tools = result.Tools
	return nil
}

func (s *Server) callTool(name string, arguments map[string]interface{}) (*types.MCPToolResult, error) {
	msg := &Message{
		JSONRPC: "2.0",
		ID:      nextID(),
		Method:  "tools/call",
		Params: ToolCallParams{
			Name:      name,
			Arguments: arguments,
		},
	}

	if err := s.send(msg); err != nil {
		return nil, err
	}

	resp, err := s.receive()
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return &types.MCPToolResult{
			Content: resp.Error.Message,
			IsError: true,
		}, nil
	}

	// Parse result
	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, err
	}

	var result ToolCallResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, err
	}

	// Combine content blocks
	var content string
	for _, block := range result.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	return &types.MCPToolResult{
		Content: content,
		IsError: result.IsError,
	}, nil
}

// BuiltinTools returns sosomi's built-in MCP tools
func BuiltinTools() []types.MCPTool {
	return []types.MCPTool{
		{
			Name:        "execute_command",
			Description: "Execute a shell command and return the output",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The shell command to execute",
					},
					"workdir": map[string]interface{}{
						"type":        "string",
						"description": "Working directory for the command",
					},
				},
				"required": []string{"command"},
			},
		},
		{
			Name:        "read_file",
			Description: "Read the contents of a file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file to read",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "write_file",
			Description: "Write content to a file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file to write",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Content to write to the file",
					},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			Name:        "list_directory",
			Description: "List contents of a directory",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the directory to list",
					},
				},
				"required": []string{"path"},
			},
		},
	}
}

// ExecuteBuiltinTool executes a built-in tool
func ExecuteBuiltinTool(name string, arguments map[string]interface{}) (*types.MCPToolResult, error) {
	switch name {
	case "execute_command":
		cmd, ok := arguments["command"].(string)
		if !ok {
			return &types.MCPToolResult{Content: "command argument required", IsError: true}, nil
		}

		workdir, _ := arguments["workdir"].(string)
		if workdir == "" {
			workdir, _ = os.Getwd()
		}

		result := exec.Command("sh", "-c", cmd)
		result.Dir = workdir
		output, err := result.CombinedOutput()
		if err != nil {
			return &types.MCPToolResult{
				Content: string(output) + "\nError: " + err.Error(),
				IsError: true,
			}, nil
		}
		return &types.MCPToolResult{Content: string(output)}, nil

	case "read_file":
		path, ok := arguments["path"].(string)
		if !ok {
			return &types.MCPToolResult{Content: "path argument required", IsError: true}, nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return &types.MCPToolResult{Content: err.Error(), IsError: true}, nil
		}
		return &types.MCPToolResult{Content: string(content)}, nil

	case "write_file":
		path, ok := arguments["path"].(string)
		if !ok {
			return &types.MCPToolResult{Content: "path argument required", IsError: true}, nil
		}
		content, ok := arguments["content"].(string)
		if !ok {
			return &types.MCPToolResult{Content: "content argument required", IsError: true}, nil
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return &types.MCPToolResult{Content: err.Error(), IsError: true}, nil
		}
		return &types.MCPToolResult{Content: "File written successfully"}, nil

	case "list_directory":
		path, ok := arguments["path"].(string)
		if !ok {
			path = "."
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return &types.MCPToolResult{Content: err.Error(), IsError: true}, nil
		}
		var result string
		for _, entry := range entries {
			if entry.IsDir() {
				result += entry.Name() + "/\n"
			} else {
				result += entry.Name() + "\n"
			}
		}
		return &types.MCPToolResult{Content: result}, nil

	default:
		return &types.MCPToolResult{
			Content: fmt.Sprintf("unknown tool: %s", name),
			IsError: true,
		}, nil
	}
}
