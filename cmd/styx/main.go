package main

import (
	"os"

	"github.com/AdityaTaggar05/styx/internal/cli"
)

func main() {
	cmd := cli.NewRootCommand()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
