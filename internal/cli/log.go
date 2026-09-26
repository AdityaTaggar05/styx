package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/AdityaTaggar05/styx/internal/config"
	styxErrors "github.com/AdityaTaggar05/styx/internal/errors"
)

func logCmd() *cobra.Command {
	var follow bool

	cmd := &cobra.Command{
		Use:   "log",
		Short: "View daemon logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			logPath, err := daemonLogPath()
			if err != nil {
				return err
			}

			f, err := os.Open(logPath)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("%w: no daemon log at %s", styxErrors.ErrFileNotFound, logPath)
				}
				return fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
			}
			defer f.Close()

			reader := bufio.NewReader(f)
			carry := ""
			if err := pump(reader, os.Stdout, &carry); err != nil {
				return err
			}

			if !follow {
				return nil
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return nil
				case <-ticker.C:
					if err := pump(reader, os.Stdout, &carry); err != nil {
						return err
					}
				}
			}
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")

	return cmd
}

// pump writes complete lines from r to w, buffering any trailing partial line.
func pump(r *bufio.Reader, w io.Writer, carry *string) error {
	for {
		line, err := r.ReadString('\n')
		if len(line) > 0 {
			if line[len(line)-1] == '\n' {
				fmt.Fprint(w, *carry+line)
				*carry = ""
			} else {
				*carry += line
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
		}
	}
}

func daemonLogPath() (string, error) {
	cfg, err := config.LoadGlobalConfig(configPath)
	if err != nil {
		return "", err
	}
	return config.ExpandPath(cfg.Daemon.LogFile)
}
