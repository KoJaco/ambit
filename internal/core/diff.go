package core

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// DiffNames lists paths that differ from HEAD, staged and unstaged.
// Untracked paths are omitted. The same command is used for ambit check
// and the pre-commit hook: git diff --name-only --no-renames HEAD.
// A repository with no commit returns an error rather than an empty list.
func DiffNames(dir string) ([]string, error) {
	if dir == "" {
		dir = "."
	}
	cmd := exec.Command("git", "diff", "--name-only", "--no-renames", "HEAD")
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("git diff --name-only --no-renames HEAD: %s", msg)
	}
	var files []string
	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		files = append(files, toSlash(line))
	}
	return files, nil
}
