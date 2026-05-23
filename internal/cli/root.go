package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	configPath string
	verbose    bool
)

// NewRootCommand builds the entire styx CLI tree.
func NewRootCommand() *cobra.Command {
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
		newInitCmd(),
		newRemoveCmd(),
		newListCmd(),
		newAuthCmd(),
		newDaemonCmd(),
		newSyncCmd(),
		newStatusCmd(),
		newLogCmd(),
	)

	return root
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Register the current directory for syncing",
		RunE: func(cmd *cobra.Command, args []string) error {
			return notYet("init")
		},
	}
}

func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <path>",
		Short: "Remove a directory from the registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notYet("remove")
		},
	}
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all registered directories",
		RunE: func(cmd *cobra.Command, args []string) error {
			return notYet("list")
		},
	}
}

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication with data stores",
	}

	var storeName string
	cmd.PersistentFlags().StringVarP(&storeName, "store", "s", "gdrive", "Store backend name")

	cmd.AddCommand(
		&cobra.Command{
			Use:   "login",
			Short: "Authenticate with a data store",
			RunE: func(cmd *cobra.Command, args []string) error {
				return notYet("auth login")
			},
		},
		&cobra.Command{
			Use:   "logout",
			Short: "Revoke authentication",
			RunE: func(cmd *cobra.Command, args []string) error {
				return notYet("auth logout")
			},
		},
		&cobra.Command{
			Use:   "status",
			Short: "Check authentication status",
			RunE: func(cmd *cobra.Command, args []string) error {
				return notYet("auth status")
			},
		},
	)

	return cmd
}

func newDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the styxd background sync daemon",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "start",
			Short: "Start the daemon",
			RunE: func(cmd *cobra.Command, args []string) error {
				return notYet("daemon start")
			},
		},
		&cobra.Command{
			Use:   "stop",
			Short: "Stop the daemon",
			RunE: func(cmd *cobra.Command, args []string) error {
				return notYet("daemon stop")
			},
		},
		&cobra.Command{
			Use:   "status",
			Short: "Show daemon process status",
			RunE: func(cmd *cobra.Command, args []string) error {
				return notYet("daemon status")
			},
		},
		&cobra.Command{
			Use:   "install",
			Short: "Install the systemd user unit",
			RunE: func(cmd *cobra.Command, args []string) error {
				return notYet("daemon install")
			},
		},
		&cobra.Command{
			Use:   "uninstall",
			Short: "Remove the systemd user unit",
			RunE: func(cmd *cobra.Command, args []string) error {
				return notYet("daemon uninstall")
			},
		},
	)

	return cmd
}

func newSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync [path]",
		Short: "Force an immediate sync of the given directory (or all)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return notYet("sync")
		},
	}
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [path]",
		Short: "Show sync status of a directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return notYet("status")
		},
	}
}

func newLogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log",
		Short: "View daemon logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return notYet("log")
		},
	}

	cmd.Flags().BoolP("follow", "f", false, "Follow log output")

	return cmd
}

func notYet(name string) error {
	fmt.Printf("%s: not yet implemented\n", name)
	return nil
}
