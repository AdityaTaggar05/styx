package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/AdityaTaggar05/styx/internal/config"
	"github.com/AdityaTaggar05/styx/internal/daemon"
	styxErrors "github.com/AdityaTaggar05/styx/internal/errors"
)

func daemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the styxd background sync daemon",
	}

	cmd.AddCommand(
		daemonStartCmd(),
		daemonStopCmd(),
		daemonStatusCmd(),
		daemonInstallCmd(),
		daemonUninstallCmd(),
	)

	return cmd
}

func daemonStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the daemon in the background",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadGlobalConfig(configPath)
			if err != nil {
				return err
			}

			pidPath, err := config.ExpandPath(cfg.Daemon.PIDFile)
			if err != nil {
				return err
			}

			if pid, err := daemon.ReadPID(pidPath); err == nil && daemon.IsRunning(pid) {
				return fmt.Errorf("%w: pid %d", styxErrors.ErrDaemonRunning, pid)
			}

			exe, err := styxdPath()
			if err != nil {
				return err
			}

			logPath, err := config.ExpandPath(cfg.Daemon.LogFile)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
				return fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
			}
			logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
			if err != nil {
				return fmt.Errorf("%w: %v", styxErrors.ErrFilePermission, err)
			}
			defer logFile.Close()

			proc := exec.Command(exe, "--config", configPath)
			proc.Stdout = logFile
			proc.Stderr = logFile
			proc.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
			if err := proc.Start(); err != nil {
				return fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
			}

			// Wait for the daemon to write its PID file.
			for i := 0; i < 50; i++ {
				if pid, err := daemon.ReadPID(pidPath); err == nil && daemon.IsRunning(pid) {
					fmt.Printf("Daemon started (pid %d).\n", pid)
					return nil
				}
				time.Sleep(100 * time.Millisecond)
			}

			return fmt.Errorf("%w: daemon did not come up (see %s)", styxErrors.ErrDaemonNotRunning, logPath)
		},
	}
}

func daemonStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the running daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			pidPath, err := daemonPIDPath()
			if err != nil {
				return err
			}

			pid, err := daemon.ReadPID(pidPath)
			if err != nil {
				return err
			}

			if !daemon.IsRunning(pid) {
				daemon.RemovePID(pidPath)
				return fmt.Errorf("%w: stale pid %d", styxErrors.ErrDaemonPIDStale, pid)
			}

			proc, err := os.FindProcess(pid)
			if err != nil {
				return fmt.Errorf("%w: %v", styxErrors.ErrDaemonNotRunning, err)
			}
			if err := proc.Signal(syscall.SIGTERM); err != nil {
				return fmt.Errorf("%w: %v", styxErrors.ErrDaemonNotRunning, err)
			}

			for i := 0; i < 50; i++ {
				if !daemon.IsRunning(pid) {
					daemon.RemovePID(pidPath)
					fmt.Println("Daemon stopped.")
					return nil
				}
				time.Sleep(100 * time.Millisecond)
			}

			return fmt.Errorf("%w: pid %d did not exit", styxErrors.ErrDaemonRunning, pid)
		},
	}
}

func daemonStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon process status",
		RunE: func(cmd *cobra.Command, args []string) error {
			pidPath, err := daemonPIDPath()
			if err != nil {
				return err
			}

			pid, err := daemon.ReadPID(pidPath)
			if err != nil {
				fmt.Println("Daemon is not running.")
				return nil
			}

			if daemon.IsRunning(pid) {
				fmt.Printf("Daemon is running (pid %d).\n", pid)
			} else {
				fmt.Printf("Daemon is not running (stale pid file, pid %d).\n", pid)
			}
			return nil
		},
	}
}

func daemonInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install and enable the systemd user unit",
		RunE: func(cmd *cobra.Command, args []string) error {
			exe, err := styxdPath()
			if err != nil {
				return err
			}

			if err := daemon.Install(exe); err != nil {
				return err
			}

			unitPath, _ := daemon.UnitPath()
			fmt.Printf("Installed %s\n", unitPath)
			fmt.Println("Manage it with: systemctl --user status styxd")
			return nil
		},
	}
}

func daemonUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Disable and remove the systemd user unit",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := daemon.Uninstall(); err != nil {
				return err
			}
			fmt.Println("Uninstalled styxd systemd unit.")
			return nil
		},
	}
}

// daemonPIDPath resolves the configured PID file path.
func daemonPIDPath() (string, error) {
	cfg, err := config.LoadGlobalConfig(configPath)
	if err != nil {
		return "", err
	}
	return config.ExpandPath(cfg.Daemon.PIDFile)
}

// styxdPath locates the styxd binary next to the running styx executable.
func styxdPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
	}

	candidate := filepath.Join(filepath.Dir(exe), "styxd")
	if _, err := os.Stat(candidate); err != nil {
		return "", fmt.Errorf("%w: styxd not found next to %s", styxErrors.ErrFileNotFound, exe)
	}

	return candidate, nil
}
