package cmd

import (
	"strings"
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

func TestFormatFzfInput(t *testing.T) {
	entries := []switchEntry{
		{repo: "myapp", worktree: "main", branch: "main", path: "/home/user/worktrees/myapp/main"},
		{repo: "myapp", worktree: "feature-x", branch: "feature-x", path: "/home/user/worktrees/myapp/feature-x"},
		{repo: "api", worktree: "develop", branch: "develop", path: "/home/user/worktrees/api/develop"},
	}

	result := formatFzfInput(entries)
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	// Columns should be padded to align. Max widths: repo=5, worktree=9, branch=9
	// Format: "%-5s  %-9s  %-9s\t<path>"
	expected := []string{
		"myapp  main       main     \t/home/user/worktrees/myapp/main",
		"myapp  feature-x  feature-x\t/home/user/worktrees/myapp/feature-x",
		"api    develop    develop  \t/home/user/worktrees/api/develop",
	}

	for i, line := range lines {
		if line != expected[i] {
			t.Errorf("line %d:\n  got  %q\n  want %q", i, line, expected[i])
		}
	}
}

func TestFormatFzfInputEmpty(t *testing.T) {
	result := formatFzfInput(nil)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestFormatFzfHeader(t *testing.T) {
	entries := []switchEntry{
		{repo: "myapp", worktree: "main", branch: "main"},
		{repo: "myapp", worktree: "feature-x", branch: "feature-x"},
	}
	repoW, wtW, branchW := columnWidths(entries)
	header := formatFzfHeader(repoW, wtW, branchW)

	// Header should be aligned with the same widths as the data
	expected := "REPO   WORKTREE   BRANCH   "
	if header != expected {
		t.Errorf("header:\n  got  %q\n  want %q", header, expected)
	}
}

func TestColumnWidths(t *testing.T) {
	entries := []switchEntry{
		{repo: "ab", worktree: "x", branch: "short"},
		{repo: "long-repo-name", worktree: "my-worktree", branch: "b"},
	}
	repoW, wtW, branchW := columnWidths(entries)

	// "REPO" = 4, but "long-repo-name" = 14 -> 14
	if repoW != 14 {
		t.Errorf("repoW = %d, want 14", repoW)
	}
	// "WORKTREE" = 8, but "my-worktree" = 11 -> 11
	if wtW != 11 {
		t.Errorf("wtW = %d, want 11", wtW)
	}
	// "BRANCH" = 6, but "short" = 5 -> 6 (header wins)
	if branchW != 6 {
		t.Errorf("branchW = %d, want 6", branchW)
	}
}

func TestParseFzfOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantRepo string
		wantWT   string
		wantBr   string
		wantPath string
		wantErr  bool
	}{
		{
			name:     "padded line",
			input:    "myapp  feature-x  feature-x\t/home/user/worktrees/myapp/feature-x",
			wantRepo: "myapp",
			wantWT:   "feature-x",
			wantBr:   "feature-x",
			wantPath: "/home/user/worktrees/myapp/feature-x",
		},
		{
			name:     "with trailing newline",
			input:    "api    main       main     \t/home/user/worktrees/api/main\n",
			wantRepo: "api",
			wantWT:   "main",
			wantBr:   "main",
			wantPath: "/home/user/worktrees/api/main",
		},
		{
			name:    "no tab separator",
			input:   "myapp feature-x feature-x /some/path",
			wantErr: true,
		},
		{
			name:    "too few display fields",
			input:   "myapp\t/some/path",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parseFzfOutput(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseFzfOutput() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if entry.repo != tt.wantRepo {
				t.Errorf("repo = %q, want %q", entry.repo, tt.wantRepo)
			}
			if entry.worktree != tt.wantWT {
				t.Errorf("worktree = %q, want %q", entry.worktree, tt.wantWT)
			}
			if entry.branch != tt.wantBr {
				t.Errorf("branch = %q, want %q", entry.branch, tt.wantBr)
			}
			if entry.path != tt.wantPath {
				t.Errorf("path = %q, want %q", entry.path, tt.wantPath)
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
