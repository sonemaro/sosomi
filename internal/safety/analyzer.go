// Package safety provides command safety analysis
package safety

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/soroush/sosomi/internal/types"
	"mvdan.cc/sh/v3/syntax"
)

// Analyzer performs safety analysis on shell commands
type Analyzer struct {
	parser      *syntax.Parser
	customRules []CustomRule
	blockedCmds []string
	allowedPaths []string
}

// CustomRule represents a user-defined safety rule
type CustomRule struct {
	Pattern     string          `yaml:"pattern"`
	Action      string          `yaml:"action"` // warn, block, confirm
	Message     string          `yaml:"message"`
	RiskLevel   types.RiskLevel `yaml:"risk_level"`
}

// NewAnalyzer creates a new command analyzer
func NewAnalyzer(blockedCmds, allowedPaths []string) *Analyzer {
	return &Analyzer{
		parser:       syntax.NewParser(),
		blockedCmds:  blockedCmds,
		allowedPaths: allowedPaths,
	}
}

// Analyze performs a comprehensive safety analysis of a command
func (a *Analyzer) Analyze(command string) (*types.CommandAnalysis, error) {
	analysis := &types.CommandAnalysis{
		Command:    command,
		RiskLevel:  types.RiskSafe,
		Reversible: true,
	}

	// Parse the command using shell parser
	reader := strings.NewReader(command)
	prog, err := a.parser.Parse(reader, "")
	if err != nil {
		// If parsing fails, do pattern-based analysis only
		return a.patternAnalysis(command, analysis), nil
	}

	// Walk the AST to extract information
	syntax.Walk(prog, func(node syntax.Node) bool {
		switch n := node.(type) {
		case *syntax.CallExpr:
			a.analyzeCallExpr(n, analysis)
		case *syntax.Redirect:
			a.analyzeRedirect(n, analysis)
		case *syntax.BinaryCmd:
			a.analyzeBinaryCmd(n, analysis)
		}
		return true
	})

	// Pattern matching analysis
	a.patternAnalysis(command, analysis)

	// Check blocked commands
	a.checkBlockedCommands(command, analysis)

	// Check path restrictions
	a.checkPathRestrictions(analysis)

	return analysis, nil
}

// analyzeCallExpr analyzes a command call expression
func (a *Analyzer) analyzeCallExpr(call *syntax.CallExpr, analysis *types.CommandAnalysis) {
	if len(call.Args) == 0 {
		return
	}

	// Get command name
	cmdName := a.getLiteral(call.Args[0])
	if cmdName == "" {
		return
	}

	// Analyze specific commands
	switch cmdName {
	case "rm":
		a.analyzeRm(call, analysis)
	case "mv":
		a.analyzeMv(call, analysis)
	case "cp":
		a.analyzeCp(call, analysis)
	case "chmod":
		a.analyzeChmod(call, analysis)
	case "chown":
		a.analyzeChown(call, analysis)
	case "sudo":
		analysis.RequiresSudo = true
		if analysis.RiskLevel < types.RiskCaution {
			analysis.RiskLevel = types.RiskCaution
		}
		analysis.RiskReasons = append(analysis.RiskReasons, "Command requires elevated privileges")
	case "dd":
		analysis.RiskLevel = types.RiskDangerous
		analysis.Reversible = false
		analysis.RiskReasons = append(analysis.RiskReasons, "Direct disk access - potential data loss")
	}
}

// analyzeRm analyzes rm commands
func (a *Analyzer) analyzeRm(call *syntax.CallExpr, analysis *types.CommandAnalysis) {
	hasRecursive := false
	hasForce := false
	
	for _, arg := range call.Args[1:] {
		lit := a.getLiteral(arg)
		if lit == "" {
			continue
		}

		// Check flags
		if strings.HasPrefix(lit, "-") {
			if strings.Contains(lit, "r") || strings.Contains(lit, "R") {
				hasRecursive = true
			}
			if strings.Contains(lit, "f") {
				hasForce = true
			}
			continue
		}

		// Track affected paths
		analysis.AffectedPaths = append(analysis.AffectedPaths, lit)
		
		// Check for dangerous paths
		if lit == "/" || lit == "~" || lit == "$HOME" {
			analysis.RiskLevel = types.RiskCritical
			analysis.RiskReasons = append(analysis.RiskReasons, "Attempting to delete root or home directory")
		}
	}

	if hasRecursive && hasForce {
		if analysis.RiskLevel < types.RiskDangerous {
			analysis.RiskLevel = types.RiskDangerous
		}
		analysis.Reversible = false
		analysis.RiskReasons = append(analysis.RiskReasons, "Recursive force deletion cannot be undone")
	} else if hasRecursive || hasForce {
		if analysis.RiskLevel < types.RiskCaution {
			analysis.RiskLevel = types.RiskCaution
		}
		analysis.RiskReasons = append(analysis.RiskReasons, "Deletion operation")
	}

	analysis.Actions = append(analysis.Actions, "DELETE files/directories")
}

// analyzeMv analyzes mv commands
func (a *Analyzer) analyzeMv(call *syntax.CallExpr, analysis *types.CommandAnalysis) {
	for _, arg := range call.Args[1:] {
		lit := a.getLiteral(arg)
		if lit == "" || strings.HasPrefix(lit, "-") {
			continue
		}
		analysis.AffectedPaths = append(analysis.AffectedPaths, lit)
	}
	
	analysis.Actions = append(analysis.Actions, "MOVE/RENAME files")
	if analysis.RiskLevel < types.RiskCaution {
		analysis.RiskLevel = types.RiskCaution
	}
}

// analyzeCp analyzes cp commands
func (a *Analyzer) analyzeCp(call *syntax.CallExpr, analysis *types.CommandAnalysis) {
	for _, arg := range call.Args[1:] {
		lit := a.getLiteral(arg)
		if lit == "" || strings.HasPrefix(lit, "-") {
			continue
		}
		analysis.AffectedPaths = append(analysis.AffectedPaths, lit)
	}
	
	analysis.Actions = append(analysis.Actions, "COPY files")
}

// analyzeChmod analyzes chmod commands
func (a *Analyzer) analyzeChmod(call *syntax.CallExpr, analysis *types.CommandAnalysis) {
	hasRecursive := false
	
	for _, arg := range call.Args[1:] {
		lit := a.getLiteral(arg)
		if lit == "" {
			continue
		}

		if strings.HasPrefix(lit, "-") {
			if strings.Contains(lit, "R") {
				hasRecursive = true
			}
			continue
		}

		// Check for dangerous permissions
		if lit == "777" || lit == "0777" {
			analysis.RiskLevel = types.RiskDangerous
			analysis.RiskReasons = append(analysis.RiskReasons, "World-writable permissions are a security risk")
		}
		
		analysis.AffectedPaths = append(analysis.AffectedPaths, lit)
	}

	if hasRecursive {
		if analysis.RiskLevel < types.RiskCaution {
			analysis.RiskLevel = types.RiskCaution
		}
		analysis.RiskReasons = append(analysis.RiskReasons, "Recursive permission change")
	}

	analysis.Actions = append(analysis.Actions, "MODIFY permissions")
}

// analyzeChown analyzes chown commands
func (a *Analyzer) analyzeChown(call *syntax.CallExpr, analysis *types.CommandAnalysis) {
	hasRecursive := false
	
	for _, arg := range call.Args[1:] {
		lit := a.getLiteral(arg)
		if lit == "" {
			continue
		}

		if strings.HasPrefix(lit, "-") {
			if strings.Contains(lit, "R") {
				hasRecursive = true
			}
			continue
		}
		
		analysis.AffectedPaths = append(analysis.AffectedPaths, lit)
	}

	if hasRecursive {
		analysis.RiskLevel = types.RiskDangerous
		analysis.RiskReasons = append(analysis.RiskReasons, "Recursive ownership change")
	}

	analysis.Actions = append(analysis.Actions, "MODIFY ownership")
}

// analyzeRedirect analyzes redirections
func (a *Analyzer) analyzeRedirect(redir *syntax.Redirect, analysis *types.CommandAnalysis) {
	if redir.Word != nil {
		path := a.getLiteralWord(redir.Word)
		if path != "" {
			analysis.AffectedPaths = append(analysis.AffectedPaths, path)
			
			// Check if overwriting
			if redir.Op == syntax.RdrOut || redir.Op == syntax.RdrAll {
				if analysis.RiskLevel < types.RiskCaution {
					analysis.RiskLevel = types.RiskCaution
				}
				analysis.Actions = append(analysis.Actions, "OVERWRITE file: "+path)
			}
		}
	}
}

// analyzeBinaryCmd analyzes pipe and other binary commands
func (a *Analyzer) analyzeBinaryCmd(cmd *syntax.BinaryCmd, analysis *types.CommandAnalysis) {
	// Check for dangerous pipe patterns like curl | sh
	if cmd.Op == syntax.Pipe {
		// This is a pipe - check for curl/wget piped to sh/bash
		// The pattern matching will catch this
	}
}

// patternAnalysis performs regex-based pattern matching
func (a *Analyzer) patternAnalysis(command string, analysis *types.CommandAnalysis) *types.CommandAnalysis {
	for _, pattern := range GetDangerousPatterns() {
		if pattern.Pattern.MatchString(command) {
			if pattern.RiskLevel > analysis.RiskLevel {
				analysis.RiskLevel = pattern.RiskLevel
			}
			analysis.Patterns = append(analysis.Patterns, types.MatchedPattern{
				Pattern:     pattern.Pattern.String(),
				Description: pattern.Description,
				RiskLevel:   pattern.RiskLevel,
			})
			analysis.RiskReasons = append(analysis.RiskReasons, pattern.Description)
			
			if pattern.RiskLevel >= types.RiskDangerous {
				analysis.Reversible = false
			}
		}
	}
	return analysis
}

// checkBlockedCommands checks if the command contains blocked commands
func (a *Analyzer) checkBlockedCommands(command string, analysis *types.CommandAnalysis) {
	for _, blocked := range a.blockedCmds {
		if strings.Contains(command, blocked) {
			analysis.RiskLevel = types.RiskCritical
			analysis.RiskReasons = append(analysis.RiskReasons, "Command '"+blocked+"' is blocked by configuration")
		}
	}
}

// checkPathRestrictions checks if affected paths are within allowed paths
func (a *Analyzer) checkPathRestrictions(analysis *types.CommandAnalysis) {
	if len(a.allowedPaths) == 0 {
		return // No restrictions
	}

	for _, path := range analysis.AffectedPaths {
		// Expand path
		if strings.HasPrefix(path, "~") {
			home, _ := os.UserHomeDir()
			path = filepath.Join(home, path[1:])
		}
		
		allowed := false
		for _, allowedPath := range a.allowedPaths {
			if strings.HasPrefix(allowedPath, "~") {
				home, _ := os.UserHomeDir()
				allowedPath = filepath.Join(home, allowedPath[1:])
			}
			
			if strings.HasPrefix(path, allowedPath) {
				allowed = true
				break
			}
		}
		
		if !allowed {
			if analysis.RiskLevel < types.RiskCaution {
				analysis.RiskLevel = types.RiskCaution
			}
			analysis.RiskReasons = append(analysis.RiskReasons, "Path '"+path+"' is outside allowed directories")
		}
	}
}

// getLiteral extracts a literal string from a word
func (a *Analyzer) getLiteral(word *syntax.Word) string {
	return a.getLiteralWord(word)
}

// getLiteralWord extracts a literal string from a word
func (a *Analyzer) getLiteralWord(word *syntax.Word) string {
	if word == nil || len(word.Parts) == 0 {
		return ""
	}
	
	var result strings.Builder
	for _, part := range word.Parts {
		if lit, ok := part.(*syntax.Lit); ok {
			result.WriteString(lit.Value)
		}
	}
	return result.String()
}

// GetAffectedFiles expands paths and returns file information
func (a *Analyzer) GetAffectedFiles(analysis *types.CommandAnalysis) ([]types.FileInfo, error) {
	var files []types.FileInfo
	
	for _, path := range analysis.AffectedPaths {
		// Expand home directory
		if strings.HasPrefix(path, "~") {
			home, _ := os.UserHomeDir()
			path = filepath.Join(home, path[1:])
		}
		
		// Check if path exists
		info, err := os.Stat(path)
		if err != nil {
			// Path doesn't exist or error accessing
			continue
		}
		
		fileInfo := types.FileInfo{
			Path:  path,
			Size:  info.Size(),
			IsDir: info.IsDir(),
		}
		
		// Count files in directory
		if info.IsDir() {
			count := 0
			filepath.Walk(path, func(p string, i os.FileInfo, err error) error {
				if err == nil {
					count++
				}
				return nil
			})
			fileInfo.FileCount = count
		}
		
		files = append(files, fileInfo)
	}
	
	return files, nil
}
