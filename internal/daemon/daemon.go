package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/AdityaTaggar05/styx/internal/config"
	styxErrors "github.com/AdityaTaggar05/styx/internal/errors"
	"github.com/AdityaTaggar05/styx/internal/store"
	"github.com/AdityaTaggar05/styx/internal/sync"
)

// [DAEMON]

// Daemon watches every registered directory and syncs it on local change,
// plus a periodic full sync to pick up remote changes.
type Daemon struct {
	cfg    *config.GlobalConfig
	logger *slog.Logger
}

// New creates a daemon from an already-loaded global config.
func New(cfg *config.GlobalConfig, logger *slog.Logger) *Daemon {
	return &Daemon{cfg: cfg, logger: logger}
}

// worker holds the per-directory sync state.
type worker struct {
	ctx       context.Context
	root      string
	cfg       *config.LocalConfig
	st        store.DataStore
	debouncer *Debouncer
	logger    *slog.Logger
}

func (w *worker) syncOnce() {
	if err := sync.Sync(w.ctx, w.root, w.cfg, w.st, w.logger); err != nil {
		w.logger.Error("sync failed", "error", err)
	}
}

// Run blocks until ctx is cancelled, syncing directories in the background.
func (d *Daemon) Run(ctx context.Context) error {
	if len(d.cfg.Directories) == 0 {
		d.logger.Warn("no directories registered — nothing to watch")
		return nil
	}

	// [PID GUARD]
	pidPath, err := config.ExpandPath(d.cfg.Daemon.PIDFile)
	if err != nil {
		return fmt.Errorf("%w: %v", styxErrors.ErrConfigValidate, err)
	}
	if existing, err := ReadPID(pidPath); err == nil && IsRunning(existing) {
		return fmt.Errorf("%w: pid %d", styxErrors.ErrDaemonRunning, existing)
	}
	if err := WritePID(pidPath); err != nil {
		return err
	}
	defer RemovePID(pidPath)

	// [WORKERS]
	workers := make(map[string]*worker)
	var roots []string
	debounce := time.Duration(d.cfg.Daemon.DebounceMS) * time.Millisecond

	for _, root := range d.cfg.Directories {
		localCfg, err := config.LoadLocalConfig(root)
		if err != nil {
			d.logger.Warn("skipping directory", "dir", root, "error", err)
			continue
		}

		st, err := store.Get(localCfg.Store, storeConfigToMap(d.cfg.Stores[localCfg.Store]))
		if err != nil {
			d.logger.Warn("skipping directory", "dir", root, "error", err)
			continue
		}

		authed, err := st.IsAuthenticated(ctx)
		if err != nil || !authed {
			d.logger.Warn("skipping directory: not authenticated", "dir", root, "store", localCfg.Store)
			continue
		}

		w := &worker{
			ctx:    ctx,
			root:   root,
			cfg:    localCfg,
			st:     st,
			logger: d.logger.With("dir", root),
		}
		w.debouncer = NewDebouncer(debounce, w.syncOnce)
		workers[root] = w
		roots = append(roots, root)
	}

	if len(workers) == 0 {
		d.logger.Warn("no usable directories — exiting")
		return nil
	}

	// [WATCHER]
	watcher, err := NewWatcher(roots, d.logger, func(root, path string) {
		w, ok := workers[root]
		if !ok {
			return
		}
		d.logger.Debug("change detected", "dir", root, "path", path)
		w.debouncer.Trigger()
	})
	if err != nil {
		return fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
	}
	defer watcher.Close()

	go watcher.Run(ctx)

	// [PERIODIC POLL]
	ticker := time.NewTicker(time.Duration(d.cfg.Daemon.PollIntervalMS) * time.Millisecond)
	defer ticker.Stop()

	d.logger.Info("daemon started",
		"directories", len(workers),
		"debounce_ms", d.cfg.Daemon.DebounceMS,
		"poll_interval_ms", d.cfg.Daemon.PollIntervalMS,
		"pid", pidPath)

	for {
		select {
		case <-ctx.Done():
			d.logger.Info("shutting down")
			for _, w := range workers {
				w.debouncer.Stop()
			}
			for _, w := range workers {
				w.debouncer.Wait()
			}
			return nil

		case <-ticker.C:
			d.logger.Debug("periodic poll")
			for _, w := range workers {
				w.debouncer.Trigger()
			}
		}
	}
}

// storeConfigToMap flattens a StoreConfig into the map the store registry expects.
func storeConfigToMap(cfg config.StoreConfig) map[string]string {
	return map[string]string{
		"client_id":     cfg.ClientID,
		"client_secret": cfg.ClientSecret,
		"token_file":    cfg.TokenFile,
	}
}
