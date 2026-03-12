package git

import (
	"bytes"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// run executes a git command in the given directory and returns stdout.
func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(stderr.String()), err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// CloneBare clones a repository as a bare clone and configures the fetch
// refspec so that `git fetch` picks up new remote branches.
func CloneBare(url, destDir string) error {
	cmd := exec.Command("git", "clone", "--bare", url, destDir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone --bare: %s: %w", strings.TrimSpace(stderr.String()), err)
	}
	// git clone --bare does not set a fetch refspec, which means git fetch
	// won't pick up new remote branches. Configure it to use standard
	// remote-tracking refs so fetch doesn't conflict with checked-out branches.
	if _, err := run(destDir, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"); err != nil {
		return fmt.Errorf("configuring fetch refspec: %w", err)
	}
	return nil
}

// Fetch fetches all remotes in the bare repo.
func Fetch(bareDir string) error {
	_, err := run(bareDir, "fetch", "--all", "--prune")
	return err
}

// AddWorktree creates a new worktree with a new branch based on baseBranch.
func AddWorktree(bareDir, worktreePath, branch, baseBranch string) error {
	_, err := run(bareDir, "worktree", "add", "-b", branch, worktreePath, baseBranch)
	return err
}

// AddWorktreeExisting creates a new worktree for an existing branch.
func AddWorktreeExisting(bareDir, worktreePath, branch string) error {
	_, err := run(bareDir, "worktree", "add", worktreePath, branch)
	return err
}

// RemoveWorktree removes a worktree.
func RemoveWorktree(bareDir, worktreePath string) error {
	_, err := run(bareDir, "worktree", "remove", "--force", worktreePath)
	return err
}

// WorktreeInfo holds parsed worktree list information.
type WorktreeInfo struct {
	Path   string
	HEAD   string
	Branch string
	Bare   bool
}

// ListWorktrees returns the list of worktrees for a bare repo.
func ListWorktrees(bareDir string) ([]WorktreeInfo, error) {
	out, err := run(bareDir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	var worktrees []WorktreeInfo
	var current WorktreeInfo

	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			if current.Path != "" {
				worktrees = append(worktrees, current)
			}
			current = WorktreeInfo{Path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "HEAD "):
			current.HEAD = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			// Strip refs/heads/ prefix to get bare branch name
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
		case line == "bare":
			current.Bare = true
		}
	}
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}

// ListBranches returns all available branch names in a bare repo.
// It combines local branches (refs/heads/*) and remote-tracking branches
// (refs/remotes/origin/*), returning deduplicated short names.
func ListBranches(bareDir string) ([]string, error) {
	out, err := run(bareDir, "for-each-ref", "--format=%(refname)", "refs/heads/", "refs/remotes/origin/")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	seen := make(map[string]struct{})
	var branches []string
	for _, ref := range strings.Split(out, "\n") {
		var name string
		switch {
		case strings.HasPrefix(ref, "refs/heads/"):
			name = strings.TrimPrefix(ref, "refs/heads/")
		case strings.HasPrefix(ref, "refs/remotes/origin/"):
			name = strings.TrimPrefix(ref, "refs/remotes/origin/")
			if name == "HEAD" {
				continue
			}
		default:
			continue
		}
		if _, exists := seen[name]; !exists {
			seen[name] = struct{}{}
			branches = append(branches, name)
		}
	}
	return branches, nil
}

// BranchExists checks if a branch exists locally.
func BranchExists(bareDir, branch string) bool {
	_, err := run(bareDir, "rev-parse", "--verify", "refs/heads/"+branch)
	return err == nil
}

// RemoteBranchExists checks if a branch exists on the remote.
func RemoteBranchExists(bareDir, branch string) bool {
	_, err := run(bareDir, "rev-parse", "--verify", "refs/remotes/origin/"+branch)
	return err == nil
}

// IsBranchMerged checks if a branch has been merged into the target branch.
func IsBranchMerged(bareDir, branch, target string) (bool, error) {
	out, err := run(bareDir, "branch", "--merged", target)
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(strings.TrimPrefix(line, "*"))
		name = strings.TrimSpace(name)
		if name == branch {
			return true, nil
		}
	}
	return false, nil
}

// DeleteBranch deletes a local branch.
func DeleteBranch(bareDir, branch string) error {
	_, err := run(bareDir, "branch", "-D", branch)
	return err
}

// DefaultBranch detects the default branch of the remote.
func DefaultBranch(bareDir string) (string, error) {
	out, err := run(bareDir, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err != nil {
		// Fallback: try common defaults
		for _, name := range []string{"main", "master"} {
			if BranchExists(bareDir, name) || RemoteBranchExists(bareDir, name) {
				return name, nil
			}
		}
		return "", fmt.Errorf("could not detect default branch: %w", err)
	}
	// Output is like refs/remotes/origin/main
	return strings.TrimPrefix(out, "refs/remotes/origin/"), nil
}

// StatusInfo holds git status information for a worktree.
type StatusInfo struct {
	Branch       string
	Dirty        bool
	FileCount    int
	Ahead        int
	Behind       int
	UpstreamGone bool
}

// GetStatus returns the git status for a worktree directory.
func GetStatus(worktreePath string) (*StatusInfo, error) {
	out, err := run(worktreePath, "status", "--porcelain=v2", "--branch")
	if err != nil {
		return nil, err
	}

	info := &StatusInfo{}
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "# branch.head "):
			info.Branch = strings.TrimPrefix(line, "# branch.head ")
		case strings.HasPrefix(line, "# branch.ab "):
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				info.Ahead, _ = strconv.Atoi(strings.TrimPrefix(parts[2], "+"))
				info.Behind, _ = strconv.Atoi(strings.TrimPrefix(parts[3], "-"))
			}
		case strings.HasPrefix(line, "# branch.upstream "):
			// upstream exists
		case line == "# branch.upstream":
			info.UpstreamGone = true
		case strings.HasPrefix(line, "1 ") || strings.HasPrefix(line, "2 ") || strings.HasPrefix(line, "? ") || strings.HasPrefix(line, "u "):
			info.Dirty = true
			info.FileCount++
		}
	}
	return info, nil
}

// GetLastModified walks a directory and returns the most recent modification time,
// ignoring .git directories.
func GetLastModified(dir string) (time.Time, error) {
	var latest time.Time
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}
		base := filepath.Base(path)
		if d.IsDir() && (base == ".git" || base == ".bare") {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return nil
			}
			if info.ModTime().After(latest) {
				latest = info.ModTime()
			}
		}
		return nil
	})
	return latest, err
}
