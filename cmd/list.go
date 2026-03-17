package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/paulssonkalle/worktree/internal/worktree"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list [repo]",
	Aliases: []string{"ls"},
	Short:   "List worktrees",
	Long:    "List worktrees for a specific repository, or all worktrees across all repositories.",
	Args:    maxArgs(1, "repo (optional)"),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return repoNames(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
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

		if len(infos) == 0 {
			fmt.Println("No worktrees found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "REPO\tWORKTREE\tBRANCH\tPINNED\tPATH")
		for _, info := range infos {
			pinned := ""
			if info.Pinned {
				pinned = "yes"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				info.Repo, info.Name, info.Branch, pinned, info.Path)
		}
		w.Flush()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
