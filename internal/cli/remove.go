package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/AdityaTaggar05/styx/internal/config"
)

func removeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <path>",
		Short: "Remove a directory from the registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadGlobalConfig(configPath)
			if err != nil {
				return err
			}

			if err := config.RemoveDirectory(cfg, args[0]); err != nil {
				return err
			}

			if err := config.SaveGlobalConfig(configPath, cfg); err != nil {
				return err
			}

			fmt.Printf("Removed %s from registry.\n", args[0])
			return nil
		},
	}
}
