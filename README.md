# 🐚 Sosomi - Safe AI Shell Assistant

Sosomi is a powerful CLI tool that converts natural language to shell commands with advanced safety features. It works with OpenAI, Ollama, LM Studio, llama.cpp, and other OpenAI-compatible endpoints.

## Features

- **🤖 AI-Powered Command Generation**: Convert natural language to shell commands
- **🛡️ Safety Guardrails**: Pattern-based and AST-parsed command analysis
- **⚡ Multiple Providers**: OpenAI, Ollama, LM Studio, llama.cpp, and generic OpenAI-compatible endpoints
- **📝 Audit Logging**: Full history of all commands with searchable database
- **🔄 MCP Support**: Model Context Protocol for extensibility
- **🎨 Beautiful UI**: Rich terminal output with colors and progress indicators
- **💬 Chat Mode**: Interactive REPL for conversational interaction

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/soroush/sosomi.git
cd sosomi

# Build the binary
go build -o sosomi ./cmd/sosomi

# Install to local bin
mv sosomi ~/.local/bin/
# Or for system-wide:
# sudo mv sosomi /usr/local/bin/
```

### Shell Integration (Recommended)

For zsh, add to `~/.zshrc`:
```bash
source /path/to/sosomi/scripts/zsh-integration.zsh
```

For bash, add to `~/.bashrc`:
```bash
source /path/to/sosomi/scripts/bash-integration.bash
```

This enables:
- `Ctrl+G` to invoke sosomi from anywhere
- Tab completion
- Handy aliases: `s`, `sq`, `sd`, `sa`

## Quick Start

```bash
# Set your API key (for OpenAI)
export OPENAI_API_KEY="your-key-here"

# Basic usage
sosomi "list all files larger than 100MB"

# Dry-run mode (no execution)
sosomi "delete all .tmp files" --dry-run

# Auto-execute safe commands
sosomi "show disk usage" --auto

# Use local model (Ollama)
sosomi "find all TODO comments" -p ollama -m llama3.2

# Interactive chat mode
sosomi chat
```

## Configuration

Create `~/.config/sosomi/config.yaml`:

```yaml
# AI Provider Settings
provider: openai              # openai, ollama, lmstudio, llamacpp, generic
model: gpt-4o                 # Model to use
api_key: ""                   # Set via env var OPENAI_API_KEY or SOSOMI_API_KEY

# Local Model Endpoints
ollama_endpoint: http://localhost:11434
lmstudio_endpoint: http://localhost:1234/v1
llamacpp_endpoint: http://localhost:8080

# Safety Settings
safety_profile: moderate      # strict, moderate, permissive
auto_execute_safe: false      # Auto-execute low-risk commands
require_confirmation: true    # Always ask before executing
blocked_commands:             # Always blocked
  - shutdown
  - reboot
  - init 0
  - init 6

# History Settings
history_enabled: true
history_db_path: ~/.sosomi/history.db
history_retention_days: 30

# UI Settings
color_enabled: true
show_explanations: true

# Advanced
timeout_seconds: 30
stream_output: true
```

### Environment Variables

- `SOSOMI_API_KEY` or `OPENAI_API_KEY`: API key for OpenAI
- `SOSOMI_PROVIDER`: Default provider
- `SOSOMI_MODEL`: Default model

## Usage Examples

### Basic Commands

```bash
# Find files
sosomi "find all Python files modified in the last week"

# System info
sosomi "show CPU and memory usage"

# Git operations
sosomi "create a new git branch called feature-login"

# File operations
sosomi "compress all log files older than 30 days"
```

### Safety Features

```bash
# Dry-run to preview what would happen
sosomi "remove all node_modules directories" --dry-run

# Use strict safety profile
sosomi "delete old files" --profile strict
```

### Working with Local Models

```bash
# Using Ollama
sosomi "list all docker containers" -p ollama -m llama3.2

# Using LM Studio
sosomi "show network interfaces" -p lmstudio

# Using llama.cpp server
sosomi "check disk health" -p llamacpp
```

### History

```bash
# View command history
sosomi history

# Show history statistics
sosomi history stats
```

### Configuration

```bash
# Show current configuration
sosomi config show

# Set a configuration value
sosomi config set provider ollama
sosomi config set model llama3.2
```

## Risk Levels

Sosomi analyzes commands and assigns risk levels:

| Level | Icon | Description |
|-------|------|-------------|
| SAFE | 🟢 | Read-only or low-risk operations |
| CAUTION | 🟡 | Modifying operations, review recommended |
| DANGEROUS | 🟠 | High-risk operations, review carefully |
| CRITICAL | 🔴 | Blocked by default, could cause data loss |

## Command Flow

1. **Input**: User provides natural language prompt
2. **Context**: Sosomi gathers system context (OS, shell, git status, etc.)
3. **Generation**: AI generates shell command with explanation
4. **Analysis**: Command is analyzed for safety using pattern matching and shell AST parsing
5. **Display**: Command, explanation, and risk level are shown
6. **Confirmation**: User can execute, modify, explain, or dry-run
7. **Execution**: Command runs with output capture
8. **Logging**: Everything is logged for audit

## Architecture

```
sosomi/
├── cmd/sosomi/          # CLI entry point
├── internal/
│   ├── ai/              # AI provider implementations
│   ├── config/          # Configuration management
│   ├── history/         # SQLite audit logging
│   ├── mcp/             # Model Context Protocol
│   ├── safety/          # Command safety analysis
│   ├── shell/           # System context and execution
│   ├── types/           # Shared type definitions
│   └── ui/              # Terminal UI components
└── scripts/             # Shell integration scripts
```

## Safety Patterns

Sosomi recognizes dangerous patterns including:

- Recursive force delete (`rm -rf`)
- Disk operations (`dd`, `mkfs`, `fdisk`)
- System modification (`chmod -R`, `chown -R`)
- Network exposure (port forwarding, firewall rules)
- History manipulation
- Credential exposure
- Fork bombs and malicious patterns

## MCP (Model Context Protocol)

Sosomi supports MCP for extensibility. Built-in tools:

- `execute_command`: Run shell commands
- `read_file`: Read file contents
- `write_file`: Write to files
- `list_directory`: List directory contents

Configure custom MCP servers in `config.yaml`:

```yaml
mcp_enabled: true
mcp_servers:
  - path/to/mcp-server
```

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

MIT License - See [LICENSE](LICENSE) for details.

## Acknowledgments

- Inspired by [shell-gpt](https://github.com/TheR1D/shell_gpt), [aichat](https://github.com/sigoden/aichat), and [ai-shell](https://github.com/BuilderIO/ai-shell)
- Built with [go-openai](https://github.com/sashabaranov/go-openai), [cobra](https://github.com/spf13/cobra), [viper](https://github.com/spf13/viper), and [mvdan/sh](https://github.com/mvdan/sh)
