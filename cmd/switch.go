package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/paulssonkalle/worktree/internal/repository"
	"github.com/paulssonkalle/worktree/internal/worktree"
	"github.com/spf13/cobra"
)

type switchEntry struct {
	repo     string
	worktree string
	branch   string
	path     string
}

var switchCmd = &cobra.Command{
	Use:   "switch [repo] [worktree]",
	Short: "Switch to a worktree directory",
	Long: `Switch to a worktree directory by printing its path to stdout.

With no arguments or one argument, an interactive fuzzy finder (fzf) is
launched to select a worktree. With two arguments, the path is printed
directly.

To change your shell's working directory, add a shell function:

  # bash / zsh
  wt() { local dir; dir=$(worktree switch "$@") && cd "$dir"; }

  # fish
  function wt; set dir (worktree switch $argv); and cd $dir; end`,
	Args: maxArgs(2, "repo (optional)", "worktree (optional)"),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return repoNames(cmd, args, toComplete)
		}
		if len(args) == 1 {
			return worktreeNames(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 2 args: direct path, no fzf needed
		if len(args) == 2 {
			return switchDirect(args[0], args[1])
		}

		// 0 or 1 args: need fzf
		if _, err := exec.LookPath("fzf"); err != nil {
			return fmt.Errorf("fzf is not installed; install it (https://github.com/junegunn/fzf) or provide explicit args: worktree switch <repo> <worktree>")
		}

		entries, err := buildSwitchEntries(args)
		if err != nil {
			return err
		}

		if len(entries) == 0 {
			return fmt.Errorf("no worktrees found")
		}

		selected, err := runFzf(entries)
		if err != nil {
			return err
		}

		fmt.Println(selected.path)
		return nil
	},
}

func switchDirect(repoName, worktreeName string) error {
	// Validate repo exists
	if _, err := repository.Get(repoName); err != nil {
		return err
	}

	path := worktree.WorktreeDir(repoName, worktreeName)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("worktree %q not found in repo %q", worktreeName, repoName)
	}

	fmt.Println(path)
	return nil
}

func buildSwitchEntries(args []string) ([]switchEntry, error) {
	var infos []worktree.Info
	var err error

	if len(args) == 1 {
		infos, err = worktree.List(args[0])
	} else {
		infos, err = worktree.ListAll()
	}
	if err != nil {
		return nil, err
	}

	entries := make([]switchEntry, len(infos))
	for i, info := range infos {
		entries[i] = switchEntry{
			repo:     info.Repo,
			worktree: info.Name,
			branch:   info.Branch,
			path:     info.Path,
		}
	}
	return entries, nil
}

// columnWidths calculates the max width needed for each display column.
func columnWidths(entries []switchEntry) (repoW, wtW, branchW int) {
	repoW = len("REPO")
	wtW = len("WORKTREE")
	branchW = len("BRANCH")
	for _, e := range entries {
		if len(e.repo) > repoW {
			repoW = len(e.repo)
		}
		if len(e.worktree) > wtW {
			wtW = len(e.worktree)
		}
		if len(e.branch) > branchW {
			branchW = len(e.branch)
		}
	}
	return
}

// formatFzfHeader formats the column header line with padded widths.
func formatFzfHeader(repoW, wtW, branchW int) string {
	return fmt.Sprintf("%-*s  %-*s  %-*s", repoW, "REPO", wtW, "WORKTREE", branchW, "BRANCH")
}

// formatFzfInput formats entries as padded display columns followed by a
// tab-separated path. The display part is in field 1 (before the tab) and the
// path is in field 2, so fzf can show only the display part with --with-nth=1.
func formatFzfInput(entries []switchEntry) string {
	if len(entries) == 0 {
		return ""
	}
	repoW, wtW, branchW := columnWidths(entries)
	var b strings.Builder
	for _, e := range entries {
		display := fmt.Sprintf("%-*s  %-*s  %-*s", repoW, e.repo, wtW, e.worktree, branchW, e.branch)
		fmt.Fprintf(&b, "%s\t%s\n", display, e.path)
	}
	return b.String()
}

// parseFzfOutput parses an fzf output line back into a switchEntry.
// The format is: "<padded display>\t<path>"
func parseFzfOutput(line string) (switchEntry, error) {
	line = strings.TrimRight(line, "\n")
	tabIdx := strings.LastIndex(line, "\t")
	if tabIdx < 0 {
		return switchEntry{}, fmt.Errorf("unexpected fzf output: %q", line)
	}
	display := line[:tabIdx]
	path := line[tabIdx+1:]

	fields := strings.Fields(display)
	if len(fields) < 3 {
		return switchEntry{}, fmt.Errorf("unexpected fzf output: %q", line)
	}
	return switchEntry{
		repo:     fields[0],
		worktree: fields[1],
		branch:   fields[2],
		path:     path,
	}, nil
}

func runFzf(entries []switchEntry) (switchEntry, error) {
	input := formatFzfInput(entries)
	repoW, wtW, branchW := columnWidths(entries)
	header := formatFzfHeader(repoW, wtW, branchW)

	fzfCmd := exec.Command("fzf",
		"--header", header,
		"--delimiter", "\t",
		"--with-nth", "1",
		"--no-multi",
	)
	fzfCmd.Stdin = strings.NewReader(input)
	fzfCmd.Stderr = os.Stderr

	var stdout bytes.Buffer
	fzfCmd.Stdout = &stdout

	if err := fzfCmd.Run(); err != nil {
		// fzf exits 130 on Ctrl-C / Esc
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 130 {
			os.Exit(1)
		}
		// fzf exits 1 when no match
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return switchEntry{}, fmt.Errorf("no match found")
		}
		return switchEntry{}, fmt.Errorf("fzf failed: %w", err)
	}

	return parseFzfOutput(stdout.String())
}

func init() {
	rootCmd.AddCommand(switchCmd)
}
