package errors

import "errors"

// Config errors
var (
	ErrConfigNotFound    = errors.New("styx/config: not found")
	ErrConfigParse       = errors.New("styx/config: parse failure")
	ErrConfigValidate    = errors.New("styx/config: validation failure")
	ErrConfigNoStore     = errors.New("styx/config: store not configured")
	ErrConfigNoDirectory = errors.New("styx/config: directory not registered")
)

// Auth errors
var (
	ErrAuthRequired  = errors.New("styx/auth: authentication required")
	ErrAuthExpired   = errors.New("styx/auth: token expired")
	ErrAuthInvalid   = errors.New("styx/auth: invalid credentials")
	ErrAuthCancelled = errors.New("styx/auth: user cancelled")
)

// Network errors
var (
	ErrNetworkTimeout   = errors.New("styx/network: request timed out")
	ErrNetworkRefused   = errors.New("styx/network: connection refused")
	ErrNetworkRateLimit = errors.New("styx/network: rate limited")
	ErrNetworkDNS       = errors.New("styx/network: DNS resolution failed")
)

// Store errors
var (
	ErrStoreNotFound    = errors.New("styx/store: file not found")
	ErrStorePermission  = errors.New("styx/store: permission denied")
	ErrStoreQuota       = errors.New("styx/store: quota exceeded")
	ErrStoreConflict    = errors.New("styx/store: remote conflict")
	ErrStoreUnavailable = errors.New("styx/store: service unavailable")
	ErrStoreUnknown     = errors.New("styx/store: unknown store backend")
)

// Sync errors
var (
	ErrSyncConflict        = errors.New("styx/sync: conflict detected")
	ErrSyncHashMismatch    = errors.New("styx/sync: hash mismatch after transfer")
	ErrSyncManifestCorrupt = errors.New("styx/sync: manifest corrupt")
	ErrSyncManifestLocked  = errors.New("styx/sync: manifest locked by another process")
)

// Daemon errors
var (
	ErrDaemonRunning    = errors.New("styx/daemon: already running")
	ErrDaemonNotRunning = errors.New("styx/daemon: not running")
	ErrDaemonPIDStale   = errors.New("styx/daemon: stale PID file")
	ErrDaemonSystemd    = errors.New("styx/daemon: systemd operation failed")
)

// File I/O errors
var (
	ErrFileNotFound   = errors.New("styx/file: not found")
	ErrFilePermission = errors.New("styx/file: permission denied")
	ErrFileIO         = errors.New("styx/file: I/O error")
	ErrFileLocked     = errors.New("styx/file: locked by another process")
	ErrFileIsDir      = errors.New("styx/file: expected file, got directory")
)
