package main

import (
	"fmt"
	"os"

	"github.com/pformoso-deus-ai/csk/internal/cli"
	"github.com/pformoso-deus-ai/csk/internal/exitcode"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "csk:", err)
		os.Exit(exitcode.From(err))
	}
}
