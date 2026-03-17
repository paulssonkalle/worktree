package cmd

import (
	"fmt"
	"os"

	"github.com/paulssonkalle/worktree-cli/internal/worktree"
	"github.com/paulssonkalle/worktree-cli/internal/zoxide"
	"github.com/spf13/cobra"
)

var zoxideCmd = &cobra.Command{
	Use:   "zoxide",
	Short: "Manage zoxide integration",
}

var zoxideSyncCmd = &cobra.Command{
	Use:   "sync [repo]",
	Short: "Add all worktree paths to zoxide",
	Long: `Add all worktree paths to zoxide so they can be jumped to with z.
If a repository name is given, only that repo's worktrees are added.
Otherwise all worktrees across every repository are added.`,
	Args: maxArgs(1, "repo"),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return repoNames(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if !zoxide.IsAvailable() {
			return fmt.Errorf("zoxide is not installed (see https://github.com/ajeetdsouza/zoxide)")
		}

		var infos []worktree.Info
		var err error
		if len(args) == 1 {
			infos, err = worktree.List(args[0])
		} else {
			infos, err = worktree.ListAll()
		}
		if err != nil {
			return err
		}

		added := 0
		for _, info := range infos {
			if err := zoxide.Add(info.Path); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to add %s: %v\n", info.Path, err)
				continue
			}
			fmt.Println(info.Path)
			added++
		}

		fmt.Printf("Added %d worktree(s) to zoxide\n", added)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(zoxideCmd)
	zoxideCmd.AddCommand(zoxideSyncCmd)
}
