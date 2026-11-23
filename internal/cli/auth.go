package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/AdityaTaggar05/styx/internal/config"
	styxErrors "github.com/AdityaTaggar05/styx/internal/errors"
	"github.com/AdityaTaggar05/styx/internal/store"
)

func authLoginCmd() *cobra.Command {
	var storeName string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with a data store",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadGlobalConfig(configPath)
			if err != nil {
				return err
			}

			s, err := store.Get(storeName, cfg.Stores[storeName])
			if err != nil {
				return err
			}

			authURL := s.AuthURL()

			// Key-based stores (e.g. S3) have no browser flow; validate directly.
			if authURL == "" {
				if err := s.Authenticate(context.Background(), ""); err != nil {
					return err
				}
				fmt.Printf("Authenticated with %s.\n", storeName)
				return nil
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			codeCh := make(chan string, 1)
			errCh := make(chan error, 1)

			mux := http.NewServeMux()
			mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
				code := r.URL.Query().Get("code")
				if code == "" {
					http.Error(w, "missing authorization code", http.StatusBadRequest)
					errCh <- fmt.Errorf("%w: no authorization code received", styxErrors.ErrAuthCancelled)
					return
				}
				fmt.Fprintln(w, "Styx: authorization complete. You may close this window.")
				codeCh <- code
			})

			srv := &http.Server{Addr: ":8972", Handler: mux}
			go func() {
				if err := srv.ListenAndServe(); err != http.ErrServerClosed {
					errCh <- err
				}
			}()

			fmt.Printf("Open this URL in your browser to authenticate with %s:\n\n  %s\n\n", storeName, authURL)
			openBrowser(authURL)

			select {
			case code := <-codeCh:
				srv.Shutdown(ctx)
				if err := s.Authenticate(ctx, code); err != nil {
					return err
				}
				fmt.Printf("Authenticated with %s successfully.\n", storeName)
				// Save token path to config so it persists
				if _, ok := cfg.Stores[storeName]; !ok {
					cfg.Stores[storeName] = map[string]string{
						"token_file": fmt.Sprintf("~/.styx/%s-token.enc", storeName),
					}
					config.SaveGlobalConfig(configPath, cfg)
				}
				return nil

			case err := <-errCh:
				srv.Shutdown(ctx)
				return err
			}
		},
	}

	cmd.Flags().StringVarP(&storeName, "store", "s", "gdrive", "Store backend name")
	return cmd
}

func authLogoutCmd() *cobra.Command {
	var storeName string

	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Revoke authentication",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadGlobalConfig(configPath)
			if err != nil {
				return err
			}

			s, err := store.Get(storeName, cfg.Stores[storeName])
			if err != nil {
				return err
			}

			if err := s.Logout(context.Background()); err != nil {
				return err
			}

			fmt.Printf("Logged out from %s.\n", storeName)
			return nil
		},
	}

	cmd.Flags().StringVarP(&storeName, "store", "s", "gdrive", "Store backend name")
	return cmd
}

func authStatusCmd() *cobra.Command {
	var storeName string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadGlobalConfig(configPath)
			if err != nil {
				return err
			}

			s, err := store.Get(storeName, cfg.Stores[storeName])
			if err != nil {
				return err
			}

			authed, err := s.IsAuthenticated(context.Background())
			if err != nil {
				return err
			}

			if authed {
				fmt.Printf("Authenticated with %s.\n", storeName)
			} else {
				fmt.Printf("Not authenticated with %s. Run `styx auth login --store %s`.\n", storeName, storeName)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&storeName, "store", "s", "gdrive", "Store backend name")
	return cmd
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		return
	}
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Run()
}
