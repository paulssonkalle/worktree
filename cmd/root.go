package cmd

import (
	"fmt"
	"strings"

	"github.com/paulssonkalle/worktree/internal/config"
	"github.com/paulssonkalle/worktree/internal/git"
	"github.com/paulssonkalle/worktree/internal/repository"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	version = "0.1.0"
)

var rootCmd = &cobra.Command{
	Use:   "worktree",
	Short: "Manage Git worktrees organized by repository",
	Long: `A CLI tool for managing Git worktrees organized by repository.

Repositories are bare-cloned Git repos. Worktrees are created as Git worktrees
within each repository directory, allowing you to work on multiple branches
simultaneously.`,
	Version: version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cfgFile != "" {
			config.SetConfigPath(cfgFile)
		}
		return nil
	},
	SilenceUsage: true,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.config/worktree/config.toml)")
}

// repoNames returns repository names for shell completion.
func repoNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	st, err := config.LoadState()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	names := make([]string, 0, len(st.Repositories))
	for name := range st.Repositories {
		names = append(names, name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// worktreeNames returns worktree names for a repository for shell completion.
func worktreeNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	st, err := config.LoadState()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	repo, exists := st.Repositories[args[0]]
	if !exists {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	names := make([]string, 0, len(repo.Worktrees))
	for name := range repo.Worktrees {
		names = append(names, name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// branchNames returns remote branch names for a repository, filtering out
// branches that already have a worktree. Used for shell completion.
func branchNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	repoName := args[0]
	st, err := config.LoadState()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	repo, exists := st.Repositories[repoName]
	if !exists {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	bareDir := repository.BareDir(repoName)
	branches, err := git.ListBranches(bareDir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Build set of existing worktree names to filter out
	existing := make(map[string]struct{}, len(repo.Worktrees))
	for name := range repo.Worktrees {
		existing[name] = struct{}{}
	}

	filtered := make([]string, 0, len(branches))
	for _, b := range branches {
		if _, has := existing[b]; !has {
			filtered = append(filtered, b)
		}
	}
	return filtered, cobra.ShellCompDirectiveNoFileComp
}

// allBranchNames returns all branch names for a repository for shell completion.
// Unlike branchNames, this does not filter out branches that already have a worktree.
func allBranchNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	repoName := args[0]
	st, err := config.LoadState()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if _, exists := st.Repositories[repoName]; !exists {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	bareDir := repository.BareDir(repoName)
	branches, err := git.ListBranches(bareDir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return branches, cobra.ShellCompDirectiveNoFileComp
}

// exactArgs returns a cobra argument validator that requires exactly n args
// and shows the expected argument names in the error message.
// argNames should describe each positional argument, e.g. "name", "repo-url".
func exactArgs(argNames ...string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != len(argNames) {
			return fmt.Errorf(
				"requires %d argument(s): %s\n\nUsage:\n  %s",
				len(argNames),
				strings.Join(argNames, ", "),
				cmd.UseLine(),
			)
		}
		return nil
	}
}

// rangeArgs returns a cobra argument validator that requires between min and max args
// and shows the expected argument names in the error message.
// argNames should describe each positional argument. Optional args should be suffixed with "?".
func rangeArgs(min, max int, argNames ...string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < min || len(args) > max {
			return fmt.Errorf(
				"requires between %d and %d argument(s): %s\n\nUsage:\n  %s",
				min, max,
				strings.Join(argNames, ", "),
				cmd.UseLine(),
			)
		}
		return nil
	}
}

// maxArgs returns a cobra argument validator that requires at most max args
// and shows the expected argument names in the error message.
func maxArgs(max int, argNames ...string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) > max {
			return fmt.Errorf(
				"accepts at most %d argument(s): %s\n\nUsage:\n  %s",
				max,
				strings.Join(argNames, ", "),
				cmd.UseLine(),
			)
		}
		return nil
	}
}
