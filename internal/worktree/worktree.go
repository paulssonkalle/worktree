package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/paulssonkalle/worktree/internal/config"
	"github.com/paulssonkalle/worktree/internal/git"
	"github.com/paulssonkalle/worktree/internal/repository"
	"github.com/paulssonkalle/worktree/internal/zoxide"
)

// Info holds display information about a worktree.
type Info struct {
	Repo    string
	Name    string
	Branch  string
	Path    string
	Pinned  bool
	Dirty   bool
	Files   int
	Ahead   int
	Behind  int
	LastMod time.Time
}

// AddOptions configures worktree creation.
type AddOptions struct {
	BaseBranch string
	NoSymlinks bool
}

// Add creates a new worktree in a repository.
func Add(repoName, branchName string, opts AddOptions) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	repo, exists := cfg.Repositories[repoName]
	if !exists {
		return fmt.Errorf("repository %q not found", repoName)
	}

	baseBranch := opts.BaseBranch
	if baseBranch == "" {
		baseBranch = repo.DefaultBranch
	}

	bareDir := repository.BareDir(repoName)
	wtPath := WorktreeDir(repoName, branchName)

	// Check if worktree directory already exists
	if _, err := os.Stat(wtPath); err == nil {
		return fmt.Errorf("worktree %q already exists at %s", branchName, wtPath)
	}

	// Fetch latest before creating worktree
	fmt.Printf("Fetching latest changes...\n")
	if err := git.Fetch(bareDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: fetch failed: %v\n", err)
	}

	// Check if branch already exists locally
	if git.BranchExists(bareDir, branchName) {
		fmt.Printf("Checking out existing branch %q...\n", branchName)
		if err := git.AddWorktreeExisting(bareDir, wtPath, branchName); err != nil {
			return fmt.Errorf("creating worktree: %w", err)
		}
	} else if git.RemoteBranchExists(bareDir, branchName) {
		// Branch exists on remote but not locally - check it out so git
		// auto-creates a local tracking branch from the remote ref.
		fmt.Printf("Checking out remote branch %q...\n", branchName)
		if err := git.AddWorktreeExisting(bareDir, wtPath, branchName); err != nil {
			return fmt.Errorf("creating worktree: %w", err)
		}
	} else {
		// Determine the base ref - prefer remote tracking branch
		baseRef := baseBranch
		if git.RemoteBranchExists(bareDir, baseBranch) {
			baseRef = "origin/" + baseBranch
		}
		fmt.Printf("Creating branch %q from %s...\n", branchName, baseRef)
		if err := git.AddWorktree(bareDir, wtPath, branchName, baseRef); err != nil {
			return fmt.Errorf("creating worktree: %w", err)
		}
		// git auto-sets upstream to the base tracking branch (e.g. origin/main)
		// when creating from a remote ref. Unset it so the user can push with
		// `git push -u origin <branch>` to create the correct remote tracking branch.
		if err := git.UnsetUpstreamTracking(wtPath, branchName); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not unset upstream tracking: %v\n", err)
		}
	}

	// Set up upstream tracking if a matching remote branch exists
	if git.RemoteBranchExists(bareDir, branchName) {
		if err := git.SetUpstreamTracking(wtPath, branchName); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not set upstream tracking: %v\n", err)
		}
	}

	// Update config
	if repo.Worktrees == nil {
		repo.Worktrees = make(map[string]config.WorktreeConfig)
	}
	repo.Worktrees[config.SanitizeBranchName(branchName)] = config.WorktreeConfig{Pinned: false}
	cfg.Repositories[repoName] = repo
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("Worktree %q created at %s\n", branchName, wtPath)

	// Set up shared IDE settings symlinks
	if !opts.NoSymlinks {
		repoDir := repository.RepositoryDir(repoName)
		if err := SetupSymlinks(repoDir, wtPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not set up symlinks: %v\n", err)
		}
	}

	if cfg.Zoxide && zoxide.IsAvailable() {
		if err := zoxide.Add(wtPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not add to zoxide: %v\n", err)
		}
	}

	return nil
}

// Remove removes a worktree from a repository.
func Remove(repoName, worktreeName string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	repo, exists := cfg.Repositories[repoName]
	if !exists {
		return fmt.Errorf("repository %q not found", repoName)
	}

	bareDir := repository.BareDir(repoName)
	wtPath := WorktreeDir(repoName, worktreeName)

	if err := git.RemoveWorktree(bareDir, wtPath); err != nil {
		return fmt.Errorf("removing worktree: %w", err)
	}

	// Clean up directory if it still exists
	if err := os.RemoveAll(wtPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not remove directory %s: %v\n", wtPath, err)
	}

	delete(repo.Worktrees, config.SanitizeBranchName(worktreeName))
	cfg.Repositories[repoName] = repo
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("Worktree %q removed from repo %q\n", worktreeName, repoName)
	return nil
}

// Rename renames a worktree's branch and directory.
// It renames the git branch, moves the worktree directory, and updates the config.
func Rename(repoName, oldBranch, newBranch string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	repo, exists := cfg.Repositories[repoName]
	if !exists {
		return fmt.Errorf("repository %q not found", repoName)
	}

	oldSanitized := config.SanitizeBranchName(oldBranch)
	newSanitized := config.SanitizeBranchName(newBranch)

	// Verify old worktree exists in config
	oldWtCfg, exists := repo.Worktrees[oldSanitized]
	if !exists {
		return fmt.Errorf("worktree %q not found in repo %q", oldBranch, repoName)
	}

	// Verify new name doesn't already exist
	if _, exists := repo.Worktrees[newSanitized]; exists {
		return fmt.Errorf("worktree %q already exists in repo %q", newBranch, repoName)
	}

	bareDir := repository.BareDir(repoName)
	oldPath := WorktreeDir(repoName, oldBranch)
	newPath := WorktreeDir(repoName, newBranch)

	// Verify old path exists on disk
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return fmt.Errorf("worktree directory %q does not exist", oldPath)
	}

	// Verify new path doesn't exist on disk
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("directory %q already exists", newPath)
	}

	// Verify new branch name doesn't already exist
	if git.BranchExists(bareDir, newBranch) {
		return fmt.Errorf("branch %q already exists", newBranch)
	}

	// Resolve the actual branch name for the worktree (the old branch may have
	// slashes that got sanitized for the directory name but the git branch
	// keeps the original name). We look it up from git to be precise.
	gitWorktrees, err := git.ListWorktrees(bareDir)
	if err != nil {
		return fmt.Errorf("listing worktrees: %w", err)
	}

	// Resolve symlinks for path comparison (macOS /var -> /private/var)
	resolvedOldPath, _ := resolvePathSafe(oldPath)

	var actualOldBranch string
	for _, gwt := range gitWorktrees {
		resolvedGwtPath, _ := resolvePathSafe(gwt.Path)
		if resolvedGwtPath == resolvedOldPath {
			actualOldBranch = gwt.Branch
			break
		}
	}
	if actualOldBranch == "" {
		return fmt.Errorf("could not find git branch for worktree at %q", oldPath)
	}

	// 1. Rename the git branch
	fmt.Printf("Renaming branch %q to %q...\n", actualOldBranch, newBranch)
	if err := git.RenameBranch(bareDir, actualOldBranch, newBranch); err != nil {
		return fmt.Errorf("renaming branch: %w", err)
	}

	// 2. Move the worktree directory
	fmt.Printf("Moving worktree from %q to %q...\n", oldPath, newPath)
	if err := git.MoveWorktree(bareDir, oldPath, newPath); err != nil {
		// Try to rollback the branch rename
		_ = git.RenameBranch(bareDir, newBranch, actualOldBranch)
		return fmt.Errorf("moving worktree: %w", err)
	}

	// 3. Update config: move entry preserving pinned state
	delete(repo.Worktrees, oldSanitized)
	if repo.Worktrees == nil {
		repo.Worktrees = make(map[string]config.WorktreeConfig)
	}
	repo.Worktrees[newSanitized] = oldWtCfg
	cfg.Repositories[repoName] = repo
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	// 4. Update zoxide if enabled
	if cfg.Zoxide && zoxide.IsAvailable() {
		if err := zoxide.Remove(oldPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not remove old path from zoxide: %v\n", err)
		}
		if err := zoxide.Add(newPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not add new path to zoxide: %v\n", err)
		}
	}

	fmt.Printf("Worktree renamed from %q to %q\n", oldBranch, newBranch)
	return nil
}

// resolvePathSafe resolves symlinks, returning the original path on error.
func resolvePathSafe(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path, err
	}
	return resolved, nil
}

// List returns worktree info for a repository.
func List(repoName string) ([]Info, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	repo, exists := cfg.Repositories[repoName]
	if !exists {
		return nil, fmt.Errorf("repository %q not found", repoName)
	}

	bareDir := repository.BareDir(repoName)
	gitWorktrees, err := git.ListWorktrees(bareDir)
	if err != nil {
		return nil, fmt.Errorf("listing worktrees: %w", err)
	}

	var infos []Info
	for _, gwt := range gitWorktrees {
		if gwt.Bare {
			continue // skip the bare repo itself
		}
		name := filepath.Base(gwt.Path)
		wtCfg, hasCfg := repo.Worktrees[name]

		info := Info{
			Repo:   repoName,
			Name:   name,
			Branch: gwt.Branch,
			Path:   gwt.Path,
		}
		if hasCfg {
			info.Pinned = wtCfg.Pinned
		}

		infos = append(infos, info)
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})

	return infos, nil
}

// ListAll returns worktree info across all repositories.
func ListAll() ([]Info, error) {
	repos, err := repository.List()
	if err != nil {
		return nil, err
	}

	var all []Info
	for _, r := range repos {
		infos, err := List(r)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not list worktrees for %q: %v\n", r, err)
			continue
		}
		all = append(all, infos...)
	}
	return all, nil
}

// Pin marks a worktree as pinned (excluded from cleanup).
func Pin(repoName, worktreeName string) error {
	return setPinned(repoName, worktreeName, true)
}

// Unpin marks a worktree as unpinned.
func Unpin(repoName, worktreeName string) error {
	return setPinned(repoName, worktreeName, false)
}

func setPinned(repoName, worktreeName string, pinned bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	repo, exists := cfg.Repositories[repoName]
	if !exists {
		return fmt.Errorf("repository %q not found", repoName)
	}

	if repo.Worktrees == nil {
		repo.Worktrees = make(map[string]config.WorktreeConfig)
	}

	wtCfg, exists := repo.Worktrees[config.SanitizeBranchName(worktreeName)]
	if !exists {
		// Check if the worktree actually exists on disk
		wtPath := WorktreeDir(repoName, worktreeName)
		if _, err := os.Stat(wtPath); os.IsNotExist(err) {
			return fmt.Errorf("worktree %q not found in repo %q", worktreeName, repoName)
		}
		wtCfg = config.WorktreeConfig{}
	}

	wtCfg.Pinned = pinned
	repo.Worktrees[config.SanitizeBranchName(worktreeName)] = wtCfg
	cfg.Repositories[repoName] = repo

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	action := "pinned"
	if !pinned {
		action = "unpinned"
	}
	fmt.Printf("Worktree %q in repo %q %s\n", worktreeName, repoName, action)
	return nil
}

// CleanupResult holds information about a cleanup operation.
type CleanupResult struct {
	Repo    string
	Name    string
	Path    string
	LastMod time.Time
	Removed bool
}

// Cleanup removes worktrees that haven't been modified in the given number of days.
// Pinned worktrees are skipped. Returns a list of results.
func Cleanup(days int, dryRun bool, repoFilter string) ([]CleanupResult, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	threshold := time.Now().AddDate(0, 0, -days)
	var results []CleanupResult

	repos := make([]string, 0)
	if repoFilter != "" {
		if _, exists := cfg.Repositories[repoFilter]; !exists {
			return nil, fmt.Errorf("repository %q not found", repoFilter)
		}
		repos = append(repos, repoFilter)
	} else {
		for name := range cfg.Repositories {
			repos = append(repos, name)
		}
		sort.Strings(repos)
	}

	for _, repoName := range repos {
		repo := cfg.Repositories[repoName]
		bareDir := repository.BareDir(repoName)

		gitWorktrees, err := git.ListWorktrees(bareDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not list worktrees for %q: %v\n", repoName, err)
			continue
		}

		for _, gwt := range gitWorktrees {
			if gwt.Bare {
				continue
			}
			name := filepath.Base(gwt.Path)

			// Check if pinned
			if wtCfg, ok := repo.Worktrees[name]; ok && wtCfg.Pinned {
				continue
			}

			lastMod, err := git.GetLastModified(gwt.Path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not check last modified for %s: %v\n", gwt.Path, err)
				continue
			}

			if lastMod.Before(threshold) {
				result := CleanupResult{
					Repo:    repoName,
					Name:    name,
					Path:    gwt.Path,
					LastMod: lastMod,
				}

				if !dryRun {
					if err := Remove(repoName, name); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: could not remove %s/%s: %v\n", repoName, name, err)
						continue
					}
					result.Removed = true
				}

				results = append(results, result)
			}
		}
	}

	return results, nil
}

// PruneResult holds information about a prune operation.
type PruneResult struct {
	Repo    string
	Name    string
	Branch  string
	Reason  string
	Removed bool
}

// Prune removes worktrees whose branches have been merged or deleted on the remote.
// Pinned worktrees are skipped.
func Prune(repoFilter string, dryRun bool) ([]PruneResult, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	var results []PruneResult

	repos := make([]string, 0)
	if repoFilter != "" {
		if _, exists := cfg.Repositories[repoFilter]; !exists {
			return nil, fmt.Errorf("repository %q not found", repoFilter)
		}
		repos = append(repos, repoFilter)
	} else {
		for name := range cfg.Repositories {
			repos = append(repos, name)
		}
		sort.Strings(repos)
	}

	for _, repoName := range repos {
		repo := cfg.Repositories[repoName]
		bareDir := repository.BareDir(repoName)

		// Fetch latest
		fmt.Printf("Fetching latest for %s...\n", repoName)
		if err := git.Fetch(bareDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: fetch failed for %q: %v\n", repoName, err)
		}

		gitWorktrees, err := git.ListWorktrees(bareDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not list worktrees for %q: %v\n", repoName, err)
			continue
		}

		for _, gwt := range gitWorktrees {
			if gwt.Bare {
				continue
			}
			name := filepath.Base(gwt.Path)

			// Check if pinned
			if wtCfg, ok := repo.Worktrees[name]; ok && wtCfg.Pinned {
				continue
			}

			// Skip the default branch
			if gwt.Branch == repo.DefaultBranch {
				continue
			}

			var reason string

			// Check if branch was merged
			merged, err := git.IsBranchMerged(bareDir, gwt.Branch, repo.DefaultBranch)
			if err == nil && merged {
				reason = "branch merged into " + repo.DefaultBranch
			}

			// Check if remote branch was deleted
			if reason == "" && !git.RemoteBranchExists(bareDir, gwt.Branch) {
				reason = "remote branch deleted"
			}

			if reason == "" {
				continue
			}

			result := PruneResult{
				Repo:   repoName,
				Name:   name,
				Branch: gwt.Branch,
				Reason: reason,
			}

			if !dryRun {
				if err := Remove(repoName, name); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not remove %s/%s: %v\n", repoName, name, err)
					continue
				}
				result.Removed = true
			}

			results = append(results, result)
		}
	}

	return results, nil
}

// Status returns detailed status info for a specific worktree.
func Status(repoName, worktreeName string) (*Info, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	repo, exists := cfg.Repositories[repoName]
	if !exists {
		return nil, fmt.Errorf("repository %q not found", repoName)
	}

	wtPath := WorktreeDir(repoName, worktreeName)
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("worktree %q not found at %s", worktreeName, wtPath)
	}

	status, err := git.GetStatus(wtPath)
	if err != nil {
		return nil, fmt.Errorf("getting status: %w", err)
	}

	wtCfg := repo.Worktrees[config.SanitizeBranchName(worktreeName)]

	return &Info{
		Repo:   repoName,
		Name:   worktreeName,
		Branch: status.Branch,
		Path:   wtPath,
		Pinned: wtCfg.Pinned,
		Dirty:  status.Dirty,
		Files:  status.FileCount,
		Ahead:  status.Ahead,
		Behind: status.Behind,
	}, nil
}

// WorktreeDir returns the path for a worktree.
func WorktreeDir(repoName, worktreeName string) string {
	return filepath.Join(repository.RepositoryDir(repoName), config.SanitizeBranchName(worktreeName))
}
