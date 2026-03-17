package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/paulssonkalle/worktree/internal/config"
	"github.com/paulssonkalle/worktree/internal/repository"
	"github.com/paulssonkalle/worktree/internal/worktree"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open <repo> [worktree]",
	Short: "Open a worktree in your configured editor",
	Long: `Open a worktree directory in your configured editor.
If no worktree is specified, opens the repository directory.

The editor can be configured in the config file (default: code).`,
	Args: rangeArgs(1, 2, "repo", "worktree (optional)"),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return repoNames(cmd, args, toComplete)
		}
		if len(args) == 1 {
			return worktreeNames(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		editor := cfg.Editor
		if editor == "" {
			editor = os.Getenv("EDITOR")
		}
		if editor == "" {
			return fmt.Errorf("no editor configured; set 'editor' in config or $EDITOR environment variable")
		}

		var path string
		if len(args) == 2 {
			path = worktree.WorktreeDir(args[0], args[1])
		} else {
			path = repository.RepositoryDir(args[0])
		}

		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("path does not exist: %s", path)
		}

		fmt.Printf("Opening %s with %s...\n", path, editor)
		editorCmd := exec.Command(editor, path)
		editorCmd.Stdin = os.Stdin
		editorCmd.Stdout = os.Stdout
		editorCmd.Stderr = os.Stderr
		return editorCmd.Run()
	},
}

func init() {
	rootCmd.AddCommand(openCmd)
}
