package main

import (
	"os"

	"github.com/paulssonkalle/worktree-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
