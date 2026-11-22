package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// hookScript runs `ambit check` from PATH, falling back to npx. It embeds no
// absolute path, so it survives upgrades and moves of the installing binary.
const hookScript = `#!/bin/sh
if command -v ambit >/dev/null 2>&1; then
  exec ambit check
else
  exec npx --yes @kojaco/ambit check
fi
`

// InstallHook writes a pre-commit hook that runs ` + "`ambit check`" + ` via PATH or
// npx. An existing hook is left unchanged.
func InstallHook(dir string) error {
	if dir == "" {
		dir = "."
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
	return os.WriteFile(path, []byte(hookScript), 0o755)
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
