package cmd

import (
	"errors"

	huh "charm.land/huh/v2"
	"github.com/paulssonkalle/worktree/internal/tui"
	"github.com/spf13/cobra"
)

var interactiveAddCmd = &cobra.Command{
	Use:     "interactive-add",
	Aliases: []string{"ia"},
	Short:   "Interactively create a new worktree",
	Long:    "Launch a TUI to select a repository, branch, and base branch, then create a worktree and connect via sesh.",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := tui.InteractiveAdd()
		if errors.Is(err, huh.ErrUserAborted) {
			return nil
		}
		return err
	},
}

func init() {
	rootCmd.AddCommand(interactiveAddCmd)
}
