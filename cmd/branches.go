package cmd

import (
	"fmt"

	"github.com/paulssonkalle/worktree/internal/git"
	"github.com/paulssonkalle/worktree/internal/repository"
	"github.com/spf13/cobra"
)

var branchesCmd = &cobra.Command{
	Use:   "branches <repo>",
	Short: "List branches available in a repository",
	Args:  exactArgs("repo"),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return repoNames(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		bareDir := repository.BareDir(args[0])
		noFetch, _ := cmd.Flags().GetBool("no-fetch")
		if !noFetch {
			if err := git.Fetch(bareDir); err != nil {
				return err
			}
		}
		branches, err := git.ListBranches(bareDir)
		if err != nil {
			return err
		}
		for _, branch := range branches {
			fmt.Println(branch)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(branchesCmd)
	branchesCmd.Flags().Bool("no-fetch", false, "skip fetching latest changes before listing branches")
}
