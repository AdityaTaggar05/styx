package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/AdityaTaggar05/styx/internal/config"
	styxErrors "github.com/AdityaTaggar05/styx/internal/errors"
	"github.com/AdityaTaggar05/styx/internal/store"
)

func setupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Guided setup to bootstrap the Styx configuration",
		Long: `Setup creates the global configuration directory and file,
then guides you through configuring available cloud store backends.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			expanded, err := config.ExpandPath(configPath)
			if err != nil {
				return err
			}

			if _, err := os.Stat(expanded); err == nil {
				fmt.Printf("Global config already exists at %s\n", expanded)
				fmt.Println("Run `styx auth login --store <name>` to authenticate.")
				return nil
			}

			cfg := config.DefaultGlobalConfig()

			available := store.List()
			if len(available) == 0 {
				fmt.Println("No store backends found. Rebuild styx with at least one backend.")
				return nil
			}

			globalDir, err := config.ExpandPath(config.GlobalDir)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(globalDir, 0o755); err != nil {
				return fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
			}

			if err := config.Validate(cfg); err != nil {
				return err
			}

			if err := config.SaveGlobalConfig(configPath, cfg); err != nil {
				return err
			}

			fmt.Printf("\nSetup complete! Config written to %s\n", expanded)
			fmt.Println("\nAvailable store backends:")
			for _, name := range available {
				fmt.Printf("  %s\n", name)
			}
			fmt.Println()
			for _, name := range available {
				fmt.Printf("Run `styx auth login --store %s` to authenticate.\n", name)
			}

			return nil
		},
	}
}
