package cmd

import (
	"fmt"

	"github.com/paulssonkalle/worktree/internal/worktree"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:     "remove <repo> <worktree>",
	Aliases: []string{"rm"},
	Short:   "Remove a worktree from a repository",
	Long: `Remove a worktree from a repository.

By default, the local Git branch is also deleted if it has no unpushed commits
or uncommitted changes. The default branch is never deleted.

Use --keep-branch to always keep the local branch, or --force-branch to delete
it even when there are unpushed changes.`,
	Args: exactArgs("repo", "worktree"),
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
		keepBranch, _ := cmd.Flags().GetBool("keep-branch")
		forceBranch, _ := cmd.Flags().GetBool("force-branch")

		if keepBranch && forceBranch {
			return fmt.Errorf("--keep-branch and --force-branch are mutually exclusive")
		}

		return worktree.RemoveWithOptions(args[0], args[1], worktree.RemoveOptions{
			KeepBranch:  keepBranch,
			ForceBranch: forceBranch,
		})
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
	removeCmd.Flags().Bool("keep-branch", false, "keep the local Git branch after removing the worktree")
	removeCmd.Flags().Bool("force-branch", false, "delete the local Git branch even if it has unpushed changes")
}
