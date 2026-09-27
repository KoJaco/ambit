// Command ambit is the ambit CLI.
package main

import (
	"fmt"
	"os"

	"github.com/KoJaco/ambit/internal/core"
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
	default:
		fmt.Fprintf(os.Stderr, "ambit: unknown command %q\n", os.Args[1])
		usage(os.Stderr)
		os.Exit(1)
	}
}

func usage(w *os.File) {
	fmt.Fprintf(w, "usage: ambit init [dir]\n\nScaffold a .arch model in dir (default: the current directory).\n")
}
