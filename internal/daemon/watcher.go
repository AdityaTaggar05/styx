package daemon

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// [WATCHER]

// Watcher wraps fsnotify with recursive directory watching and maps each event
// back to the registered root directory that owns it.
type Watcher struct {
	fsw     *fsnotify.Watcher
	roots   []string
	logger  *slog.Logger
	onEvent func(root, path string)
}

// NewWatcher creates a watcher over the given registered roots and adds watches
// for every subdirectory found at startup.
func NewWatcher(roots []string, logger *slog.Logger, onEvent func(root, path string)) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{fsw: fsw, roots: roots, logger: logger, onEvent: onEvent}
	for _, root := range roots {
		w.addRecursive(root)
	}

	return w, nil
}

// Run processes events until ctx is cancelled or the watcher is closed.
func (w *Watcher) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			w.handle(ev)
		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			w.logger.Warn("watcher error", "error", err)
		}
	}
}

// Close releases the underlying fsnotify watcher.
func (w *Watcher) Close() error {
	return w.fsw.Close()
}

func (w *Watcher) handle(ev fsnotify.Event) {
	// Ignore internal state and atomic-write scratch files.
	if isInternal(ev.Name) {
		return
	}

	// A newly created directory needs its own watch (fsnotify is not recursive).
	if ev.Op&fsnotify.Create != 0 {
		if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
			w.addRecursive(ev.Name)
		}
	}

	root, ok := w.owner(ev.Name)
	if !ok {
		return
	}
	w.onEvent(root, ev.Name)
}

// addRecursive watches root and every subdirectory beneath it.
func (w *Watcher) addRecursive(root string) {
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if !d.IsDir() {
			return nil
		}
		if isInternal(path) {
			return filepath.SkipDir
		}
		if err := w.fsw.Add(path); err != nil {
			w.logger.Warn("watch failed", "path", path, "error", err)
		}
		return nil
	})
	if err != nil {
		w.logger.Warn("walk failed", "root", root, "error", err)
	}
}

// owner returns the registered root that contains path (longest match wins).
func (w *Watcher) owner(path string) (string, bool) {
	best := ""
	for _, root := range w.roots {
		if path == root || strings.HasPrefix(path, root+string(filepath.Separator)) {
			if len(root) > len(best) {
				best = root
			}
		}
	}
	return best, best != ""
}

// isInternal reports whether path is under a .styx directory or is a scratch file.
func isInternal(path string) bool {
	if strings.HasSuffix(path, ".tmp") {
		return true
	}
	for _, seg := range strings.Split(path, string(filepath.Separator)) {
		if seg == ".styx" {
			return true
		}
	}
	return false
}
