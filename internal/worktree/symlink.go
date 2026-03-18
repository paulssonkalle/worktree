package worktree

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/paulssonkalle/worktree/internal/config"
	"github.com/paulssonkalle/worktree/internal/git"
	"github.com/paulssonkalle/worktree/internal/repository"
)

var defaultSymlinkPaths = []string{
	".idea/runConfigurations/",
	".idea/dataSources.xml",
	".idea/dataSources.local.xml",
	".idea/codeStyles/",
	".idea/inspectionProfiles/",
	".idea/scopes/",
	".run/",
}

func init() {
	// Register the symlink setup callback with the repository package
	// to avoid circular imports (repository -> worktree).
	repository.SetupSymlinksFunc = SetupSymlinks
}

// SetupSymlinks creates symlinks in a worktree for shared IDE settings.
func SetupSymlinks(repoDir, worktreePath string) error {
	// Safety check: ensure this is a worktree-cli managed repo
	bareDir := filepath.Join(repoDir, ".bare")
	if _, err := os.Stat(bareDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Warning: %s does not contain a .bare/ directory, skipping symlink setup\n", repoDir)
		return nil
	}

	paths, err := parseSymlinkConfig(repoDir)
	if err != nil {
		return err
	}

	if err := ensureSharedDirs(repoDir, paths); err != nil {
		return err
	}

	if err := createSymlinks(repoDir, worktreePath, paths); err != nil {
		return err
	}

	return nil
}

// SetupSymlinksAll sets up symlinks for all worktrees in a repository.
func SetupSymlinksAll(repoName string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if _, exists := cfg.Repositories[repoName]; !exists {
		return fmt.Errorf("repository %q not found", repoName)
	}

	repoDir := repository.RepositoryDir(repoName)
	bareDir := repository.BareDir(repoName)

	gitWorktrees, err := git.ListWorktrees(bareDir)
	if err != nil {
		return fmt.Errorf("listing worktrees: %w", err)
	}

	for _, gwt := range gitWorktrees {
		if gwt.Bare {
			continue
		}
		fmt.Printf("Setting up symlinks for %s...\n", gwt.Path)
		if err := SetupSymlinks(repoDir, gwt.Path); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not set up symlinks for %s: %v\n", gwt.Path, err)
		}
	}

	return nil
}

// parseSymlinkConfig reads the symlink path list from <repoDir>/.worktree-symlinks.
// If the file does not exist, it returns the default paths.
func parseSymlinkConfig(repoDir string) ([]string, error) {
	configPath := filepath.Join(repoDir, ".worktree-symlinks")

	f, err := os.Open(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultSymlinkPaths, nil
		}
		return nil, fmt.Errorf("reading symlink config: %w", err)
	}
	defer f.Close()

	var paths []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		paths = append(paths, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading symlink config: %w", err)
	}

	return paths, nil
}

// ensureSharedDirs creates the shared directories in the repo root
// that symlinks will point to.
func ensureSharedDirs(repoDir string, paths []string) error {
	for _, path := range paths {
		var sharedPath string
		if strings.HasPrefix(path, ".idea/") {
			// .idea/X -> .idea-shared/X
			rest := strings.TrimPrefix(path, ".idea/")
			sharedPath = filepath.Join(repoDir, ".idea-shared", rest)
		} else {
			sharedPath = filepath.Join(repoDir, path)
		}

		if strings.HasSuffix(path, "/") {
			// Directory: create the full path
			if err := os.MkdirAll(sharedPath, 0o755); err != nil {
				return fmt.Errorf("creating shared directory %s: %w", sharedPath, err)
			}
		} else {
			// File: create the parent directory
			parentDir := filepath.Dir(sharedPath)
			if err := os.MkdirAll(parentDir, 0o755); err != nil {
				return fmt.Errorf("creating shared directory %s: %w", parentDir, err)
			}
		}
	}
	return nil
}

// createSymlinks creates symlinks in the worktree pointing to shared resources.
func createSymlinks(repoDir, worktreePath string, paths []string) error {
	for _, path := range paths {
		var sharedSource string
		var symlinkDest string

		if strings.HasPrefix(path, ".idea/") {
			rest := strings.TrimPrefix(path, ".idea/")
			rest = strings.TrimSuffix(rest, "/")
			sharedSource = filepath.Join(repoDir, ".idea-shared", rest)
			symlinkDest = filepath.Join(worktreePath, ".idea", rest)

			// Ensure .idea/ directory exists in the worktree
			ideaDir := filepath.Join(worktreePath, ".idea")
			if err := os.MkdirAll(ideaDir, 0o755); err != nil {
				return fmt.Errorf("creating .idea directory: %w", err)
			}
		} else {
			cleanPath := strings.TrimSuffix(path, "/")
			sharedSource = filepath.Join(repoDir, cleanPath)
			symlinkDest = filepath.Join(worktreePath, cleanPath)
		}

		// Check if destination already exists
		fi, err := os.Lstat(symlinkDest)
		if err == nil {
			if fi.Mode()&os.ModeSymlink != 0 {
				// Already a symlink, skip silently
				continue
			}
			// Exists but not a symlink
			fmt.Fprintf(os.Stderr, "Warning: %s already exists (not a symlink), skipping\n", symlinkDest)
			continue
		}

		// Compute relative path from symlink's parent to the shared source
		symlinkDir := filepath.Dir(symlinkDest)
		relPath, err := filepath.Rel(symlinkDir, sharedSource)
		if err != nil {
			return fmt.Errorf("computing relative path: %w", err)
		}

		if err := os.Symlink(relPath, symlinkDest); err != nil {
			return fmt.Errorf("creating symlink %s -> %s: %w", symlinkDest, relPath, err)
		}
	}
	return nil
}
