package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestState creates a temp dir, sets both config and state paths to files
// inside it, and returns both paths. Singletons are reset via SetConfigPath and
// SetStatePath so each test starts clean.
func setupTestState(t *testing.T) (configPath string, statePath string) {
	t.Helper()
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")
	stPath := filepath.Join(tmpDir, "state.toml")
	SetConfigPath(cfgPath)
	SetStatePath(stPath)
	return cfgPath, stPath
}

func TestLoadStateCreatesDefaultState(t *testing.T) {
	_, statePath := setupTestState(t)

	s, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState() error: %v", err)
	}

	if s.Repositories == nil {
		t.Error("Repositories is nil, want initialized map")
	}
	if len(s.Repositories) != 0 {
		t.Errorf("Repositories has %d entries, want 0", len(s.Repositories))
	}

	// State file should have been created on disk.
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Error("state file was not created on disk")
	}
}

func TestStateSaveAndLoad(t *testing.T) {
	_, _ = setupTestState(t)

	original := &State{
		Repositories: map[string]RepositoryConfig{
			"myapp": {
				RepoURL:       "git@github.com:user/myapp.git",
				DefaultBranch: "main",
				BasePath:      "/custom/path",
				Worktrees: map[string]WorktreeConfig{
					"feature-x": {Pinned: true},
					"bugfix-y":  {Pinned: false},
				},
			},
			"lib": {
				RepoURL:       "https://github.com/org/lib.git",
				DefaultBranch: "develop",
			},
		},
	}

	if err := SaveState(original); err != nil {
		t.Fatalf("SaveState() error: %v", err)
	}

	// Force fresh load from disk.
	ReloadState()
	loaded, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState() after reload error: %v", err)
	}

	if len(loaded.Repositories) != 2 {
		t.Fatalf("Repositories count = %d, want 2", len(loaded.Repositories))
	}

	repo, ok := loaded.Repositories["myapp"]
	if !ok {
		t.Fatal("repository 'myapp' not found after load")
	}
	if repo.RepoURL != "git@github.com:user/myapp.git" {
		t.Errorf("RepoURL = %q, want %q", repo.RepoURL, "git@github.com:user/myapp.git")
	}
	if repo.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want %q", repo.DefaultBranch, "main")
	}
	if repo.BasePath != "/custom/path" {
		t.Errorf("BasePath = %q, want %q", repo.BasePath, "/custom/path")
	}

	wt, ok := repo.Worktrees["feature-x"]
	if !ok {
		t.Fatal("worktree 'feature-x' not found")
	}
	if !wt.Pinned {
		t.Error("feature-x Pinned = false, want true")
	}

	wt2, ok := repo.Worktrees["bugfix-y"]
	if !ok {
		t.Fatal("worktree 'bugfix-y' not found")
	}
	if wt2.Pinned {
		t.Error("bugfix-y Pinned = true, want false")
	}

	lib, ok := loaded.Repositories["lib"]
	if !ok {
		t.Fatal("repository 'lib' not found after load")
	}
	if lib.DefaultBranch != "develop" {
		t.Errorf("lib DefaultBranch = %q, want %q", lib.DefaultBranch, "develop")
	}
}

func TestStateSaveCreatesParentDirs(t *testing.T) {
	tmpDir := t.TempDir()
	deepPath := filepath.Join(tmpDir, "a", "b", "c", "state.toml")
	SetConfigPath(filepath.Join(tmpDir, "config.toml"))
	SetStatePath(deepPath)

	s := &State{
		Repositories: make(map[string]RepositoryConfig),
	}
	if err := SaveState(s); err != nil {
		t.Fatalf("SaveState() error: %v", err)
	}

	if _, err := os.Stat(deepPath); os.IsNotExist(err) {
		t.Error("state file was not created at deeply nested path")
	}
}

func TestStateSingletonBehavior(t *testing.T) {
	_, _ = setupTestState(t)

	s1, err := LoadState()
	if err != nil {
		t.Fatalf("first LoadState() error: %v", err)
	}

	s2, err := LoadState()
	if err != nil {
		t.Fatalf("second LoadState() error: %v", err)
	}

	if s1 != s2 {
		t.Error("LoadState() returned different pointers, want same singleton instance")
	}
}

func TestStateReloadResetsSingleton(t *testing.T) {
	_, _ = setupTestState(t)

	s1, err := LoadState()
	if err != nil {
		t.Fatalf("first LoadState() error: %v", err)
	}

	ReloadState()

	s2, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState() after ReloadState() error: %v", err)
	}

	if s1 == s2 {
		t.Error("LoadState() returned same pointer after ReloadState(), want fresh instance")
	}
}

func TestSetStatePathResetsSingleton(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")
	path1 := filepath.Join(tmpDir, "state1.toml")
	path2 := filepath.Join(tmpDir, "state2.toml")

	SetConfigPath(cfgPath)

	// Write two different state files.
	state1 := &State{
		Repositories: map[string]RepositoryConfig{
			"repo1": {RepoURL: "url1", DefaultBranch: "main"},
		},
	}
	state2 := &State{
		Repositories: map[string]RepositoryConfig{
			"repo2": {RepoURL: "url2", DefaultBranch: "develop"},
		},
	}

	SetStatePath(path1)
	if err := SaveState(state1); err != nil {
		t.Fatalf("SaveState(state1) error: %v", err)
	}

	SetStatePath(path2)
	if err := SaveState(state2); err != nil {
		t.Fatalf("SaveState(state2) error: %v", err)
	}

	// Now load from path1.
	SetStatePath(path1)
	s1, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState() path1 error: %v", err)
	}
	if _, ok := s1.Repositories["repo1"]; !ok {
		t.Error("expected repo1 in state loaded from path1")
	}

	// Switch to path2 — SetStatePath should reset the singleton.
	SetStatePath(path2)
	s2, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState() path2 error: %v", err)
	}
	if _, ok := s2.Repositories["repo2"]; !ok {
		t.Error("expected repo2 in state loaded from path2")
	}
}

func TestMigrateFromOldConfig(t *testing.T) {
	cfgPath, statePath := setupTestState(t)

	// Write an old-format config.toml that has [repositories].
	oldConfig := `base_path = "~/worktrees"
editor = "nvim"
cleanup_threshold_days = 14
zoxide = true

[repositories.myapp]
repo_url = "git@github.com:user/myapp.git"
default_branch = "main"

[repositories.myapp.worktrees.feature-x]
pinned = true

[repositories.lib]
repo_url = "https://github.com/org/lib.git"
default_branch = "develop"
base_path = "/opt/worktrees"
`
	if err := os.WriteFile(cfgPath, []byte(oldConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	// No state.toml exists — LoadState should trigger migration.
	s, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState() error: %v", err)
	}

	// (a) state.toml should exist on disk.
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Error("state.toml was not created during migration")
	}

	// (b) config.toml should have been rewritten without repositories.
	cfgData, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("reading config.toml after migration: %v", err)
	}
	if strings.Contains(string(cfgData), "[repositories") {
		t.Error("config.toml still contains [repositories] after migration")
	}

	// (c) state should contain the migrated repo data.
	if len(s.Repositories) != 2 {
		t.Fatalf("state Repositories count = %d, want 2", len(s.Repositories))
	}

	myapp, ok := s.Repositories["myapp"]
	if !ok {
		t.Fatal("repository 'myapp' not found in state")
	}
	if myapp.RepoURL != "git@github.com:user/myapp.git" {
		t.Errorf("myapp RepoURL = %q, want %q", myapp.RepoURL, "git@github.com:user/myapp.git")
	}
	if myapp.DefaultBranch != "main" {
		t.Errorf("myapp DefaultBranch = %q, want %q", myapp.DefaultBranch, "main")
	}

	wt, ok := myapp.Worktrees["feature-x"]
	if !ok {
		t.Fatal("worktree 'feature-x' not found in migrated myapp")
	}
	if !wt.Pinned {
		t.Error("feature-x Pinned = false, want true")
	}

	lib, ok := s.Repositories["lib"]
	if !ok {
		t.Fatal("repository 'lib' not found in state")
	}
	if lib.BasePath != "/opt/worktrees" {
		t.Errorf("lib BasePath = %q, want %q", lib.BasePath, "/opt/worktrees")
	}
}

func TestMigratePreservesConfigSettings(t *testing.T) {
	cfgPath, _ := setupTestState(t)

	oldConfig := `base_path = "~/my-trees"
editor = "emacs"
cleanup_threshold_days = 7
zoxide = true

[repositories.myapp]
repo_url = "git@github.com:user/myapp.git"
default_branch = "main"
`
	if err := os.WriteFile(cfgPath, []byte(oldConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	// Trigger migration via LoadState.
	if _, err := LoadState(); err != nil {
		t.Fatalf("LoadState() error: %v", err)
	}

	// Reload config from the rewritten file.
	Reload()
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() after migration error: %v", err)
	}

	if cfg.BasePath != "~/my-trees" {
		t.Errorf("BasePath = %q, want %q", cfg.BasePath, "~/my-trees")
	}
	if cfg.Editor != "emacs" {
		t.Errorf("Editor = %q, want %q", cfg.Editor, "emacs")
	}
	if cfg.CleanupThresholdDays != 7 {
		t.Errorf("CleanupThresholdDays = %d, want %d", cfg.CleanupThresholdDays, 7)
	}
	if !cfg.Zoxide {
		t.Error("Zoxide = false, want true")
	}
}

func TestNoMigrationWhenConfigHasNoRepos(t *testing.T) {
	cfgPath, _ := setupTestState(t)

	configContent := `base_path = "~/worktrees"
editor = "code"
cleanup_threshold_days = 30
`
	if err := os.WriteFile(cfgPath, []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState() error: %v", err)
	}

	if len(s.Repositories) != 0 {
		t.Errorf("Repositories count = %d, want 0 (no migration expected)", len(s.Repositories))
	}
}

func TestLoadStateInvalidTOML(t *testing.T) {
	_, statePath := setupTestState(t)

	if err := os.WriteFile(statePath, []byte("this is [not valid toml {{{"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadState()
	if err == nil {
		t.Error("LoadState() returned nil error for invalid TOML, want error")
	}
}
