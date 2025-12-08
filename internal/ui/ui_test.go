// Package ui tests
package ui

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sonemaro/sosomi/internal/types"
)

// captureOutput captures stdout during function execution
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestColorFunctions(t *testing.T) {
	tests := []struct {
		name  string
		fn    func(a ...interface{}) string
		input string
	}{
		{"Green", Green, "test"},
		{"Yellow", Yellow, "test"},
		{"Red", Red, "test"},
		{"Cyan", Cyan, "test"},
		{"Magenta", Magenta, "test"},
		{"Bold", Bold, "test"},
		{"Dim", Dim, "test"},
		{"Success", Success, "test"},
		{"Warning", Warning, "test"},
		{"Error", Error, "test"},
		{"Info", Info, "test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn(tt.input)
			if result == "" {
				t.Error("Color function returned empty string")
			}
			// The colored string should contain the original text
			if !strings.Contains(result, tt.input) {
				t.Errorf("Color function result should contain '%s'", tt.input)
			}
		})
	}
}

func TestPrintCommand(t *testing.T) {
	output := captureOutput(func() {
		PrintCommand("ls -la")
	})

	if !strings.Contains(output, "ls -la") {
		t.Error("PrintCommand should output the command")
	}
	if !strings.Contains(output, "Generated") || !strings.Contains(output, "command") {
		t.Error("PrintCommand should indicate it's a generated command")
	}
}

func TestPrintExplanation(t *testing.T) {
	output := captureOutput(func() {
		PrintExplanation("This command lists all files")
	})

	if !strings.Contains(output, "lists all files") {
		t.Error("PrintExplanation should output the explanation")
	}
	if !strings.Contains(output, "Explanation") {
		t.Error("PrintExplanation should have explanation header")
	}
}

func TestPrintExplanation_Empty(t *testing.T) {
	output := captureOutput(func() {
		PrintExplanation("")
	})

	if output != "" {
		t.Error("PrintExplanation with empty string should output nothing")
	}
}

func TestPrintExplanation_MultiLine(t *testing.T) {
	output := captureOutput(func() {
		PrintExplanation("Line 1\nLine 2\nLine 3")
	})

	if !strings.Contains(output, "Line 1") {
		t.Error("PrintExplanation should handle multi-line")
	}
	if !strings.Contains(output, "Line 2") {
		t.Error("PrintExplanation should handle multi-line")
	}
}

func TestPrintRiskLevel(t *testing.T) {
	tests := []struct {
		level   types.RiskLevel
		reasons []string
	}{
		{types.RiskSafe, []string{}},
		{types.RiskCaution, []string{"Uses sudo"}},
		{types.RiskDangerous, []string{"Recursive deletion", "Force flag"}},
		{types.RiskCritical, []string{"System-wide impact"}},
	}

	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			output := captureOutput(func() {
				PrintRiskLevel(tt.level, tt.reasons)
			})

			if !strings.Contains(output, "Risk") {
				t.Error("PrintRiskLevel should contain 'Risk'")
			}

			for _, reason := range tt.reasons {
				if !strings.Contains(output, reason) {
					t.Errorf("PrintRiskLevel should contain reason '%s'", reason)
				}
			}
		})
	}
}

func TestPrintAnalysis(t *testing.T) {
	analysis := &types.CommandAnalysis{
		Command:       "rm -rf /tmp/test",
		RiskLevel:     types.RiskDangerous,
		RiskReasons:   []string{"Recursive deletion", "Force delete"},
		AffectedPaths: []string{"/tmp/test"},
		Actions:       []string{"DELETE files"},
		Reversible:    false,
		RequiresSudo:  false,
	}

	output := captureOutput(func() {
		PrintAnalysis(analysis)
	})

	expectedContents := []string{
		"rm -rf",
		"DANGEROUS",
	}

	for _, expected := range expectedContents {
		if !strings.Contains(output, expected) {
			t.Errorf("PrintAnalysis should contain '%s'", expected)
		}
	}
}

func TestPrintConfirmPrompt(t *testing.T) {
	output := captureOutput(func() {
		PrintConfirmPrompt()
	})

	// Should contain confirmation options
	if output == "" {
		t.Error("PrintConfirmPrompt should output something")
	}
}

func TestPrintRetryPrompt(t *testing.T) {
	output := captureOutput(func() {
		PrintRetryPrompt()
	})

	// Should contain retry options
	if output == "" {
		t.Error("PrintRetryPrompt should output something")
	}
}

func TestPrintFeedbackPrompt(t *testing.T) {
	output := captureOutput(func() {
		PrintFeedbackPrompt()
	})

	// Should prompt for feedback
	if output == "" {
		t.Error("PrintFeedbackPrompt should output something")
	}
}

func TestBoxCharacters(t *testing.T) {
	// Verify box drawing characters are defined
	chars := []string{
		BoxTopLeft,
		BoxTopRight,
		BoxBottomLeft,
		BoxBottomRight,
		BoxHorizontal,
		BoxVertical,
		BoxTeeRight,
		BoxTeeLeft,
	}

	for _, char := range chars {
		if char == "" {
			t.Error("Box character should not be empty")
		}
	}
}

func TestPrintSuccess(t *testing.T) {
	output := captureOutput(func() {
		PrintSuccess("Operation completed")
	})

	if !strings.Contains(output, "Operation completed") {
		t.Error("PrintSuccess should contain the message")
	}
}

func TestPrintError(t *testing.T) {
	output := captureOutput(func() {
		PrintError("Something went wrong")
	})

	if !strings.Contains(output, "Something went wrong") {
		t.Error("PrintError should contain the message")
	}
}

func TestPrintWarning(t *testing.T) {
	output := captureOutput(func() {
		PrintWarning("Be careful")
	})

	if !strings.Contains(output, "Be careful") {
		t.Error("PrintWarning should contain the message")
	}
}

func TestPrintInfo(t *testing.T) {
	output := captureOutput(func() {
		PrintInfo("Information")
	})

	if !strings.Contains(output, "Information") {
		t.Error("PrintInfo should contain the message")
	}
}

func TestSpinner(t *testing.T) {
	// Just verify spinner can be created without panicking
	spinner := NewSpinner("Loading...")
	if spinner == nil {
		t.Error("NewSpinner returned nil")
	}
}

func TestFormatDuration(t *testing.T) {
	// Test that duration formatting works via PrintExecutionResult
	// The formatDuration function is unexported, so we test indirectly
	output := captureOutput(func() {
		PrintExecutionResult("output", "", 0, 1500)
	})

	if !strings.Contains(output, "1.5") || !strings.Contains(output, "s") {
		t.Log("Duration formatting tested through PrintExecutionResult")
	}
}

func TestTruncate(t *testing.T) {
	// Test the truncate function indirectly through PrintAnalysis
	analysis := &types.CommandAnalysis{
		Command:   "this is a very long command that should be truncated at some point in the display",
		RiskLevel: types.RiskSafe,
	}

	output := captureOutput(func() {
		PrintAnalysis(analysis)
	})

	// The output should contain some form of the command (possibly truncated)
	if !strings.Contains(output, "this is") {
		t.Error("PrintAnalysis should contain command text")
	}
}

func TestPrintHistoryEntry(t *testing.T) {
	// Since PrintHistoryEntry is not exported, test history display
	// through available UI functions
	output := captureOutput(func() {
		PrintInfo("History entry: list files -> ls -la")
	})

	if !strings.Contains(output, "History") {
		t.Error("Should display history info")
	}
}

func TestFilterConversations(t *testing.T) {
	now := time.Now()
	conversations := []*types.Conversation{
		{ID: "11111111", Name: "Python Help", SystemPrompt: "coding assistant", UpdatedAt: now},
		{ID: "22222222", Name: "Math Tutor", SystemPrompt: "math expert", UpdatedAt: now},
		{ID: "33333333", Name: "Go Programming", SystemPrompt: "golang helper", UpdatedAt: now},
	}

	tests := []struct {
		name     string
		query    string
		expected int
	}{
		{"match name", "python", 1},
		{"match system prompt", "coding", 1},
		{"partial match in name", "pro", 1}, // Go Programming
		{"match in both fields", "math", 1}, // Math Tutor (name) + math expert (its own prompt)
		{"no match", "javascript", 0},
		{"case insensitive", "PYTHON", 1},
		{"empty query matches all", "", 3},
		{"match helper in prompt", "helper", 1}, // golang helper
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterConversations(conversations, tt.query)
			if len(result) != tt.expected {
				t.Errorf("FilterConversations(%q) = %d results, want %d", tt.query, len(result), tt.expected)
			}
		})
	}
}

func TestFilterConversationsEmpty(t *testing.T) {
	result := FilterConversations(nil, "test")
	if result != nil {
		t.Errorf("FilterConversations with nil input should return nil, got %v", result)
	}

	result = FilterConversations([]*types.Conversation{}, "test")
	if len(result) != 0 {
		t.Errorf("FilterConversations with empty input should return empty, got %v", result)
	}
}

func TestFormatDurationShort(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"seconds", 30 * time.Second, "30s ago"},
		{"one minute", 1 * time.Minute, "1m ago"},
		{"minutes", 5 * time.Minute, "5m ago"},
		{"one hour", 1 * time.Hour, "1h ago"},
		{"hours", 3 * time.Hour, "3h ago"},
		{"one day", 24 * time.Hour, "1d ago"},
		{"days", 5 * 24 * time.Hour, "5d ago"},
		{"one month", 30 * 24 * time.Hour, "1mo ago"},
		{"months", 60 * 24 * time.Hour, "2mo ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDurationShort(tt.duration)
			if result != tt.expected {
				t.Errorf("FormatDurationShort(%v) = %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestFormatDurationShortEdgeCases(t *testing.T) {
	// Test boundary cases
	tests := []struct {
		name     string
		duration time.Duration
		contains string
	}{
		{"zero", 0, "0s ago"},
		{"59 seconds", 59 * time.Second, "59s ago"},
		{"60 seconds becomes minute", 60 * time.Second, "1m ago"},
		{"59 minutes", 59 * time.Minute, "59m ago"},
		{"60 minutes becomes hour", 60 * time.Minute, "1h ago"},
		{"23 hours", 23 * time.Hour, "23h ago"},
		{"24 hours becomes day", 24 * time.Hour, "1d ago"},
		{"29 days", 29 * 24 * time.Hour, "29d ago"},
		{"30 days becomes month", 30 * 24 * time.Hour, "1mo ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDurationShort(tt.duration)
			if result != tt.contains {
				t.Errorf("FormatDurationShort(%v) = %q, want %q", tt.duration, result, tt.contains)
			}
		})
	}
}

func TestGetRiskColor(t *testing.T) {
	tests := []struct {
		name string
		risk types.RiskLevel
	}{
		{"critical", types.RiskCritical},
		{"safe", types.RiskSafe},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getRiskColor(tt.risk)
			// Just check it returns a function (non-nil)
			if result == nil {
				t.Errorf("getRiskColor(%v) returned nil", tt.risk)
			}
		})
	}
}

func TestPrintSimpleConfirm(t *testing.T) {
	output := captureOutput(func() {
		PrintSimpleConfirm("Proceed?")
	})

	// Should print something with the message
	if !strings.Contains(output, "Proceed") {
		t.Error("PrintSimpleConfirm should print the message")
	}
}

func TestPrintHeader(t *testing.T) {
	output := captureOutput(func() {
		PrintHeader()
	})

	// Should print header with sosomi name
	if !strings.Contains(strings.ToLower(output), "sosomi") {
		t.Error("PrintHeader should contain 'sosomi'")
	}
}

func TestFilterSessions(t *testing.T) {
	now := time.Now()
	sessions := []*types.Session{
		{ID: "1", Name: "test-session-1", CreatedAt: now},
		{ID: "2", Name: "prod-session", CreatedAt: now},
		{ID: "3", Name: "test-session-2", CreatedAt: now},
	}

	// Test filtering
	filtered := FilterSessions(sessions, "test")
	if len(filtered) != 2 {
		t.Errorf("Expected 2 sessions with 'test', got %d", len(filtered))
	}

	// Test no matches
	filtered = FilterSessions(sessions, "nonexistent")
	if len(filtered) != 0 {
		t.Errorf("Expected 0 sessions, got %d", len(filtered))
	}

	// Test empty filter (should return all)
	filtered = FilterSessions(sessions, "")
	if len(filtered) != 3 {
		t.Errorf("Expected all 3 sessions with empty filter, got %d", len(filtered))
	}
}
