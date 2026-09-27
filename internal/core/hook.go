package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// InstallHook writes a pre-commit hook that runs this ambit binary's check
// command. An existing hook is left unchanged.
func InstallHook(dir string) error {
	if dir == "" {
		dir = "."
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	hooks, err := gitPath(dir, "hooks")
	if err != nil {
		return err
	}
	path := filepath.Join(hooks, "pre-commit")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("refused to overwrite %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	script := "#!/bin/sh\nexec " + shellQuote(exe) + " check\n"
	return os.WriteFile(path, []byte(script), 0o755)
}

func gitPath(dir, name string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-path", name)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git rev-parse --git-path %s: %s", name, msg)
	}
	p := strings.TrimSpace(string(out))
	if p == "" {
		return "", fmt.Errorf("git rev-parse --git-path %s: empty path", name)
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	return p, nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
