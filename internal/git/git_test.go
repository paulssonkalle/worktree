package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paulssonkalle/worktree/internal/testutil"
)

// initBareRepo creates a temporary normal git repo with one commit on "main",
// then clones it as bare. Returns the bare repo path.
//
// Note: A bare clone puts remote branches directly into refs/heads/ (not refs/remotes/origin/).
// This means RemoteBranchExists() will return false for branches in a bare clone,
// which matches production behavior after `git clone --bare`.
func initBareRepo(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	srcDir := filepath.Join(tmpDir, "src")
	bareDir := filepath.Join(tmpDir, "bare")

	testutil.RunGit(t, "", "init", srcDir)
	testutil.RunGit(t, srcDir, "checkout", "-b", "main")
	testutil.WriteFile(t, filepath.Join(srcDir, "README.md"), "# Test")
	testutil.RunGit(t, srcDir, "add", ".")
	testutil.RunGit(t, srcDir, "commit", "-m", "initial commit")

	testutil.RunGit(t, "", "clone", "--bare", srcDir, bareDir)

	return bareDir
}

// resolvePath resolves symlinks in a path to handle macOS /var -> /private/var.
func resolvePath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q) error: %v", path, err)
	}
	return resolved
}

func TestCloneBare(t *testing.T) {
	tmpDir := t.TempDir()

	srcDir := filepath.Join(tmpDir, "src")
	bareDir := filepath.Join(tmpDir, "clone")

	testutil.RunGit(t, "", "init", srcDir)
	testutil.RunGit(t, srcDir, "checkout", "-b", "main")
	testutil.WriteFile(t, filepath.Join(srcDir, "file.txt"), "hello")
	testutil.RunGit(t, srcDir, "add", ".")
	testutil.RunGit(t, srcDir, "commit", "-m", "first")

	if err := CloneBare(srcDir, bareDir); err != nil {
		t.Fatalf("CloneBare() error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(bareDir, "HEAD")); os.IsNotExist(err) {
		t.Error("bare repo missing HEAD file")
	}
}

func TestCloneBareInvalidURL(t *testing.T) {
	tmpDir := t.TempDir()
	bareDir := filepath.Join(tmpDir, "clone")

	err := CloneBare("/nonexistent/repo", bareDir)
	if err == nil {
		t.Error("CloneBare() with invalid URL returned nil error, want error")
	}
}

func TestFetch(t *testing.T) {
	bareDir := initBareRepo(t)

	if err := Fetch(bareDir); err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
}

func TestAddWorktreeAndList(t *testing.T) {
	bareDir := initBareRepo(t)
	tmpDir := t.TempDir()
	wtPath := filepath.Join(tmpDir, "feature-wt")

	if err := AddWorktree(bareDir, wtPath, "feature-branch", "main"); err != nil {
		t.Fatalf("AddWorktree() error: %v", err)
	}

	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("worktree directory was not created")
	}

	worktrees, err := ListWorktrees(bareDir)
	if err != nil {
		t.Fatalf("ListWorktrees() error: %v", err)
	}

	if len(worktrees) < 2 {
		t.Fatalf("ListWorktrees() returned %d entries, want at least 2", len(worktrees))
	}

	// Use resolved paths for comparison (macOS /var -> /private/var)
	resolvedWtPath := resolvePath(t, wtPath)

	var found bool
	for _, wt := range worktrees {
		if wt.Path == resolvedWtPath {
			found = true
			if wt.Branch != "feature-branch" {
				t.Errorf("Branch = %q, want %q", wt.Branch, "feature-branch")
			}
			if wt.Bare {
				t.Error("feature worktree has Bare = true, want false")
			}
			if wt.HEAD == "" {
				t.Error("feature worktree has empty HEAD")
			}
		}
	}
	if !found {
		t.Errorf("feature worktree not found in ListWorktrees() output. Looking for %q, got:", resolvedWtPath)
		for _, wt := range worktrees {
			t.Logf("  path=%q branch=%q bare=%v", wt.Path, wt.Branch, wt.Bare)
		}
	}

	var bareFound bool
	for _, wt := range worktrees {
		if wt.Bare {
			bareFound = true
		}
	}
	if !bareFound {
		t.Error("bare repo entry not found in ListWorktrees() output")
	}
}

func TestAddWorktreeExisting(t *testing.T) {
	bareDir := initBareRepo(t)
	wtPath := filepath.Join(t.TempDir(), "main-wt")

	if err := AddWorktreeExisting(bareDir, wtPath, "main"); err != nil {
		t.Fatalf("AddWorktreeExisting() error: %v", err)
	}

	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("worktree directory was not created")
	}
}

func TestRemoveWorktree(t *testing.T) {
	bareDir := initBareRepo(t)
	tmpDir := t.TempDir()
	wtPath := filepath.Join(tmpDir, "to-remove")
	// Resolve path BEFORE removal (macOS /var -> /private/var)
	resolvedBare := resolvePath(t, bareDir)

	if err := AddWorktree(bareDir, wtPath, "temp-branch", "main"); err != nil {
		t.Fatalf("AddWorktree() error: %v", err)
	}

	resolvedWtPath := resolvePath(t, wtPath)

	if err := RemoveWorktree(bareDir, wtPath); err != nil {
		t.Fatalf("RemoveWorktree() error: %v", err)
	}

	// After removal, listing should not contain this worktree
	worktrees, err := ListWorktrees(resolvedBare)
	if err != nil {
		t.Fatalf("ListWorktrees() error: %v", err)
	}
	for _, wt := range worktrees {
		if wt.Path == resolvedWtPath {
			t.Error("worktree still present after RemoveWorktree()")
		}
	}
}

func TestListBranches(t *testing.T) {
	bareDir := initBareRepo(t)

	// Initially should have just "main"
	branches, err := ListBranches(bareDir)
	if err != nil {
		t.Fatalf("ListBranches() error: %v", err)
	}
	if len(branches) != 1 || branches[0] != "main" {
		t.Errorf("ListBranches() = %v, want [main]", branches)
	}

	// Create additional branches
	testutil.RunGit(t, bareDir, "branch", "feature-a", "main")
	testutil.RunGit(t, bareDir, "branch", "feature-b", "main")

	branches, err = ListBranches(bareDir)
	if err != nil {
		t.Fatalf("ListBranches() error: %v", err)
	}
	if len(branches) != 3 {
		t.Fatalf("ListBranches() returned %d branches, want 3: %v", len(branches), branches)
	}

	expected := map[string]bool{"main": true, "feature-a": true, "feature-b": true}
	for _, b := range branches {
		if !expected[b] {
			t.Errorf("unexpected branch %q in ListBranches() result", b)
		}
	}
}

func TestListBranchesEmpty(t *testing.T) {
	// A bare repo with no branches should return nil
	tmpDir := t.TempDir()
	bareDir := filepath.Join(tmpDir, "empty-bare")
	testutil.RunGit(t, "", "init", "--bare", bareDir)

	branches, err := ListBranches(bareDir)
	if err != nil {
		t.Fatalf("ListBranches() on empty repo error: %v", err)
	}
	if branches != nil {
		t.Errorf("ListBranches() on empty repo = %v, want nil", branches)
	}
}

func TestBranchExists(t *testing.T) {
	bareDir := initBareRepo(t)

	if !BranchExists(bareDir, "main") {
		t.Error("BranchExists('main') = false, want true")
	}
	if BranchExists(bareDir, "nonexistent-branch") {
		t.Error("BranchExists('nonexistent-branch') = true, want false")
	}
}

func TestRemoteBranchExists(t *testing.T) {
	// In a bare clone, remote branches live under refs/heads/ not refs/remotes/origin/.
	// RemoteBranchExists checks refs/remotes/origin/<branch>, so it will return false
	// for all branches in a standard bare clone. This test verifies that behavior.
	bareDir := initBareRepo(t)

	// No refs/remotes/origin/* exist in a bare clone
	if RemoteBranchExists(bareDir, "main") {
		t.Error("RemoteBranchExists('main') = true in bare clone, want false (bare clones don't have refs/remotes/origin/)")
	}
	if RemoteBranchExists(bareDir, "nonexistent") {
		t.Error("RemoteBranchExists('nonexistent') = true, want false")
	}
}

func TestRemoteBranchExistsWithManualRef(t *testing.T) {
	// Test that RemoteBranchExists works when refs/remotes/origin/ refs exist.
	// We manually create such a ref to simulate what happens after reconfiguring
	// fetch refspecs and fetching.
	bareDir := initBareRepo(t)

	// Get main's commit hash and create a remote tracking ref manually
	testutil.RunGit(t, bareDir, "update-ref", "refs/remotes/origin/main", "refs/heads/main")

	if !RemoteBranchExists(bareDir, "main") {
		t.Error("RemoteBranchExists('main') = false after manual ref creation, want true")
	}
	if RemoteBranchExists(bareDir, "nonexistent") {
		t.Error("RemoteBranchExists('nonexistent') = true, want false")
	}
}

func TestDefaultBranch(t *testing.T) {
	// DefaultBranch falls back to BranchExists for "main"/"master" when
	// symbolic-ref fails, which is the case for simple bare clones.
	bareDir := initBareRepo(t)

	branch, err := DefaultBranch(bareDir)
	if err != nil {
		t.Fatalf("DefaultBranch() error: %v", err)
	}
	if branch != "main" {
		t.Errorf("DefaultBranch() = %q, want %q", branch, "main")
	}
}

func TestDefaultBranchWithSymbolicRef(t *testing.T) {
	bareDir := initBareRepo(t)

	// Manually set up origin/HEAD symbolic ref to simulate a proper remote
	testutil.RunGit(t, bareDir, "update-ref", "refs/remotes/origin/main", "refs/heads/main")
	testutil.RunGit(t, bareDir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")

	branch, err := DefaultBranch(bareDir)
	if err != nil {
		t.Fatalf("DefaultBranch() error: %v", err)
	}
	if branch != "main" {
		t.Errorf("DefaultBranch() = %q, want %q", branch, "main")
	}
}

func TestDefaultBranchUndetectable(t *testing.T) {
	tmpDir := t.TempDir()
	bareDir := filepath.Join(tmpDir, "empty-bare")

	// Create a bare repo with a branch named "develop" (not main/master)
	srcDir := filepath.Join(tmpDir, "src")
	testutil.RunGit(t, "", "init", srcDir)
	testutil.RunGit(t, srcDir, "checkout", "-b", "develop")
	testutil.WriteFile(t, filepath.Join(srcDir, "file.txt"), "test")
	testutil.RunGit(t, srcDir, "add", ".")
	testutil.RunGit(t, srcDir, "commit", "-m", "init")
	testutil.RunGit(t, "", "clone", "--bare", srcDir, bareDir)

	_, err := DefaultBranch(bareDir)
	if err == nil {
		t.Error("DefaultBranch() returned nil error for repo with no main/master, want error")
	}
}

func TestIsBranchMerged(t *testing.T) {
	bareDir := initBareRepo(t)
	wtDir := filepath.Join(t.TempDir(), "main-wt")

	if err := AddWorktreeExisting(bareDir, wtDir, "main"); err != nil {
		t.Fatalf("AddWorktreeExisting() error: %v", err)
	}

	// Create a feature branch at the same commit as main (trivially merged)
	testutil.RunGit(t, wtDir, "branch", "merged-feature")

	merged, err := IsBranchMerged(bareDir, "merged-feature", "main")
	if err != nil {
		t.Fatalf("IsBranchMerged() error: %v", err)
	}
	if !merged {
		t.Error("IsBranchMerged() = false for trivially merged branch, want true")
	}
}

func TestIsBranchNotMerged(t *testing.T) {
	bareDir := initBareRepo(t)
	wtPath := filepath.Join(t.TempDir(), "diverged-wt")

	if err := AddWorktree(bareDir, wtPath, "diverged", "main"); err != nil {
		t.Fatalf("AddWorktree() error: %v", err)
	}

	testutil.WriteFile(t, filepath.Join(wtPath, "new-file.txt"), "diverged content")
	testutil.RunGit(t, wtPath, "add", ".")
	testutil.RunGit(t, wtPath, "commit", "-m", "diverge")

	merged, err := IsBranchMerged(bareDir, "diverged", "main")
	if err != nil {
		t.Fatalf("IsBranchMerged() error: %v", err)
	}
	if merged {
		t.Error("IsBranchMerged() = true for diverged branch, want false")
	}
}

func TestDeleteBranch(t *testing.T) {
	bareDir := initBareRepo(t)
	wtPath := filepath.Join(t.TempDir(), "delete-wt")

	if err := AddWorktree(bareDir, wtPath, "to-delete", "main"); err != nil {
		t.Fatalf("AddWorktree() error: %v", err)
	}
	if err := RemoveWorktree(bareDir, wtPath); err != nil {
		t.Fatalf("RemoveWorktree() error: %v", err)
	}

	if !BranchExists(bareDir, "to-delete") {
		t.Fatal("branch 'to-delete' doesn't exist before deletion")
	}

	if err := DeleteBranch(bareDir, "to-delete"); err != nil {
		t.Fatalf("DeleteBranch() error: %v", err)
	}

	if BranchExists(bareDir, "to-delete") {
		t.Error("branch 'to-delete' still exists after DeleteBranch()")
	}
}

func TestGetStatusClean(t *testing.T) {
	bareDir := initBareRepo(t)
	wtPath := filepath.Join(t.TempDir(), "status-wt")

	if err := AddWorktreeExisting(bareDir, wtPath, "main"); err != nil {
		t.Fatalf("AddWorktreeExisting() error: %v", err)
	}

	status, err := GetStatus(wtPath)
	if err != nil {
		t.Fatalf("GetStatus() error: %v", err)
	}
	if status.Branch != "main" {
		t.Errorf("Branch = %q, want %q", status.Branch, "main")
	}
	if status.Dirty {
		t.Error("Dirty = true for clean worktree, want false")
	}
	if status.FileCount != 0 {
		t.Errorf("FileCount = %d for clean worktree, want 0", status.FileCount)
	}
}

func TestGetStatusDirty(t *testing.T) {
	bareDir := initBareRepo(t)
	wtPath := filepath.Join(t.TempDir(), "status-wt")

	if err := AddWorktreeExisting(bareDir, wtPath, "main"); err != nil {
		t.Fatalf("AddWorktreeExisting() error: %v", err)
	}

	testutil.WriteFile(t, filepath.Join(wtPath, "dirty.txt"), "dirty content")

	status, err := GetStatus(wtPath)
	if err != nil {
		t.Fatalf("GetStatus() error: %v", err)
	}
	if !status.Dirty {
		t.Error("Dirty = false after adding untracked file, want true")
	}
	if status.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1", status.FileCount)
	}
}

func TestGetStatusMultipleChanges(t *testing.T) {
	bareDir := initBareRepo(t)
	wtPath := filepath.Join(t.TempDir(), "multi-wt")

	if err := AddWorktreeExisting(bareDir, wtPath, "main"); err != nil {
		t.Fatalf("AddWorktreeExisting() error: %v", err)
	}

	testutil.WriteFile(t, filepath.Join(wtPath, "file1.txt"), "one")
	testutil.WriteFile(t, filepath.Join(wtPath, "file2.txt"), "two")
	testutil.WriteFile(t, filepath.Join(wtPath, "file3.txt"), "three")

	status, err := GetStatus(wtPath)
	if err != nil {
		t.Fatalf("GetStatus() error: %v", err)
	}
	if status.FileCount != 3 {
		t.Errorf("FileCount = %d, want 3", status.FileCount)
	}
	if !status.Dirty {
		t.Error("Dirty = false with 3 untracked files, want true")
	}
}

func TestGetLastModified(t *testing.T) {
	tmpDir := t.TempDir()

	testutil.WriteFile(t, filepath.Join(tmpDir, "old.txt"), "old")
	time.Sleep(50 * time.Millisecond)
	testutil.WriteFile(t, filepath.Join(tmpDir, "new.txt"), "new")

	lastMod, err := GetLastModified(tmpDir)
	if err != nil {
		t.Fatalf("GetLastModified() error: %v", err)
	}
	if lastMod.IsZero() {
		t.Error("GetLastModified() returned zero time")
	}
	if time.Since(lastMod) > 5*time.Second {
		t.Errorf("GetLastModified() returned time %v, expected within last 5 seconds", lastMod)
	}
}

func TestGetLastModifiedSkipsGitDir(t *testing.T) {
	tmpDir := t.TempDir()

	testutil.WriteFile(t, filepath.Join(tmpDir, ".git", "something"), "git internal")
	testutil.WriteFile(t, filepath.Join(tmpDir, ".bare", "something"), "bare internal")

	oldFile := filepath.Join(tmpDir, "content.txt")
	testutil.WriteFile(t, oldFile, "content")

	lastMod, err := GetLastModified(tmpDir)
	if err != nil {
		t.Fatalf("GetLastModified() error: %v", err)
	}

	info, _ := os.Stat(oldFile)
	if !lastMod.Equal(info.ModTime()) {
		t.Errorf("GetLastModified() = %v, want %v (content.txt modtime)", lastMod, info.ModTime())
	}
}

func TestGetLastModifiedEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	lastMod, err := GetLastModified(tmpDir)
	if err != nil {
		t.Fatalf("GetLastModified() error: %v", err)
	}
	if !lastMod.IsZero() {
		t.Errorf("GetLastModified() on empty dir = %v, want zero time", lastMod)
	}
}

func TestListWorktreesBareOnly(t *testing.T) {
	bareDir := initBareRepo(t)

	worktrees, err := ListWorktrees(bareDir)
	if err != nil {
		t.Fatalf("ListWorktrees() error: %v", err)
	}

	bareCount := 0
	for _, wt := range worktrees {
		if wt.Bare {
			bareCount++
		}
	}
	if bareCount != 1 {
		t.Errorf("expected 1 bare entry, got %d (total entries: %d)", bareCount, len(worktrees))
	}
}

func TestSetUpstreamTracking(t *testing.T) {
	bareDir := initBareRepo(t)
	wtPath := filepath.Join(t.TempDir(), "tracking-wt")

	// Configure fetch refspec and fetch to create refs/remotes/origin/* refs,
	// mirroring what CloneBare does in production.
	testutil.RunGit(t, bareDir, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	testutil.RunGit(t, bareDir, "fetch", "--all")

	// Create a worktree for main
	if err := AddWorktreeExisting(bareDir, wtPath, "main"); err != nil {
		t.Fatalf("AddWorktreeExisting() error: %v", err)
	}

	// Set upstream tracking
	if err := SetUpstreamTracking(wtPath, "main"); err != nil {
		t.Fatalf("SetUpstreamTracking() error: %v", err)
	}

	// Verify upstream is set by checking git status output
	status, err := GetStatus(wtPath)
	if err != nil {
		t.Fatalf("GetStatus() error: %v", err)
	}
	if status.UpstreamGone {
		t.Error("UpstreamGone = true after setting upstream, want false")
	}
}

func TestSetUpstreamTrackingNoRemote(t *testing.T) {
	bareDir := initBareRepo(t)
	wtPath := filepath.Join(t.TempDir(), "no-remote-wt")

	if err := AddWorktree(bareDir, wtPath, "orphan-branch", "main"); err != nil {
		t.Fatalf("AddWorktree() error: %v", err)
	}

	// No refs/remotes/origin/orphan-branch exists, so this should fail
	err := SetUpstreamTracking(wtPath, "orphan-branch")
	if err == nil {
		t.Error("SetUpstreamTracking() returned nil error for nonexistent remote branch, want error")
	}
}

func TestUnsetUpstreamTracking(t *testing.T) {
	bareDir := initBareRepo(t)
	wtPath := filepath.Join(t.TempDir(), "unset-wt")

	// Configure fetch refspec and fetch to create refs/remotes/origin/* refs.
	testutil.RunGit(t, bareDir, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	testutil.RunGit(t, bareDir, "fetch", "--all")

	// Create a new branch from origin/main — git auto-sets upstream to origin/main
	if err := AddWorktree(bareDir, wtPath, "new-feature", "origin/main"); err != nil {
		t.Fatalf("AddWorktree() error: %v", err)
	}

	// Verify upstream was auto-set by git
	status, err := GetStatus(wtPath)
	if err != nil {
		t.Fatalf("GetStatus() error: %v", err)
	}
	if status.UpstreamGone {
		t.Error("expected upstream to be set after creating from remote ref")
	}

	// Unset the upstream
	if err := UnsetUpstreamTracking(wtPath, "new-feature"); err != nil {
		t.Fatalf("UnsetUpstreamTracking() error: %v", err)
	}

	// Verify upstream is gone — git branch -vv should not show [...] tracking info
	branchOut := testutil.RunGit(t, wtPath, "branch", "-vv")
	if strings.Contains(branchOut, "[origin/") {
		t.Errorf("upstream still set after UnsetUpstreamTracking(), branch -vv: %s", branchOut)
	}
}

func TestRenameBranch(t *testing.T) {
	bareDir := initBareRepo(t)

	// Create a branch to rename
	testutil.RunGit(t, bareDir, "branch", "old-name", "main")

	if !BranchExists(bareDir, "old-name") {
		t.Fatal("branch 'old-name' doesn't exist before rename")
	}

	if err := RenameBranch(bareDir, "old-name", "new-name"); err != nil {
		t.Fatalf("RenameBranch() error: %v", err)
	}

	if BranchExists(bareDir, "old-name") {
		t.Error("branch 'old-name' still exists after RenameBranch()")
	}
	if !BranchExists(bareDir, "new-name") {
		t.Error("branch 'new-name' doesn't exist after RenameBranch()")
	}
}

func TestRenameBranchNonexistent(t *testing.T) {
	bareDir := initBareRepo(t)

	err := RenameBranch(bareDir, "nonexistent-branch", "new-name")
	if err == nil {
		t.Error("RenameBranch() with nonexistent branch returned nil error, want error")
	}
}

func TestMoveWorktree(t *testing.T) {
	bareDir := initBareRepo(t)
	tmpDir := t.TempDir()
	oldPath := filepath.Join(tmpDir, "old-wt")
	newPath := filepath.Join(tmpDir, "new-wt")

	if err := AddWorktree(bareDir, oldPath, "move-branch", "main"); err != nil {
		t.Fatalf("AddWorktree() error: %v", err)
	}

	if err := MoveWorktree(bareDir, oldPath, newPath); err != nil {
		t.Fatalf("MoveWorktree() error: %v", err)
	}

	// Old path should no longer exist
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("old worktree path still exists after MoveWorktree()")
	}

	// New path should exist
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Error("new worktree path does not exist after MoveWorktree()")
	}

	// Verify worktree is listed at new path with correct branch
	resolvedNewPath := resolvePath(t, newPath)
	worktrees, err := ListWorktrees(bareDir)
	if err != nil {
		t.Fatalf("ListWorktrees() error: %v", err)
	}

	var found bool
	for _, wt := range worktrees {
		if wt.Path == resolvedNewPath {
			found = true
			if wt.Branch != "move-branch" {
				t.Errorf("Branch = %q, want %q", wt.Branch, "move-branch")
			}
		}
	}
	if !found {
		t.Errorf("moved worktree not found at %q in ListWorktrees() output", resolvedNewPath)
		for _, wt := range worktrees {
			t.Logf("  path=%q branch=%q bare=%v", wt.Path, wt.Branch, wt.Bare)
		}
	}
}

func TestMoveWorktreeNonexistent(t *testing.T) {
	bareDir := initBareRepo(t)
	tmpDir := t.TempDir()

	err := MoveWorktree(bareDir, filepath.Join(tmpDir, "nonexistent-wt"), filepath.Join(tmpDir, "dest-wt"))
	if err == nil {
		t.Error("MoveWorktree() with nonexistent path returned nil error, want error")
	}
}
