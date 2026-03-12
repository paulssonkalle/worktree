package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/paulssonkalle/worktree-cli/internal/repository"
	"github.com/paulssonkalle/worktree-cli/internal/worktree"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [repo]",
	Short: "Show status of worktrees",
	Long:  "Show Git status (branch, dirty state, ahead/behind) for all worktrees or for a specific repository.",
	Args:  maxArgs(1, "repo (optional)"),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return repoNames(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		var repos []string
		var err error

		if len(args) == 1 {
			// Validate repository exists
			if _, err := repository.Get(args[0]); err != nil {
				return err
			}
			repos = []string{args[0]}
		} else {
			repos, err = repository.List()
			if err != nil {
				return err
			}
		}

		if len(repos) == 0 {
			fmt.Println("No repositories configured.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "REPO\tWORKTREE\tBRANCH\tSTATUS\tAHEAD/BEHIND\tPINNED")

		for _, repoName := range repos {
			infos, err := worktree.List(repoName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not list worktrees for %q: %v\n", repoName, err)
				continue
			}

			for _, info := range infos {
				statusInfo, err := worktree.Status(repoName, info.Name)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not get status for %s/%s: %v\n", repoName, info.Name, err)
					continue
				}

				status := "clean"
				if statusInfo.Dirty {
					status = fmt.Sprintf("dirty (%d files)", statusInfo.Files)
				}

				ab := "-"
				if statusInfo.Ahead > 0 || statusInfo.Behind > 0 {
					ab = fmt.Sprintf("+%d/-%d", statusInfo.Ahead, statusInfo.Behind)
				}

				pinned := ""
				if statusInfo.Pinned {
					pinned = "yes"
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
					repoName, info.Name, statusInfo.Branch, status, ab, pinned)
			}
		}
		w.Flush()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
