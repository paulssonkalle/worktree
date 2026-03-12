package cmd

import (
	"github.com/paulssonkalle/worktree-cli/internal/worktree"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:     "remove <repo> <worktree>",
	Aliases: []string{"rm"},
	Short:   "Remove a worktree from a repository",
	Args:    exactArgs("repo", "worktree"),
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
		return worktree.Remove(args[0], args[1])
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
