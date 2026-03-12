package repository

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/paulssonkalle/worktree-cli/internal/config"
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

	cfg := &config.Config{
		BasePath:             basePath,
		Editor:               "code",
		CleanupThresholdDays: 30,
		Repositories:         make(map[string]config.RepositoryConfig),
	}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save config: %v", err)
	}
	config.Reload()

	// Create a source git repo to clone from
	srcDir := filepath.Join(tmpDir, "source-repo")
	runGit(t, "", "init", srcDir)
	runGit(t, srcDir, "checkout", "-b", "main")
	writeFile(t, filepath.Join(srcDir, "README.md"), "# Test repo")
	runGit(t, srcDir, "add", ".")
	runGit(t, srcDir, "commit", "-m", "initial commit")

	return basePath, srcDir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %s\n%s", args, err, string(out))
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAdd(t *testing.T) {
	basePath, srcDir := setupTestEnv(t)

	if err := Add("myrepo", srcDir, "", ""); err != nil {
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
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load config: %v", err)
	}
	repo, exists := cfg.Repositories["myrepo"]
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

	if err := Add("myrepo", srcDir, "main", ""); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	config.Reload()
	cfg, _ := config.Load()
	repo := cfg.Repositories["myrepo"]
	if repo.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want %q", repo.DefaultBranch, "main")
	}
}

func TestAddDuplicateRepo(t *testing.T) {
	_, srcDir := setupTestEnv(t)

	if err := Add("myrepo", srcDir, "", ""); err != nil {
		t.Fatalf("first Add() error: %v", err)
	}

	err := Add("myrepo", srcDir, "", "")
	if err == nil {
		t.Error("second Add() returned nil error, want duplicate error")
	}
}

func TestAddInvalidRepo(t *testing.T) {
	setupTestEnv(t)

	err := Add("bad-repo", "/nonexistent/repo", "main", "")
	if err == nil {
		t.Error("Add() with invalid repo URL returned nil error, want error")
	}
}

func TestRemove(t *testing.T) {
	basePath, srcDir := setupTestEnv(t)

	if err := Add("myrepo", srcDir, "", ""); err != nil {
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
	cfg, _ := config.Load()
	if _, exists := cfg.Repositories["myrepo"]; exists {
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
	if err := Add("beta-repo", srcDir, "", ""); err != nil {
		t.Fatalf("Add(beta) error: %v", err)
	}
	config.Reload()
	if err := Add("alpha-repo", srcDir, "", ""); err != nil {
		t.Fatalf("Add(alpha) error: %v", err)
	}

	config.Reload()
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

	if err := Add("myrepo", srcDir, "", ""); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	config.Reload()
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

	if err := Add("custom-repo", srcDir, "", customBase); err != nil {
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
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load config: %v", err)
	}
	repo, exists := cfg.Repositories["custom-repo"]
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
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load config: %v", err)
	}
	cfg.Repositories["custom-base-repo"] = config.RepositoryConfig{
		RepoURL:       "https://example.com/repo.git",
		DefaultBranch: "main",
		BasePath:      customBase,
		Worktrees:     make(map[string]config.WorktreeConfig),
	}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save config: %v", err)
	}
	config.Reload()

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
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load config: %v", err)
	}
	cfg.Repositories["no-base-repo"] = config.RepositoryConfig{
		RepoURL:       "https://example.com/repo.git",
		DefaultBranch: "main",
		BasePath:      "",
		Worktrees:     make(map[string]config.WorktreeConfig),
	}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save config: %v", err)
	}
	config.Reload()

	dir := RepositoryDir("no-base-repo")
	expected := filepath.Join(basePath, "no-base-repo")
	if dir != expected {
		t.Errorf("RepositoryDir() = %q, want %q", dir, expected)
	}
}

func TestBareDir(t *testing.T) {
	basePath, _ := setupTestEnv(t)

	config.Reload()
	dir := BareDir("testrepo")
	expected := filepath.Join(basePath, "testrepo", ".bare")
	if dir != expected {
		t.Errorf("BareDir() = %q, want %q", dir, expected)
	}
}
