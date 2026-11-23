package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/KoJaco/ambit/internal/core"
)

// parseInitArgs parses `ambit init [dir] [--method id]` (flag and dir in any order).
func parseInitArgs(args []string) (dir, method string, err error) {
	dir = "."
	haveDir := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--method":
			if i+1 >= len(args) || args[i+1] == "" {
				return "", "", fmt.Errorf("--method needs a method id")
			}
			method = args[i+1]
			i++
		case strings.HasPrefix(a, "--method="):
			method = strings.TrimPrefix(a, "--method=")
			if method == "" {
				return "", "", fmt.Errorf("--method needs a method id")
			}
		case strings.HasPrefix(a, "-"):
			return "", "", fmt.Errorf("unknown flag %q", a)
		case haveDir:
			return "", "", fmt.Errorf("unexpected argument %q", a)
		default:
			dir, haveDir = a, true
		}
	}
	return dir, method, nil
}

// isTerminal reports whether f is a character device (an interactive terminal).
func isTerminal(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

// promptMethod asks which method to use and returns its id. Empty input picks freeform.
func promptMethod(in io.Reader, out io.Writer, methods []core.MethodSummary, highlight bool) (string, error) {
	fmt.Fprintln(out, "Choose a workflow method:")
	for i, m := range methods {
		line := fmt.Sprintf("  %d) %s - %s", i+1, m.ID, m.Name)
		if m.ID == core.FreeformMethod {
			line += " (default)"
			if highlight {
				line = "\x1b[1m" + line + "\x1b[0m"
			}
		}
		fmt.Fprintln(out, line)
	}
	fmt.Fprint(out, "Method [freeform]: ")
	text, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && text == "" {
		return core.FreeformMethod, nil
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return core.FreeformMethod, nil
	}
	if n, convErr := strconv.Atoi(text); convErr == nil {
		if n >= 1 && n <= len(methods) {
			return methods[n-1].ID, nil
		}
		return "", fmt.Errorf("no method number %d", n)
	}
	for _, m := range methods {
		if m.ID == text {
			return m.ID, nil
		}
	}
	return "", fmt.Errorf("unknown method %q", text)
}

// cmdInit scaffolds .arch and optionally selects a method. interactive enables the prompt.
func cmdInit(args []string, stdin io.Reader, interactive bool, stdout, stderr io.Writer) int {
	dir, method, err := parseInitArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "ambit init: %v\n", err)
		return 2
	}
	if err := core.Init(dir); err != nil {
		fmt.Fprintf(stderr, "ambit init: %v\n", err)
		return 1
	}
	if method == "" && interactive {
		idx, err := core.Open(dir)
		if err != nil {
			fmt.Fprintf(stderr, "ambit init: %v\n", err)
			return 1
		}
		method, err = promptMethod(stdin, stdout, idx.ListMethods(), true)
		if err != nil {
			fmt.Fprintf(stderr, "ambit init: %v\n", err)
			return 1
		}
	}
	if method != "" {
		idx, err := core.Open(dir)
		if err != nil {
			fmt.Fprintf(stderr, "ambit init: %v\n", err)
			return 1
		}
		res, err := idx.SetMethod(method)
		if err != nil {
			fmt.Fprintf(stderr, "ambit init: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "method: %s\n", res.Method)
		if res.Stage != "" {
			fmt.Fprintf(stdout, "stage: %s\n", res.Stage)
		}
	}
	fmt.Fprintln(stdout, "Run `ambit hook install` to install a pre-commit hook. ambit init does not install one.")
	return 0
}

const methodUsage = "usage: ambit method list\n       ambit method show [id]\n       ambit method set <id>\n       ambit method export-skill [--target claude|cursor] [--out path|-]\n"

func cmdMethod(dir string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, methodUsage)
		return 2
	}
	idx, err := core.Open(dir)
	if err != nil {
		fmt.Fprintf(stderr, "ambit method: %v\n", err)
		return 1
	}
	for _, w := range idx.Warnings() {
		fmt.Fprintln(stderr, w.String())
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			fmt.Fprint(stderr, methodUsage)
			return 2
		}
		for _, m := range idx.ListMethods() {
			mark := " "
			if m.Active {
				mark = "*"
			}
			fmt.Fprintf(stdout, "%s %-14s %-14s %-8s %s\n", mark, m.ID, m.Name, m.Source, m.Description)
		}
	case "show":
		if len(args) > 2 {
			fmt.Fprint(stderr, methodUsage)
			return 2
		}
		m := idx.ActiveMethod()
		if len(args) == 2 {
			if m, err = idx.MethodByID(args[1]); err != nil {
				fmt.Fprintf(stderr, "ambit method: %v\n", err)
				return 1
			}
		}
		showMethod(stdout, m, idx.ActiveMethod().ID == m.ID, idx.EffectiveStage())
	case "set":
		if len(args) != 2 {
			fmt.Fprint(stderr, methodUsage)
			return 2
		}
		res, err := idx.SetMethod(args[1])
		if err != nil {
			fmt.Fprintf(stderr, "ambit method: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "method: %s\n", res.Method)
		switch {
		case res.StageReset && res.Stage != "":
			fmt.Fprintf(stdout, "stage reset from %q to %q (not valid in this method)\n", res.PreviousStage, res.Stage)
		case res.StageReset:
			fmt.Fprintf(stdout, "stage %q cleared (this method has no stages)\n", res.PreviousStage)
		case res.Stage != "":
			fmt.Fprintf(stdout, "stage: %s\n", res.Stage)
		}
	case "export-skill":
		return exportSkill(dir, idx.ActiveMethod(), args[1:], stdout, stderr)
	default:
		fmt.Fprint(stderr, methodUsage)
		return 2
	}
	return 0
}

func showMethod(w io.Writer, m *core.Method, active bool, stage string) {
	fmt.Fprintf(w, "id: %s\nname: %s\nsource: %s\nactive: %t\n", m.ID, m.Name, m.Source, active)
	if m.Description != "" {
		fmt.Fprintf(w, "description: %s\n", m.Description)
	}
	if len(m.Stages) == 0 {
		fmt.Fprintln(w, "stages: none")
	} else {
		ids := make([]string, len(m.Stages))
		for i, s := range m.Stages {
			ids[i] = s.ID
		}
		fmt.Fprintf(w, "stages: %s (initial: %s)\n", strings.Join(ids, ", "), m.InitialStage)
		if active {
			fmt.Fprintf(w, "stage: %s\n", stage)
		}
		for _, s := range m.Stages {
			if refused := m.Gates.Status[s.ID]; len(refused) > 0 {
				fmt.Fprintf(w, "gate: while %s, agents may not set status %s\n", s.ID, strings.Join(refused, ", "))
			}
		}
	}
	for _, id := range core.LintRuleIDs() {
		if set := m.Lint[id]; set.Severity == core.LintAdvisory || set.Severity == core.LintEnforced {
			fmt.Fprintf(w, "lint %s: %s\n", id, set.Severity)
		}
	}
	fmt.Fprintf(w, "check include_drift: %t\n\nguidance:\n%s\n", m.Check.IncludeDrift, m.Guidance)
}

const generatedNote = "Generated by `ambit method export-skill`. Do not edit. The ambit MCP server is the source of truth: call get_started for the live guidance and regenerate this file after changing the method."

// formatSkill renders the method as a Claude Code skill or a Cursor rule.
func formatSkill(m *core.Method, target string) (string, error) {
	desc := strconv.Quote(fmt.Sprintf("Ambit workflow method %q (%s). Use when modelling or building with ambit.", m.Name, m.ID))
	var b strings.Builder
	switch target {
	case "claude":
		fmt.Fprintf(&b, "---\nname: ambit-method\ndescription: %s\n---\n", desc)
	case "cursor":
		fmt.Fprintf(&b, "---\ndescription: %s\nalwaysApply: false\n---\n", desc)
	default:
		return "", fmt.Errorf("unknown target %q (want claude or cursor)", target)
	}
	fmt.Fprintf(&b, "\n<!-- %s -->\n\n# Ambit workflow: %s\n\n%s\n", generatedNote, m.Name, m.Guidance)
	return b.String(), nil
}

func exportSkill(dir string, m *core.Method, args []string, stdout, stderr io.Writer) int {
	target, out := "claude", ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--target" && i+1 < len(args):
			target = args[i+1]
			i++
		case args[i] == "--out" && i+1 < len(args):
			out = args[i+1]
			i++
		default:
			fmt.Fprint(stderr, methodUsage)
			return 2
		}
	}
	text, err := formatSkill(m, target)
	if err != nil {
		fmt.Fprintf(stderr, "ambit method export-skill: %v\n", err)
		return 2
	}
	if out == "-" {
		fmt.Fprint(stdout, text)
		return 0
	}
	if out == "" {
		out = filepath.Join(dir, ".claude", "skills", "ambit-method", "SKILL.md")
		if target == "cursor" {
			out = filepath.Join(dir, ".cursor", "rules", "ambit-method.mdc")
		}
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		fmt.Fprintf(stderr, "ambit method export-skill: %v\n", err)
		return 1
	}
	if err := os.WriteFile(out, []byte(text), 0o644); err != nil {
		fmt.Fprintf(stderr, "ambit method export-skill: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote %s\n", out)
	return 0
}

const stageUsage = "usage: ambit stage [show]\n       ambit stage set <id>\n"

func cmdStage(dir string, args []string, stdout, stderr io.Writer) int {
	idx, err := core.Open(dir)
	if err != nil {
		fmt.Fprintf(stderr, "ambit stage: %v\n", err)
		return 1
	}
	m := idx.ActiveMethod()
	switch {
	case len(args) == 0 || (len(args) == 1 && args[0] == "show"):
		if len(m.Stages) == 0 {
			fmt.Fprintf(stdout, "method %s has no stages\n", m.ID)
			return 0
		}
		ids := make([]string, len(m.Stages))
		for i, s := range m.Stages {
			ids[i] = s.ID
		}
		fmt.Fprintf(stdout, "method: %s\nstage: %s\nstages: %s\n", m.ID, idx.EffectiveStage(), strings.Join(ids, ", "))
	case len(args) == 2 && args[0] == "set":
		if err := idx.SetStage(args[1]); err != nil {
			fmt.Fprintf(stderr, "ambit stage: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "stage: %s\n", args[1])
	default:
		fmt.Fprint(stderr, stageUsage)
		return 2
	}
	return 0
}

func cmdCheckModel(dir string, args []string, stdout, stderr io.Writer) int {
	var strict bool
	var opts core.CheckModelOptions
	for _, a := range args {
		switch a {
		case "--strict":
			strict = true
		case "--drift":
			opts.IncludeDrift = true
		case "--all":
			opts.AllRules = true
		default:
			fmt.Fprintln(stderr, "usage: ambit check-model [--strict] [--drift] [--all]")
			return 2
		}
	}
	idx, err := core.Open(dir)
	if err != nil {
		fmt.Fprintf(stderr, "ambit check-model: %v\n", err)
		return 1
	}
	res, err := core.CheckModel(idx, opts)
	if err != nil {
		fmt.Fprintf(stderr, "ambit check-model: %v\n", err)
		return 1
	}
	fmt.Fprint(stdout, formatLint(res))
	if res.Drift != nil {
		if text := core.FormatDrift(*res.Drift); text != "" {
			fmt.Fprint(stdout, "\ndrift:\n"+text)
		}
	}
	if res.DriftError != "" {
		fmt.Fprintf(stderr, "ambit check-model: drift: %s\n", res.DriftError)
	}
	if strict && !res.OK {
		return 1
	}
	return 0
}

func formatLint(res core.LintResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "method: %s", res.Method)
	if res.Stage != "" {
		fmt.Fprintf(&b, "  stage: %s", res.Stage)
	}
	b.WriteString("\n")
	for _, sev := range []string{core.LintEnforced, core.LintAdvisory} {
		var rows []core.LintFinding
		for _, f := range res.Findings {
			if f.Severity == sev {
				rows = append(rows, f)
			}
		}
		if len(rows) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n%s (%d):\n", sev, len(rows))
		for _, f := range rows {
			where := string(f.NodeID)
			if where == "" {
				where = string(f.RelationshipID)
			}
			if where != "" {
				where += ": "
			}
			fmt.Fprintf(&b, "  [%s] %s%s\n", f.Rule, where, f.Message)
		}
	}
	if res.Counts.Total == 0 {
		b.WriteString("no findings\n")
	}
	return b.String()
}
