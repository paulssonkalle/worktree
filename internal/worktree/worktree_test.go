package worktree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paulssonkalle/worktree/internal/config"
	"github.com/paulssonkalle/worktree/internal/git"
	"github.com/paulssonkalle/worktree/internal/repository"
	"github.com/paulssonkalle/worktree/internal/testutil"
)

// setupTestEnv creates a temp directory with a config, a source repo, and adds
// a repository via repository.Add(). Returns (basePath, repoName).
// The repository will have a "main" branch worktree already created and pinned.
func setupTestEnv(t *testing.T) (string, string) {
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

	// Create a source git repo
	srcDir := filepath.Join(tmpDir, "source-repo")
	testutil.RunGit(t, "", "init", srcDir)
	testutil.RunGit(t, srcDir, "checkout", "-b", "main")
	testutil.WriteFile(t, filepath.Join(srcDir, "README.md"), "# Test project")
	testutil.RunGit(t, srcDir, "add", ".")
	testutil.RunGit(t, srcDir, "commit", "-m", "initial commit")

	// Add the repository (this clones bare + creates "main" worktree pinned)
	if err := repository.Add("testproj", srcDir, "", "", true); err != nil {
		t.Fatalf("repository.Add() error: %v", err)
	}
	config.Reload()

	return basePath, "testproj"
}

func TestAdd(t *testing.T) {
	basePath, repoName := setupTestEnv(t)

	if err := Add(repoName, "feature-x", AddOptions{NoSymlinks: true}); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	// Verify worktree directory was created
	wtDir := filepath.Join(basePath, repoName, "feature-x")
	if _, err := os.Stat(wtDir); os.IsNotExist(err) {
		t.Error("worktree directory was not created")
	}

	// Verify config was updated
	config.Reload()
	cfg, _ := config.Load()
	repo := cfg.Repositories[repoName]
	if _, exists := repo.Worktrees["feature-x"]; !exists {
		t.Error("worktree not found in config after Add()")
	}
}

func TestAddExistingBranch(t *testing.T) {
	_, repoName := setupTestEnv(t)

	// "main" branch already exists. Create a worktree for a new branch, then
	// remove the worktree with KeepBranch (preserving the branch), then re-add using existing branch.
	if err := Add(repoName, "reuse-branch", AddOptions{NoSymlinks: true}); err != nil {
		t.Fatalf("first Add() error: %v", err)
	}

	// Remove it
	if err := RemoveWithOptions(repoName, "reuse-branch", RemoveOptions{KeepBranch: true}); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	config.Reload()

	// Re-add - the branch "reuse-branch" now exists locally
	if err := Add(repoName, "reuse-branch", AddOptions{NoSymlinks: true}); err != nil {
		t.Fatalf("second Add() (existing branch) error: %v", err)
	}
}

func TestAddDuplicateWorktree(t *testing.T) {
	_, repoName := setupTestEnv(t)

	if err := Add(repoName, "feature-dup", AddOptions{NoSymlinks: true}); err != nil {
		t.Fatalf("first Add() error: %v", err)
	}

	err := Add(repoName, "feature-dup", AddOptions{NoSymlinks: true})
	if err == nil {
		t.Error("second Add() with same name returned nil error, want error")
	}
}

func TestAddNonexistentRepo(t *testing.T) {
	setupTestEnv(t)

	err := Add("nonexistent", "some-branch", AddOptions{NoSymlinks: true})
	if err == nil {
		t.Error("Add() for nonexistent repository returned nil error, want error")
	}
}

func TestAddWithBaseBranch(t *testing.T) {
	basePath, repoName := setupTestEnv(t)
	bareDir := repository.BareDir(repoName)
	mainWT := filepath.Join(basePath, repoName, "main")

	// Create a feature branch with an extra commit so it diverges from main.
	if err := Add(repoName, "feature-base", AddOptions{NoSymlinks: true}); err != nil {
		t.Fatalf("Add(feature-base) error: %v", err)
	}
	config.Reload()

	featureDir := filepath.Join(basePath, repoName, "feature-base")
	testutil.WriteFile(t, filepath.Join(featureDir, "extra.txt"), "diverged")
	testutil.RunGit(t, featureDir, "add", ".")
	testutil.RunGit(t, featureDir, "commit", "-m", "diverge from main")

	// Get the commit hashes so we can verify later.
	featureHead := strings.TrimSpace(testutil.RunGit(t, featureDir, "rev-parse", "HEAD"))
	mainHead := strings.TrimSpace(testutil.RunGit(t, mainWT, "rev-parse", "HEAD"))

	if featureHead == mainHead {
		t.Fatal("feature-base should have diverged from main, but commits are the same")
	}

	// Create a new branch based on feature-base (local branch).
	if err := Add(repoName, "child-of-feature", AddOptions{
		BaseBranch: "feature-base",
		NoSymlinks: true,
	}); err != nil {
		t.Fatalf("Add(child-of-feature) error: %v", err)
	}
	config.Reload()

	childDir := filepath.Join(basePath, repoName, "child-of-feature")
	childHead := strings.TrimSpace(testutil.RunGit(t, childDir, "rev-parse", "HEAD"))

	// child-of-feature should be based on feature-base's tip, not main.
	if childHead != featureHead {
		t.Errorf("child-of-feature HEAD = %s, want %s (feature-base tip); got main tip instead = %s",
			childHead, featureHead, mainHead)
	}

	// Verify the branch exists in the bare repo.
	if !git.BranchExists(bareDir, "child-of-feature") {
		t.Error("child-of-feature branch not found in bare repo")
	}
}

func TestAddWithBaseBranchRemoteOnly(t *testing.T) {
	// Set up manually to access srcDir for creating a remote-only branch.
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

	srcDir := filepath.Join(tmpDir, "source-repo")
	testutil.RunGit(t, "", "init", srcDir)
	testutil.RunGit(t, srcDir, "checkout", "-b", "main")
	testutil.WriteFile(t, filepath.Join(srcDir, "README.md"), "# Test")
	testutil.RunGit(t, srcDir, "add", ".")
	testutil.RunGit(t, srcDir, "commit", "-m", "initial commit")

	// Create a feature branch in the source repo with an extra commit.
	testutil.RunGit(t, srcDir, "checkout", "-b", "feature/remote-base")
	testutil.WriteFile(t, filepath.Join(srcDir, "remote.txt"), "remote content")
	testutil.RunGit(t, srcDir, "add", ".")
	testutil.RunGit(t, srcDir, "commit", "-m", "remote feature commit")
	remoteFeatureHead := strings.TrimSpace(testutil.RunGit(t, srcDir, "rev-parse", "HEAD"))
	testutil.RunGit(t, srcDir, "checkout", "main")

	repoName := "testproj"
	if err := repository.Add(repoName, srcDir, "", "", true); err != nil {
		t.Fatalf("repository.Add() error: %v", err)
	}
	config.Reload()

	// feature/remote-base only exists on origin, not locally.
	// Create a new branch based on it.
	if err := Add(repoName, "child-of-remote", AddOptions{
		BaseBranch: "feature/remote-base",
		NoSymlinks: true,
	}); err != nil {
		t.Fatalf("Add(child-of-remote) error: %v", err)
	}

	childDir := filepath.Join(basePath, repoName, "child-of-remote")
	childHead := strings.TrimSpace(testutil.RunGit(t, childDir, "rev-parse", "HEAD"))

	if childHead != remoteFeatureHead {
		t.Errorf("child-of-remote HEAD = %s, want %s (remote feature tip)", childHead, remoteFeatureHead)
	}
}

func TestAddWithBaseBranchNotFound(t *testing.T) {
	setupTestEnv(t)

	err := Add("testproj", "new-branch", AddOptions{
		BaseBranch: "nonexistent-base",
		NoSymlinks: true,
	})
	if err == nil {
		t.Fatal("Add() with nonexistent base branch returned nil error, want error")
	}
	if !strings.Contains(err.Error(), "not found locally or on remote") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "not found locally or on remote")
	}
}

func TestAddRemoteOnlyBranch(t *testing.T) {
	// Set up the environment manually so we can access srcDir.
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

	srcDir := filepath.Join(tmpDir, "source-repo")
	testutil.RunGit(t, "", "init", srcDir)
	testutil.RunGit(t, srcDir, "checkout", "-b", "main")
	testutil.WriteFile(t, filepath.Join(srcDir, "README.md"), "# Test project")
	testutil.RunGit(t, srcDir, "add", ".")
	testutil.RunGit(t, srcDir, "commit", "-m", "initial commit")

	repoName := "testproj"
	if err := repository.Add(repoName, srcDir, "", "", true); err != nil {
		t.Fatalf("repository.Add() error: %v", err)
	}
	config.Reload()

	// Create a branch in the source repo (acts as remote) that does not
	// exist locally in the bare clone.
	testutil.RunGit(t, srcDir, "checkout", "-b", "feature/remote-only")
	testutil.WriteFile(t, filepath.Join(srcDir, "remote-file.txt"), "remote content")
	testutil.RunGit(t, srcDir, "add", ".")
	testutil.RunGit(t, srcDir, "commit", "-m", "remote only commit")
	testutil.RunGit(t, srcDir, "checkout", "main")

	// Add the worktree - this should fetch and check out the remote branch
	if err := Add(repoName, "feature/remote-only", AddOptions{NoSymlinks: true}); err != nil {
		t.Fatalf("Add() error for remote-only branch: %v", err)
	}

	// Verify the worktree directory was created with sanitized name
	wtDir := filepath.Join(basePath, repoName, "feature-remote-only")
	if _, err := os.Stat(wtDir); os.IsNotExist(err) {
		t.Error("worktree directory was not created for remote-only branch")
	}

	// Verify the file from the remote branch is present
	remoteFile := filepath.Join(wtDir, "remote-file.txt")
	if _, err := os.Stat(remoteFile); os.IsNotExist(err) {
		t.Error("remote-file.txt not found - branch content was not checked out from remote")
	}

	// Verify config was updated with sanitized name
	config.Reload()
	cfg, _ = config.Load()
	repo := cfg.Repositories[repoName]
	if _, exists := repo.Worktrees["feature-remote-only"]; !exists {
		t.Error("worktree not found in config after Add() with remote-only branch")
	}
}

func TestRemove(t *testing.T) {
	basePath, repoName := setupTestEnv(t)

	if err := Add(repoName, "to-remove", AddOptions{NoSymlinks: true}); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	if err := Remove(repoName, "to-remove"); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	// Verify directory is gone
	wtDir := filepath.Join(basePath, repoName, "to-remove")
	if _, err := os.Stat(wtDir); !os.IsNotExist(err) {
		t.Error("worktree directory still exists after Remove()")
	}

	// Verify config was updated
	config.Reload()
	cfg, _ := config.Load()
	repo := cfg.Repositories[repoName]
	if _, exists := repo.Worktrees["to-remove"]; exists {
		t.Error("worktree still in config after Remove()")
	}

	// Verify branch was deleted (clean branch should be deleted by default)
	bareDir := repository.BareDir(repoName)
	if git.BranchExists(bareDir, "to-remove") {
		t.Error("branch still exists after Remove() of clean worktree, want deleted")
	}
}

func TestRemoveNonexistentRepo(t *testing.T) {
	setupTestEnv(t)

	err := Remove("nonexistent", "some-wt")
	if err == nil {
		t.Error("Remove() for nonexistent repository returned nil error, want error")
	}
}

func TestRemoveKeepsBranchWhenDirty(t *testing.T) {
	basePath, repoName := setupTestEnv(t)
	bareDir := repository.BareDir(repoName)

	if err := Add(repoName, "dirty-branch", AddOptions{NoSymlinks: true}); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	config.Reload()

	// Make the worktree dirty with uncommitted changes
	testutil.WriteFile(t, filepath.Join(basePath, repoName, "dirty-branch", "uncommitted.txt"), "dirty")

	if err := Remove(repoName, "dirty-branch"); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	// Branch should still exist because it had uncommitted changes
	if !git.BranchExists(bareDir, "dirty-branch") {
		t.Error("branch was deleted despite having uncommitted changes, want kept")
	}
}

func TestRemoveKeepsBranchWithUnpushedCommits(t *testing.T) {
	basePath, repoName := setupTestEnv(t)
	bareDir := repository.BareDir(repoName)

	if err := Add(repoName, "unpushed-branch", AddOptions{NoSymlinks: true}); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	config.Reload()

	// Create a commit that is not pushed (ahead of upstream)
	wtDir := filepath.Join(basePath, repoName, "unpushed-branch")
	testutil.WriteFile(t, filepath.Join(wtDir, "new-file.txt"), "content")
	testutil.RunGit(t, wtDir, "add", ".")
	testutil.RunGit(t, wtDir, "commit", "-m", "unpushed commit")

	if err := Remove(repoName, "unpushed-branch"); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	// Branch should still exist because it has unpushed commits
	if !git.BranchExists(bareDir, "unpushed-branch") {
		t.Error("branch was deleted despite having unpushed commits, want kept")
	}
}

func TestRemoveKeepsBranchFlag(t *testing.T) {
	_, repoName := setupTestEnv(t)
	bareDir := repository.BareDir(repoName)

	if err := Add(repoName, "keep-me", AddOptions{NoSymlinks: true}); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	config.Reload()

	if err := RemoveWithOptions(repoName, "keep-me", RemoveOptions{KeepBranch: true}); err != nil {
		t.Fatalf("RemoveWithOptions() error: %v", err)
	}

	// Branch should still exist because KeepBranch was set
	if !git.BranchExists(bareDir, "keep-me") {
		t.Error("branch was deleted despite KeepBranch=true, want kept")
	}
}

func TestRemoveForceBranchDeletesDirty(t *testing.T) {
	basePath, repoName := setupTestEnv(t)
	bareDir := repository.BareDir(repoName)

	if err := Add(repoName, "force-delete", AddOptions{NoSymlinks: true}); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	config.Reload()

	// Make the worktree dirty
	testutil.WriteFile(t, filepath.Join(basePath, repoName, "force-delete", "uncommitted.txt"), "dirty")

	if err := RemoveWithOptions(repoName, "force-delete", RemoveOptions{ForceBranch: true}); err != nil {
		t.Fatalf("RemoveWithOptions() error: %v", err)
	}

	// Branch should be deleted because ForceBranch was set
	if git.BranchExists(bareDir, "force-delete") {
		t.Error("branch still exists despite ForceBranch=true, want deleted")
	}
}

func TestRemoveNeverDeletesDefaultBranch(t *testing.T) {
	_, repoName := setupTestEnv(t)
	bareDir := repository.BareDir(repoName)

	// Unpin main so it can be removed
	if err := Unpin(repoName, "main"); err != nil {
		t.Fatalf("Unpin() error: %v", err)
	}
	config.Reload()

	if err := RemoveWithOptions(repoName, "main", RemoveOptions{ForceBranch: true}); err != nil {
		t.Fatalf("RemoveWithOptions() error: %v", err)
	}

	// Default branch should never be deleted even with ForceBranch
	if !git.BranchExists(bareDir, "main") {
		t.Error("default branch was deleted, should never be deleted")
	}
}

func TestList(t *testing.T) {
	_, repoName := setupTestEnv(t)

	// Initially should have the "main" worktree from repository.Add()
	infos, err := List(repoName)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("List() returned %d entries, want 1 (main)", len(infos))
	}
	if infos[0].Name != "main" {
		t.Errorf("infos[0].Name = %q, want %q", infos[0].Name, "main")
	}
	if !infos[0].Pinned {
		t.Error("main worktree should be pinned")
	}

	// Add another worktree
	if err := Add(repoName, "feature-a", AddOptions{NoSymlinks: true}); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	config.Reload()
	infos, err = List(repoName)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("List() returned %d entries, want 2", len(infos))
	}

	// Should be sorted by name
	if infos[0].Name != "feature-a" {
		t.Errorf("infos[0].Name = %q, want %q", infos[0].Name, "feature-a")
	}
	if infos[1].Name != "main" {
		t.Errorf("infos[1].Name = %q, want %q", infos[1].Name, "main")
	}
}

func TestListNonexistentRepo(t *testing.T) {
	setupTestEnv(t)

	_, err := List("nonexistent")
	if err == nil {
		t.Error("List() for nonexistent repository returned nil error, want error")
	}
}

func TestListAll(t *testing.T) {
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

	// Create source repo
	srcDir := filepath.Join(tmpDir, "source-repo")
	testutil.RunGit(t, "", "init", srcDir)
	testutil.RunGit(t, srcDir, "checkout", "-b", "main")
	testutil.WriteFile(t, filepath.Join(srcDir, "README.md"), "test")
	testutil.RunGit(t, srcDir, "add", ".")
	testutil.RunGit(t, srcDir, "commit", "-m", "init")

	// Add two repositories
	if err := repository.Add("repo-a", srcDir, "", "", true); err != nil {
		t.Fatalf("repository.Add(repo-a) error: %v", err)
	}
	config.Reload()
	if err := repository.Add("repo-b", srcDir, "", "", true); err != nil {
		t.Fatalf("repository.Add(repo-b) error: %v", err)
	}
	config.Reload()

	infos, err := ListAll()
	if err != nil {
		t.Fatalf("ListAll() error: %v", err)
	}

	// Each repository has a "main" worktree
	if len(infos) != 2 {
		t.Fatalf("ListAll() returned %d entries, want 2", len(infos))
	}
}

func TestPin(t *testing.T) {
	_, repoName := setupTestEnv(t)

	// Add an unpinned worktree
	if err := Add(repoName, "feature-pin", AddOptions{NoSymlinks: true}); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	config.Reload()

	// Verify it starts unpinned
	cfg, _ := config.Load()
	wt := cfg.Repositories[repoName].Worktrees["feature-pin"]
	if wt.Pinned {
		t.Error("new worktree should start unpinned")
	}

	// Pin it
	if err := Pin(repoName, "feature-pin"); err != nil {
		t.Fatalf("Pin() error: %v", err)
	}

	config.Reload()
	cfg, _ = config.Load()
	wt = cfg.Repositories[repoName].Worktrees["feature-pin"]
	if !wt.Pinned {
		t.Error("Pinned = false after Pin(), want true")
	}
}

func TestUnpin(t *testing.T) {
	_, repoName := setupTestEnv(t)

	// "main" starts pinned
	if err := Unpin(repoName, "main"); err != nil {
		t.Fatalf("Unpin() error: %v", err)
	}

	config.Reload()
	cfg, _ := config.Load()
	wt := cfg.Repositories[repoName].Worktrees["main"]
	if wt.Pinned {
		t.Error("Pinned = true after Unpin(), want false")
	}
}

func TestPinNonexistentRepo(t *testing.T) {
	setupTestEnv(t)

	err := Pin("nonexistent", "wt")
	if err == nil {
		t.Error("Pin() for nonexistent repository returned nil error, want error")
	}
}

func TestPinNonexistentWorktree(t *testing.T) {
	_, repoName := setupTestEnv(t)

	err := Pin(repoName, "nonexistent-wt")
	if err == nil {
		t.Error("Pin() for nonexistent worktree returned nil error, want error")
	}
}

func TestStatus(t *testing.T) {
	basePath, repoName := setupTestEnv(t)

	info, err := Status(repoName, "main")
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}

	if info.Repo != repoName {
		t.Errorf("Repo = %q, want %q", info.Repo, repoName)
	}
	if info.Name != "main" {
		t.Errorf("Name = %q, want %q", info.Name, "main")
	}
	if info.Branch != "main" {
		t.Errorf("Branch = %q, want %q", info.Branch, "main")
	}
	if !info.Pinned {
		t.Error("main worktree Pinned = false, want true")
	}
	if info.Dirty {
		t.Error("Dirty = true for clean worktree, want false")
	}

	// Make it dirty
	testutil.WriteFile(t, filepath.Join(basePath, repoName, "main", "dirty.txt"), "dirty")

	info, err = Status(repoName, "main")
	if err != nil {
		t.Fatalf("Status() after dirtying error: %v", err)
	}
	if !info.Dirty {
		t.Error("Dirty = false after adding file, want true")
	}
	if info.Files != 1 {
		t.Errorf("Files = %d, want 1", info.Files)
	}
}

func TestStatusNonexistentRepo(t *testing.T) {
	setupTestEnv(t)

	_, err := Status("nonexistent", "main")
	if err == nil {
		t.Error("Status() for nonexistent repository returned nil error, want error")
	}
}

func TestStatusNonexistentWorktree(t *testing.T) {
	_, repoName := setupTestEnv(t)

	_, err := Status(repoName, "nonexistent-wt")
	if err == nil {
		t.Error("Status() for nonexistent worktree returned nil error, want error")
	}
}

func TestCleanup(t *testing.T) {
	_, repoName := setupTestEnv(t)

	// Add an unpinned worktree
	if err := Add(repoName, "old-feature", AddOptions{NoSymlinks: true}); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	config.Reload()

	// Cleanup with 0 days threshold should catch everything unpinned
	results, err := Cleanup(0, true, repoName)
	if err != nil {
		t.Fatalf("Cleanup() error: %v", err)
	}

	// Should find "old-feature" but NOT "main" (pinned)
	var foundOld bool
	for _, r := range results {
		if r.Name == "old-feature" {
			foundOld = true
		}
		if r.Name == "main" {
			t.Error("Cleanup reported pinned 'main' worktree, should be skipped")
		}
	}
	if !foundOld {
		t.Error("Cleanup did not find 'old-feature' worktree")
	}
}

func TestCleanupDryRunDoesNotRemove(t *testing.T) {
	basePath, repoName := setupTestEnv(t)

	if err := Add(repoName, "dry-run-wt", AddOptions{NoSymlinks: true}); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	config.Reload()

	results, err := Cleanup(0, true, repoName)
	if err != nil {
		t.Fatalf("Cleanup(dryRun=true) error: %v", err)
	}

	// Should report but not remove
	for _, r := range results {
		if r.Removed {
			t.Error("dry run should not remove worktrees")
		}
	}

	// Directory should still exist
	wtDir := filepath.Join(basePath, repoName, "dry-run-wt")
	if _, err := os.Stat(wtDir); os.IsNotExist(err) {
		t.Error("worktree directory was removed during dry run")
	}
}

func TestCleanupActualRemoval(t *testing.T) {
	basePath, repoName := setupTestEnv(t)

	if err := Add(repoName, "remove-me", AddOptions{NoSymlinks: true}); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	config.Reload()

	// Set the last modified time to old by touching a file in the past
	wtDir := filepath.Join(basePath, repoName, "remove-me")
	oldTime := time.Now().Add(-60 * 24 * time.Hour)
	filepath.Walk(wtDir, func(path string, info os.FileInfo, err error) error {
		if err == nil {
			os.Chtimes(path, oldTime, oldTime)
		}
		return nil
	})

	results, err := Cleanup(1, false, repoName)
	if err != nil {
		t.Fatalf("Cleanup(dryRun=false) error: %v", err)
	}

	var foundRemoved bool
	for _, r := range results {
		if r.Name == "remove-me" && r.Removed {
			foundRemoved = true
		}
	}
	if !foundRemoved {
		t.Error("Cleanup did not remove 'remove-me' worktree")
	}
}

func TestCleanupNonexistentRepo(t *testing.T) {
	setupTestEnv(t)

	_, err := Cleanup(30, true, "nonexistent")
	if err == nil {
		t.Error("Cleanup() for nonexistent repository returned nil error, want error")
	}
}

func TestCleanupSkipsPinned(t *testing.T) {
	_, repoName := setupTestEnv(t)

	// Only "main" exists and it's pinned
	results, err := Cleanup(0, true, repoName)
	if err != nil {
		t.Fatalf("Cleanup() error: %v", err)
	}

	for _, r := range results {
		if r.Name == "main" {
			t.Error("Cleanup should skip pinned 'main' worktree")
		}
	}
}

func TestWorktreeDir(t *testing.T) {
	basePath, repoName := setupTestEnv(t)

	dir := WorktreeDir(repoName, "feature-x")
	expected := filepath.Join(basePath, repoName, "feature-x")
	if dir != expected {
		t.Errorf("WorktreeDir() = %q, want %q", dir, expected)
	}
}

func TestWorktreeDirSanitizesSlashes(t *testing.T) {
	basePath, repoName := setupTestEnv(t)

	tests := []struct {
		name       string
		branchName string
		wantDir    string
	}{
		{
			name:       "single slash",
			branchName: "feature/login",
			wantDir:    "feature-login",
		},
		{
			name:       "multiple slashes",
			branchName: "feature/TCM-274/implement-spring-security",
			wantDir:    "feature-TCM-274-implement-spring-security",
		},
		{
			name:       "no slashes unchanged",
			branchName: "main",
			wantDir:    "main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := WorktreeDir(repoName, tt.branchName)
			expected := filepath.Join(basePath, repoName, tt.wantDir)
			if dir != expected {
				t.Errorf("WorktreeDir(%q, %q) = %q, want %q", repoName, tt.branchName, dir, expected)
			}
		})
	}
}

func TestPruneSkipsPinnedAndDefaultBranch(t *testing.T) {
	_, repoName := setupTestEnv(t)

	// "main" is pinned and is the default branch - should never appear in prune results
	results, err := Prune(repoName, true)
	if err != nil {
		t.Fatalf("Prune() error: %v", err)
	}

	for _, r := range results {
		if r.Name == "main" {
			t.Error("Prune should skip pinned/default 'main' worktree")
		}
	}
}

func TestPruneNonexistentRepo(t *testing.T) {
	setupTestEnv(t)

	_, err := Prune("nonexistent", true)
	if err == nil {
		t.Error("Prune() for nonexistent repository returned nil error, want error")
	}
}

func TestRename(t *testing.T) {
	basePath, repoName := setupTestEnv(t)

	if err := Add(repoName, "feature-x", AddOptions{NoSymlinks: true}); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	config.Reload()

	if err := Rename(repoName, "feature-x", "feature-y"); err != nil {
		t.Fatalf("Rename() error: %v", err)
	}

	// Verify old directory is gone
	oldDir := filepath.Join(basePath, repoName, "feature-x")
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Error("old worktree directory still exists after Rename()")
	}

	// Verify new directory exists
	newDir := filepath.Join(basePath, repoName, "feature-y")
	if _, err := os.Stat(newDir); os.IsNotExist(err) {
		t.Error("new worktree directory was not created after Rename()")
	}

	// Verify config updated
	config.Reload()
	cfg, _ := config.Load()
	repo := cfg.Repositories[repoName]
	if _, exists := repo.Worktrees["feature-x"]; exists {
		t.Error("old worktree key still in config after Rename()")
	}
	if _, exists := repo.Worktrees["feature-y"]; !exists {
		t.Error("new worktree key not found in config after Rename()")
	}

	// Verify git branches
	bareDir := repository.BareDir(repoName)
	if git.BranchExists(bareDir, "feature-x") {
		t.Error("old branch still exists in git after Rename()")
	}
	if !git.BranchExists(bareDir, "feature-y") {
		t.Error("new branch does not exist in git after Rename()")
	}
}

func TestRenamePreservesPinnedState(t *testing.T) {
	_, repoName := setupTestEnv(t)

	if err := Add(repoName, "feature-pinned", AddOptions{NoSymlinks: true}); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	config.Reload()

	if err := Pin(repoName, "feature-pinned"); err != nil {
		t.Fatalf("Pin() error: %v", err)
	}
	config.Reload()

	if err := Rename(repoName, "feature-pinned", "feature-renamed"); err != nil {
		t.Fatalf("Rename() error: %v", err)
	}

	config.Reload()
	cfg, _ := config.Load()
	wt, exists := cfg.Repositories[repoName].Worktrees["feature-renamed"]
	if !exists {
		t.Fatal("renamed worktree not found in config")
	}
	if !wt.Pinned {
		t.Error("Pinned = false after Rename(), want true (should preserve pinned state)")
	}
}

func TestRenameWithSlashes(t *testing.T) {
	basePath, repoName := setupTestEnv(t)

	if err := Add(repoName, "feature/old-name", AddOptions{NoSymlinks: true}); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	config.Reload()

	if err := Rename(repoName, "feature/old-name", "feature/new-name"); err != nil {
		t.Fatalf("Rename() error: %v", err)
	}

	// Verify new directory uses sanitized name
	newDir := filepath.Join(basePath, repoName, "feature-new-name")
	if _, err := os.Stat(newDir); os.IsNotExist(err) {
		t.Error("new worktree directory (sanitized) was not created after Rename()")
	}

	// Verify config key is sanitized
	config.Reload()
	cfg, _ := config.Load()
	repo := cfg.Repositories[repoName]
	if _, exists := repo.Worktrees["feature-new-name"]; !exists {
		t.Error("new worktree key (sanitized) not found in config after Rename()")
	}
}

func TestRenameNonexistentRepo(t *testing.T) {
	setupTestEnv(t)

	err := Rename("nonexistent", "feature-x", "feature-y")
	if err == nil {
		t.Error("Rename() for nonexistent repository returned nil error, want error")
	}
}

func TestRenameNonexistentWorktree(t *testing.T) {
	_, repoName := setupTestEnv(t)

	err := Rename(repoName, "nonexistent-wt", "feature-y")
	if err == nil {
		t.Error("Rename() for nonexistent worktree returned nil error, want error")
	}
}

func TestRenameTargetExists(t *testing.T) {
	_, repoName := setupTestEnv(t)

	if err := Add(repoName, "feature-a", AddOptions{NoSymlinks: true}); err != nil {
		t.Fatalf("Add(feature-a) error: %v", err)
	}
	if err := Add(repoName, "feature-b", AddOptions{NoSymlinks: true}); err != nil {
		t.Fatalf("Add(feature-b) error: %v", err)
	}
	config.Reload()

	err := Rename(repoName, "feature-a", "feature-b")
	if err == nil {
		t.Error("Rename() to existing target returned nil error, want error")
	}
}
