// Command codex-handoff provides a focused terminal workflow for handing
// selected Codex projects and their sessions to another user. Project source
// code and Codex authentication data are never included in the bundle.
package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/ahmojo/codex-claude-transfer/internal/cli"
)

func main() {
	args := os.Args[1:]
	if len(args) == 1 && (args[0] == "version" || args[0] == "--version" || args[0] == "-V") {
		fmt.Printf("codex-handoff %s (%s/%s, %s)\n", cli.VersionString(), runtime.GOOS, runtime.GOARCH, runtime.Version())
		return
	}
	if len(args) == 0 {
		args = []string{"handoff", "export"}
	} else if len(args) == 1 {
		switch args[0] {
		case "export", "import", "inspect":
			args = []string{"handoff", args[0]}
		}
	} else if args[0] == "export" || args[0] == "import" {
		if !hasArg(args, "--include-archived") {
			args = append(args, "--include-archived")
		}
		if args[0] == "export" && !hasArg(args, "--redact") && !hasArg(args, "--allow-secrets") {
			args = append(args, "--redact")
		}
	}
	os.Exit(cli.Run(args, os.Stdout, os.Stderr))
}

func hasArg(args []string, name string) bool {
	for _, arg := range args {
		if arg == name {
			return true
		}
	}
	return false
}
