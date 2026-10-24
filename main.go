package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/tbeijen/secretd/internal/cmd"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "exec":
		err = cmd.Exec(os.Args[2:])
	case "context":
		err = cmd.ContextCmd(os.Args[2:], os.Stdout)
	case "version":
		fmt.Println(version)
	case "--help", "-h", "help":
		usage()
	default:
		err = fmt.Errorf("unknown command %q, see secretd help", os.Args[1])
	}

	var exitErr *cmd.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.Code)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "secretd: %s\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `Usage: secretd <command> [args]

Commands:
  exec [--context X] -- <cmd>...   Mint credentials and run <cmd> with them
  context show [--context X]       Show the context and profiles for the cwd
  context list                     List all contexts
  version                          Print version

In <cmd>, @SECRETD_DIR@ is replaced with the session dir.

Environment:
  SECRETD_CONFIG_DIR   Config directory (default: ~/.config/secretd)
`)
}
