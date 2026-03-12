# worktree

A CLI tool for managing Git worktrees organized by repository.

## How it works

`worktree` manages repositories as bare clones and creates Git worktrees for
each branch you work on. This lets you have multiple branches checked out
simultaneously without stashing, cloning multiple copies, or losing context.

```
~/worktrees/
  my-app/
    .bare/            # bare clone of the repository
    main/             # worktree for the default branch (pinned)
    feature-login/    # worktree for a feature branch
    fix-header/       # worktree for a bugfix branch
  api/
    .bare/
    main/
    refactor-auth/
```

## Installation

Requires Go 1.25 or later.

```bash
go install github.com/paulssonkalle/worktree-cli@latest
```

The binary is named `worktree`.

## Quick start

```bash
# Initialize config (optional — created automatically on first use)
worktree config init

# Add a repository (clones it as a bare repo and creates a default branch worktree)
worktree repo add my-app git@github.com:user/my-app.git

# Create a worktree for a new feature branch
worktree add my-app feature-login

# List all worktrees
worktree list

# Check Git status across all worktrees
worktree status

# Switch to a worktree interactively (requires fzf)
worktree switch

# Open a worktree in your editor
worktree open my-app feature-login

# Clean up worktrees whose branches have been merged or deleted
worktree prune --dry-run
worktree prune
```

## Shell integration

### Directory switching

`worktree switch` prints the selected worktree's path to stdout. To make it
change your shell's working directory, add a wrapper function:

```bash
# bash / zsh — add to ~/.bashrc or ~/.zshrc
wt() { local dir; dir=$(worktree switch "$@") && cd "$dir"; }
```

```fish
# fish — add to ~/.config/fish/config.fish
function wt; set dir (worktree switch $argv); and cd $dir; end
```

Then use `wt` to interactively pick a worktree and cd into it, or `wt my-app feature-login` to switch directly.

### Shell completion

```bash
# bash
source <(worktree completion bash)

# zsh
worktree completion zsh > "${fpath[1]}/_worktree"

# fish
worktree completion fish | source
```

Run `worktree completion --help` for persistent installation instructions.

## Commands

| Command | Description |
|---|---|
| `repo add <name> <url>` | Add a repository (bare clone) |
| `repo remove <name>` | Remove a repository and all its worktrees |
| `repo list` | List configured repositories |
| `add <repo> <branch>` | Create a worktree for a branch |
| `remove <repo> <worktree>` | Remove a worktree |
| `list [repo]` | List worktrees |
| `status [repo]` | Show Git status of worktrees |
| `switch [repo] [worktree]` | Switch to a worktree (interactive with fzf) |
| `open <repo> [worktree]` | Open a worktree in your editor |
| `fetch [repo]` | Fetch latest from remotes |
| `pin <repo> <worktree>` | Pin a worktree (excluded from cleanup/prune) |
| `unpin <repo> <worktree>` | Unpin a worktree |
| `cleanup` | Remove worktrees not modified in N days |
| `prune [repo]` | Remove worktrees with merged/deleted branches |
| `config init` | Create the default config file |
| `config path` | Print the config file path |
| `config edit` | Open the config file in your editor |
| `completion <shell>` | Generate shell completion scripts |

Run `worktree <command> --help` for detailed usage and flags.

## Configuration

Config file: `~/.config/worktree/config.toml`

```toml
# Base directory where all repositories and worktrees are stored.
# Supports ~ for home directory.
base_path = "~/worktrees"

# Editor command for 'open' and 'config edit'.
# Falls back to $EDITOR if not set.
editor = "code"

# Default number of days for the 'cleanup' command.
# Worktrees not modified in this many days are eligible for removal.
cleanup_threshold_days = 30

# Repositories are added automatically by 'repo add'.
# You generally don't need to edit this section by hand.
[repositories.my-app]
repo_url = "git@github.com:user/my-app.git"
default_branch = "main"

  [repositories.my-app.worktrees.main]
  pinned = true

  [repositories.my-app.worktrees.feature-login]
  pinned = false
```
