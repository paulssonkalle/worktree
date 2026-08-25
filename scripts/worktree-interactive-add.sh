#!/usr/bin/env bash
set -euo pipefail

# Ensure mise-managed tools (worktree, sesh, fzf) are on PATH
eval "$(mise activate bash 2>/dev/null)" || true

# Step 1: Pick a repo
REPO=$(worktree repo list --names-only | fzf-tmux -p 60%,50% \
  --no-sort --border-label ' Pick repo ' --prompt '📦 ' \
  --header 'Select a repository')

if [[ -z "$REPO" ]]; then
  exit 0
fi

# Step 2: Fetch latest and get default branch
worktree fetch "$REPO" > /dev/null 2>&1
DEFAULT_BRANCH=$(worktree repo list 2>/dev/null | awk -v repo="$REPO" 'NR > 1 && $1 == repo {print $3}')

# Step 3: Pick or type a branch name
# Use --print-query so typed text is returned even with no match
BRANCH_SELECTION=$(worktree branches --no-fetch "$REPO" | fzf-tmux -p 60%,50% \
  --no-sort --border-label ' Pick or type branch ' --prompt '🌿 ' \
  --header 'Select existing branch or type new name' \
  --print-query \
  || true)

# fzf --print-query outputs: line 1 = query, line 2 = selected match (if any)
QUERY=$(echo "$BRANCH_SELECTION" | sed -n '1p')
MATCH=$(echo "$BRANCH_SELECTION" | sed -n '2p')

BRANCH="${MATCH:-$QUERY}"

if [[ -z "$BRANCH" ]]; then
  exit 0
fi

# Step 4: Base branch prompt with default prefilled
BASE_BRANCH=$(echo "" | fzf-tmux -p 60%,20% \
  --border-label ' Base branch ' --prompt '🏠 ' \
  --header "Base branch (default: $DEFAULT_BRANCH)" \
  --print-query \
  --query "$DEFAULT_BRANCH" \
  || true)

# Take the query (first line) as the base branch
BASE_BRANCH=$(echo "$BASE_BRANCH" | sed -n '1p')
BASE_BRANCH="${BASE_BRANCH:-$DEFAULT_BRANCH}"

# Step 5: Create the worktree
worktree add "$REPO" "$BRANCH" --base "$BASE_BRANCH" --no-fetch > /dev/null 2>&1

# Step 6: Get worktree path and connect via sesh
# The worktree path follows the pattern: <repo-dir>/<sanitized-branch>
WORKTREE_PATH=$(worktree list "$REPO" | awk -v branch="$BRANCH" '$3 == branch {print $NF}')

if [[ -n "$WORKTREE_PATH" ]] && command -v sesh &>/dev/null; then
  sesh connect "$WORKTREE_PATH" > /dev/null 2>&1
fi
