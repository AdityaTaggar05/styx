package main

import (
	"os"

	"github.com/AdityaTaggar05/styx/internal/cli"
	_ "github.com/AdityaTaggar05/styx/internal/store/gdrive"
)

func main() {
	cmd := cli.RootCommand()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
