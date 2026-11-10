package utils

import (
	"os"
	"path/filepath"
)

// EnsureWorkDir switches the working directory to the executable's directory
// when fsb.env is missing from the current directory but present next to the
// binary. It returns the new directory, or "" when nothing changed.
func EnsureWorkDir() (string, error) {
	if _, err := os.Stat("fsb.env"); err == nil {
		return "", nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	dir := filepath.Dir(exe)
	if _, err := os.Stat(filepath.Join(dir, "fsb.env")); err != nil {
		return "", nil
	}
	if err := os.Chdir(dir); err != nil {
		return "", err
	}
	return dir, nil
}
