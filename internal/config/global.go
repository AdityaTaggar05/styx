package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/BurntSushi/toml"

	styxErrors "github.com/AdityaTaggar05/styx/internal/errors"
)

// GlobalConfig is the top-level configuration stored at ~/.styx/config.toml.
type GlobalConfig struct {
	Version     int                    `toml:"version"`
	Stores      map[string]StoreConfig `toml:"stores"`
	Directories []string               `toml:"directories"`
	Daemon      DaemonConfig           `toml:"daemon"`
}

// StoreConfig holds OAuth credentials and token path for a single backend.
type StoreConfig struct {
	ClientID     string `toml:"client_id"`
	ClientSecret string `toml:"client_secret"`
	TokenFile    string `toml:"token_file,omitempty"`
}

// DaemonConfig controls daemon runtime behavior.
type DaemonConfig struct {
	LogLevel   string `toml:"log_level,omitempty"`
	LogFile    string `toml:"log_file,omitempty"`
	DebounceMS int    `toml:"debounce_ms,omitempty"`
	PIDFile    string `toml:"pid_file,omitempty"`
}

// DefaultGlobalConfig returns a GlobalConfig with sensible defaults.
func DefaultGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		Version:     1,
		Stores:      make(map[string]StoreConfig),
		Directories: []string{},
		Daemon: DaemonConfig{
			LogLevel:   "info",
			LogFile:    "~/.styx/styxd.log",
			DebounceMS: 10000,
			PIDFile:    "~/.styx/styxd.pid",
		},
	}
}

var validLogLevels = map[string]bool{
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
}

// LoadGlobalConfig reads and parses the TOML config file at path.
// Merges missing fields with defaults from DefaultGlobalConfig.
func LoadGlobalConfig(path string) (*GlobalConfig, error) {
	expanded, err := ExpandPath(path)
	if err != nil {
		return nil, err
	}

	cfg := DefaultGlobalConfig()
	if _, err := toml.DecodeFile(expanded, cfg); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", styxErrors.ErrConfigNotFound, expanded)
		}
		return nil, fmt.Errorf("%w: %v", styxErrors.ErrConfigParse, err)
	}

	if cfg.Stores == nil {
		cfg.Stores = make(map[string]StoreConfig)
	}
	if err := Validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// SaveGlobalConfig writes the config as TOML to path, creating parent directories.
func SaveGlobalConfig(path string, cfg *GlobalConfig) error {
	expanded, err := ExpandPath(path)
	if err != nil {
		return err
	}

	dir := filepath.Dir(expanded)
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
	}

	f, err := os.Create(expanded)
	if err != nil {
		return fmt.Errorf("%w: %v", styxErrors.ErrFilePermission, err)
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return fmt.Errorf("%w: %v", styxErrors.ErrConfigParse, err)
	}

	return nil
}

// Validate checks the config for correctness.
func Validate(cfg *GlobalConfig) error {
	if cfg.Daemon.DebounceMS <= 0 {
		return fmt.Errorf("%w: debounce_ms must be positive", styxErrors.ErrConfigValidate)
	}
	if !validLogLevels[cfg.Daemon.LogLevel] {
		return fmt.Errorf("%w: invalid log_level %q (must be debug, info, warn, error)", styxErrors.ErrConfigValidate, cfg.Daemon.LogLevel)
	}
	return nil
}

// AddDirectory appends dir to the Directories slice if not already present.
func AddDirectory(cfg *GlobalConfig, dir string) error {
	if slices.Contains(cfg.Directories, dir) {
		return nil
	}
	cfg.Directories = append(cfg.Directories, dir)
	return nil
}

// RemoveDirectory removes dir from the Directories slice.
// Returns ErrConfigNoDirectory if dir is not registered.
func RemoveDirectory(cfg *GlobalConfig, dir string) error {
	for i, d := range cfg.Directories {
		if d == dir {
			cfg.Directories = append(cfg.Directories[:i], cfg.Directories[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("%w: %s", styxErrors.ErrConfigNoDirectory, dir)
}
