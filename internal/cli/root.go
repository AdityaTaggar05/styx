package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	configPath string
	verbose    bool
)

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

func daemonCmd() *cobra.Command {
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

func logCmd() *cobra.Command {
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
