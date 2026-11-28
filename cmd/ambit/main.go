// Command ambit is the ambit CLI.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/KoJaco/ambit/internal/core"
	"github.com/KoJaco/ambit/internal/httpapi"
	"github.com/KoJaco/ambit/internal/mcp"
)

// version is overridden at build time: -ldflags "-X main.version=1.2.3".
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(1)
	}
	switch os.Args[1] {
	case "version":
		if len(os.Args) != 2 {
			usage(os.Stderr)
			os.Exit(1)
		}
		fmt.Printf("ambit %s\n", version)
	case "init":
		os.Exit(cmdInit(os.Args[2:], os.Stdin, isTerminal(os.Stdin), os.Stdout, os.Stderr))
	case "method":
		os.Exit(cmdMethod(".", os.Args[2:], os.Stdout, os.Stderr))
	case "brief":
		os.Exit(cmdBrief(".", os.Args[2:], os.Stdout, os.Stderr))
	case "stage":
		os.Exit(cmdStage(".", os.Args[2:], os.Stdout, os.Stderr))
	case "check-model":
		os.Exit(cmdCheckModel(".", os.Args[2:], os.Stdout, os.Stderr))
	case "check":
		if len(os.Args) != 2 {
			usage(os.Stderr)
			os.Exit(1)
		}
		report, warnings, err := core.Check(".")
		for _, w := range warnings {
			fmt.Fprintln(os.Stderr, w)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "ambit check: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(report)
	case "drift":
		strict := false
		var opts core.DriftOptions
		opts.Root = "."
		for args := os.Args[2:]; len(args) > 0; args = args[1:] {
			switch {
			case args[0] == "--strict":
				strict = true
			case (args[0] == "--node" || args[0] == "--worktree") && len(args) > 1:
				if args[0] == "--node" {
					opts.NodeID = core.NodeID(args[1])
				} else {
					opts.FilesDir = args[1]
				}
				args = args[1:]
			default:
				usage(os.Stderr)
				os.Exit(1)
			}
		}
		idx, err := core.Open(".")
		if err != nil {
			fmt.Fprintf(os.Stderr, "ambit drift: %v\n", err)
			os.Exit(1)
		}
		report, err := core.DriftWith(idx, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ambit drift: %v\n", err)
			os.Exit(1)
		}
		if text := core.FormatDrift(report); text != "" {
			fmt.Print(text)
			if strict {
				os.Exit(1)
			}
		}
	case "config":
		os.Exit(cmdConfig(".", os.Args[2:], os.Stdout, os.Stderr))
	case "hook":
		if len(os.Args) != 3 || os.Args[2] != "install" {
			usage(os.Stderr)
			os.Exit(1)
		}
		if err := core.InstallHook("."); err != nil {
			fmt.Fprintf(os.Stderr, "ambit hook install: %v\n", err)
			os.Exit(1)
		}
	case "mcp":
		dir, opts, err := parseMCPArgs(os.Args[2:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "ambit mcp: %v\n", err)
			usage(os.Stderr)
			os.Exit(1)
		}
		if err := mcp.RunWith(context.Background(), dir, opts); err != nil && err != context.Canceled {
			fmt.Fprintf(os.Stderr, "ambit mcp: %v\n", err)
			if strings.Contains(err.Error(), ".arch directory is missing") {
				fmt.Fprintln(os.Stderr, "ambit mcp: run `ambit init` in the project root (or pass the directory: ambit mcp [dir]).")
			}
			os.Exit(1)
		}
	case "start":
		addr, err := parseStartArgs(os.Args[2:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "ambit start: %v\n", err)
			usage(os.Stderr)
			os.Exit(1)
		}
		if err := httpapi.ValidateAddr(addr); err != nil {
			fmt.Fprintf(os.Stderr, "ambit start: %v\n", err)
			os.Exit(1)
		}
		idx, err := core.Open(".")
		if err != nil {
			fmt.Fprintf(os.Stderr, "ambit start: %v\n", err)
			os.Exit(1)
		}
		if warning := core.IgnoreWarning("."); warning != "" {
			fmt.Fprintln(os.Stderr, warning)
		}
		if err := httpapi.ListenAndServe(addr, idx); err != nil {
			fmt.Fprintf(os.Stderr, "ambit start: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "ambit: unknown command %q\n", os.Args[1])
		usage(os.Stderr)
		os.Exit(1)
	}
}

// parseMCPArgs parses `ambit mcp [dir] [--role architect]`. The default role registers no
// accept_proposal tool.
func parseMCPArgs(args []string) (string, mcp.Options, error) {
	dir := "."
	haveDir := false
	var opts mcp.Options
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--role" || strings.HasPrefix(a, "--role="):
			role := strings.TrimPrefix(a, "--role=")
			if a == "--role" {
				if i+1 >= len(args) {
					return "", opts, fmt.Errorf("--role needs a value")
				}
				i++
				role = args[i]
			}
			if role != mcp.RoleArchitect {
				return "", opts, fmt.Errorf("unknown role %q (only %q is supported)", role, mcp.RoleArchitect)
			}
			opts.Role = role
		case strings.HasPrefix(a, "-"):
			return "", opts, fmt.Errorf("unknown flag %q", a)
		case haveDir:
			return "", opts, fmt.Errorf("unexpected argument %q", a)
		default:
			dir, haveDir = a, true
		}
	}
	return dir, opts, nil
}

func parseStartArgs(args []string) (string, error) {
	if len(args) == 0 {
		return httpapi.DefaultAddr, nil
	}
	if len(args) == 2 && args[0] == "--addr" && args[1] != "" {
		return args[1], nil
	}
	return "", fmt.Errorf("usage: ambit start [--addr %s]", httpapi.DefaultAddr)
}

func usage(w *os.File) {
	fmt.Fprintf(w, "usage: ambit init [dir] [--method id] [--scope whole|partial] [--part name=glob,glob] [--npx] [--no-mcp]\n       ambit method list|show [id]|set <id>|export-skill [--target claude|cursor] [--out path|-] [--force]\n       ambit stage [show|set <id>]\n       ambit brief export [--node id|--all] [--out dir] [--force]\n       ambit check-model [--strict] [--drift|--no-drift] [--all]\n       ambit check\n       ambit drift [--strict] [--node id] [--worktree path]\n       ambit config drift-ignore add <glob>\n       ambit hook install\n       ambit mcp [dir] [--role architect]\n       ambit start [--addr %s]\n       ambit version\n\nScaffold a .arch model with init (prompts for a workflow method on a terminal; also writes .mcp.json unless --no-mcp). method and stage show or change the workflow method and project stage. brief export writes each node's agent brief to .arch/briefs/<node>.md. check-model lints the model against the method (--strict exits 1 on enforced findings). check reports files outside the assigned scope.\ndrift reports files no node owns, globs matching no files and overlapping ownership (--strict exits 1 on findings; --node limits it to one node; --worktree reads files from a git worktree of this repository).\nmcp serves the MCP tool surface over stdio for coding harnesses (--role architect also registers accept_proposal, which only works when a human has turned on proposal_policy.auto_accept in the build stage).\nstart serves the local HTTP API on a loopback address.\n", httpapi.DefaultAddr)
}
