package repository

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paulssonkalle/worktree/internal/config"
	"github.com/paulssonkalle/worktree/internal/testutil"
)

// setupTestEnv creates a temp directory structure with a config pointing to it,
// and a source git repo to clone from. Returns (basePath, sourceRepoPath).
func setupTestEnv(t *testing.T) (string, string) {
	t.Helper()
	tmpDir := t.TempDir()

	// Set up config
	configPath := filepath.Join(tmpDir, "config.toml")
	basePath := filepath.Join(tmpDir, "worktrees")
	config.SetConfigPath(configPath)
	config.SetStatePath(filepath.Join(tmpDir, "state.toml"))

	cfg := &config.Config{
		BasePath:             basePath,
		Editor:               "code",
		CleanupThresholdDays: 30,
	}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save config: %v", err)
	}

	st := &config.State{
		Repositories: make(map[string]config.RepositoryConfig),
	}
	if err := config.SaveState(st); err != nil {
		t.Fatalf("Save state: %v", err)
	}

	config.Reload()
	config.ReloadState()

	// Create a source git repo to clone from
	srcDir := filepath.Join(tmpDir, "source-repo")
	testutil.RunGit(t, "", "init", srcDir)
	testutil.RunGit(t, srcDir, "checkout", "-b", "main")
	testutil.WriteFile(t, filepath.Join(srcDir, "README.md"), "# Test repo")
	testutil.RunGit(t, srcDir, "add", ".")
	testutil.RunGit(t, srcDir, "commit", "-m", "initial commit")

	return basePath, srcDir
}

func TestAdd(t *testing.T) {
	basePath, srcDir := setupTestEnv(t)

	if err := Add("myrepo", srcDir, "", "", false); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	// Verify repository directory was created
	repoDir := filepath.Join(basePath, "myrepo")
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		t.Error("repository directory was not created")
	}

	// Verify bare repo exists
	bareDir := filepath.Join(repoDir, ".bare")
	if _, err := os.Stat(filepath.Join(bareDir, "HEAD")); os.IsNotExist(err) {
		t.Error("bare repo was not created")
	}

	// Verify default branch worktree was created
	mainWT := filepath.Join(repoDir, "main")
	if _, err := os.Stat(mainWT); os.IsNotExist(err) {
		t.Error("default branch worktree was not created")
	}

	// Verify config was updated
	config.Reload()
	config.ReloadState()
	st, err := config.LoadState()
	if err != nil {
		t.Fatalf("Load state: %v", err)
	}
	repo, exists := st.Repositories["myrepo"]
	if !exists {
		t.Fatal("repository not found in config after Add()")
	}
	if repo.RepoURL != srcDir {
		t.Errorf("RepoURL = %q, want %q", repo.RepoURL, srcDir)
	}
	if repo.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want %q", repo.DefaultBranch, "main")
	}

	// Verify default branch worktree is pinned
	wtCfg, exists := repo.Worktrees["main"]
	if !exists {
		t.Fatal("default branch worktree config not found")
	}
	if !wtCfg.Pinned {
		t.Error("default branch worktree Pinned = false, want true")
	}
}

func TestAddWithExplicitDefaultBranch(t *testing.T) {
	_, srcDir := setupTestEnv(t)

	if err := Add("myrepo", srcDir, "main", "", false); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	config.Reload()
	config.ReloadState()
	st, _ := config.LoadState()
	repo := st.Repositories["myrepo"]
	if repo.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want %q", repo.DefaultBranch, "main")
	}
}

func TestAddDuplicateRepo(t *testing.T) {
	_, srcDir := setupTestEnv(t)

	if err := Add("myrepo", srcDir, "", "", false); err != nil {
		t.Fatalf("first Add() error: %v", err)
	}

	err := Add("myrepo", srcDir, "", "", false)
	if err == nil {
		t.Error("second Add() returned nil error, want duplicate error")
	}
}

func TestAddInvalidRepo(t *testing.T) {
	setupTestEnv(t)

	err := Add("bad-repo", "/nonexistent/repo", "main", "", false)
	if err == nil {
		t.Error("Add() with invalid repo URL returned nil error, want error")
	}
}

func TestRemove(t *testing.T) {
	basePath, srcDir := setupTestEnv(t)

	if err := Add("myrepo", srcDir, "", "", false); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	// Remove without --force should fail
	err := Remove("myrepo", false)
	if err == nil {
		t.Error("Remove() without force returned nil error, want error")
	}

	// Remove with --force should succeed
	if err := Remove("myrepo", true); err != nil {
		t.Fatalf("Remove(force=true) error: %v", err)
	}

	// Verify directory was removed
	repoDir := filepath.Join(basePath, "myrepo")
	if _, err := os.Stat(repoDir); !os.IsNotExist(err) {
		t.Error("repository directory still exists after Remove()")
	}

	// Verify config was updated
	config.Reload()
	config.ReloadState()
	st, _ := config.LoadState()
	if _, exists := st.Repositories["myrepo"]; exists {
		t.Error("repository still in config after Remove()")
	}
}

func TestRemoveNonexistent(t *testing.T) {
	setupTestEnv(t)

	err := Remove("nonexistent", true)
	if err == nil {
		t.Error("Remove() of nonexistent repository returned nil error, want error")
	}
}

func TestList(t *testing.T) {
	_, srcDir := setupTestEnv(t)

	// Empty list initially
	names, err := List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("List() returned %d names, want 0", len(names))
	}

	// Add two repositories
	if err := Add("beta-repo", srcDir, "", "", false); err != nil {
		t.Fatalf("Add(beta) error: %v", err)
	}
	config.Reload()
	config.ReloadState()
	if err := Add("alpha-repo", srcDir, "", "", false); err != nil {
		t.Fatalf("Add(alpha) error: %v", err)
	}

	config.Reload()
	config.ReloadState()
	names, err = List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("List() returned %d names, want 2", len(names))
	}

	// Should be sorted alphabetically
	if names[0] != "alpha-repo" {
		t.Errorf("names[0] = %q, want %q", names[0], "alpha-repo")
	}
	if names[1] != "beta-repo" {
		t.Errorf("names[1] = %q, want %q", names[1], "beta-repo")
	}
}

func TestGet(t *testing.T) {
	_, srcDir := setupTestEnv(t)

	if err := Add("myrepo", srcDir, "", "", false); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	config.Reload()
	config.ReloadState()
	repo, err := Get("myrepo")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if repo.RepoURL != srcDir {
		t.Errorf("RepoURL = %q, want %q", repo.RepoURL, srcDir)
	}
	if repo.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want %q", repo.DefaultBranch, "main")
	}
}

func TestGetNonexistent(t *testing.T) {
	setupTestEnv(t)

	_, err := Get("nonexistent")
	if err == nil {
		t.Error("Get() of nonexistent repository returned nil error, want error")
	}
}

func TestAddWithCustomBasePath(t *testing.T) {
	_, srcDir := setupTestEnv(t)
	tmpDir := t.TempDir()
	customBase := filepath.Join(tmpDir, "custom-worktrees")

	if err := Add("custom-repo", srcDir, "", customBase, false); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	// Verify the repository was created under the custom base path
	repoDir := filepath.Join(customBase, "custom-repo")
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		t.Error("repository directory was not created under custom base path")
	}

	// Verify bare repo exists under custom base
	bareDir := filepath.Join(repoDir, ".bare")
	if _, err := os.Stat(filepath.Join(bareDir, "HEAD")); os.IsNotExist(err) {
		t.Error("bare repo was not created under custom base path")
	}

	// Verify default branch worktree was created under custom base
	mainWT := filepath.Join(repoDir, "main")
	if _, err := os.Stat(mainWT); os.IsNotExist(err) {
		t.Error("default branch worktree was not created under custom base path")
	}

	// Verify config stores the custom base path
	config.Reload()
	config.ReloadState()
	st, err := config.LoadState()
	if err != nil {
		t.Fatalf("Load state: %v", err)
	}
	repo, exists := st.Repositories["custom-repo"]
	if !exists {
		t.Fatal("repository not found in config after Add()")
	}
	if repo.BasePath != customBase {
		t.Errorf("BasePath = %q, want %q", repo.BasePath, customBase)
	}
}

func TestRepositoryDir(t *testing.T) {
	basePath, _ := setupTestEnv(t)

	config.Reload()
	config.ReloadState()
	dir := RepositoryDir("testrepo")
	expected := filepath.Join(basePath, "testrepo")
	if dir != expected {
		t.Errorf("RepositoryDir() = %q, want %q", dir, expected)
	}
}

func TestRepositoryDirWithPerRepoBasePath(t *testing.T) {
	_, _ = setupTestEnv(t)
	tmpDir := t.TempDir()
	customBase := filepath.Join(tmpDir, "per-repo-base")

	// Manually set up a repo config with a per-repo base path
	config.Reload()
	config.ReloadState()
	st, err := config.LoadState()
	if err != nil {
		t.Fatalf("Load state: %v", err)
	}
	st.Repositories["custom-base-repo"] = config.RepositoryConfig{
		RepoURL:       "https://example.com/repo.git",
		DefaultBranch: "main",
		BasePath:      customBase,
		Worktrees:     make(map[string]config.WorktreeConfig),
	}
	if err := config.SaveState(st); err != nil {
		t.Fatalf("Save state: %v", err)
	}
	config.ReloadState()

	dir := RepositoryDir("custom-base-repo")
	expected := filepath.Join(customBase, "custom-base-repo")
	if dir != expected {
		t.Errorf("RepositoryDir() = %q, want %q", dir, expected)
	}
}

func TestRepositoryDirFallsBackToGlobalBasePath(t *testing.T) {
	basePath, _ := setupTestEnv(t)

	// Repo with empty BasePath should use global
	config.Reload()
	config.ReloadState()
	st, err := config.LoadState()
	if err != nil {
		t.Fatalf("Load state: %v", err)
	}
	st.Repositories["no-base-repo"] = config.RepositoryConfig{
		RepoURL:       "https://example.com/repo.git",
		DefaultBranch: "main",
		BasePath:      "",
		Worktrees:     make(map[string]config.WorktreeConfig),
	}
	if err := config.SaveState(st); err != nil {
		t.Fatalf("Save state: %v", err)
	}
	config.ReloadState()

	dir := RepositoryDir("no-base-repo")
	expected := filepath.Join(basePath, "no-base-repo")
	if dir != expected {
		t.Errorf("RepositoryDir() = %q, want %q", dir, expected)
	}
}

func TestBareDir(t *testing.T) {
	basePath, _ := setupTestEnv(t)

	config.Reload()
	config.ReloadState()
	dir := BareDir("testrepo")
	expected := filepath.Join(basePath, "testrepo", ".bare")
	if dir != expected {
		t.Errorf("BareDir() = %q, want %q", dir, expected)
	}
}
