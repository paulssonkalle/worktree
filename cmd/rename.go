package cmd

import (
	"github.com/paulssonkalle/worktree/internal/worktree"
	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:   "rename <repo> <old-branch> <new-branch>",
	Short: "Rename a worktree's branch and directory",
	Long: `Rename a worktree by changing both its Git branch name and directory.

This command renames the Git branch, moves the worktree directory to match
the new name, and updates the configuration. Pinned status is preserved.`,
	Args: exactArgs("repo", "old-branch", "new-branch"),
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
		return worktree.Rename(args[0], args[1], args[2])
	},
}

func init() {
	rootCmd.AddCommand(renameCmd)
}
