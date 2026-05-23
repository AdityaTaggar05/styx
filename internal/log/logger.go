package log

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	styxErrors "github.com/AdityaTaggar05/styx/internal/errors"
)

// LevelFromString converts a string level name to slog.Level.
func LevelFromString(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("%w: unknown log level %q", styxErrors.ErrConfigValidate, s)
	}
}

// SetupCLI returns a logger that writes human-readable text to stderr.
func SetupCLI(level slog.Level) *slog.Logger {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}

// SetupDaemon returns a logger that writes to both stderr (text) and a JSON log file.
// Falls back to stderr-only if the JSON file cannot be opened.
func SetupDaemon(level slog.Level, jsonPath string) *slog.Logger {
	handlers := []slog.Handler{
		slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}),
	}

	if jsonPath != "" {
		f, err := os.OpenFile(jsonPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err == nil {
			handlers = append(handlers, slog.NewJSONHandler(f, &slog.HandlerOptions{Level: level}))
		}
	}

	return slog.New(slog.NewMultiHandler(handlers...))
}
