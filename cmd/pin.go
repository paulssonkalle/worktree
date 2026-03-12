package cmd

import (
	"github.com/paulssonkalle/worktree-cli/internal/worktree"
	"github.com/spf13/cobra"
)

var pinCmd = &cobra.Command{
	Use:   "pin <repo> <worktree>",
	Short: "Pin a worktree to exclude it from cleanup",
	Args:  exactArgs("repo", "worktree"),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return repoNames(cmd, args, toComplete)
		}
		if len(args) == 1 {
			return worktreeNames(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return worktree.Pin(args[0], args[1])
	},
}

var unpinCmd = &cobra.Command{
	Use:   "unpin <repo> <worktree>",
	Short: "Unpin a worktree so it can be cleaned up",
	Args:  exactArgs("repo", "worktree"),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return repoNames(cmd, args, toComplete)
		}
		if len(args) == 1 {
			return worktreeNames(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return worktree.Unpin(args[0], args[1])
	},
}

func init() {
	rootCmd.AddCommand(pinCmd)
	rootCmd.AddCommand(unpinCmd)
}
