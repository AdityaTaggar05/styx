package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"

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

				st, err := store.Get(localCfg.Store, cfg.Stores[localCfg.Store])
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
				if err := sync.Sync(context.Background(), dir, localCfg, st, cliLogger()); err != nil {
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

				st, err := store.Get(localCfg.Store, cfg.Stores[localCfg.Store])
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

				var pushNew, pushUpdate, pullNew, pullUpdate, restore, conflicts, delLocal, clean []string
				for _, a := range actions {
					switch a.Type {
					case sync.ActionPushNew:
						pushNew = append(pushNew, a.Path)
					case sync.ActionPushUpdate:
						pushUpdate = append(pushUpdate, a.Path)
					case sync.ActionPullNew:
						pullNew = append(pullNew, a.Path)
					case sync.ActionPullUpdate:
						pullUpdate = append(pullUpdate, a.Path)
					case sync.ActionRestoreLocal:
						restore = append(restore, a.Path)
					case sync.ActionConflict:
						conflicts = append(conflicts, a.Path)
					case sync.ActionDeleteLocal:
						delLocal = append(delLocal, a.Path)
					case sync.ActionForgetManifest:
						clean = append(clean, a.Path)
					}
				}

				fmt.Printf("  to push:    %d new, %d updated\n", len(pushNew), len(pushUpdate))
				printStatusPaths(pushNew, "+")
				printStatusPaths(pushUpdate, "~")

				if len(restore) > 0 {
					fmt.Printf("  to pull:    %d new, %d updated, %d restore\n", len(pullNew), len(pullUpdate), len(restore))
				} else {
					fmt.Printf("  to pull:    %d new, %d updated\n", len(pullNew), len(pullUpdate))
				}
				printStatusPaths(pullNew, "+")
				printStatusPaths(pullUpdate, "~")
				printStatusPaths(restore, "r")

				fmt.Printf("  conflicts:  %d\n", len(conflicts))
				printStatusPaths(conflicts, "!")

				fmt.Printf("  deletions:  %d local, %d tracked\n", len(delLocal), len(clean))
				printStatusPaths(delLocal, "-")
				printStatusPaths(clean, ".")
			}

			return nil
		},
	}
}

// printStatusPaths prints paths sorted, one per line, prefixed by marker.
func printStatusPaths(paths []string, marker string) {
	if len(paths) == 0 {
		return
	}
	slices.Sort(paths)
	for _, p := range paths {
		fmt.Printf("    %s %s\n", marker, p)
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
