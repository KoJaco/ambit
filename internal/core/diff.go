package core

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// DiffNames lists paths that differ from HEAD, staged and unstaged.
// Untracked paths are omitted. The same command is used for ambit check
// and the pre-commit hook: git diff --name-only --no-renames HEAD (or the
// empty tree when HEAD does not exist yet).
// A repository with no commit yet is diffed against the empty tree, so
// everything staged or on disk and tracked counts as changed.
func DiffNames(dir string) ([]string, error) {
	if dir == "" {
		dir = "."
	}
	base := "HEAD"
	probe := exec.Command("git", "rev-parse", "--verify", "--quiet", "HEAD")
	probe.Dir = dir
	if err := probe.Run(); err != nil {
		// Exit 1 means a valid repo without HEAD; anything else (for example
		// 128, not a repository) is left for the diff below to report.
		if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
			tree, terr := emptyTreeID(dir)
			if terr != nil {
				return nil, terr
			}
			base = tree
		}
	}
	cmd := exec.Command("git", "diff", "--name-only", "--no-renames", base)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("git diff --name-only --no-renames %s: %s", base, msg)
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

// emptyTreeID returns the empty tree object id for the repository's hash
// algorithm (SHA-1 and SHA-256 repositories differ).
func emptyTreeID(dir string) (string, error) {
	cmd := exec.Command("git", "hash-object", "-t", "tree", "--stdin")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader("")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git hash-object -t tree: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
