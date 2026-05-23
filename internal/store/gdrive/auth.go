package gdrive

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	styxErrors "github.com/AdityaTaggar05/styx/internal/errors"
)

const (
	redirectURI  = "http://localhost:8972/callback"
	driveScope   = "https://www.googleapis.com/auth/drive"
	driveScopeRO = "https://www.googleapis.com/auth/drive.readonly"
)

func (g *GDrive) oauthConfig() *oauth2.Config {
	if g.oauthCfg == nil {
		g.oauthCfg = &oauth2.Config{
			ClientID:     g.clientID,
			ClientSecret: g.clientSecret,
			RedirectURL:  redirectURI,
			Scopes:       []string{driveScope},
			Endpoint:     google.Endpoint,
		}
	}
	return g.oauthCfg
}

// AuthURL returns the Google OAuth2 consent URL.
func (g *GDrive) AuthURL() string {
	return g.oauthConfig().AuthCodeURL("state", oauth2.AccessTypeOffline)
}

// Authenticate exchanges an authorization code for a token and persists it.
func (g *GDrive) Authenticate(ctx context.Context, code string) error {
	tok, err := g.oauthConfig().Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("%w: %v", styxErrors.ErrAuthInvalid, err)
	}
	return g.saveToken(tok)
}

// IsAuthenticated returns true if a valid, non-expired token exists.
func (g *GDrive) IsAuthenticated(ctx context.Context) (bool, error) {
	tok, err := g.loadToken()
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if tok == nil {
		return false, nil
	}

	g.tokenSrc = g.oauthConfig().TokenSource(ctx, tok)
	refreshed, err := g.tokenSrc.Token()
	if err != nil {
		return false, nil
	}

	if refreshed.AccessToken != tok.AccessToken {
		g.saveToken(refreshed)
	}

	return true, nil
}

// Logout removes the persisted token file.
func (g *GDrive) Logout(ctx context.Context) error {
	if err := os.Remove(g.tokenPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
	}
	g.tokenSrc = nil
	g.oauthCfg = nil
	return nil
}

// loadToken reads and decrypts the token from disk.
func (g *GDrive) loadToken() (*oauth2.Token, error) {
	data, err := os.ReadFile(g.tokenPath)
	if err != nil {
		return nil, err
	}

	plain, err := decrypt(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", styxErrors.ErrAuthInvalid, err)
	}

	var tok oauth2.Token
	if err := json.Unmarshal(plain, &tok); err != nil {
		return nil, fmt.Errorf("%w: %v", styxErrors.ErrAuthInvalid, err)
	}

	return &tok, nil
}

// saveToken encrypts and writes the token to disk.
func (g *GDrive) saveToken(tok *oauth2.Token) error {
	plain, err := json.Marshal(tok)
	if err != nil {
		return err
	}

	encrypted, err := encrypt(plain)
	if err != nil {
		return err
	}

	return os.WriteFile(g.tokenPath, encrypted, 0o600)
}
