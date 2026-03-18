package cmd

import (
	"github.com/paulssonkalle/worktree/internal/worktree"
	"github.com/spf13/cobra"
)

var symlinksCmd = &cobra.Command{
	Use:   "symlinks <repo>",
	Short: "Set up shared IDE settings symlinks for all worktrees",
	Long: `Set up symlinks for sharing IntelliJ IDEA settings across all worktrees
in a repository. This creates a shared .idea-shared/ directory in the repo root
and symlinks specific settings entries from each worktree's .idea/ directory.

This is useful for retroactively setting up symlinks for existing worktrees.
New worktrees created with 'worktree add' will have symlinks set up automatically.

The default symlinked settings include run configurations, code styles, inspection
profiles, scopes, and data sources. Override the defaults by creating a
.worktree-symlinks file in the repo root.`,
	Args: exactArgs("repo"),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return repoNames(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return worktree.SetupSymlinksAll(args[0])
	},
}

func init() {
	rootCmd.AddCommand(symlinksCmd)
}
