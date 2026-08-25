package tui

import (
	"fmt"
	"os/exec"
	"strings"

	"charm.land/bubbles/v2/key"
	huh "charm.land/huh/v2"
	"charm.land/huh/v2/spinner"
	"github.com/paulssonkalle/worktree/internal/git"
	"github.com/paulssonkalle/worktree/internal/repository"
	"github.com/paulssonkalle/worktree/internal/worktree"
)

// runField wraps a field in a Form so that the keymap (including Escape to
// quit) is applied at the form level where it actually takes effect.
func runField(field huh.Field, theme huh.ThemeFunc, keymap *huh.KeyMap) error {
	return huh.NewForm(
		huh.NewGroup(field),
	).
		WithTheme(theme).
		WithKeyMap(keymap).
		WithShowHelp(false).
		Run()
}

// InteractiveAdd runs a TUI flow to create a new worktree.
func InteractiveAdd() error {
	theme := everforestTheme()
	keymap := huh.NewDefaultKeyMap()
	keymap.Quit = key.NewBinding(key.WithKeys("ctrl+c", "esc"))

	// Step 1: Pick a repo
	repoNames, err := repository.List()
	if err != nil {
		return fmt.Errorf("listing repositories: %w", err)
	}
	if len(repoNames) == 0 {
		return fmt.Errorf("no repositories configured; use 'worktree repo add' to add one")
	}

	var repoOptions []huh.Option[string]
	for _, name := range repoNames {
		repoOptions = append(repoOptions, huh.NewOption(name, name))
	}

	var repo string
	err = runField(
		huh.NewSelect[string]().
			Title("Select a repository").
			Options(repoOptions...).
			Value(&repo).
			Filtering(true),
		theme, keymap,
	)
	if err != nil {
		return err
	}

	// Step 2: Get branches and default branch
	bareDir := repository.BareDir(repo)
	branches, err := git.ListBranches(bareDir)
	if err != nil {
		return fmt.Errorf("listing branches: %w", err)
	}

	repoCfg, err := repository.Get(repo)
	if err != nil {
		return fmt.Errorf("getting repository config: %w", err)
	}
	defaultBranch := repoCfg.DefaultBranch

	// Step 3: Pick or type a branch name (with optional fetch)
	const newBranchSentinel = "__new_branch__"
	const fetchSentinel = "__fetch_origin__"

	var branch string
	for {
		branchOptions := []huh.Option[string]{
			huh.NewOption("New branch...", newBranchSentinel),
			huh.NewOption("Fetch from origin...", fetchSentinel),
		}
		for _, b := range branches {
			branchOptions = append(branchOptions, huh.NewOption(b, b))
		}

		branch = ""
		err = runField(
			huh.NewSelect[string]().
				Title("Select a branch").
				Options(branchOptions...).
				Value(&branch).
				Filtering(true).
				Height(15),
			theme, keymap,
		)
		if err != nil {
			return err
		}

		if branch == fetchSentinel {
			var fetchErr error
			err = spinner.New().
				Title(fmt.Sprintf("Fetching %s...", repo)).
				Action(func() {
					fetchErr = git.Fetch(bareDir)
				}).
				Run()
			if err != nil {
				return err
			}
			if fetchErr != nil {
				return fmt.Errorf("fetching %s: %w", repo, fetchErr)
			}
			branches, err = git.ListBranches(bareDir)
			if err != nil {
				return fmt.Errorf("listing branches: %w", err)
			}
			continue
		}

		break
	}

	if branch == newBranchSentinel {
		branch = ""
		err = runField(
			huh.NewInput().
				Title("New branch name").
				Prompt("> ").
				Value(&branch).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("branch name cannot be empty")
					}
					return nil
				}),
			theme, keymap,
		)
		if err != nil {
			return err
		}
	}

	// Step 5: Base branch (prefilled with default)
	baseBranch := defaultBranch
	err = runField(
		huh.NewInput().
			Title("Base branch").
			Prompt("> ").
			Value(&baseBranch),
		theme, keymap,
	)
	if err != nil {
		return err
	}
	if strings.TrimSpace(baseBranch) == "" {
		baseBranch = defaultBranch
	}

	// Step 6: Create the worktree (with spinner)
	var addErr error
	err = spinner.New().
		Title(fmt.Sprintf("Creating worktree %s...", branch)).
		Action(func() {
			addErr = worktree.Add(repo, branch, worktree.AddOptions{
				BaseBranch: baseBranch,
				NoFetch:    true,
			})
		}).
		Run()
	if err != nil {
		return err
	}
	if addErr != nil {
		return fmt.Errorf("creating worktree: %w", addErr)
	}

	// Step 7: Connect via sesh
	wtPath := worktree.WorktreeDir(repo, branch)
	if _, err := exec.LookPath("sesh"); err == nil {
		cmd := exec.Command("sesh", "connect", wtPath)
		cmd.Stdin = nil
		cmd.Stdout = nil
		cmd.Stderr = nil
		_ = cmd.Run()
	}

	return nil
}
