package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	toml "github.com/pelletier/go-toml/v2"
)

// WorktreeConfig holds per-worktree configuration.
type WorktreeConfig struct {
	Pinned bool `toml:"pinned"`
}

// RepositoryConfig holds per-repository configuration.
type RepositoryConfig struct {
	RepoURL       string                    `toml:"repo_url"`
	DefaultBranch string                    `toml:"default_branch"`
	BasePath      string                    `toml:"base_path,omitempty"`
	Worktrees     map[string]WorktreeConfig `toml:"worktrees,omitempty"`
}

// Config is the top-level configuration.
type Config struct {
	BasePath             string                      `toml:"base_path"`
	Editor               string                      `toml:"editor"`
	CleanupThresholdDays int                         `toml:"cleanup_threshold_days"`
	Zoxide               bool                        `toml:"zoxide"`
	Repositories         map[string]RepositoryConfig `toml:"repositories,omitempty"`
}

var (
	cfg      *Config
	cfgOnce  sync.Once
	cfgPath  string
	firstRun bool
)

// DefaultConfigPath returns the default config file path.
func DefaultConfigPath() string {
	if cfgPath != "" {
		return cfgPath
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".config", "worktree", "config.toml")
}

// SetConfigPath overrides the config file path (useful for testing or --config flag).
func SetConfigPath(path string) {
	cfgPath = path
	cfg = nil
	cfgOnce = sync.Once{}
	firstRun = false
}

// IsFirstRun returns true if the config file was auto-created on this invocation.
func IsFirstRun() bool {
	return firstRun
}

// Load reads the config from disk, creating defaults if it doesn't exist.
func Load() (*Config, error) {
	var loadErr error
	cfgOnce.Do(func() {
		path := DefaultConfigPath()
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				cfg = defaultConfig()
				loadErr = Save(cfg)
				firstRun = true
				return
			}
			loadErr = fmt.Errorf("reading config: %w", err)
			return
		}
		var c Config
		if err := toml.Unmarshal(data, &c); err != nil {
			loadErr = fmt.Errorf("parsing config: %w", err)
			return
		}
		if c.Repositories == nil {
			c.Repositories = make(map[string]RepositoryConfig)
		}
		if c.BasePath == "" {
			c.BasePath = defaultBasePath()
		}
		if c.CleanupThresholdDays == 0 {
			c.CleanupThresholdDays = 30
		}
		cfg = &c
	})
	return cfg, loadErr
}

// Reload forces a fresh load of the config on next Load() call.
func Reload() {
	cfg = nil
	cfgOnce = sync.Once{}
	firstRun = false
}

// Save writes the config to disk.
func Save(c *Config) error {
	path := DefaultConfigPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	cfg = c
	return nil
}

// ExpandPath expands ~ to the home directory.
func ExpandPath(path string) string {
	if len(path) == 0 {
		return path
	}
	if path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}

// SanitizeBranchName replaces "/" in branch names with "-" so they can be
// used as flat directory names without creating nested subdirectories.
func SanitizeBranchName(name string) string {
	return strings.ReplaceAll(name, "/", "-")
}

func defaultConfig() *Config {
	return &Config{
		BasePath:             defaultBasePath(),
		Editor:               "code",
		CleanupThresholdDays: 30,
		Repositories:         make(map[string]RepositoryConfig),
	}
}

func defaultBasePath() string {
	return "~/worktrees"
}
