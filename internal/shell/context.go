// Package shell provides shell command execution and context detection
package shell

import (
	"bytes"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sonemaro/sosomi/internal/types"
)

// GetSystemContext gathers information about the current system
func GetSystemContext() types.SystemContext {
	ctx := types.SystemContext{
		OS:    runtime.GOOS,
		Shell: getShell(),
	}

	// Get current directory
	if dir, err := os.Getwd(); err == nil {
		ctx.CurrentDir = dir
	}

	// Get home directory
	if home, err := os.UserHomeDir(); err == nil {
		ctx.HomeDir = home
	}

	// Get username
	if u, err := user.Current(); err == nil {
		ctx.Username = u.Username
	}

	// Git information
	ctx.GitBranch = getGitBranch()
	if ctx.GitBranch != "" {
		ctx.GitStatus = getGitStatus()
		ctx.GitRemote = getGitRemote()
	}

	// Detect installed package managers
	ctx.InstalledPkgMgrs = detectPackageManagers()

	// Get recent commands from history
	ctx.RecentCmds = getRecentCommands(5)

	return ctx
}

// getShell returns the current shell
func getShell() string {
	shell := os.Getenv("SHELL")
	if shell != "" {
		return filepath.Base(shell)
	}
	return "sh"
}

// getGitBranch returns the current git branch if in a git repo
func getGitBranch() string {
	cmd := exec.Command("git", "branch", "--show-current")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// getGitStatus returns the git status (clean/dirty)
func getGitStatus() string {
	cmd := exec.Command("git", "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	if len(strings.TrimSpace(string(out))) == 0 {
		return "clean"
	}
	return "dirty"
}

// getGitRemote returns the git remote URL
func getGitRemote() string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// detectPackageManagers detects installed package managers
func detectPackageManagers() []string {
	var managers []string

	// Check common package managers
	checks := map[string]string{
		"brew":   "brew",
		"apt":    "apt",
		"yum":    "yum",
		"dnf":    "dnf",
		"pacman": "pacman",
		"npm":    "npm",
		"yarn":   "yarn",
		"pnpm":   "pnpm",
		"pip":    "pip",
		"pip3":   "pip3",
		"cargo":  "cargo",
		"go":     "go",
	}

	for name, cmd := range checks {
		if _, err := exec.LookPath(cmd); err == nil {
			managers = append(managers, name)
		}
	}

	return managers
}

// getRecentCommands returns recent commands from shell history
func getRecentCommands(count int) []string {
	var cmds []string

	// Try to read zsh history
	home, _ := os.UserHomeDir()
	histFile := filepath.Join(home, ".zsh_history")

	data, err := os.ReadFile(histFile)
	if err != nil {
		// Try bash history
		histFile = filepath.Join(home, ".bash_history")
		data, err = os.ReadFile(histFile)
		if err != nil {
			return cmds
		}
	}

	lines := strings.Split(string(data), "\n")
	start := len(lines) - count - 1
	if start < 0 {
		start = 0
	}

	for _, line := range lines[start:] {
		line = strings.TrimSpace(line)
		// Skip empty lines and zsh timestamp entries
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		// For zsh, extract command from extended format
		if idx := strings.Index(line, ";"); idx != -1 {
			line = line[idx+1:]
		}
		if line != "" {
			cmds = append(cmds, line)
		}
		if len(cmds) >= count {
			break
		}
	}

	return cmds
}

// ExecuteResult represents the result of command execution
type ExecuteResult struct {
	Command    string
	ExitCode   int
	Stdout     string
	Stderr     string
	DurationMs int64
	Error      error
}

// Execute runs a shell command
func Execute(command string, dryRun bool) (*ExecuteResult, error) {
	result := &ExecuteResult{
		Command: command,
	}

	if dryRun {
		result.Stdout = "[DRY RUN] Would execute: " + command
		return result, nil
	}

	shell := getShell()
	var cmd *exec.Cmd

	switch shell {
	case "zsh":
		cmd = exec.Command("zsh", "-c", command)
	case "bash":
		cmd = exec.Command("bash", "-c", command)
	default:
		cmd = exec.Command("sh", "-c", command)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir, _ = os.Getwd()

	start := now()
	err := cmd.Run()
	result.DurationMs = now() - start

	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
		} else {
			result.ExitCode = 1
		}
		result.Error = err
	}

	return result, nil
}

// ExecuteInteractive runs a command with interactive I/O
func ExecuteInteractive(command string) error {
	shell := getShell()
	var cmd *exec.Cmd

	switch shell {
	case "zsh":
		cmd = exec.Command("zsh", "-c", command)
	case "bash":
		cmd = exec.Command("bash", "-c", command)
	default:
		cmd = exec.Command("sh", "-c", command)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir, _ = os.Getwd()

	return cmd.Run()
}

// now returns current time in milliseconds
func now() int64 {
	return time.Now().UnixMilli()
}
