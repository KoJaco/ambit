package core

import (
	"os"
	"path/filepath"
	"strings"
)

// IgnoreEntries are the exact gitignore lines ambit init ensures.
var IgnoreEntries = []string{
	".arch/local.json",
	".arch/.cache/",
	".arch/.proposals/",
}

// Init scaffolds .arch in dir. It creates missing files and appends ignore
// entries that are not already present. It does not overwrite an existing model
// and it does not install a git hook.
func Init(dir string) error {
	if dir == "" {
		dir = "."
	}
	arch := filepath.Join(dir, ".arch")
	if err := os.MkdirAll(filepath.Join(arch, "nodes"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(arch, ".cache"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(arch, ".proposals"), 0o755); err != nil {
		return err
	}
	indexPath := filepath.Join(arch, "index.json")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		if err := writeNewIndex(indexPath); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	configPath := filepath.Join(arch, "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := writeAtomic(configPath, []byte("{}\n")); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return ensureGitignore(dir)
}

func ensureGitignore(dir string) error {
	path := filepath.Join(dir, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	existing := map[string]bool{}
	if len(data) > 0 {
		for _, line := range strings.Split(string(data), "\n") {
			existing[strings.TrimRight(line, "\r")] = true
		}
	}
	var add []string
	for _, entry := range IgnoreEntries {
		if !existing[entry] {
			add = append(add, entry)
		}
	}
	if len(add) == 0 {
		return nil
	}
	var b strings.Builder
	if len(data) == 0 {
		for _, entry := range add {
			b.WriteString(entry)
			b.WriteByte('\n')
		}
		return os.WriteFile(path, []byte(b.String()), 0o644)
	}
	if !strings.HasSuffix(string(data), "\n") {
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	for _, entry := range add {
		b.WriteString(entry)
		b.WriteByte('\n')
	}
	return os.WriteFile(path, append(data, []byte(b.String())...), 0o644)
}
