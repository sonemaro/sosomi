# Sosomi — Safe AI Shell Assistant

> **Production-Grade CLI Documentation**  
> Version 0.1.0 | December 2025

---

## Table of Contents

1. [Overview](#overview)
2. [Features](#features)
3. [Architecture](#architecture)
4. [Installation](#installation)
5. [Configuration](#configuration)
6. [Usage Guide](#usage-guide)
7. [LLM Client Mode](#llm-client-mode)
8. [Safety System](#safety-system)
9. [AI Providers](#ai-providers)
10. [Development Guide](#development-guide)
11. [Testing](#testing)
12. [Contributing](#contributing)

---

## Overview

**Sosomi** (Safe AI Shell Assistant) is a Go-based command-line tool that converts natural language prompts into shell commands with enterprise-grade safety features. It's designed to be the safest way to use AI for shell automation.

### Why Sosomi?

| Problem | Sosomi's Solution |
|---------|-------------------|
| AI can generate dangerous commands | Multi-layer safety analysis with AST parsing |
| Cloud API keys can leak | Secure credential handling via env vars or password managers |
| Commands can cause irreversible damage | Automatic backup before risky operations |
| No audit trail for AI-generated commands | Full SQLite-based command history |
| Limited to cloud AI | Support for local models (Ollama, LM Studio, llama.cpp) |

### Design Philosophy

1. **Safety First** — Every command is analyzed before execution
2. **User Control** — Always confirm, never surprise
3. **Reversibility** — Backup everything that matters
4. **Transparency** — Show what will happen before it does
5. **Flexibility** — Work with any AI provider

---

## Features

### 🤖 AI Command Generation
- Natural language to shell command translation
- Multi-provider support (OpenAI, Ollama, LM Studio, llama.cpp, generic OpenAI-compatible)
- Context-aware prompts including OS, shell, git status, and available tools
- Streaming output for real-time response
- Command refinement with feedback loop

### 💬 Chat Mode
- Interactive REPL for conversational shell command generation
- Context-aware with system information
- Command refinement with feedback loop

### 🤖 LLM Client Mode
- General-purpose LLM chat client (not shell-focused)
- Multi-conversation support with SQLite storage
- Per-conversation system prompts (viewable and editable)
- Conversation history with search
- Export/import conversations for portability
- Interactive conversation picker UI
- Token usage tracking per conversation
- Auto-generated conversation titles

### 📊 Token Tracking
- Track prompt/completion/total tokens for all API calls
- Token statistics in history and conversations
- Usage visibility with `/tokens` command

### 💡 Intelligent Help
- AI-powered help command (`sosomi ask`)
- Context-aware answers about sosomi usage
- Ready-to-use command suggestions

### 🛡️ Safety Analysis
- **AST Parsing** — Uses `mvdan.cc/sh` to parse shell syntax
- **Risk Levels** — Safe (🟢), Caution (🟡), Dangerous (🟠), Critical (🔴)
- **Pattern Matching** — 30+ dangerous patterns detected
- **Path Protection** — Warns about system directories
- **Blocked Commands** — Configurable blocklist (shutdown, reboot, fork bombs)

### 📦 Backup & Undo
- Automatic file backup before risky operations
- SHA256 verification of backed-up files
- One-command restore with `sosomi undo`
- Configurable retention (default: 7 days)
- Size limits with automatic cleanup

### 📜 History & Audit
- SQLite-based command logging
- Full audit trail: prompt, command, risk level, exit code, duration
- Search and filter capabilities
- Statistics and analytics

### 🔌 MCP (Model Context Protocol)
- Extensible tool system
- JSON-RPC 2.0 over stdio
- Multiple server support
- Protocol version 2024-11-05

### ⚙️ Configuration System
- **Layered Config** — System → User → Project → Profile → Environment
- **Profiles** — Named configurations for different contexts
- **Validation** — Comprehensive config validation with hints
- **Wizard** — Interactive first-run setup
- **Legacy Migration** — Automatic upgrade from old config format

### 💻 User Interface
- Rich terminal output with colors
- Risk visualization with emojis
- Interactive confirmation with multiple options
- Spinner animations for loading states
- Keyboard shortcuts for shell integration

---

## Architecture

### Project Structure

```
sosomi/
├── cmd/
│   └── sosomi/
│       └── main.go              # CLI entry point (~1800 lines)
├── internal/
│   ├── ai/
│   │   ├── provider.go          # Provider interface, TokenUsage, StreamChunk
│   │   ├── factory.go           # Provider factory
│   │   ├── openai.go            # OpenAI implementation with token tracking
│   │   ├── ollama.go            # Ollama implementation with token tracking
│   │   └── local.go             # LM Studio/llama.cpp/generic with token tracking
│   ├── config/
│   │   ├── config.go            # Config management (~1100 lines)
│   │   ├── profile.go           # Profile management
│   │   ├── validation.go        # Config validation
│   │   └── wizard.go            # Interactive setup
│   ├── conversation/
│   │   └── store.go             # SQLite conversation storage (NEW)
│   ├── safety/
│   │   ├── analyzer.go          # Command analysis
│   │   └── patterns.go          # Dangerous patterns
│   ├── shell/
│   │   └── context.go           # System context & execution
│   ├── history/
│   │   └── store.go             # SQLite history with token tracking
│   ├── undo/
│   │   └── backup.go            # Backup manager
│   ├── mcp/
│   │   └── mcp.go               # MCP protocol
│   ├── ui/
│   │   └── ui.go                # Terminal UI with conversation picker
│   └── types/
│       └── types.go             # Shared types incl. Conversation
├── scripts/
│   ├── zsh-integration.zsh      # Zsh bindings
│   └── bash-integration.bash    # Bash bindings
├── config.example.yaml          # Example configuration
├── Makefile                     # Build automation
├── go.mod                       # Go dependencies
└── README.md                    # User documentation
```

### Package Dependencies

```
                    ┌─────────────────┐
                    │  cmd/sosomi     │
                    │    main.go      │
                    └────────┬────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
        ▼                    ▼                    ▼
   ┌─────────┐         ┌──────────┐         ┌─────────┐
   │   ai    │         │  config  │         │ safety  │
   │         │         │          │         │         │
   │ factory │         │ profile  │         │analyzer │
   │ openai  │         │ wizard   │         │patterns │
   │ ollama  │         │validation│         │         │
   │ local   │         │          │         │         │
   └────┬────┘         └────┬─────┘         └────┬────┘
        │                   │                    │
        │              ┌────┴─────┐              │
        │              │          │              │
        ▼              ▼          ▼              ▼
   ┌─────────┐    ┌────────┐ ┌────────┐    ┌─────────┐
   │  types  │◄───│ shell  │ │  undo  │    │   ui    │
   │         │    │        │ │        │    │         │
   └─────────┘    └────────┘ └────────┘    └─────────┘
        ▲              │          │              ▲
        │              │          │              │
        │         ┌────┴──────────┴────┐    ┌────┴────┐
        └─────────│      history       │    │  mcp    │
                  │                    │    │         │
                  └────────────────────┘    └─────────┘
                             ▲
                             │
                  ┌──────────┴─────────┐
                  │   conversation     │
                  │  (LLM sessions)    │
                  └────────────────────┘
```

### Design Patterns

| Pattern | Usage |
|---------|-------|
| **Factory** | `ai.NewProvider()` creates providers by type |
| **Strategy** | `Provider` interface with multiple implementations |
| **Layered Config** | System → User → Project → Profile → Env |
| **Repository** | `history.Store` and `conversation.Store` for SQLite data access |
| **Command** | Cobra commands for CLI structure |
| **Template Method** | Common AI flow with provider-specific details |

### Data Flow

```
┌──────────────────────────────────────────────────────────────────────┐
│                         USER INPUT                                   │
│                    "list files larger than 100MB"                    │
└────────────────────────────┬─────────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────────┐
│                      CONTEXT GATHERING                               │
│  • OS: darwin    • Shell: zsh    • Git: main (clean)                │
│  • Dir: /home/user/project       • Tools: brew, npm, go            │
└────────────────────────────┬─────────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────────┐
│                      AI GENERATION                                   │
│  Provider: openai    Model: gpt-4o    Streaming: enabled            │
│  ──────────────────────────────────────────────────────             │
│  Command: find . -type f -size +100M -exec ls -lh {} \;             │
│  Risk: SAFE    Confidence: 95%                                      │
└────────────────────────────┬─────────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────────┐
│                      SAFETY ANALYSIS                                 │
│  • AST Parse → Valid syntax                                         │
│  • Pattern Match → No dangerous patterns                            │
│  • Path Check → Current directory only                              │
│  • Blocked Check → Not in blocklist                                 │
│  ──────────────────────────────────────────────────────             │
│  Result: SAFE (🟢)    Reversible: Yes    Backup: Not needed         │
└────────────────────────────┬─────────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────────┐
│                      USER CONFIRMATION                               │
│  [y] Execute  [n] Cancel  [d] Dry-run  [e] Explain  [m] Modify      │
└────────────────────────────┬─────────────────────────────────────────┘
                             │ (user presses 'y')
                             ▼
┌──────────────────────────────────────────────────────────────────────┐
│                      EXECUTION                                       │
│  • Backup affected files (if risky)                                 │
│  • Execute command                                                  │
│  • Capture output                                                   │
│  • Log to history                                                   │
└────────────────────────────┬─────────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────────┐
│                      RESULT                                          │
│  Exit Code: 0    Duration: 234ms                                    │
│  Output: ./large_file.zip  156M                                     │
│  ──────────────────────────────────────────────────────             │
│  [Enter] Done  [r] Retry with feedback                              │
└──────────────────────────────────────────────────────────────────────┘
```

---

## Installation

### From Source

```bash
# Clone
git clone https://github.com/soroush/sosomi.git
cd sosomi

# Build
make build

# Install to ~/.local/bin
make install

# Or system-wide
sudo make install-system
```

### Shell Integration

Add to your `.zshrc`:
```bash
source /path/to/sosomi/scripts/zsh-integration.zsh
```

Or `.bashrc`:
```bash
source /path/to/sosomi/scripts/bash-integration.bash
```

This enables:
- `Ctrl+G` — Quick prompt from current command line
- `ss` / `sq` / `sd` / `sa` — Command aliases

---

## Configuration

### Configuration Hierarchy

Configuration is loaded in layers, with each layer overriding the previous:

| Priority | Location | Purpose |
|----------|----------|---------|
| 1 (lowest) | `/etc/sosomi/config.yaml` | System defaults |
| 2 | `~/.config/sosomi/config.yaml` | User preferences |
| 3 | `./.sosomi/config.yaml` | Project-specific |
| 4 | `~/.config/sosomi/profiles/<name>.yaml` | Named profiles |
| 5 (highest) | `SOSOMI_*` environment variables | Runtime overrides |

### Configuration Structure

```yaml
# Config version for migration support
version: 1

# Default profile to use
default_profile: work

# ═══════════════════════════════════════════════════════════
# PROVIDER CONFIGURATION
# ═══════════════════════════════════════════════════════════
provider:
  # Supported: openai, ollama, lmstudio, llamacpp, generic
  name: openai
  
  # API endpoint
  endpoint: https://api.openai.com/v1
  
  # API key (choose ONE method):
  api_key_env: OPENAI_API_KEY           # From environment variable (recommended)
  # api_key_cmd: "op read 'OpenAI Key'" # From command (e.g., 1Password)
  # api_key: "sk-..."                   # Direct (NOT recommended)

# ═══════════════════════════════════════════════════════════
# MODEL CONFIGURATION
# ═══════════════════════════════════════════════════════════
model:
  # Model identifier
  name: gpt-4o
  
  # Generation parameters
  max_tokens: 2048
  temperature: 0.1          # Low = deterministic
  top_p: 1.0
  
  # Request handling
  timeout_seconds: 30
  max_retries: 3
  stream_output: true

# ═══════════════════════════════════════════════════════════
# SAFETY CONFIGURATION
# ═══════════════════════════════════════════════════════════
safety:
  # Levels: strict, cautious, moderate, normal, relaxed, dangerous
  level: moderate
  
  # Confirmation behavior
  require_confirmation: true
  auto_execute_safe: false
  
  # Blocked commands (even with --force)
  blocked_commands:
    - shutdown
    - reboot
    - init 0
    - init 6
    - ":(){ :|:& };:"      # Fork bomb
  
  # Extra caution for these paths
  protected_paths:
    - /
    - /etc
    - /usr
    - /bin
    - /boot
  
  # Only allow operations in these paths (empty = allow all)
  # allowed_paths:
  #   - /home/user/projects

# ═══════════════════════════════════════════════════════════
# BACKUP CONFIGURATION
# ═══════════════════════════════════════════════════════════
backup:
  enabled: true
  dir: ~/.local/share/sosomi/backups
  retention_days: 7
  max_size_mb: 500
  exclude:
    - node_modules
    - .git
    - __pycache__
    - "*.log"
    - .DS_Store

# ═══════════════════════════════════════════════════════════
# HISTORY CONFIGURATION
# ═══════════════════════════════════════════════════════════
history:
  enabled: true
  db_path: ~/.local/share/sosomi/history.db
  retention_days: 30

# ═══════════════════════════════════════════════════════════
# LLM CLIENT MODE CONFIGURATION
# ═══════════════════════════════════════════════════════════
llm:
  enabled: true
  db_path: ~/.local/share/sosomi/conversations.db
  default_system_prompt: "You are a helpful assistant."
  generate_titles: true
  retention_days: 90

# ═══════════════════════════════════════════════════════════
# MCP CONFIGURATION
# ═══════════════════════════════════════════════════════════
mcp:
  enabled: true
  servers: []
  tools_dir: ~/.config/sosomi/mcp_tools

# ═══════════════════════════════════════════════════════════
# UI CONFIGURATION
# ═══════════════════════════════════════════════════════════
ui:
  color_enabled: true
  show_explanations: true
  show_timings: true
  compact_mode: false
  language: en

# ═══════════════════════════════════════════════════════════
# SHELL CONFIGURATION
# ═══════════════════════════════════════════════════════════
shell:
  # default_shell: /bin/zsh
  capture_output: true
  output_max_lines: 100

# ═══════════════════════════════════════════════════════════
# COMMAND ALIASES
# ═══════════════════════════════════════════════════════════
aliases:
  ll: "ls -la"
  gs: "git status"
```

### Profiles

Profiles allow switching between configurations:

```bash
# Create a profile
sosomi profile create work

# List profiles
sosomi profile list

# Use a profile
sosomi --profile work "find large files"

# Set default profile
sosomi profile use work

# Export/Import (secrets stripped)
sosomi profile export work work.yaml
sosomi profile import work.yaml
```

### Environment Variables

Override any setting with `SOSOMI_` prefix:

```bash
export SOSOMI_PROVIDER_NAME=ollama
export SOSOMI_MODEL_NAME=llama3.2
export SOSOMI_SAFETY_LEVEL=strict
```

---

## Usage Guide

### Basic Usage

```bash
# Generate and execute a command
sosomi "list all files larger than 100MB"

# Generate with explanation only
sosomi -e "compress all images in this folder"

# Dry-run (no execution)
sosomi -d "delete all .tmp files"

# Auto-execute safe commands
sosomi -a "show current directory"

# Silent mode
sosomi -s "check disk usage"

# Use specific profile
sosomi -p work "deploy to production"
```

### Interactive Mode

```bash
# Shell command assistant (focused on generating shell commands)
sosomi chat
```

Commands in chat mode:
- `/help` — Show help
- `/history` — Recent commands
- `/quit` — Exit

### Intelligent Help

```bash
# Ask AI for help with sosomi commands
sosomi ask "how do I continue an existing conversation?"
sosomi ask "what providers are supported?"
sosomi ask "how to use ollama with local models?"
```

### History & Undo

```bash
# View recent commands
sosomi history

# Search history
sosomi history search "docker"

# View statistics
sosomi history stats

# Undo last command
sosomi undo

# List available backups
sosomi undo list
```

### Configuration Management

```bash
# Initialize (interactive wizard)
sosomi init

# Show current config
sosomi config show

# Set a value
sosomi config set provider.name ollama

# Validate configuration
sosomi config validate

# Show config paths
sosomi config path

# Edit config in $EDITOR
sosomi config edit
```

### Risk Level Indicators

| Level | Emoji | Description | Examples |
|-------|-------|-------------|----------|
| Safe | 🟢 | Read-only, no side effects | `ls`, `cat`, `pwd` |
| Caution | 🟡 | Minor changes, reversible | `mkdir`, `touch`, `git add` |
| Dangerous | 🟠 | Significant changes | `rm`, `mv`, `chmod` |
| Critical | 🔴 | System-level, irreversible | `rm -rf /`, `mkfs`, `dd` |

---

## LLM Client Mode

Sosomi includes a general-purpose LLM chat client separate from the shell command mode. This is useful for general AI conversations while keeping them organized and persistent.

### Starting Conversations

```bash
# Start a new conversation
sosomi llm

# Start with a name
sosomi llm "Python Help"

# Start with a custom system prompt
sosomi llm -s "You are a Python expert who writes clean, idiomatic code."

# Both name and system prompt
sosomi llm "Code Review" -s "You are a senior developer reviewing code."
```

### Continuing Conversations

```bash
# Continue by ID (first 8 chars shown in list)
sosomi llm -c abc12345

# Continue by name
sosomi llm -c "Python Help"

# Interactive picker UI
sosomi llm pick
sosomi llm pick -p 5  # 5 items per page
```

### In-Conversation Commands

| Command | Description |
|---------|-------------|
| `/help` | Show available commands |
| `/info` | Show conversation details (ID, tokens, messages) |
| `/history` | Display full conversation history |
| `/system` | View/edit system prompt |
| `/tokens` | Show token usage |
| `/clear` | Clear the screen |
| `/quit` | Exit the conversation |

### Managing Conversations

```bash
# List all conversations
sosomi llm list

# Delete a conversation
sosomi llm delete "Python Help"
sosomi llm delete abc12345

# Export for backup/portability
sosomi llm export "Python Help" backup.json

# Import from file
sosomi llm import backup.json

# View statistics
sosomi llm stats
```

### Interactive Picker

The `sosomi llm pick` command opens a full-screen interactive UI:

```
  💬 Conversation Picker
  Select a conversation to continue or start a new one

──────────────────────────────────────────────────────────────────────────────
  #   ID        Name                              Msgs      Tokens    Updated
──────────────────────────────────────────────────────────────────────────────
  1   abc12345  Python Help                       12        1,245     2h ago
  2   def67890  Math Tutor                        8         892       1d ago
  3   ghi11223  Go Programming                    24        3,421     3d ago
──────────────────────────────────────────────────────────────────────────────
  Page 1/2  [n] Next

  [1-9] Select  [c] Create new  [s] Search  [q] Quit
```

Controls:
- `1-9` — Select conversation by number
- `c` — Create new conversation
- `s` — Search by name or system prompt
- `n/p` — Next/Previous page
- `q` — Quit

### Editing System Prompts

Use `/system` within a conversation:

```
you> /system

📋 Current system prompt:
You are a helpful assistant.

Enter new system prompt (empty to keep current, 'clear' to remove): You are a Python expert.
✓ System prompt updated
```

### Token Tracking

Every conversation tracks token usage:

```
you> /tokens
💰 Tokens used: 1,245

you> /info

📝 Conversation Info:
  ID:        abc12345-...
  Name:      Python Help
  Provider:  openai
  Model:     gpt-4o
  Messages:  12
  Tokens:    1,245
  Created:   2025-12-06 10:30
  Updated:   2025-12-06 14:22

  📋 System Prompt:
     You are a Python expert who writes clean, idiomatic code.
```

### Conversation Storage

Conversations are stored in SQLite at `~/.local/share/sosomi/conversations.db`:

- **conversations** table: ID, name, system prompt, provider, model, timestamps, token counts
- **messages** table: Role (user/assistant/system), content, tokens, timestamp

---

## Safety System

### Analysis Pipeline

```
Command String
     │
     ▼
┌─────────────────────────────────────┐
│         1. SHELL PARSER             │
│    mvdan.cc/sh AST generation       │
│    Syntax validation                │
└────────────────┬────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────┐
│         2. AST ANALYSIS             │
│    • Command identification         │
│    • Argument extraction            │
│    • Redirect detection             │
│    • Pipeline analysis              │
└────────────────┬────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────┐
│      3. COMMAND-SPECIFIC            │
│    • rm: recursive, force flags     │
│    • chmod: dangerous permissions   │
│    • mv/cp: overwrite detection     │
│    • curl: pipe to shell            │
└────────────────┬────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────┐
│       4. PATTERN MATCHING           │
│    30+ regex patterns for:          │
│    • Fork bombs                     │
│    • Disk wiping                    │
│    • Privilege escalation           │
│    • Dangerous downloads            │
└────────────────┬────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────┐
│       5. PATH RESTRICTIONS          │
│    • Check protected paths          │
│    • Enforce allowed paths          │
│    • Validate file existence        │
└────────────────┬────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────┐
│       6. BLOCKED COMMANDS           │
│    • User-configured blocklist      │
│    • System blocklist               │
└────────────────┬────────────────────┘
                 │
                 ▼
        CommandAnalysis Result
```

### Dangerous Pattern Categories

**Critical (🔴):**
- `rm -rf /` or `rm -rf /*`
- Fork bombs: `:(){ :|:& };:`
- Disk overwrite: `dd if=/dev/zero of=/dev/sda`
- Format commands: `mkfs.*`

**Dangerous (🟠):**
- `chmod 777` on directories
- `curl | sh` or `wget | bash`
- `sudo rm -rf`
- Wildcard deletes: `rm -rf *`

**Caution (🟡):**
- `sudo` commands
- `kill -9`
- `git push --force`
- `docker system prune`

### Backup Strategy

```
Before risky command execution:
     │
     ▼
┌─────────────────────────────────────┐
│    1. Identify affected files       │
│       from CommandAnalysis          │
└────────────────┬────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────┐
│    2. Filter by exclusion list      │
│       (node_modules, .git, etc.)    │
└────────────────┬────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────┐
│    3. Check size limits             │
│       (default: 500MB total)        │
└────────────────┬────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────┐
│    4. Copy files with structure     │
│       Preserve permissions          │
│       Compute SHA256 hashes         │
└────────────────┬────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────┐
│    5. Save metadata.json            │
│       Command, timestamp, files     │
└─────────────────────────────────────┘
```

---

## AI Providers

### Provider Comparison

| Provider | Type | Endpoint | Best For |
|----------|------|----------|----------|
| OpenAI | Cloud | api.openai.com | Best quality, production use |
| Ollama | Local | localhost:11434 | Privacy, offline use |
| LM Studio | Local | localhost:1234 | Easy local setup |
| llama.cpp | Local | localhost:8080 | Performance, customization |
| Generic | Cloud/Local | Custom | Any OpenAI-compatible API |

### OpenAI

```yaml
provider:
  name: openai
  endpoint: https://api.openai.com/v1
  api_key_env: OPENAI_API_KEY

model:
  name: gpt-4o  # or gpt-4o-mini, gpt-4-turbo
```

### Ollama

```yaml
provider:
  name: ollama
  endpoint: http://localhost:11434

model:
  name: llama3.2  # or codellama, mistral, qwen2.5-coder
```

### LM Studio

```yaml
provider:
  name: lmstudio
  endpoint: http://localhost:1234/v1

model:
  name: local-model  # LM Studio serves loaded model
```

### llama.cpp

```yaml
provider:
  name: llamacpp
  endpoint: http://localhost:8080/v1

model:
  name: model  # llama.cpp server serves loaded model
```

### Azure OpenAI

```yaml
provider:
  name: generic
  endpoint: https://YOUR-RESOURCE.openai.azure.com/openai/deployments/YOUR-DEPLOYMENT/
  api_key_env: AZURE_OPENAI_KEY

model:
  name: gpt-4
```

---

## Development Guide

### Prerequisites

- Go 1.21+
- Make
- golangci-lint (for linting)

### Setup

```bash
# Clone repository
git clone https://github.com/soroush/sosomi.git
cd sosomi

# Install dependencies
make deps

# Build
make build

# Run tests
make test
```

### Code Style

- Follow standard Go conventions
- Use `gofmt` and `goimports`
- Run `make lint` before committing
- Document exported functions
- Write tests for new features

### Adding a New AI Provider

1. Create `internal/ai/newprovider.go`:

```go
package ai

type NewProvider struct {
    client   *http.Client
    endpoint string
    model    string
}

func NewNewProvider(endpoint, model string) (*NewProvider, error) {
    // Initialize
}

func (p *NewProvider) Name() string {
    return "newprovider"
}

func (p *NewProvider) GenerateCommand(ctx context.Context, prompt string, sysContext *types.SystemContext) (*types.CommandResponse, error) {
    // Implementation
}

// Implement remaining Provider interface methods...
```

2. Add to factory (`internal/ai/factory.go`):

```go
func NewProvider(providerType, apiKey, endpoint, model string) (Provider, error) {
    switch providerType {
    // ... existing cases
    case "newprovider":
        return NewNewProvider(endpoint, model)
    }
}
```

3. Add tests in `internal/ai/newprovider_test.go`

### Adding a Safety Pattern

Edit `internal/safety/patterns.go`:

```go
var dangerPatterns = []DangerPattern{
    // ... existing patterns
    {
        Pattern:     regexp.MustCompile(`your-regex-here`),
        Description: "Description of what this catches",
        RiskLevel:   types.RiskDangerous,
        Category:    "CATEGORY",
    },
}
```

### Project Conventions

| Convention | Description |
|------------|-------------|
| Package layout | `cmd/` for binaries, `internal/` for private packages |
| Error handling | Return errors, don't panic |
| Configuration | Use `config.Get()` singleton |
| Testing | Table-driven tests with `*_test.go` files |
| Documentation | GoDoc comments on exports |

---

## Testing

### Running Tests

```bash
# All tests
make test

# With coverage
make test-coverage

# Specific package
go test ./internal/config/...

# Verbose
go test -v ./internal/...

# Single test
go test -run TestDefaultConfig ./internal/config/
```

### Test Coverage

Current coverage breakdown:

| Package | Tests | Description |
|---------|-------|-------------|
| `internal/ai` | 42 | Provider implementations |
| `internal/config` | 60 | Config, profiles, validation |
| `internal/safety` | 35 | Analyzer, patterns |
| `internal/history` | 18 | SQLite store |
| `internal/conversation` | 15 | LLM conversation store |
| `internal/undo` | 22 | Backup manager |
| `internal/shell` | 15 | Context, execution |
| `internal/types` | 12 | Type methods |
| `internal/ui` | 12 | UI components, conversation picker |
| `internal/mcp` | 29 | MCP protocol |
| **Total** | **260** | |

### Writing Tests

```go
func TestFeatureName(t *testing.T) {
    // Table-driven tests
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"basic case", "input1", "output1"},
        {"edge case", "input2", "output2"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := Function(tt.input)
            if result != tt.expected {
                t.Errorf("got %v, want %v", result, tt.expected)
            }
        })
    }
}
```

---

## Contributing

### Workflow

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make changes with tests
4. Run `make lint && make test`
5. Commit: `git commit -m 'Add amazing feature'`
6. Push: `git push origin feature/amazing-feature`
7. Open a Pull Request

### Commit Messages

Follow conventional commits:

```
feat: add new AI provider support
fix: handle edge case in safety analyzer
docs: update configuration examples
test: add tests for profile management
refactor: simplify config loading
```

### Code Review Checklist

- [ ] Tests pass (`make test`)
- [ ] Lint passes (`make lint`)
- [ ] Documentation updated
- [ ] Backward compatible (or migration provided)
- [ ] Security considered

---

## License

MIT License — See LICENSE file for details.

---

## Acknowledgments

- [Cobra](https://github.com/spf13/cobra) — CLI framework
- [mvdan.cc/sh](https://github.com/mvdan/sh) — Shell parser
- [fatih/color](https://github.com/fatih/color) — Terminal colors
- [sashabaranov/go-openai](https://github.com/sashabaranov/go-openai) — OpenAI client

---

<div align="center">

**Made with ❤️ for safe AI-assisted shell automation**

</div>
