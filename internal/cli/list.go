package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/AdityaTaggar05/styx/internal/config"
)

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all registered directories",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadGlobalConfig(configPath)
			if err != nil {
				return err
			}

			if len(cfg.Directories) == 0 {
				fmt.Println("No directories registered. Use `styx init` to add one.")
				return nil
			}

			fmt.Println("Registered directories:")
			for _, dir := range cfg.Directories {
				fmt.Printf("  %s\n", dir)
			}
			return nil
		},
	}
}
