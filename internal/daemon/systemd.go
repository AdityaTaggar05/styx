package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	styxErrors "github.com/AdityaTaggar05/styx/internal/errors"
)

// [SYSTEMD]

const unitName = "styxd.service"

const unitTemplate = `[Unit]
Description=Styx sync daemon
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`

// UnitPath returns the systemd user unit path for the styxd service.
func UnitPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "systemd", "user", unitName), nil
}

// Install writes the systemd user unit and enables it.
func Install(styxdPath string) error {
	abs, err := filepath.Abs(styxdPath)
	if err != nil {
		return fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
	}

	unitPath, err := UnitPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
	}

	content := fmt.Sprintf(unitTemplate, abs)
	if err := os.WriteFile(unitPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
	}

	if err := systemctl("daemon-reload"); err != nil {
		return err
	}
	if err := systemctl("enable", "--now", unitName); err != nil {
		return err
	}

	return nil
}

// Uninstall stops, disables and removes the systemd user unit.
func Uninstall() error {
	unitPath, err := UnitPath()
	if err != nil {
		return err
	}

	if _, statErr := os.Stat(unitPath); statErr != nil {
		return fmt.Errorf("%w: unit %s is not installed", styxErrors.ErrDaemonSystemd, unitName)
	}

	// Best-effort stop/disable — the unit may already be inactive.
	_ = systemctl("disable", "--now", unitName)

	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
	}

	if err := systemctl("daemon-reload"); err != nil {
		return err
	}

	return nil
}

func systemctl(args ...string) error {
	cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: systemctl %v: %v: %s", styxErrors.ErrDaemonSystemd, args, err, out)
	}
	return nil
}
