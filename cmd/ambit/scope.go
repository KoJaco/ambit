package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/KoJaco/ambit/internal/core"
)

// initScope holds the --scope and --part flags of ambit init.
type initScope struct {
	// Scope is "whole", "partial" or "" (not given).
	Scope string
	// Parts are the raw --part values (name=glob,glob).
	Parts []string
}

// splitInitScope removes --scope and --part (and their values) from args and returns them.
func splitInitScope(args []string) ([]string, initScope, error) {
	var rest []string
	var sc initScope
	for i := 0; i < len(args); i++ {
		a := args[i]
		value := func(flag string) (string, error) {
			if v, ok := strings.CutPrefix(a, flag+"="); ok {
				if v == "" {
					return "", fmt.Errorf("%s needs a value", flag)
				}
				return v, nil
			}
			if i+1 >= len(args) || args[i+1] == "" {
				return "", fmt.Errorf("%s needs a value", flag)
			}
			i++
			return args[i], nil
		}
		switch {
		case a == "--scope" || strings.HasPrefix(a, "--scope="):
			v, err := value("--scope")
			if err != nil {
				return nil, sc, err
			}
			if v != core.ScopeWhole && v != core.ScopePartial {
				return nil, sc, fmt.Errorf("--scope must be %s or %s, not %q", core.ScopeWhole, core.ScopePartial, v)
			}
			sc.Scope = v
		case a == "--part" || strings.HasPrefix(a, "--part="):
			v, err := value("--part")
			if err != nil {
				return nil, sc, err
			}
			sc.Parts = append(sc.Parts, v)
		default:
			rest = append(rest, a)
		}
	}
	if len(sc.Parts) > 0 {
		if sc.Scope == core.ScopeWhole {
			return nil, sc, fmt.Errorf("--part needs --scope partial")
		}
		sc.Scope = core.ScopePartial
	}
	return rest, sc, nil
}

// parsePartFlag parses "name=glob,glob" into a part. The id is the slug of the name. Commas
// inside {braces} belong to the glob.
func parsePartFlag(v string) (core.Part, error) {
	name, globs, ok := strings.Cut(v, "=")
	name = strings.TrimSpace(name)
	if !ok || name == "" || strings.TrimSpace(globs) == "" {
		return core.Part{}, fmt.Errorf("--part %q must look like name=glob,glob", v)
	}
	id := slugForPart(name)
	if id == "" {
		return core.Part{}, fmt.Errorf("--part %q: name has no letters or digits", v)
	}
	return core.Part{ID: id, Name: name, Include: splitGlobs(globs)}, nil
}

func slugForPart(name string) string {
	var b strings.Builder
	prev := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prev = false
		} else if !prev && b.Len() > 0 {
			b.WriteByte('-')
			prev = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func splitGlobs(s string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	out = append(out, strings.TrimSpace(s[start:]))
	var kept []string
	for _, g := range out {
		if g != "" {
			kept = append(kept, g)
		}
	}
	return kept
}

// promptScope asks whether the model covers the whole repository or only parts of it.
// Empty input or end of input picks whole.
func promptScope(in io.Reader, out io.Writer) (string, error) {
	fmt.Fprintln(out, "Model scope:")
	fmt.Fprintln(out, "  1) whole - the model describes the whole repository (default)")
	fmt.Fprintln(out, "  2) partial - the model describes only some parts of it; drift ignores the rest")
	fmt.Fprint(out, "Scope [whole]: ")
	text, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && text == "" {
		return core.ScopeWhole, nil
	}
	switch strings.TrimSpace(text) {
	case "", "1", core.ScopeWhole:
		return core.ScopeWhole, nil
	case "2", core.ScopePartial:
		return core.ScopePartial, nil
	}
	return "", fmt.Errorf("unknown scope %q", strings.TrimSpace(text))
}

// promptPart asks for a first part as name=glob,glob. Empty input skips it.
func promptPart(in io.Reader, out io.Writer) (string, error) {
	fmt.Fprint(out, "First part as name=glob,glob (empty to add parts later): ")
	text, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && text == "" {
		return "", nil
	}
	return strings.TrimSpace(text), nil
}
