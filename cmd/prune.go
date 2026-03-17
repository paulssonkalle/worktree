package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/paulssonkalle/worktree/internal/worktree"
	"github.com/spf13/cobra"
)

var pruneCmd = &cobra.Command{
	Use:   "prune [repo]",
	Short: "Remove worktrees whose branches are merged or deleted",
	Long: `Remove worktrees whose branches have been merged into the default branch
or deleted from the remote. Pinned worktrees are always excluded.
Use --dry-run to preview what would be removed.`,
	Args: maxArgs(1, "repo (optional)"),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return repoNames(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		repoFilter := ""
		if len(args) == 1 {
			repoFilter = args[0]
		}

		if dryRun {
			fmt.Println("Dry run: showing worktrees with merged/deleted branches")
		} else {
			fmt.Println("Pruning worktrees with merged/deleted branches...")
		}

		results, err := worktree.Prune(repoFilter, dryRun)
		if err != nil {
			return err
		}

		if len(results) == 0 {
			fmt.Println("No worktrees to prune.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "REPO\tWORKTREE\tBRANCH\tREASON\tACTION")
		for _, r := range results {
			action := "would remove"
			if r.Removed {
				action = "removed"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				r.Repo, r.Name, r.Branch, r.Reason, action)
		}
		w.Flush()

		if dryRun {
			fmt.Printf("\nRun without --dry-run to remove these worktrees.\n")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pruneCmd)
	pruneCmd.Flags().Bool("dry-run", false, "preview what would be removed without actually removing")
}
