package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paulssonkalle/worktree-cli/internal/config"
	"github.com/paulssonkalle/worktree-cli/internal/git"
	"github.com/paulssonkalle/worktree-cli/internal/repository"
	"github.com/paulssonkalle/worktree-cli/internal/testutil"
	"github.com/paulssonkalle/worktree-cli/internal/worktree"
)

// setupIntegrationEnv creates a complete test environment: temp dirs for config
// and base path, and a source git repo. Returns (basePath, sourceRepoDir).
func setupIntegrationEnv(t *testing.T) (string, string) {
	t.Helper()
	tmpDir := t.TempDir()

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

	// Create a source git repo with multiple commits
	srcDir := filepath.Join(tmpDir, "source-repo")
	testutil.RunGit(t, "", "init", srcDir)
	testutil.RunGit(t, srcDir, "checkout", "-b", "main")
	testutil.WriteFile(t, filepath.Join(srcDir, "README.md"), "# Integration Test Project")
	testutil.RunGit(t, srcDir, "add", ".")
	testutil.RunGit(t, srcDir, "commit", "-m", "initial commit")

	testutil.WriteFile(t, filepath.Join(srcDir, "src", "main.go"), "package main\n\nfunc main() {}\n")
	testutil.RunGit(t, srcDir, "add", ".")
	testutil.RunGit(t, srcDir, "commit", "-m", "add main.go")

	return basePath, srcDir
}

// TestFullWorkflow exercises the complete lifecycle:
// 1. Add repository (bare clone + default branch worktree pinned)
// 2. Add feature worktree
// 3. Verify listing
// 4. Get status (clean and dirty)
// 5. Pin/unpin worktrees
// 6. Remove worktree
// 7. Remove repository
func TestFullWorkflow(t *testing.T) {
	basePath, srcDir := setupIntegrationEnv(t)

	// === 1. Add repository ===
	if err := repository.Add("webapp", srcDir, "", ""); err != nil {
		t.Fatalf("repository.Add() error: %v", err)
	}

	// Verify repository structure on disk
	repoDir := filepath.Join(basePath, "webapp")
	bareDir := filepath.Join(repoDir, ".bare")
	mainWT := filepath.Join(repoDir, "main")

	for _, dir := range []string{repoDir, bareDir, mainWT} {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Fatalf("expected directory %q to exist", dir)
		}
	}

	// Verify files from source repo are in the worktree
	readmePath := filepath.Join(mainWT, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		t.Error("README.md not found in main worktree")
	}
	mainGoPath := filepath.Join(mainWT, "src", "main.go")
	if _, err := os.Stat(mainGoPath); os.IsNotExist(err) {
		t.Error("src/main.go not found in main worktree")
	}

	// Verify config
	config.Reload()
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load config: %v", err)
	}
	repo := cfg.Repositories["webapp"]
	if repo.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want %q", repo.DefaultBranch, "main")
	}
	if !repo.Worktrees["main"].Pinned {
		t.Error("main worktree should be pinned after repository.Add()")
	}

	// === 2. Add feature worktree ===
	if err := worktree.Add("webapp", "feature-auth", ""); err != nil {
		t.Fatalf("worktree.Add() error: %v", err)
	}

	featureDir := filepath.Join(repoDir, "feature-auth")
	if _, err := os.Stat(featureDir); os.IsNotExist(err) {
		t.Error("feature worktree directory was not created")
	}

	// Feature worktree should also have the source files
	if _, err := os.Stat(filepath.Join(featureDir, "README.md")); os.IsNotExist(err) {
		t.Error("README.md not found in feature worktree")
	}

	// === 3. List worktrees ===
	config.Reload()
	infos, err := worktree.List("webapp")
	if err != nil {
		t.Fatalf("worktree.List() error: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("List() returned %d entries, want 2", len(infos))
	}

	// Should be sorted: feature-auth, main
	if infos[0].Name != "feature-auth" || infos[1].Name != "main" {
		t.Errorf("List() order: [%q, %q], want [feature-auth, main]",
			infos[0].Name, infos[1].Name)
	}

	// === 4. Status (clean) ===
	status, err := worktree.Status("webapp", "feature-auth")
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if status.Dirty {
		t.Error("feature-auth should be clean initially")
	}
	if status.Branch != "feature-auth" {
		t.Errorf("Status.Branch = %q, want %q", status.Branch, "feature-auth")
	}

	// === 4b. Status (dirty) ===
	testutil.WriteFile(t, filepath.Join(featureDir, "auth.go"), "package auth\n")
	status, err = worktree.Status("webapp", "feature-auth")
	if err != nil {
		t.Fatalf("Status() after dirtying error: %v", err)
	}
	if !status.Dirty {
		t.Error("feature-auth should be dirty after adding file")
	}
	if status.Files != 1 {
		t.Errorf("Files = %d, want 1", status.Files)
	}

	// === 5. Pin/Unpin ===
	if err := worktree.Pin("webapp", "feature-auth"); err != nil {
		t.Fatalf("Pin() error: %v", err)
	}
	config.Reload()
	cfg, _ = config.Load()
	if !cfg.Repositories["webapp"].Worktrees["feature-auth"].Pinned {
		t.Error("feature-auth should be pinned after Pin()")
	}

	if err := worktree.Unpin("webapp", "feature-auth"); err != nil {
		t.Fatalf("Unpin() error: %v", err)
	}
	config.Reload()
	cfg, _ = config.Load()
	if cfg.Repositories["webapp"].Worktrees["feature-auth"].Pinned {
		t.Error("feature-auth should be unpinned after Unpin()")
	}

	// === 6. Remove worktree ===
	if err := worktree.Remove("webapp", "feature-auth"); err != nil {
		t.Fatalf("worktree.Remove() error: %v", err)
	}
	if _, err := os.Stat(featureDir); !os.IsNotExist(err) {
		t.Error("feature worktree directory still exists after Remove()")
	}

	config.Reload()
	infos, _ = worktree.List("webapp")
	if len(infos) != 1 {
		t.Errorf("List() after removal returned %d entries, want 1", len(infos))
	}

	// === 7. Remove repository ===
	config.Reload()
	if err := repository.Remove("webapp", true); err != nil {
		t.Fatalf("repository.Remove() error: %v", err)
	}
	if _, err := os.Stat(repoDir); !os.IsNotExist(err) {
		t.Error("repository directory still exists after repository.Remove()")
	}

	config.Reload()
	cfg, _ = config.Load()
	if _, exists := cfg.Repositories["webapp"]; exists {
		t.Error("repository still in config after repository.Remove()")
	}
}

// TestMultipleReposIsolation verifies that multiple repositories don't interfere.
func TestMultipleReposIsolation(t *testing.T) {
	_, srcDir := setupIntegrationEnv(t)

	// Add two repositories from the same source
	if err := repository.Add("repo-alpha", srcDir, "", ""); err != nil {
		t.Fatalf("repository.Add(repo-alpha) error: %v", err)
	}
	config.Reload()
	if err := repository.Add("repo-beta", srcDir, "", ""); err != nil {
		t.Fatalf("repository.Add(repo-beta) error: %v", err)
	}
	config.Reload()

	// Add worktree to alpha only
	if err := worktree.Add("repo-alpha", "feature-x", ""); err != nil {
		t.Fatalf("worktree.Add(repo-alpha) error: %v", err)
	}
	config.Reload()

	// Alpha should have 2 worktrees, beta should have 1
	alphaInfos, _ := worktree.List("repo-alpha")
	betaInfos, _ := worktree.List("repo-beta")

	if len(alphaInfos) != 2 {
		t.Errorf("repo-alpha has %d worktrees, want 2", len(alphaInfos))
	}
	if len(betaInfos) != 1 {
		t.Errorf("repo-beta has %d worktrees, want 1", len(betaInfos))
	}

	// ListAll should show 3 total
	config.Reload()
	all, _ := worktree.ListAll()
	if len(all) != 3 {
		t.Errorf("ListAll() returned %d entries, want 3", len(all))
	}

	// Remove alpha should not affect beta
	config.Reload()
	if err := repository.Remove("repo-alpha", true); err != nil {
		t.Fatalf("repository.Remove(repo-alpha) error: %v", err)
	}
	config.Reload()

	betaInfos, _ = worktree.List("repo-beta")
	if len(betaInfos) != 1 {
		t.Errorf("repo-beta has %d worktrees after removing alpha, want 1", len(betaInfos))
	}
}

// TestCleanupIntegration verifies that cleanup correctly identifies and removes
// stale worktrees while preserving pinned ones.
func TestCleanupIntegration(t *testing.T) {
	_, srcDir := setupIntegrationEnv(t)

	if err := repository.Add("cleanup-test", srcDir, "", ""); err != nil {
		t.Fatalf("repository.Add() error: %v", err)
	}
	config.Reload()

	// Add two feature worktrees (unpinned)
	if err := worktree.Add("cleanup-test", "stale-feature", ""); err != nil {
		t.Fatalf("worktree.Add(stale) error: %v", err)
	}
	config.Reload()
	if err := worktree.Add("cleanup-test", "fresh-feature", ""); err != nil {
		t.Fatalf("worktree.Add(fresh) error: %v", err)
	}
	config.Reload()

	// Dry run with 0 days should find unpinned worktrees
	results, err := worktree.Cleanup(0, true, "cleanup-test")
	if err != nil {
		t.Fatalf("Cleanup(dryRun) error: %v", err)
	}

	var foundStale, foundFresh, foundMain bool
	for _, r := range results {
		switch r.Name {
		case "stale-feature":
			foundStale = true
		case "fresh-feature":
			foundFresh = true
		case "main":
			foundMain = true
		}
	}

	if !foundStale {
		t.Error("Cleanup dry run did not find 'stale-feature'")
	}
	if !foundFresh {
		t.Error("Cleanup dry run did not find 'fresh-feature'")
	}
	if foundMain {
		t.Error("Cleanup dry run should not include pinned 'main'")
	}

	for _, r := range results {
		if r.Removed {
			t.Errorf("dry run marked %q as removed", r.Name)
		}
	}
}

// TestRepoListSorted verifies that repository listing is alphabetically sorted.
func TestRepoListSorted(t *testing.T) {
	_, srcDir := setupIntegrationEnv(t)

	names := []string{"zulu", "alpha", "mike", "bravo"}
	for _, name := range names {
		config.Reload()
		if err := repository.Add(name, srcDir, "", ""); err != nil {
			t.Fatalf("repository.Add(%s) error: %v", name, err)
		}
	}

	config.Reload()
	listed, err := repository.List()
	if err != nil {
		t.Fatalf("repository.List() error: %v", err)
	}

	expected := []string{"alpha", "bravo", "mike", "zulu"}
	if len(listed) != len(expected) {
		t.Fatalf("List() returned %d items, want %d", len(listed), len(expected))
	}
	for i, name := range expected {
		if listed[i] != name {
			t.Errorf("listed[%d] = %q, want %q", i, listed[i], name)
		}
	}
}

// TestFetchIntegration verifies that fetching a repository picks up new branches
// created in the source repo after the initial clone.
func TestFetchIntegration(t *testing.T) {
	_, srcDir := setupIntegrationEnv(t)

	config.Reload()
	if err := repository.Add("fetch-test", srcDir, "", ""); err != nil {
		t.Fatalf("repository.Add() error: %v", err)
	}

	bareDir := repository.BareDir("fetch-test")

	// Initially should only have "main"
	branches, err := git.ListBranches(bareDir)
	if err != nil {
		t.Fatalf("ListBranches() error: %v", err)
	}
	if len(branches) != 1 || branches[0] != "main" {
		t.Fatalf("initial branches = %v, want [main]", branches)
	}

	// Create a new branch in the source repo
	testutil.RunGit(t, srcDir, "checkout", "-b", "feature-remote")
	testutil.WriteFile(t, filepath.Join(srcDir, "new-file.txt"), "new content")
	testutil.RunGit(t, srcDir, "add", ".")
	testutil.RunGit(t, srcDir, "commit", "-m", "add feature")
	testutil.RunGit(t, srcDir, "checkout", "main")

	// Before fetch, new branch should not be visible
	branches, err = git.ListBranches(bareDir)
	if err != nil {
		t.Fatalf("ListBranches() before fetch error: %v", err)
	}
	for _, b := range branches {
		if b == "feature-remote" {
			t.Fatal("feature-remote should not exist before fetch")
		}
	}

	// Fetch
	if err := git.Fetch(bareDir); err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}

	// After fetch, new branch should be visible
	branches, err = git.ListBranches(bareDir)
	if err != nil {
		t.Fatalf("ListBranches() after fetch error: %v", err)
	}
	found := false
	for _, b := range branches {
		if b == "feature-remote" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("feature-remote not found after fetch, got branches: %v", branches)
	}
}
