package gdrive

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"

	"github.com/AdityaTaggar05/styx/internal/config"
	"github.com/AdityaTaggar05/styx/internal/store"
	styxErrors "github.com/AdityaTaggar05/styx/internal/errors"
)

// These are set at build time via -ldflags. Use empty strings for development.
var (
	ClientID     string
	ClientSecret string
)

// GDrive implements store.DataStore for Google Drive.
type GDrive struct {
	clientID     string
	clientSecret string
	tokenPath    string
	oauthCfg     *oauth2.Config
	tokenSrc     oauth2.TokenSource
}

// New creates a GDrive backend from a config map.
// Uses bundled credentials when config values are empty.
func New(cfg map[string]string) (*GDrive, error) {
	clientID := cfg["client_id"]
	clientSecret := cfg["client_secret"]
	tokenFile := cfg["token_file"]

	if clientID == "" {
		clientID = ClientID
	}
	if clientSecret == "" {
		clientSecret = ClientSecret
	}
	if tokenFile == "" {
		tokenFile = "~/.styx/gdrive-token.enc"
	}

	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("%w: gdrive requires client_id and client_secret (build with -ldflags)", styxErrors.ErrConfigValidate)
	}

	tokenPath, err := config.ExpandPath(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("%w: expanding token path: %v", styxErrors.ErrConfigValidate, err)
	}

	return &GDrive{
		clientID:     clientID,
		clientSecret: clientSecret,
		tokenPath:    tokenPath,
	}, nil
}

func init() {
	store.Register("gdrive", func(cfg map[string]string) (store.DataStore, error) {
		return New(cfg)
	})
}

func (g *GDrive) Name() string { return "gdrive" }

// [FILE CRUD]

func (g *GDrive) Pull(ctx context.Context, remotePath, localPath string) error {
	return fmt.Errorf("not implemented")
}

func (g *GDrive) Push(ctx context.Context, localPath, remotePath string) (*store.FileMeta, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *GDrive) Delete(ctx context.Context, remotePath string) error {
	return fmt.Errorf("not implemented")
}

// [METADATA]

func (g *GDrive) Hash(ctx context.Context, remotePath string) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (g *GDrive) List(ctx context.Context, remotePath string) ([]store.FileMeta, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *GDrive) ListRecursive(ctx context.Context, remotePath string) ([]store.FileMeta, error) {
	return nil, fmt.Errorf("not implemented")
}

// [CHANGE DETECTION]

func (g *GDrive) ChangesSince(ctx context.Context, cursor string) ([]store.Change, string, error) {
	return nil, "", fmt.Errorf("not implemented")
}
