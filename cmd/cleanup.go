package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/paulssonkalle/worktree-cli/internal/config"
	"github.com/paulssonkalle/worktree-cli/internal/worktree"
	"github.com/spf13/cobra"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove worktrees not modified in a given number of days",
	Long: `Remove worktrees that haven't been modified in a specified number of days.
Pinned worktrees are always excluded. Use --dry-run to preview what would be removed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		days, _ := cmd.Flags().GetInt("days")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		repo, _ := cmd.Flags().GetString("repo")

		if days == 0 {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			days = cfg.CleanupThresholdDays
		}

		if dryRun {
			fmt.Printf("Dry run: showing worktrees not modified in %d days\n\n", days)
		} else {
			fmt.Printf("Removing worktrees not modified in %d days...\n\n", days)
		}

		results, err := worktree.Cleanup(days, dryRun, repo)
		if err != nil {
			return err
		}

		if len(results) == 0 {
			fmt.Println("No worktrees to clean up.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "REPO\tWORKTREE\tLAST MODIFIED\tACTION")
		for _, r := range results {
			age := time.Since(r.LastMod).Round(24 * time.Hour)
			action := "would remove"
			if r.Removed {
				action = "removed"
			}
			fmt.Fprintf(w, "%s\t%s\t%s ago\t%s\n",
				r.Repo, r.Name, formatDuration(age), action)
		}
		w.Flush()

		if dryRun {
			fmt.Printf("\nRun without --dry-run to remove these worktrees.\n")
		}
		return nil
	},
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	if days == 0 {
		return "< 1 day"
	}
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}

func init() {
	rootCmd.AddCommand(cleanupCmd)
	cleanupCmd.Flags().Int("days", 0, "number of days since last modification (default: from config)")
	cleanupCmd.Flags().Bool("dry-run", false, "preview what would be removed without actually removing")
	cleanupCmd.Flags().String("repo", "", "limit cleanup to a specific repository")
	_ = cleanupCmd.RegisterFlagCompletionFunc("repo", repoNames)
}
