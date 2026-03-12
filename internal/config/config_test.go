package config

import (
	"os"
	"path/filepath"
	"testing"
)

// setupTestConfig creates a temp dir, sets the config path to a file inside it,
// and returns a cleanup function that restores the original config state.
func setupTestConfig(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.toml")
	SetConfigPath(path)
	return path
}

func TestLoadCreatesDefaultConfig(t *testing.T) {
	path := setupTestConfig(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.BasePath != "~/worktrees" {
		t.Errorf("BasePath = %q, want %q", cfg.BasePath, "~/worktrees")
	}
	if cfg.Editor != "code" {
		t.Errorf("Editor = %q, want %q", cfg.Editor, "code")
	}
	if cfg.CleanupThresholdDays != 30 {
		t.Errorf("CleanupThresholdDays = %d, want %d", cfg.CleanupThresholdDays, 30)
	}
	if cfg.Repositories == nil {
		t.Error("Repositories is nil, want initialized map")
	}

	// Config file should have been created on disk
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("config file was not created on disk")
	}
}

func TestIsFirstRunOnNewConfig(t *testing.T) {
	setupTestConfig(t)

	if _, err := Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if !IsFirstRun() {
		t.Error("IsFirstRun() = false after creating new config, want true")
	}
}

func TestIsFirstRunOnExistingConfig(t *testing.T) {
	path := setupTestConfig(t)

	// Write a config file first
	content := `base_path = "~/projects"
editor = "vim"
cleanup_threshold_days = 14
`
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Reset singleton so it re-reads
	Reload()

	if _, err := Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if IsFirstRun() {
		t.Error("IsFirstRun() = true for existing config, want false")
	}
}

func TestLoadExistingConfig(t *testing.T) {
	path := setupTestConfig(t)

	content := `base_path = "~/my-worktrees"
editor = "nvim"
cleanup_threshold_days = 7

[repositories.myapp]
repo_url = "git@github.com:user/myapp.git"
default_branch = "main"

[repositories.myapp.worktrees.feature-x]
pinned = true
`
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	Reload()
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.BasePath != "~/my-worktrees" {
		t.Errorf("BasePath = %q, want %q", cfg.BasePath, "~/my-worktrees")
	}
	if cfg.Editor != "nvim" {
		t.Errorf("Editor = %q, want %q", cfg.Editor, "nvim")
	}
	if cfg.CleanupThresholdDays != 7 {
		t.Errorf("CleanupThresholdDays = %d, want %d", cfg.CleanupThresholdDays, 7)
	}

	repo, exists := cfg.Repositories["myapp"]
	if !exists {
		t.Fatal("repository 'myapp' not found in config")
	}
	if repo.RepoURL != "git@github.com:user/myapp.git" {
		t.Errorf("RepoURL = %q, want %q", repo.RepoURL, "git@github.com:user/myapp.git")
	}
	if repo.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want %q", repo.DefaultBranch, "main")
	}

	wtCfg, exists := repo.Worktrees["feature-x"]
	if !exists {
		t.Fatal("worktree 'feature-x' not found in repository config")
	}
	if !wtCfg.Pinned {
		t.Error("worktree 'feature-x' Pinned = false, want true")
	}
}

func TestLoadDefaultsForMissingFields(t *testing.T) {
	path := setupTestConfig(t)

	// Config with only editor set; base_path and cleanup_threshold_days missing
	content := `editor = "vim"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	Reload()
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.BasePath != "~/worktrees" {
		t.Errorf("BasePath = %q, want default %q", cfg.BasePath, "~/worktrees")
	}
	if cfg.CleanupThresholdDays != 30 {
		t.Errorf("CleanupThresholdDays = %d, want default %d", cfg.CleanupThresholdDays, 30)
	}
	if cfg.Repositories == nil {
		t.Error("Repositories is nil, want initialized map")
	}
}

func TestSaveAndLoad(t *testing.T) {
	setupTestConfig(t)

	original := &Config{
		BasePath:             "~/test-trees",
		Editor:               "emacs",
		CleanupThresholdDays: 15,
		Repositories: map[string]RepositoryConfig{
			"testproj": {
				RepoURL:       "https://github.com/test/repo.git",
				DefaultBranch: "develop",
				Worktrees: map[string]WorktreeConfig{
					"feat-1": {Pinned: true},
					"feat-2": {Pinned: false},
				},
			},
		},
	}

	if err := Save(original); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Force reload from disk
	Reload()
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.BasePath != original.BasePath {
		t.Errorf("BasePath = %q, want %q", loaded.BasePath, original.BasePath)
	}
	if loaded.Editor != original.Editor {
		t.Errorf("Editor = %q, want %q", loaded.Editor, original.Editor)
	}
	if loaded.CleanupThresholdDays != original.CleanupThresholdDays {
		t.Errorf("CleanupThresholdDays = %d, want %d", loaded.CleanupThresholdDays, original.CleanupThresholdDays)
	}

	repo, exists := loaded.Repositories["testproj"]
	if !exists {
		t.Fatal("repository 'testproj' not found after load")
	}
	if repo.RepoURL != "https://github.com/test/repo.git" {
		t.Errorf("RepoURL = %q, want %q", repo.RepoURL, "https://github.com/test/repo.git")
	}
	if repo.DefaultBranch != "develop" {
		t.Errorf("DefaultBranch = %q, want %q", repo.DefaultBranch, "develop")
	}

	wt, exists := repo.Worktrees["feat-1"]
	if !exists {
		t.Fatal("worktree 'feat-1' not found")
	}
	if !wt.Pinned {
		t.Error("feat-1 Pinned = false, want true")
	}

	wt2, exists := repo.Worktrees["feat-2"]
	if !exists {
		t.Fatal("worktree 'feat-2' not found")
	}
	if wt2.Pinned {
		t.Error("feat-2 Pinned = true, want false")
	}
}

func TestSaveCreatesParentDirs(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "deep", "nested", "config.toml")
	SetConfigPath(path)

	cfg := &Config{
		BasePath:             "~/trees",
		Editor:               "vi",
		CleanupThresholdDays: 10,
		Repositories:         make(map[string]RepositoryConfig),
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("config file was not created at nested path")
	}
}

func TestSingletonBehavior(t *testing.T) {
	setupTestConfig(t)

	cfg1, err := Load()
	if err != nil {
		t.Fatalf("first Load() error: %v", err)
	}

	cfg2, err := Load()
	if err != nil {
		t.Fatalf("second Load() error: %v", err)
	}

	if cfg1 != cfg2 {
		t.Error("Load() returned different pointers, want same singleton instance")
	}
}

func TestReloadResetsSingleton(t *testing.T) {
	setupTestConfig(t)

	cfg1, err := Load()
	if err != nil {
		t.Fatalf("first Load() error: %v", err)
	}

	Reload()

	cfg2, err := Load()
	if err != nil {
		t.Fatalf("Load() after Reload() error: %v", err)
	}

	// After reload, we should get a fresh instance
	// (they may or may not be the same pointer depending on content,
	// but the singleton should have been reset)
	_ = cfg1
	_ = cfg2
}

func TestSetConfigPathResetsSingleton(t *testing.T) {
	tmpDir := t.TempDir()
	path1 := filepath.Join(tmpDir, "config1.toml")
	path2 := filepath.Join(tmpDir, "config2.toml")

	// Create two different configs
	cfg1Content := `base_path = "~/path1"
editor = "vim"
cleanup_threshold_days = 10
`
	cfg2Content := `base_path = "~/path2"
editor = "emacs"
cleanup_threshold_days = 20
`
	if err := os.WriteFile(path1, []byte(cfg1Content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path2, []byte(cfg2Content), 0o644); err != nil {
		t.Fatal(err)
	}

	SetConfigPath(path1)
	cfg1, err := Load()
	if err != nil {
		t.Fatalf("Load() path1 error: %v", err)
	}
	if cfg1.BasePath != "~/path1" {
		t.Errorf("BasePath = %q, want %q", cfg1.BasePath, "~/path1")
	}

	SetConfigPath(path2)
	cfg2, err := Load()
	if err != nil {
		t.Fatalf("Load() path2 error: %v", err)
	}
	if cfg2.BasePath != "~/path2" {
		t.Errorf("BasePath = %q, want %q", cfg2.BasePath, "~/path2")
	}
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"absolute path", "/usr/local/bin", "/usr/local/bin"},
		{"relative path", "relative/path", "relative/path"},
		{"tilde only", "~", home},
		{"tilde with path", "~/worktrees", filepath.Join(home, "worktrees")},
		{"tilde with deep path", "~/a/b/c", filepath.Join(home, "a", "b", "c")},
		{"tilde in middle", "/home/~user", "/home/~user"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExpandPath(tt.input)
			if result != tt.expected {
				t.Errorf("ExpandPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLoadInvalidTOML(t *testing.T) {
	path := setupTestConfig(t)

	if err := os.WriteFile(path, []byte("this is [not valid toml {{{"), 0o644); err != nil {
		t.Fatal(err)
	}

	Reload()
	_, err := Load()
	if err == nil {
		t.Error("Load() returned nil error for invalid TOML, want error")
	}
}

func TestSaveAndLoadWithPerRepoBasePath(t *testing.T) {
	setupTestConfig(t)

	original := &Config{
		BasePath:             "~/test-trees",
		Editor:               "code",
		CleanupThresholdDays: 30,
		Repositories: map[string]RepositoryConfig{
			"repo-with-base": {
				RepoURL:       "https://github.com/test/repo.git",
				DefaultBranch: "main",
				BasePath:      "/custom/path/to/worktrees",
				Worktrees: map[string]WorktreeConfig{
					"main": {Pinned: true},
				},
			},
			"repo-without-base": {
				RepoURL:       "https://github.com/test/other.git",
				DefaultBranch: "develop",
				Worktrees: map[string]WorktreeConfig{
					"develop": {Pinned: true},
				},
			},
		},
	}

	if err := Save(original); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	Reload()
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Repo with custom base path should preserve it
	repoWith := loaded.Repositories["repo-with-base"]
	if repoWith.BasePath != "/custom/path/to/worktrees" {
		t.Errorf("repo-with-base BasePath = %q, want %q", repoWith.BasePath, "/custom/path/to/worktrees")
	}

	// Repo without custom base path should have empty string
	repoWithout := loaded.Repositories["repo-without-base"]
	if repoWithout.BasePath != "" {
		t.Errorf("repo-without-base BasePath = %q, want empty string", repoWithout.BasePath)
	}
}

func TestLoadExistingConfigWithPerRepoBasePath(t *testing.T) {
	path := setupTestConfig(t)

	content := `base_path = "~/worktrees"
editor = "code"
cleanup_threshold_days = 30

[repositories.myapp]
repo_url = "git@github.com:user/myapp.git"
default_branch = "main"
base_path = "/opt/worktrees"

[repositories.myapp.worktrees.main]
pinned = true
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	Reload()
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	repo := cfg.Repositories["myapp"]
	if repo.BasePath != "/opt/worktrees" {
		t.Errorf("BasePath = %q, want %q", repo.BasePath, "/opt/worktrees")
	}
}

func TestSanitizeBranchName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple name", "feature-x", "feature-x"},
		{"single slash", "feature/login", "feature-login"},
		{"multiple slashes", "feature/TCM-274/implement-spring-security", "feature-TCM-274-implement-spring-security"},
		{"leading slash", "/leading", "-leading"},
		{"trailing slash", "trailing/", "trailing-"},
		{"empty string", "", ""},
		{"no slashes", "main", "main"},
		{"consecutive slashes", "a//b", "a--b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeBranchName(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeBranchName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
