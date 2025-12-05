// Package safety provides command safety analysis
package safety

import (
	"regexp"

	"github.com/soroush/sosomi/internal/types"
)

// DangerPattern represents a dangerous command pattern
type DangerPattern struct {
	Pattern     *regexp.Regexp
	Description string
	RiskLevel   types.RiskLevel
	Category    string
}

// dangerousPatterns contains all known dangerous command patterns
var dangerousPatterns = []DangerPattern{
	// CRITICAL - System destruction
	{
		Pattern:     regexp.MustCompile(`rm\s+(-[rfRv]+\s+)*(/|~/?$)`),
		Description: "Delete from root or home directory",
		RiskLevel:   types.RiskCritical,
		Category:    "filesystem",
	},
	{
		Pattern:     regexp.MustCompile(`rm\s+-rf\s+/\s*$`),
		Description: "Wipe entire filesystem",
		RiskLevel:   types.RiskCritical,
		Category:    "filesystem",
	},
	{
		Pattern:     regexp.MustCompile(`dd\s+.*of=/dev/(sd[a-z]|nvme|disk|hd[a-z])`),
		Description: "Direct disk write - can destroy data",
		RiskLevel:   types.RiskCritical,
		Category:    "disk",
	},
	{
		Pattern:     regexp.MustCompile(`mkfs(\.[a-z0-9]+)?\s+`),
		Description: "Format filesystem",
		RiskLevel:   types.RiskCritical,
		Category:    "disk",
	},
	{
		Pattern:     regexp.MustCompile(`:\s*\(\s*\)\s*\{\s*:\s*\|\s*:\s*&\s*\}\s*;\s*:`),
		Description: "Fork bomb - will crash system",
		RiskLevel:   types.RiskCritical,
		Category:    "system",
	},
	{
		Pattern:     regexp.MustCompile(`>\s*/dev/(sd[a-z]|nvme|disk|hd[a-z])`),
		Description: "Overwrite disk device",
		RiskLevel:   types.RiskCritical,
		Category:    "disk",
	},
	{
		Pattern:     regexp.MustCompile(`mv\s+(/|~)\s+`),
		Description: "Move root or home directory",
		RiskLevel:   types.RiskCritical,
		Category:    "filesystem",
	},
	{
		Pattern:     regexp.MustCompile(`chmod\s+(-R\s+)?0?00?0\s+/`),
		Description: "Remove all permissions from root",
		RiskLevel:   types.RiskCritical,
		Category:    "permissions",
	},

	// DANGEROUS - Major system changes
	{
		Pattern:     regexp.MustCompile(`chmod\s+-R\s+777`),
		Description: "World-writable permissions (security risk)",
		RiskLevel:   types.RiskDangerous,
		Category:    "permissions",
	},
	{
		Pattern:     regexp.MustCompile(`chown\s+-R\s+`),
		Description: "Recursive ownership change",
		RiskLevel:   types.RiskDangerous,
		Category:    "permissions",
	},
	{
		Pattern:     regexp.MustCompile(`curl\s+.*\|\s*(ba)?sh`),
		Description: "Pipe URL to shell - potential malware",
		RiskLevel:   types.RiskDangerous,
		Category:    "network",
	},
	{
		Pattern:     regexp.MustCompile(`wget\s+.*(-O\s*-|--output-document\s*=?\s*-).*\|\s*(ba)?sh`),
		Description: "Download and execute - potential malware",
		RiskLevel:   types.RiskDangerous,
		Category:    "network",
	},
	{
		Pattern:     regexp.MustCompile(`sudo\s+rm\s+-rf`),
		Description: "Privileged recursive deletion",
		RiskLevel:   types.RiskDangerous,
		Category:    "filesystem",
	},
	{
		Pattern:     regexp.MustCompile(`>\s*/etc/`),
		Description: "Overwrite system configuration",
		RiskLevel:   types.RiskDangerous,
		Category:    "system",
	},
	{
		Pattern:     regexp.MustCompile(`rm\s+.*\*\s*$`),
		Description: "Delete with wildcard",
		RiskLevel:   types.RiskDangerous,
		Category:    "filesystem",
	},
	{
		Pattern:     regexp.MustCompile(`fdisk|parted|diskutil\s+erase`),
		Description: "Disk partition modification",
		RiskLevel:   types.RiskDangerous,
		Category:    "disk",
	},
	{
		Pattern:     regexp.MustCompile(`launchctl\s+unload.*com\.apple`),
		Description: "Unload system service",
		RiskLevel:   types.RiskDangerous,
		Category:    "system",
	},

	// CAUTION - Potentially risky
	{
		Pattern:     regexp.MustCompile(`rm\s+-[rf]+`),
		Description: "Recursive or force delete",
		RiskLevel:   types.RiskCaution,
		Category:    "filesystem",
	},
	{
		Pattern:     regexp.MustCompile(`sudo\s+`),
		Description: "Elevated privileges",
		RiskLevel:   types.RiskCaution,
		Category:    "system",
	},
	{
		Pattern:     regexp.MustCompile(`kill\s+-9`),
		Description: "Force kill process",
		RiskLevel:   types.RiskCaution,
		Category:    "process",
	},
	{
		Pattern:     regexp.MustCompile(`pkill|killall`),
		Description: "Kill processes by name",
		RiskLevel:   types.RiskCaution,
		Category:    "process",
	},
	{
		Pattern:     regexp.MustCompile(`>\s+[^|]`),
		Description: "File overwrite redirect",
		RiskLevel:   types.RiskCaution,
		Category:    "filesystem",
	},
	{
		Pattern:     regexp.MustCompile(`git\s+push.*--force`),
		Description: "Force push can overwrite history",
		RiskLevel:   types.RiskCaution,
		Category:    "git",
	},
	{
		Pattern:     regexp.MustCompile(`git\s+reset\s+--hard`),
		Description: "Hard reset discards changes",
		RiskLevel:   types.RiskCaution,
		Category:    "git",
	},
	{
		Pattern:     regexp.MustCompile(`docker\s+system\s+prune`),
		Description: "Remove all unused Docker data",
		RiskLevel:   types.RiskCaution,
		Category:    "docker",
	},
	{
		Pattern:     regexp.MustCompile(`docker\s+rm\s+-f`),
		Description: "Force remove container",
		RiskLevel:   types.RiskCaution,
		Category:    "docker",
	},
	{
		Pattern:     regexp.MustCompile(`brew\s+uninstall|apt\s+remove|yum\s+remove`),
		Description: "Package removal",
		RiskLevel:   types.RiskCaution,
		Category:    "packages",
	},
	{
		Pattern:     regexp.MustCompile(`history\s+-c|>\s*~/\.(bash_history|zsh_history)`),
		Description: "Clear command history",
		RiskLevel:   types.RiskCaution,
		Category:    "system",
	},
	{
		Pattern:     regexp.MustCompile(`truncate\s+`),
		Description: "Truncate file (data loss)",
		RiskLevel:   types.RiskCaution,
		Category:    "filesystem",
	},
	{
		Pattern:     regexp.MustCompile(`shred\s+`),
		Description: "Secure delete (unrecoverable)",
		RiskLevel:   types.RiskCaution,
		Category:    "filesystem",
	},
}

// GetDangerousPatterns returns all dangerous patterns
func GetDangerousPatterns() []DangerPattern {
	return dangerousPatterns
}

// GetPatternsByCategory returns patterns filtered by category
func GetPatternsByCategory(category string) []DangerPattern {
	var result []DangerPattern
	for _, p := range dangerousPatterns {
		if p.Category == category {
			result = append(result, p)
		}
	}
	return result
}

// GetPatternsByRiskLevel returns patterns filtered by risk level
func GetPatternsByRiskLevel(level types.RiskLevel) []DangerPattern {
	var result []DangerPattern
	for _, p := range dangerousPatterns {
		if p.RiskLevel == level {
			result = append(result, p)
		}
	}
	return result
}
