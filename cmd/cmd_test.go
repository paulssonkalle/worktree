package cmd

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestExactArgs(t *testing.T) {
	validator := exactArgs("name", "repo-url")
	cmd := &cobra.Command{Use: "test <name> <repo-url>"}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"exact match", []string{"myrepo", "https://example.com"}, false},
		{"too few", []string{"myrepo"}, true},
		{"too many", []string{"a", "b", "c"}, true},
		{"none", []string{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator(cmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("exactArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExactArgsErrorMessage(t *testing.T) {
	validator := exactArgs("name", "repo-url")
	cmd := &cobra.Command{Use: "repo add <name> <repo-url>"}

	err := validator(cmd, []string{"only-one"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	msg := err.Error()
	// Should contain the argument names
	if !contains(msg, "name, repo-url") {
		t.Errorf("error message %q does not contain argument names", msg)
	}
	// Should contain the count
	if !contains(msg, "2 argument(s)") {
		t.Errorf("error message %q does not contain argument count", msg)
	}
}

func TestRangeArgs(t *testing.T) {
	validator := rangeArgs(1, 2, "repo", "worktree?")
	cmd := &cobra.Command{Use: "list [repo] [worktree]"}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"min args", []string{"myrepo"}, false},
		{"max args", []string{"myrepo", "feature"}, false},
		{"too few", []string{}, true},
		{"too many", []string{"a", "b", "c"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator(cmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("rangeArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRangeArgsErrorMessage(t *testing.T) {
	validator := rangeArgs(1, 2, "repo", "worktree?")
	cmd := &cobra.Command{Use: "list [repo] [worktree]"}

	err := validator(cmd, []string{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	msg := err.Error()
	if !contains(msg, "between 1 and 2") {
		t.Errorf("error message %q does not contain range", msg)
	}
	if !contains(msg, "repo, worktree?") {
		t.Errorf("error message %q does not contain argument names", msg)
	}
}

func TestMaxArgs(t *testing.T) {
	validator := maxArgs(1, "repo?")
	cmd := &cobra.Command{Use: "status [repo]"}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no args", []string{}, false},
		{"one arg", []string{"myrepo"}, false},
		{"too many", []string{"a", "b"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator(cmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("maxArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMaxArgsErrorMessage(t *testing.T) {
	validator := maxArgs(1, "repo?")
	cmd := &cobra.Command{Use: "status [repo]"}

	err := validator(cmd, []string{"a", "b"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	msg := err.Error()
	if !contains(msg, "at most 1") {
		t.Errorf("error message %q does not contain max count", msg)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"zero", 0, "< 1 day"},
		{"less than a day", 12 * time.Hour, "< 1 day"},
		{"one day", 24 * time.Hour, "1 day"},
		{"one day plus", 36 * time.Hour, "1 day"},
		{"two days", 48 * time.Hour, "2 days"},
		{"many days", 30 * 24 * time.Hour, "30 days"},
		{"365 days", 365 * 24 * time.Hour, "365 days"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
