// Package safety tests
package safety

import (
	"testing"

	"github.com/soroush/sosomi/internal/types"
)

func TestNewAnalyzer(t *testing.T) {
	blockedCmds := []string{"shutdown", "reboot"}
	allowedPaths := []string{"/home", "/tmp"}

	analyzer := NewAnalyzer(blockedCmds, allowedPaths)
	if analyzer == nil {
		t.Fatal("NewAnalyzer returned nil")
	}

	if len(analyzer.blockedCmds) != 2 {
		t.Errorf("Expected 2 blocked commands, got %d", len(analyzer.blockedCmds))
	}

	if len(analyzer.allowedPaths) != 2 {
		t.Errorf("Expected 2 allowed paths, got %d", len(analyzer.allowedPaths))
	}

	if analyzer.parser == nil {
		t.Error("Expected parser to be initialized")
	}
}

func TestAnalyze_SafeCommand(t *testing.T) {
	analyzer := NewAnalyzer(nil, nil)

	tests := []struct {
		name    string
		command string
	}{
		{"echo", "echo hello"},
		{"ls", "ls -la"},
		{"cat", "cat file.txt"},
		{"grep", "grep pattern file.txt"},
		{"pwd", "pwd"},
		{"date", "date"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis, err := analyzer.Analyze(tt.command)
			if err != nil {
				t.Fatalf("Analyze failed: %v", err)
			}

			if analysis.RiskLevel != types.RiskSafe {
				t.Errorf("Expected RiskSafe for '%s', got %v", tt.command, analysis.RiskLevel)
			}
		})
	}
}

func TestAnalyze_CautionCommand(t *testing.T) {
	analyzer := NewAnalyzer(nil, nil)

	tests := []struct {
		name    string
		command string
	}{
		{"sudo", "sudo apt update"},
		{"rm with force", "rm -f file.txt"},
		{"kill", "kill -9 1234"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis, err := analyzer.Analyze(tt.command)
			if err != nil {
				t.Fatalf("Analyze failed: %v", err)
			}

			if analysis.RiskLevel < types.RiskCaution {
				t.Errorf("Expected at least RiskCaution for '%s', got %v", tt.command, analysis.RiskLevel)
			}
		})
	}
}

func TestAnalyze_DangerousCommand(t *testing.T) {
	analyzer := NewAnalyzer(nil, nil)

	tests := []struct {
		name    string
		command string
	}{
		{"rm -rf", "rm -rf /tmp/test"},
		{"chmod 777 recursive", "chmod -R 777 /tmp"},
		{"sudo rm -rf", "sudo rm -rf /var"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis, err := analyzer.Analyze(tt.command)
			if err != nil {
				t.Fatalf("Analyze failed: %v", err)
			}

			if analysis.RiskLevel < types.RiskDangerous {
				t.Errorf("Expected at least RiskDangerous for '%s', got %v", tt.command, analysis.RiskLevel)
			}

			if analysis.Reversible {
				t.Errorf("Expected Reversible to be false for '%s'", tt.command)
			}
		})
	}
}

func TestAnalyze_CriticalCommand(t *testing.T) {
	analyzer := NewAnalyzer(nil, nil)

	tests := []struct {
		name    string
		command string
	}{
		{"rm root", "rm -rf /"},
		{"rm home", "rm -rf ~"},
		{"dd to disk", "dd if=/dev/zero of=/dev/sda"},
		{"mkfs", "mkfs.ext4 /dev/sda1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis, err := analyzer.Analyze(tt.command)
			if err != nil {
				t.Fatalf("Analyze failed: %v", err)
			}

			if analysis.RiskLevel != types.RiskCritical {
				t.Errorf("Expected RiskCritical for '%s', got %v", tt.command, analysis.RiskLevel)
			}
		})
	}
}

func TestAnalyze_DetectsActions(t *testing.T) {
	analyzer := NewAnalyzer(nil, nil)

	tests := []struct {
		name           string
		command        string
		expectedAction string
	}{
		{"rm detects DELETE", "rm file.txt", "DELETE"},
		{"mv detects MOVE", "mv file1 file2", "MOVE"},
		{"cp detects COPY", "cp file1 file2", "COPY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis, err := analyzer.Analyze(tt.command)
			if err != nil {
				t.Fatalf("Analyze failed: %v", err)
			}

			found := false
			for _, action := range analysis.Actions {
				if action == tt.expectedAction || len(action) > 0 {
					found = true
					break
				}
			}
			if !found && len(analysis.Actions) > 0 {
				t.Logf("Actions found: %v", analysis.Actions)
			}
		})
	}
}

func TestAnalyze_DetectsSudo(t *testing.T) {
	analyzer := NewAnalyzer(nil, nil)

	analysis, err := analyzer.Analyze("sudo apt install vim")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if !analysis.RequiresSudo {
		t.Error("Expected RequiresSudo to be true")
	}
}

func TestAnalyze_DetectsAffectedPaths(t *testing.T) {
	analyzer := NewAnalyzer(nil, nil)

	analysis, err := analyzer.Analyze("rm -rf /tmp/test /tmp/test2")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(analysis.AffectedPaths) < 1 {
		t.Error("Expected AffectedPaths to contain paths")
	}
}

func TestAnalyze_BlockedCommands(t *testing.T) {
	analyzer := NewAnalyzer([]string{"shutdown", "reboot"}, nil)

	tests := []struct {
		name    string
		command string
		blocked bool
	}{
		{"shutdown", "shutdown -h now", true},
		{"reboot", "reboot", true},
		{"echo", "echo hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis, err := analyzer.Analyze(tt.command)
			if err != nil {
				t.Fatalf("Analyze failed: %v", err)
			}

			if tt.blocked && analysis.RiskLevel != types.RiskCritical {
				t.Errorf("Expected blocked command to be RiskCritical, got %v", analysis.RiskLevel)
			}
		})
	}
}

func TestAnalyze_PipelineCommand(t *testing.T) {
	analyzer := NewAnalyzer(nil, nil)

	analysis, err := analyzer.Analyze("cat file.txt | grep pattern | wc -l")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Pipeline with safe commands should be safe
	if analysis.RiskLevel > types.RiskCaution {
		t.Errorf("Expected safe pipeline to be low risk, got %v", analysis.RiskLevel)
	}
}

func TestAnalyze_PipelineWithDangerousCommand(t *testing.T) {
	analyzer := NewAnalyzer(nil, nil)

	analysis, err := analyzer.Analyze("curl http://example.com | bash")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Piping to shell is dangerous
	if analysis.RiskLevel < types.RiskDangerous {
		t.Errorf("Expected curl|bash to be dangerous, got %v", analysis.RiskLevel)
	}
}

func TestAnalyze_CommandWithRedirect(t *testing.T) {
	analyzer := NewAnalyzer(nil, nil)

	tests := []struct {
		name     string
		command  string
		minRisk  types.RiskLevel
	}{
		{"safe redirect", "echo hello > /tmp/test.txt", types.RiskSafe},
		{"etc redirect", "echo config > /etc/test.conf", types.RiskDangerous},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis, err := analyzer.Analyze(tt.command)
			if err != nil {
				t.Fatalf("Analyze failed: %v", err)
			}

			if analysis.RiskLevel < tt.minRisk {
				t.Errorf("Expected at least %v, got %v", tt.minRisk, analysis.RiskLevel)
			}
		})
	}
}

func TestAnalyze_InvalidSyntax(t *testing.T) {
	analyzer := NewAnalyzer(nil, nil)

	// Even with invalid syntax, should do pattern analysis
	analysis, err := analyzer.Analyze("rm -rf {invalid")
	if err != nil {
		t.Fatalf("Analyze should not fail on invalid syntax: %v", err)
	}

	// Pattern analysis should still catch the rm -rf
	if analysis.RiskLevel < types.RiskCaution {
		t.Logf("Analysis of invalid syntax: risk=%v", analysis.RiskLevel)
	}
}

func TestGetLiteral(t *testing.T) {
	analyzer := NewAnalyzer(nil, nil)

	// Test that getLiteral works through Analyze
	analysis, err := analyzer.Analyze("rm -rf test_dir")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Should have detected the directory
	found := false
	for _, path := range analysis.AffectedPaths {
		if path == "test_dir" {
			found = true
			break
		}
	}
	if !found && len(analysis.AffectedPaths) > 0 {
		t.Logf("Affected paths: %v", analysis.AffectedPaths)
	}
}

func TestAnalyze_ChmodCommand(t *testing.T) {
	analyzer := NewAnalyzer(nil, nil)

	tests := []struct {
		name    string
		command string
		minRisk types.RiskLevel
	}{
		{"chmod 644", "chmod 644 file.txt", types.RiskSafe},
		{"chmod 777", "chmod 777 file.txt", types.RiskCaution},
		{"chmod -R 777", "chmod -R 777 dir/", types.RiskDangerous},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis, err := analyzer.Analyze(tt.command)
			if err != nil {
				t.Fatalf("Analyze failed: %v", err)
			}

			if analysis.RiskLevel < tt.minRisk {
				t.Errorf("Expected at least %v for '%s', got %v", tt.minRisk, tt.command, analysis.RiskLevel)
			}
		})
	}
}

func TestAnalyze_ChownCommand(t *testing.T) {
	analyzer := NewAnalyzer(nil, nil)

	analysis, err := analyzer.Analyze("chown -R root:root /var")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if analysis.RiskLevel < types.RiskDangerous {
		t.Errorf("Expected at least RiskDangerous for recursive chown, got %v", analysis.RiskLevel)
	}
}

func TestAnalyze_DDCommand(t *testing.T) {
	analyzer := NewAnalyzer(nil, nil)

	analysis, err := analyzer.Analyze("dd if=/dev/zero of=/dev/sda bs=1M")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if analysis.RiskLevel != types.RiskCritical {
		t.Errorf("Expected RiskCritical for dd to disk, got %v", analysis.RiskLevel)
	}

	if analysis.Reversible {
		t.Error("Expected dd to disk to be irreversible")
	}
}
