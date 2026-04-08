package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/paulssonkalle/worktree/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  "View and manage the worktree CLI configuration and state files.",
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the config and state file paths",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		editState, _ := cmd.Flags().GetBool("state")
		if editState {
			fmt.Println(config.DefaultStatePath())
		} else {
			fmt.Println(config.DefaultConfigPath())
		}
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create the default config file",
	Long: `Create the default config file if it doesn't exist.
If it already exists, print its current contents.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := config.DefaultConfigPath()

		_, err := config.Load()
		if err != nil {
			return err
		}

		if config.IsFirstRun() {
			fmt.Printf("Config created at %s\n", path)
			fmt.Println("Edit base_path to set where repositories and worktrees are stored.")
		} else {
			fmt.Printf("Config already exists at %s\n", path)
		}

		fmt.Println()
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading config: %w", err)
		}
		fmt.Print(string(data))
		return nil
	},
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open the config file in your editor",
	Long: `Open the config or state file in your editor.
Use --state to edit the state file (repositories and worktrees).`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		editState, _ := cmd.Flags().GetBool("state")

		var path string
		if editState {
			// Ensure state file exists
			_, err := config.LoadState()
			if err != nil {
				return err
			}
			path = config.DefaultStatePath()
		} else {
			// Ensure config file exists
			_, err := config.Load()
			if err != nil {
				return err
			}
			path = config.DefaultConfigPath()
		}

		editor := os.Getenv("EDITOR")
		if editor == "" {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			editor = cfg.Editor
		}
		if editor == "" {
			return fmt.Errorf("no editor configured; set $EDITOR or 'editor' in config")
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
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configEditCmd)

	configEditCmd.Flags().Bool("state", false, "edit the state file (repositories and worktrees) instead of config")
	configPathCmd.Flags().Bool("state", false, "print the state file path instead of config")
}
