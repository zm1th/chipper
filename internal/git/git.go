package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%s", strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func IsInsideRepo() bool {
	_, err := run("rev-parse", "--git-dir")
	return err == nil
}

func Root() (string, error) {
	return run("rev-parse", "--show-toplevel")
}

func CurrentBranch() (string, error) {
	return run("rev-parse", "--abbrev-ref", "HEAD")
}

func IsOnBranch(branch string) (bool, error) {
	current, err := CurrentBranch()
	if err != nil {
		return false, err
	}
	return current == branch, nil
}

func HasRemote() bool {
	out, err := run("remote")
	return err == nil && strings.TrimSpace(out) != ""
}

func IsUpToDate(branch string) (bool, error) {
	if _, err := run("fetch", "--quiet"); err != nil {
		return false, fmt.Errorf("git fetch failed: %w", err)
	}
	local, err := run("rev-parse", branch)
	if err != nil {
		return false, err
	}
	remote, err := run("rev-parse", "origin/"+branch)
	if err != nil {
		// No remote tracking branch — treat as up to date
		return true, nil
	}
	return local == remote, nil
}

func CreateAndCheckoutBranch(name string) error {
	_, err := run("checkout", "-b", name)
	return err
}

// ChangedFiles returns all changed or untracked files as reported by git status.
// Each entry is the file path relative to the repo root.
type FileStatus struct {
	Path   string
	Binary bool
}

func ChangedFiles() ([]FileStatus, error) {
	out, err := run("status", "--porcelain")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	var files []FileStatus
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		// Handle renames: "old -> new"
		if strings.Contains(path, " -> ") {
			parts := strings.SplitN(path, " -> ", 2)
			path = parts[1]
		}
		files = append(files, FileStatus{
			Path:   path,
			Binary: isBinary(path),
		})
	}
	return files, nil
}

func StageFiles(files []string) error {
	if len(files) == 0 {
		return nil
	}
	args := append([]string{"add", "--"}, files...)
	_, err := run(args...)
	return err
}

func Commit(message string) error {
	_, err := run("commit", "-m", message)
	return err
}

func Push(branch string) error {
	_, err := run("push", "-u", "origin", branch)
	return err
}

func isBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 8000)
	n, _ := f.Read(buf)
	for _, b := range buf[:n] {
		if b == 0 {
			return true
		}
	}
	return false
}
