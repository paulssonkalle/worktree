package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	toml "github.com/pelletier/go-toml/v2"
)

// State holds machine-specific state: repository definitions and worktree entries.
// This is stored separately from Config so the config file can be version-controlled
// without machine-specific data.
type State struct {
	Repositories map[string]RepositoryConfig `toml:"repositories,omitempty"`
}

var (
	state     *State
	stateErr  error
	stateOnce sync.Once
	statePath string
)

// DefaultStatePath returns the default state file path.
func DefaultStatePath() string {
	if statePath != "" {
		return statePath
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".local", "state", "worktree", "state.toml")
}

// SetStatePath overrides the state file path (useful for testing).
func SetStatePath(path string) {
	statePath = path
	state = nil
	stateErr = nil
	stateOnce = sync.Once{}
}

// LoadState reads the state from disk, creating an empty state if it doesn't exist.
// On first load, it also checks whether the config file contains repository data
// from an older version and migrates it to the state file.
func LoadState() (*State, error) {
	stateOnce.Do(func() {
		path := DefaultStatePath()
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				state = defaultState()
				// Check for migration from old config format
				if migrated, migrateErr := migrateFromConfig(state); migrateErr != nil {
					stateErr = fmt.Errorf("migrating state from config: %w", migrateErr)
					return
				} else if migrated {
					stateErr = SaveState(state)
					return
				}
				stateErr = SaveState(state)
				return
			}
			stateErr = fmt.Errorf("reading state: %w", err)
			return
		}
		var s State
		if err := toml.Unmarshal(data, &s); err != nil {
			stateErr = fmt.Errorf("parsing state: %w", err)
			return
		}
		if s.Repositories == nil {
			s.Repositories = make(map[string]RepositoryConfig)
		}
		state = &s
	})
	return state, stateErr
}

// SaveState writes the state to disk.
func SaveState(s *State) error {
	path := DefaultStatePath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating state dir: %w", err)
	}
	data, err := toml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing state: %w", err)
	}
	state = s
	return nil
}

// ReloadState forces a fresh load of the state on next LoadState() call.
func ReloadState() {
	state = nil
	stateErr = nil
	stateOnce = sync.Once{}
}

// migrateFromConfig checks if the config file contains repository data from an
// older version and migrates it to the state. Returns true if migration occurred.
func migrateFromConfig(s *State) (bool, error) {
	path := DefaultConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		// No config file either — nothing to migrate.
		return false, nil
	}

	// Parse the config file using a temporary struct that includes repositories
	// (the old format).
	var oldFormat struct {
		BasePath             string                      `toml:"base_path"`
		Editor               string                      `toml:"editor"`
		CleanupThresholdDays int                         `toml:"cleanup_threshold_days"`
		Zoxide               bool                        `toml:"zoxide"`
		Repositories         map[string]RepositoryConfig `toml:"repositories,omitempty"`
	}
	if err := toml.Unmarshal(data, &oldFormat); err != nil {
		return false, nil // Can't parse — let config.Load() handle the error
	}

	if len(oldFormat.Repositories) == 0 {
		return false, nil // Nothing to migrate
	}

	// Merge repositories into state
	for name, repo := range oldFormat.Repositories {
		s.Repositories[name] = repo
	}

	// Rewrite the config file without repositories
	newCfg := &Config{
		BasePath:             oldFormat.BasePath,
		Editor:               oldFormat.Editor,
		CleanupThresholdDays: oldFormat.CleanupThresholdDays,
		Zoxide:               oldFormat.Zoxide,
	}
	if err := Save(newCfg); err != nil {
		return false, fmt.Errorf("rewriting config after migration: %w", err)
	}

	// Reset config singleton so it picks up the cleaned version
	Reload()

	fmt.Printf("Migrated repository data from %s to %s\n", DefaultConfigPath(), DefaultStatePath())
	return true, nil
}

func defaultState() *State {
	return &State{
		Repositories: make(map[string]RepositoryConfig),
	}
}
