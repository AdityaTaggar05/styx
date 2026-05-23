package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/AdityaTaggar05/styx/internal/config"
	styxErrors "github.com/AdityaTaggar05/styx/internal/errors"
)

func initCmd() *cobra.Command {
	var storeName string
	var remoteRoot string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Register the current directory for syncing",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
			}

			cfg, err := config.LoadGlobalConfig(configPath)
			if err != nil {
				return err
			}

			styxDir := filepath.Join(cwd, config.StyxDirName)
			if _, err := os.Stat(styxDir); err == nil {
				return fmt.Errorf("%w: .styx already exists in %s", styxErrors.ErrFileIO, cwd)
			}

			if remoteRoot == "" {
				fmt.Print("Remote path on ", storeName, ": ")
				fmt.Scanln(&remoteRoot)
			}
			remoteRoot = strings.TrimSpace(remoteRoot)
			if remoteRoot == "" {
				return fmt.Errorf("%w: remote_root must not be empty", styxErrors.ErrConfigValidate)
			}

			localCfg := config.DefaultLocalConfig(storeName, remoteRoot)
			if err := config.SaveLocalConfig(cwd, localCfg); err != nil {
				return err
			}

			if err := writeEmptyManifest(cwd, storeName, remoteRoot); err != nil {
				return err
			}

			if err := config.AddDirectory(cfg, cwd); err != nil {
				return err
			}

			if err := config.SaveGlobalConfig(configPath, cfg); err != nil {
				return err
			}

			fmt.Printf("Initialized %s → %s:%s\n", cwd, storeName, remoteRoot)
			return nil
		},
	}

	cmd.Flags().StringVarP(&storeName, "store", "s", "gdrive", "Store backend name")
	cmd.Flags().StringVarP(&remoteRoot, "root", "r", "", "Remote path on the store (prompts if empty)")
	return cmd
}

// manifestV1 is the minimal manifest written during init.
type manifestV1 struct {
	Version      int               `json:"version"`
	Store        string            `json:"store"`
	RemoteRoot   string            `json:"remote_root"`
	LastSync     string            `json:"last_sync"`
	RemoteCursor string            `json:"remote_cursor"`
	Files        map[string]string `json:"files"`
}

func writeEmptyManifest(dir, storeName, remoteRoot string) error {
	m := manifestV1{
		Version:    1,
		Store:      storeName,
		RemoteRoot: remoteRoot,
		Files:      make(map[string]string),
	}

	path := filepath.Join(dir, config.StyxDirName, config.ManifestFile)
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}
