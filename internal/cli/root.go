package cli

import (
	"log/slog"

	"github.com/spf13/cobra"

	styxLog "github.com/AdityaTaggar05/styx/internal/log"
)

var (
	configPath string
	verbose    bool
)

// cliLogger builds a stderr text logger, honoring the --verbose flag.
func cliLogger() *slog.Logger {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	return styxLog.SetupCLI(level)
}

// RootCommand builds the entire styx CLI tree.
func RootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "styx",
		Short: "Filesystem sync tool for cloud data stores",
		Long: `Styx watches local directories and syncs changes to online data stores.
Configure backends, register directories, and let the daemon keep everything in sync.`,
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVar(&configPath, "config", "~/.styx/config.toml", "Path to global config")
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable debug logging")

	root.AddCommand(
		setupCmd(),
		initCmd(),
		removeCmd(),
		listCmd(),
		authCmd(),
		daemonCmd(),
		syncCmd(),
		statusCmd(),
		logCmd(),
	)

	return root
}

func authCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication with data stores",
	}

	cmd.AddCommand(
		authLoginCmd(),
		authLogoutCmd(),
		authStatusCmd(),
	)

	return cmd
}
