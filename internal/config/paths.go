package config

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	StyxDirName = ".styx"

	GlobalDir   = "~/.styx"
	ConfigFile   = "config.toml"
	ManifestFile = "manifest.json"
	PIDFile     = "styxd.pid"
	LogFile     = "styxd.log"
	TokenFile   = "gdrive-token.enc"
)

func GlobalConfigPath() (string, error) {
	return ExpandPath(filepath.Join(GlobalDir, ConfigFile))
}

func GlobalPIDPath() (string, error) {
	return ExpandPath(filepath.Join(GlobalDir, PIDFile))
}

func GlobalLogPath() (string, error) {
	return ExpandPath(filepath.Join(GlobalDir, LogFile))
}

func GlobalTokenPath() (string, error) {
	return ExpandPath(filepath.Join(GlobalDir, TokenFile))
}

func LocalConfigPath(dir string) string {
	return filepath.Join(dir, StyxDirName, ConfigFile)
}

func LocalManifestPath(dir string) string {
	return filepath.Join(dir, StyxDirName, ManifestFile)
}

// ExpandPath replaces a leading ~ with the user's home directory.
func ExpandPath(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if path == "~" {
		return home, nil
	}
	if path[1] == '/' || path[1] == '\\' {
		return filepath.Join(home, path[2:]), nil
	}
	return filepath.Join(home, path[1:]), nil
}
