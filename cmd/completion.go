package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion <bash|zsh|fish|powershell>",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for worktree.

To load completions:

Bash:
  $ source <(worktree completion bash)
  # To load completions for each session, execute once:
  # Linux:
  $ worktree completion bash > /etc/bash_completion.d/worktree
  # macOS:
  $ worktree completion bash > $(brew --prefix)/etc/bash_completion.d/worktree

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. Execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc
  # To load completions for each session, execute once:
  $ worktree completion zsh > "${fpath[1]}/_worktree"
  # You will need to start a new shell for this setup to take effect.

Fish:
  $ worktree completion fish | source
  # To load completions for each session, execute once:
  $ worktree completion fish > ~/.config/fish/completions/worktree.fish

PowerShell:
  PS> worktree completion powershell | Out-String | Invoke-Expression
  # To load completions for every new session, run:
  PS> worktree completion powershell > worktree.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletionV2(os.Stdout, true)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
