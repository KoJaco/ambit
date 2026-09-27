// Command ambit is the ambit CLI.
package main

import (
	"fmt"
	"os"

	"github.com/KoJaco/ambit/internal/core"
	"github.com/KoJaco/ambit/internal/httpapi"
)

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(1)
	}
	switch os.Args[1] {
	case "init":
		if len(os.Args) > 3 {
			usage(os.Stderr)
			os.Exit(1)
		}
		dir := "."
		if len(os.Args) == 3 {
			dir = os.Args[2]
		}
		if err := core.Init(dir); err != nil {
			fmt.Fprintf(os.Stderr, "ambit init: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Run `ambit hook install` to install a pre-commit hook. ambit init does not install one.")
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
	case "hook":
		if len(os.Args) != 3 || os.Args[2] != "install" {
			usage(os.Stderr)
			os.Exit(1)
		}
		if err := core.InstallHook("."); err != nil {
			fmt.Fprintf(os.Stderr, "ambit hook install: %v\n", err)
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
	fmt.Fprintf(w, "usage: ambit init [dir]\n       ambit check\n       ambit hook install\n       ambit start [--addr %s]\n\nScaffold a .arch model with init. check reports files outside the assigned scope.\nstart serves the local HTTP API on a loopback address.\n", httpapi.DefaultAddr)
}
