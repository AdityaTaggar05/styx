package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	styxErrors "github.com/AdityaTaggar05/styx/internal/errors"
)

// LocalConfig is the per-directory configuration stored at <dir>/.styx/config.toml.
type LocalConfig struct {
	Version        int      `toml:"version"`
	Store          string   `toml:"store"`
	RemoteRoot     string   `toml:"remote_root"`
	ConflictSuffix string   `toml:"conflict_suffix,omitempty"`
	Ignore         []string `toml:"ignore,omitempty"`
}

// DefaultLocalConfig returns a LocalConfig with sensible defaults.
func DefaultLocalConfig(store, remoteRoot string) *LocalConfig {
	return &LocalConfig{
		Version:        1,
		Store:          store,
		RemoteRoot:     remoteRoot,
		ConflictSuffix: ".conflict",
		Ignore:         []string{"*.conflict"},
	}
}

// LoadLocalConfig reads and parses the TOML config from <dir>/.styx/config.toml.
func LoadLocalConfig(dir string) (*LocalConfig, error) {
	path := LocalConfigPath(dir)

	cfg := &LocalConfig{}
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", styxErrors.ErrConfigNotFound, path)
		}
		return nil, fmt.Errorf("%w: %v", styxErrors.ErrConfigParse, err)
	}

	if cfg.ConflictSuffix == "" {
		cfg.ConflictSuffix = ".conflict"
	}
	if err := ValidateLocal(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// SaveLocalConfig writes the config as TOML to <dir>/.styx/config.toml,
// creating the .styx directory if it doesn't exist.
func SaveLocalConfig(dir string, cfg *LocalConfig) error {
	styxDir := filepath.Join(dir, StyxDirName)
	if err := os.MkdirAll(styxDir, 0o755); err != nil {
		return fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
	}

	path := LocalConfigPath(dir)
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("%w: %v", styxErrors.ErrFilePermission, err)
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return fmt.Errorf("%w: %v", styxErrors.ErrConfigParse, err)
	}

	return nil
}

// ValidateLocal checks the local config for correctness.
func ValidateLocal(cfg *LocalConfig) error {
	if cfg.Store == "" {
		return fmt.Errorf("%w: store must not be empty", styxErrors.ErrConfigValidate)
	}
	if cfg.RemoteRoot == "" {
		return fmt.Errorf("%w: remote_root must not be empty", styxErrors.ErrConfigValidate)
	}
	return nil
}
