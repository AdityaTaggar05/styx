package cli

import (
	"slices"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/AdityaTaggar05/styx/internal/config"
	"github.com/AdityaTaggar05/styx/internal/store"
	"github.com/AdityaTaggar05/styx/internal/sync"
)

func syncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync [path]",
		Short: "Force an immediate sync of the given directory (or all)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadGlobalConfig(configPath)
			if err != nil {
				return err
			}

			if len(cfg.Directories) == 0 {
				fmt.Println("No directories registered. Use `styx init` to add one.")
				return nil
			}

			targets, err := resolveTargets(args, cfg)
			if err != nil {
				return err
			}

			for _, dir := range targets {
				localCfg, err := config.LoadLocalConfig(dir)
				if err != nil {
					fmt.Printf("skipping %s: %v\n", dir, err)
					continue
				}

				storeCfg := cfg.Stores[localCfg.Store]
				st, err := store.Get(localCfg.Store, storeConfigToMap(storeCfg))
				if err != nil {
					fmt.Printf("skipping %s: %v\n", dir, err)
					continue
				}

				authed, err := st.IsAuthenticated(context.Background())
				if err != nil || !authed {
					fmt.Printf("skipping %s: not authenticated with %s (run `styx auth login --store %s`)\n", dir, localCfg.Store, localCfg.Store)
					continue
				}

				fmt.Printf("Syncing %s → %s:%s\n", dir, localCfg.Store, localCfg.RemoteRoot)
				if err := sync.Sync(context.Background(), dir, localCfg, st); err != nil {
					fmt.Printf("sync failed for %s: %v\n", dir, err)
				}
			}

			return nil
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [path]",
		Short: "Show sync status of a directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadGlobalConfig(configPath)
			if err != nil {
				return err
			}

			if len(cfg.Directories) == 0 {
				fmt.Println("No directories registered. Use `styx init` to add one.")
				return nil
			}

			targets, err := resolveTargets(args, cfg)
			if err != nil {
				return err
			}

			for _, dir := range targets {
				localCfg, err := config.LoadLocalConfig(dir)
				if err != nil {
					fmt.Printf("skipping %s: %v\n", dir, err)
					continue
				}

				storeCfg := cfg.Stores[localCfg.Store]
				st, err := store.Get(localCfg.Store, storeConfigToMap(storeCfg))
				if err != nil {
					fmt.Printf("skipping %s: %v\n", dir, err)
					continue
				}

				authed, err := st.IsAuthenticated(context.Background())
				if err != nil || !authed {
					fmt.Printf("skipping %s: not authenticated (%s)\n", dir, localCfg.Store)
					continue
				}

				fmt.Printf("Status for %s → %s:%s\n", dir, localCfg.Store, localCfg.RemoteRoot)

				matcher := sync.NewMatcher(localCfg.Ignore)
				localFiles, err := sync.ScanLocal(dir, matcher)
				if err != nil {
					fmt.Printf("  scan error: %v\n", err)
					continue
				}

				remoteFiles, err := st.ListRecursive(context.Background(), localCfg.RemoteRoot)
				if err != nil {
					fmt.Printf("  remote list error: %v\n", err)
					continue
				}

				// Normalize remote paths
				prefix := ""
				if localCfg.RemoteRoot != "" && localCfg.RemoteRoot != "/" {
					prefix = localCfg.RemoteRoot + "/"
				}
				for i := range remoteFiles {
					if len(remoteFiles[i].Path) > len(prefix) && remoteFiles[i].Path[:len(prefix)] == prefix {
						remoteFiles[i].Path = remoteFiles[i].Path[len(prefix):]
					}
				}

				manifestPath := filepath.Join(dir, ".styx", "manifest.json")
				manifest, err := sync.LoadManifest(manifestPath)
				if err != nil {
					manifest = sync.NewManifest(localCfg.Store, localCfg.RemoteRoot)
				}

				actions, err := sync.Diff(dir, localFiles, manifest, remoteFiles)
				if err != nil {
					fmt.Printf("  diff error: %v\n", err)
					continue
				}

				var pushNew, pushUpdate, pullNew, pullUpdate, conflict, delLocal, clean int
				for _, a := range actions {
					switch a.Type {
					case sync.ActionPushNew:
						pushNew++
					case sync.ActionPushUpdate:
						pushUpdate++
					case sync.ActionPullNew:
						pullNew++
					case sync.ActionPullUpdate:
						pullUpdate++
					case sync.ActionConflict:
						conflict++
					case sync.ActionDeleteLocal:
						delLocal++
					case sync.ActionForgetManifest:
						clean++
					}
				}

				fmt.Printf("  to push:    %d new, %d updated\n", pushNew, pushUpdate)
				fmt.Printf("  to pull:    %d new, %d updated\n", pullNew, pullUpdate)
				fmt.Printf("  conflicts:  %d\n", conflict)
				fmt.Printf("  deletions:  %d local, %d tracked\n", delLocal, clean)
			}

			return nil
		},
	}
}

// resolveTargets returns the list of directories to operate on.
// If args has a path, it resolves `.` or relative paths and validates the dir
// is registered. If args is empty, returns all registered directories.
func resolveTargets(args []string, cfg *config.GlobalConfig) ([]string, error) {
	if len(args) == 0 {
		return cfg.Directories, nil
	}

	path := args[0]
	if path == "." {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getting current directory: %w", err)
		}
		path = cwd
	} else if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("resolving path %s: %w", path, err)
		}
		path = abs
	}

	// Verify it's registered
	found := slices.Contains(cfg.Directories, path)
	if !found {
		return nil, fmt.Errorf("%s is not a registered directory", path)
	}

	return []string{path}, nil
}
