package cmd

import (
	"github.com/paulssonkalle/worktree-cli/internal/worktree"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <repo> <branch>",
	Short: "Create a new worktree in a repository",
	Long: `Create a new worktree in a repository. The worktree name will be the branch name.

If the branch already exists, it will be checked out. Otherwise, a new branch
will be created from the repository's default branch (or the branch specified
with --base).`,
	Args: exactArgs("repo", "branch"),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return repoNames(cmd, args, toComplete)
		}
		if len(args) == 1 {
			return branchNames(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		baseBranch, _ := cmd.Flags().GetString("base")
		return worktree.Add(args[0], args[1], baseBranch)
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
	addCmd.Flags().String("base", "", "base branch to create the new branch from (default: repo's default branch)")
	addCmd.RegisterFlagCompletionFunc("base", allBranchNames)
}
