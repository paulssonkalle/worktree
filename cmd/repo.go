package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/paulssonkalle/worktree-cli/internal/repository"
	"github.com/spf13/cobra"
)

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage repositories",
	Long:  "Add, remove, and list repositories. Each repository is a bare-cloned Git repository.",
}

var repoAddCmd = &cobra.Command{
	Use:   "add <name> <repo-url>",
	Short: "Add a new repository by cloning it",
	Args:  exactArgs("name", "repo-url"),
	RunE: func(cmd *cobra.Command, args []string) error {
		defaultBranch, _ := cmd.Flags().GetString("default-branch")
		return repository.Add(args[0], args[1], defaultBranch)
	},
}

var repoRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm"},
	Short:   "Remove a repository and all its worktrees",
	Args:    exactArgs("name"),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return repoNames(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		return repository.Remove(args[0], force)
	},
}

var repoListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all repositories",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		names, err := repository.List()
		if err != nil {
			return err
		}
		if len(names) == 0 {
			fmt.Println("No repositories configured. Use 'worktree repo add' to add one.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "REPO\tURL\tBRANCH\tPATH")
		for _, name := range names {
			r, err := repository.Get(name)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, r.RepoURL, r.DefaultBranch, repository.RepositoryDir(name))
		}
		w.Flush()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(repoCmd)
	repoCmd.AddCommand(repoAddCmd)
	repoCmd.AddCommand(repoRemoveCmd)
	repoCmd.AddCommand(repoListCmd)

	repoAddCmd.Flags().String("default-branch", "", "default branch name (auto-detected if not set)")
	repoRemoveCmd.Flags().Bool("force", false, "force removal of repository and all worktrees")
}
