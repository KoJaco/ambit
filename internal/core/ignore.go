package core

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// IgnoreWarning runs git check-ignore on IgnoreEntries. An empty string means
// every path is ignored. A warning does not change the caller's exit code.
// ambit start can call this without a second copy. A missing git repository
// is a warning that the paths were not confirmed ignored.
func IgnoreWarning(dir string) string {
	if dir == "" {
		dir = "."
	}
	var notIgnored []string
	for _, entry := range IgnoreEntries {
		cmd := exec.Command("git", "check-ignore", "-q", "--", entry)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err == nil {
			continue
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			notIgnored = append(notIgnored, entry)
			continue
		}
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Sprintf("warning: could not confirm git is ignoring %s: %s", strings.Join(IgnoreEntries, ", "), msg)
	}
	if len(notIgnored) == 0 {
		return ""
	}
	return "warning: git is not ignoring " + strings.Join(notIgnored, ", ")
}
