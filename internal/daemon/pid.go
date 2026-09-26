package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	styxErrors "github.com/AdityaTaggar05/styx/internal/errors"
)

// [PID FILE]

// WritePID writes the current process ID to path, creating parent dirs.
func WritePID(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
	}

	data := strconv.Itoa(os.Getpid())
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		return fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
	}

	return nil
}

// ReadPID reads the PID from path.
// Returns ErrDaemonNotRunning when the file is absent.
func ReadPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("%w: %s", styxErrors.ErrDaemonNotRunning, path)
		}
		return 0, fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("%w: %s", styxErrors.ErrDaemonPIDStale, path)
	}

	return pid, nil
}

// RemovePID deletes the PID file. Missing file is not an error.
func RemovePID(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
	}
	return nil
}

// IsRunning reports whether a process with the given PID exists.
// Signal 0 performs error checking without actually sending a signal.
func IsRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
