package main

import (
	"fmt"
	"io"

	"github.com/KoJaco/ambit/internal/core"
)

const configUsage = "usage: ambit config drift-ignore add <glob>\n"

// cmdConfig handles `ambit config drift-ignore add <glob>`, which adds a glob to the
// drift_ignore list in .arch/config.json.
func cmdConfig(dir string, args []string, stdout, stderr io.Writer) int {
	if len(args) != 3 || args[0] != "drift-ignore" || args[1] != "add" {
		fmt.Fprint(stderr, configUsage)
		return 1
	}
	idx, err := core.Open(dir)
	if err != nil {
		fmt.Fprintf(stderr, "ambit config: %v\n", err)
		return 1
	}
	changed, err := idx.AddDriftIgnore(args[2])
	if err != nil {
		fmt.Fprintf(stderr, "ambit config: %v\n", err)
		return 1
	}
	if changed {
		fmt.Fprintf(stdout, "drift_ignore: added %s\n", args[2])
	} else {
		fmt.Fprintf(stdout, "drift_ignore: %s already present\n", args[2])
	}
	return 0
}
