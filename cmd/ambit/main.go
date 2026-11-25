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
		if len(os.Args) > 3 {
			usage(os.Stderr)
			os.Exit(1)
		}
		dir := "."
		if len(os.Args) == 3 {
			dir = os.Args[2]
		}
		if err := mcp.Run(context.Background(), dir); err != nil && err != context.Canceled {
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
	fmt.Fprintf(w, "usage: ambit init [dir] [--method id]\n       ambit method list|show [id]|set <id>|export-skill [--target claude|cursor] [--out path|-] [--force]\n       ambit stage [show|set <id>]\n       ambit check-model [--strict] [--drift|--no-drift] [--all]\n       ambit check\n       ambit drift [--strict] [--node id] [--worktree path]\n       ambit hook install\n       ambit mcp [dir]\n       ambit start [--addr %s]\n       ambit version\n\nScaffold a .arch model with init (prompts for a workflow method on a terminal). method and stage show or change the workflow method and project stage. check-model lints the model against the method (--strict exits 1 on enforced findings). check reports files outside the assigned scope.\ndrift reports files no node owns, globs matching no files and overlapping ownership (--strict exits 1 on findings; --node limits it to one node; --worktree reads files from a git worktree of this repository).\nmcp serves the MCP tool surface over stdio for coding harnesses.\nstart serves the local HTTP API on a loopback address.\n", httpapi.DefaultAddr)
}
