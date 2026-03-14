package repository

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/paulssonkalle/worktree-cli/internal/config"
	"github.com/paulssonkalle/worktree-cli/internal/git"
	"github.com/paulssonkalle/worktree-cli/internal/zoxide"
)

// Add creates a new repository by cloning a bare repo.
func Add(name, repoURL, defaultBranch, basePath string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if config.IsFirstRun() {
		path := config.DefaultConfigPath()
		fmt.Printf("Config created at %s\n", path)
		fmt.Printf("Repositories will be stored in: %s\n", config.ExpandPath(cfg.BasePath))
		fmt.Printf("Edit base_path in the config to change this before adding repositories.\n")
		fmt.Printf("  worktree config edit\n\n")
	}

	if _, exists := cfg.Repositories[name]; exists {
		return fmt.Errorf("repository %q already exists", name)
	}

	// Store initial config so RepositoryDir() can resolve the per-repo base path.
	cfg.Repositories[name] = config.RepositoryConfig{
		RepoURL:   repoURL,
		BasePath:  basePath,
		Worktrees: make(map[string]config.WorktreeConfig),
	}

	repoDir := RepositoryDir(name)
	bareDir := BareDir(name)

	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		delete(cfg.Repositories, name)
		return fmt.Errorf("creating repository directory: %w", err)
	}

	fmt.Printf("Cloning %s into %s...\n", repoURL, bareDir)
	if err := git.CloneBare(repoURL, bareDir); err != nil {
		// Clean up on failure
		os.RemoveAll(repoDir)
		delete(cfg.Repositories, name)
		return fmt.Errorf("cloning repository: %w", err)
	}

	// Detect default branch if not specified
	if defaultBranch == "" {
		detected, err := git.DefaultBranch(bareDir)
		if err != nil {
			defaultBranch = "main"
			fmt.Printf("Could not detect default branch, using %q\n", defaultBranch)
		} else {
			defaultBranch = detected
		}
	}

	// Create a worktree for the default branch and pin it
	wtPath := filepath.Join(repoDir, config.SanitizeBranchName(defaultBranch))
	fmt.Printf("Creating worktree for default branch %q...\n", defaultBranch)
	if err := git.AddWorktreeExisting(bareDir, wtPath, defaultBranch); err != nil {
		return fmt.Errorf("creating default branch worktree: %w", err)
	}
	cfg.Repositories[name] = config.RepositoryConfig{
		RepoURL:       repoURL,
		DefaultBranch: defaultBranch,
		BasePath:      basePath,
		Worktrees: map[string]config.WorktreeConfig{
			config.SanitizeBranchName(defaultBranch): {Pinned: true},
		},
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	if cfg.Zoxide && zoxide.IsAvailable() {
		if err := zoxide.Add(wtPath); err != nil {
			fmt.Printf("Warning: could not add to zoxide: %v\n", err)
		}
	}

	fmt.Printf("Worktree %q pinned (excluded from cleanup)\n", defaultBranch)
	fmt.Printf("Repository %q added (default branch: %s)\n", name, defaultBranch)
	return nil
}

// Remove deletes a repository and all its worktrees.
func Remove(name string, force bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if _, exists := cfg.Repositories[name]; !exists {
		return fmt.Errorf("repository %q not found", name)
	}

	repoDir := RepositoryDir(name)

	if !force {
		return fmt.Errorf("use --force to remove repository %q and all its worktrees at %s", name, repoDir)
	}

	if err := os.RemoveAll(repoDir); err != nil {
		return fmt.Errorf("removing repository directory: %w", err)
	}

	delete(cfg.Repositories, name)
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("Repository %q removed\n", name)
	return nil
}

// List returns sorted repository names.
func List() ([]string, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(cfg.Repositories))
	for name := range cfg.Repositories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// Get returns the config for a repository.
func Get(name string) (*config.RepositoryConfig, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	r, exists := cfg.Repositories[name]
	if !exists {
		return nil, fmt.Errorf("repository %q not found", name)
	}
	return &r, nil
}

// RepositoryDir returns the repository's directory path.
// If the repository has a per-repo BasePath configured, it takes precedence
// over the global BasePath.
func RepositoryDir(name string) string {
	cfg, _ := config.Load()
	if repo, exists := cfg.Repositories[name]; exists && repo.BasePath != "" {
		return filepath.Join(config.ExpandPath(repo.BasePath), name)
	}
	base := config.ExpandPath(cfg.BasePath)
	return filepath.Join(base, name)
}

// BareDir returns the bare repo directory for a repository.
func BareDir(name string) string {
	return filepath.Join(RepositoryDir(name), ".bare")
}
