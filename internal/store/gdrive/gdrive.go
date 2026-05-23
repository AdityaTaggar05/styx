package gdrive

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"

	"github.com/AdityaTaggar05/styx/internal/store"
	styxErrors "github.com/AdityaTaggar05/styx/internal/errors"
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
func New(cfg map[string]string) (*GDrive, error) {
	clientID := cfg["client_id"]
	clientSecret := cfg["client_secret"]
	tokenFile := cfg["token_file"]

	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("%w: gdrive requires client_id and client_secret", styxErrors.ErrConfigValidate)
	}

	return &GDrive{
		clientID:     clientID,
		clientSecret: clientSecret,
		tokenPath:    tokenFile,
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
