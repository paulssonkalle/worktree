package cmd

import (
	"fmt"
	"os"

	"github.com/paulssonkalle/worktree-cli/internal/git"
	"github.com/paulssonkalle/worktree-cli/internal/repository"
	"github.com/spf13/cobra"
)

var fetchCmd = &cobra.Command{
	Use:   "fetch [repo]",
	Short: "Fetch latest remote branches for a repository",
	Long: `Fetch all remotes for a repository's bare clone.
If no repository is specified, fetches all repositories.`,
	Args: maxArgs(1, "repo (optional)"),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return repoNames(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			return fetchRepo(args[0])
		}
		return fetchAll()
	},
}

func fetchRepo(name string) error {
	if _, err := repository.Get(name); err != nil {
		return err
	}
	bareDir := repository.BareDir(name)
	fmt.Printf("Fetching %s...\n", name)
	if err := git.Fetch(bareDir); err != nil {
		return fmt.Errorf("fetching %s: %w", name, err)
	}
	fmt.Printf("Fetched %s\n", name)
	return nil
}

func fetchAll() error {
	names, err := repository.List()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		fmt.Println("No repositories configured.")
		return nil
	}
	var failed []string
	for _, name := range names {
		bareDir := repository.BareDir(name)
		fmt.Printf("Fetching %s...\n", name)
		if err := git.Fetch(bareDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: fetch failed for %s: %v\n", name, err)
			failed = append(failed, name)
			continue
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("failed to fetch %d repo(s): %v", len(failed), failed)
	}
	fmt.Printf("Fetched %d repo(s)\n", len(names))
	return nil
}

func init() {
	rootCmd.AddCommand(fetchCmd)
}
