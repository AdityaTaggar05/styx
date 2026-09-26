package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/AdityaTaggar05/styx/internal/config"
	"github.com/AdityaTaggar05/styx/internal/daemon"
	styxLog "github.com/AdityaTaggar05/styx/internal/log"
	_ "github.com/AdityaTaggar05/styx/internal/store/gdrive"
)

func main() {
	configPath := flag.String("config", "~/.styx/config.toml", "Path to global config")
	flag.Parse()

	cfg, err := config.LoadGlobalConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "styxd: %v\n", err)
		os.Exit(1)
	}

	level, err := styxLog.LevelFromString(cfg.Daemon.LogLevel)
	if err != nil {
		level = slog.LevelInfo
	}

	logPath, err := config.ExpandPath(cfg.Daemon.LogFile)
	if err != nil {
		logPath = ""
	}

	logger := styxLog.SetupDaemon(level, logPath)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	d := daemon.New(cfg, logger)
	if err := d.Run(ctx); err != nil {
		logger.Error("daemon exited", "error", err)
		os.Exit(1)
	}
}
