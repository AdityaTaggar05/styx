package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/AdityaTaggar05/styx/internal/config"
)

func removeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <path>",
		Short: "Remove a directory from the registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			if path == "." {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("getting current directory: %w", err)
				}
				path = cwd
			} else if !filepath.IsAbs(path) {
				abs, err := filepath.Abs(path)
				if err != nil {
					return fmt.Errorf("resolving path %s: %w", path, err)
				}
				path = abs
			}

			cfg, err := config.LoadGlobalConfig(configPath)
			if err != nil {
				return err
			}

			if err := config.RemoveDirectory(cfg, path); err != nil {
				return err
			}

			if err := config.SaveGlobalConfig(configPath, cfg); err != nil {
				return err
			}

			// Clean up the .styx directory
			styxDir := filepath.Join(path, config.StyxDirName)
			if err := os.RemoveAll(styxDir); err != nil {
				return fmt.Errorf("removing .styx from %s: %w", path, err)
			}

			fmt.Printf("Removed %s from registry.\n", path)
			return nil
		},
	}
}
