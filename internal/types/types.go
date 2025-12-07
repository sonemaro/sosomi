// Package types provides shared type definitions for sosomi
package types

import "time"

// RiskLevel represents the danger level of a command
type RiskLevel int

const (
	RiskSafe RiskLevel = iota
	RiskCaution
	RiskDangerous
	RiskCritical
)

func (r RiskLevel) String() string {
	switch r {
	case RiskSafe:
		return "SAFE"
	case RiskCaution:
		return "CAUTION"
	case RiskDangerous:
		return "DANGEROUS"
	case RiskCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

func (r RiskLevel) Color() string {
	switch r {
	case RiskSafe:
		return "green"
	case RiskCaution:
		return "yellow"
	case RiskDangerous:
		return "orange"
	case RiskCritical:
		return "red"
	default:
		return "white"
	}
}

func (r RiskLevel) Emoji() string {
	switch r {
	case RiskSafe:
		return "🟢"
	case RiskCaution:
		return "🟡"
	case RiskDangerous:
		return "🟠"
	case RiskCritical:
		return "🔴"
	default:
		return "⚪"
	}
}

// SystemContext contains information about the current system state
type SystemContext struct {
	OS               string   `json:"os"`
	Shell            string   `json:"shell"`
	CurrentDir       string   `json:"current_dir"`
	HomeDir          string   `json:"home_dir"`
	Username         string   `json:"username"`
	GitBranch        string   `json:"git_branch,omitempty"`
	GitStatus        string   `json:"git_status,omitempty"`
	GitRemote        string   `json:"git_remote,omitempty"`
	EnvVars          []string `json:"env_vars,omitempty"`
	RecentCmds       []string `json:"recent_cmds,omitempty"`
	InstalledPkgMgrs []string `json:"installed_pkg_mgrs,omitempty"`
}

// CommandResponse represents the AI-generated command response
type CommandResponse struct {
	Command      string    `json:"command"`
	Explanation  string    `json:"explanation"`
	RiskLevel    RiskLevel `json:"risk_level"`
	Confidence   float64   `json:"confidence"`
	Alternatives []string  `json:"alternatives,omitempty"`
	Warnings     []string  `json:"warnings,omitempty"`
}

// CommandAnalysis contains the safety analysis of a command
type CommandAnalysis struct {
	Command       string           `json:"command"`
	RiskLevel     RiskLevel        `json:"risk_level"`
	RiskReasons   []string         `json:"risk_reasons"`
	AffectedPaths []string         `json:"affected_paths"`
	AffectedFiles []FileInfo       `json:"affected_files"`
	Actions       []string         `json:"actions"`
	Reversible    bool             `json:"reversible"`
	RequiresSudo  bool             `json:"requires_sudo"`
	Patterns      []MatchedPattern `json:"matched_patterns"`
}

// FileInfo contains information about a file that may be affected
type FileInfo struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	IsDir     bool   `json:"is_dir"`
	FileCount int    `json:"file_count,omitempty"` // for directories
}

// MatchedPattern represents a dangerous pattern that was matched
type MatchedPattern struct {
	Pattern     string    `json:"pattern"`
	Description string    `json:"description"`
	RiskLevel   RiskLevel `json:"risk_level"`
}

// HistoryEntry represents a command in the history
type HistoryEntry struct {
	ID               string    `json:"id"`
	Timestamp        time.Time `json:"timestamp"`
	Prompt           string    `json:"prompt"`
	GeneratedCmd     string    `json:"generated_cmd"`
	RiskLevel        RiskLevel `json:"risk_level"`
	Executed         bool      `json:"executed"`
	ExitCode         int       `json:"exit_code"`
	DurationMs       int64     `json:"duration_ms"`
	WorkingDir       string    `json:"working_dir"`
	Provider         string    `json:"provider"`
	Model            string    `json:"model"`
	PromptTokens     int       `json:"prompt_tokens,omitempty"`
	CompletionTokens int       `json:"completion_tokens,omitempty"`
	TotalTokens      int       `json:"total_tokens,omitempty"`
}

// BackupEntry represents a backup of files before command execution
type BackupEntry struct {
	ID         string         `json:"id"`
	CommandID  string         `json:"command_id"`
	Timestamp  time.Time      `json:"timestamp"`
	Command    string         `json:"command"`
	WorkingDir string         `json:"working_dir"`
	Files      []BackedUpFile `json:"files"`
	TotalSize  int64          `json:"total_size"`
	Restored   bool           `json:"restored"`
	RestoredAt *time.Time     `json:"restored_at,omitempty"`
}

// BackedUpFile represents a single backed up file
type BackedUpFile struct {
	OriginalPath string `json:"original_path"`
	BackupPath   string `json:"backup_path"`
	Hash         string `json:"hash"`
	Size         int64  `json:"size"`
	IsDir        bool   `json:"is_dir"`
}

// MCPTool represents a tool exposed via MCP
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// MCPToolCall represents a call to an MCP tool
type MCPToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// MCPToolResult represents the result of an MCP tool call
type MCPToolResult struct {
	Content string `json:"content"`
	IsError bool   `json:"isError,omitempty"`
}

// ExecutionMode represents how commands should be executed
type ExecutionMode int

const (
	ModeInteractive ExecutionMode = iota
	ModeAuto
	ModeDryRun
	ModeExplainOnly
	ModeSilent
)

// Conversation represents an LLM chat conversation
type Conversation struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	SystemPrompt string    `json:"system_prompt,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Provider     string    `json:"provider"`
	Model        string    `json:"model"`
	TotalTokens  int       `json:"total_tokens"`
	MessageCount int       `json:"message_count"`
}

// ConversationMessage represents a message in a conversation
type ConversationMessage struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Role           string    `json:"role"` // "system", "user", "assistant", "tool"
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
	Tokens         int       `json:"tokens,omitempty"`
}

// ConversationExport represents a conversation for export/import
type ConversationExport struct {
	Version      int                   `json:"version"`
	ExportedAt   time.Time             `json:"exported_at"`
	Conversation Conversation          `json:"conversation"`
	Messages     []ConversationMessage `json:"messages"`
}

// Session represents a shell chat session
type Session struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Provider     string    `json:"provider"`
	Model        string    `json:"model"`
	TotalTokens  int       `json:"total_tokens"`
	MessageCount int       `json:"message_count"`
	CommandCount int       `json:"command_count"`
	LastCwd      string    `json:"last_cwd"`
}

// SessionMessage represents a message in a shell session
type SessionMessage struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"` // "user", "assistant", "system", "execution"
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	Tokens    int       `json:"tokens,omitempty"`

	// Execution-specific fields (when Role == "execution")
	Command   string    `json:"command,omitempty"`
	Output    string    `json:"output,omitempty"`
	ExitCode  int       `json:"exit_code,omitempty"`
	RiskLevel RiskLevel `json:"risk_level,omitempty"`
	Duration  int64     `json:"duration_ms,omitempty"`
	Executed  bool      `json:"executed,omitempty"`
}

// SessionExport represents a session for export/import
type SessionExport struct {
	Version    int              `json:"version"`
	ExportedAt time.Time        `json:"exported_at"`
	Session    Session          `json:"session"`
	Messages   []SessionMessage `json:"messages"`
}
