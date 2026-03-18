package zoxide

import "os/exec"

// Add registers a path with zoxide by running "zoxide add <path>".
func Add(path string) error {
	return exec.Command("zoxide", "add", path).Run()
}

// Remove deregisters a path from zoxide by running "zoxide remove <path>".
func Remove(path string) error {
	return exec.Command("zoxide", "remove", path).Run()
}

// IsAvailable reports whether the zoxide binary is on $PATH.
func IsAvailable() bool {
	_, err := exec.LookPath("zoxide")
	return err == nil
}
