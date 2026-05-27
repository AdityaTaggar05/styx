package gdrive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"

	"github.com/AdityaTaggar05/styx/internal/config"
	"github.com/AdityaTaggar05/styx/internal/store"
	styxErrors "github.com/AdityaTaggar05/styx/internal/errors"
)

// [BUILD-TIME CREDENTIALS]

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
	driveSrv     *drive.Service
}

// New creates a GDrive backend from a config map.
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

// [SERVICE]

func (g *GDrive) getService(ctx context.Context) (*drive.Service, error) {
	if g.driveSrv != nil {
		return g.driveSrv, nil
	}

	tok, err := g.loadToken()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", styxErrors.ErrAuthRequired, err)
	}

	client := g.oauthConfig().Client(ctx, tok)
	svc, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", styxErrors.ErrStoreUnavailable, err)
	}

	g.driveSrv = svc
	return svc, nil
}

// [PATH RESOLUTION]

// resolveFileID walks path segments starting from Drive root and returns the
// file/folder ID of the final segment. Returns ErrStoreNotFound if any segment
// in the path is missing.
func (g *GDrive) resolveFileID(ctx context.Context, remotePath string) (string, error) {
	svc, err := g.getService(ctx)
	if err != nil {
		return "", err
	}

	segments := splitPath(remotePath)
	if len(segments) == 0 {
		return "root", nil
	}

	parentID := "root"
	for _, seg := range segments {
		q := fmt.Sprintf("name = '%s' and '%s' in parents and trashed = false", escapeQuery(seg), parentID)
		list, err := svc.Files.List().Q(q).Fields("files(id, mimeType)").PageSize(1).Do()
		if err != nil {
			return "", fmt.Errorf("%w: %v", styxErrors.ErrStoreUnavailable, err)
		}
		if len(list.Files) == 0 {
			return "", fmt.Errorf("%w: %s", styxErrors.ErrStoreNotFound, remotePath)
		}
		parentID = list.Files[0].Id
	}

	return parentID, nil
}

// resolveOrCreateFolder ensures every segment of folderPath exists on Drive,
// creating folders as needed. Returns the ID of the last folder.
func (g *GDrive) resolveOrCreateFolder(ctx context.Context, folderPath string) (string, error) {
	svc, err := g.getService(ctx)
	if err != nil {
		return "", err
	}

	parentID := "root"
	for _, seg := range splitPath(folderPath) {
		q := fmt.Sprintf("name = '%s' and '%s' in parents and mimeType = 'application/vnd.google-apps.folder' and trashed = false",
			escapeQuery(seg), parentID)
		list, err := svc.Files.List().Q(q).Fields("files(id)").PageSize(1).Do()
		if err != nil {
			return "", fmt.Errorf("%w: %v", styxErrors.ErrStoreUnavailable, err)
		}

		if len(list.Files) > 0 {
			parentID = list.Files[0].Id
			continue
		}

		// Create missing folder
		f := &drive.File{Name: seg, MimeType: "application/vnd.google-apps.folder", Parents: []string{parentID}}
		created, err := svc.Files.Create(f).Fields("id").Do()
		if err != nil {
			return "", fmt.Errorf("%w: %v", styxErrors.ErrStorePermission, err)
		}
		parentID = created.Id
	}

	return parentID, nil
}

// filePath reconstructs the full Drive path for a file by walking parent
// references up to root.
func (g *GDrive) filePath(ctx context.Context, fileID string) (string, error) {
	svc, err := g.getService(ctx)
	if err != nil {
		return "", err
	}

	path, err := g.fileIDToPath(ctx, svc, fileID)
	if err != nil {
		return "", err
	}
	return "/" + strings.Join(path, "/"), nil
}

func (g *GDrive) fileIDToPath(ctx context.Context, svc *drive.Service, fileID string) ([]string, error) {
	if fileID == "root" {
		return nil, nil
	}

	f, err := svc.Files.Get(fileID).Fields("name, parents").Do()
	if err != nil {
		return nil, err
	}

	if len(f.Parents) == 0 {
		return []string{f.Name}, nil
	}

	parentPath, err := g.fileIDToPath(ctx, svc, f.Parents[0])
	if err != nil {
		return nil, err
	}

	return append(parentPath, f.Name), nil
}

// [FILE CRUD]

func (g *GDrive) Pull(ctx context.Context, remotePath, localPath string) error {
	svc, err := g.getService(ctx)
	if err != nil {
		return err
	}

	fileID, err := g.resolveFileID(ctx, remotePath)
	if err != nil {
		return err
	}

	resp, err := svc.Files.Get(fileID).Download()
	if err != nil {
		if isNotFound(err) {
			return fmt.Errorf("%w: %s", styxErrors.ErrStoreNotFound, remotePath)
		}
		return fmt.Errorf("%w: %v", styxErrors.ErrStoreUnavailable, err)
	}
	defer resp.Body.Close()

	dir := filepath.Dir(localPath)
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("%w: %v", styxErrors.ErrFilePermission, err)
	}

	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("%w: %v", styxErrors.ErrFilePermission, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
	}

	return nil
}

func (g *GDrive) Push(ctx context.Context, localPath, remotePath string) (*store.FileMeta, error) {
	svc, err := g.getService(ctx)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", styxErrors.ErrFileNotFound, localPath)
		}
		return nil, fmt.Errorf("%w: %v", styxErrors.ErrFilePermission, err)
	}
	defer f.Close()

	dir := filepath.Dir(remotePath)
	base := filepath.Base(remotePath)

	parentID, err := g.resolveOrCreateFolder(ctx, dir)
	if err != nil {
		return nil, err
	}

	driveFile := &drive.File{Name: base, Parents: []string{parentID}}

	// Check if file already exists — update instead of creating a dup
	existingID, err := g.resolveFileID(ctx, remotePath)
	if err == nil && existingID != "" && existingID != "root" {
		updated, err := svc.Files.Update(existingID, driveFile).Media(f).Fields("id, md5Checksum, size, modifiedTime").Do()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", styxErrors.ErrStoreUnavailable, err)
		}
		return &store.FileMeta{
			Path:     remotePath,
			Hash:     "md5:" + updated.Md5Checksum,
			Size:     updated.Size,
			ModTime:  parseDriveTime(updated.ModifiedTime),
			RemoteID: updated.Id,
		}, nil
	}

	created, err := svc.Files.Create(driveFile).Media(f).Fields("id, md5Checksum, size, modifiedTime").Do()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", styxErrors.ErrStoreUnavailable, err)
	}

	return &store.FileMeta{
		Path:     remotePath,
		Hash:     "md5:" + created.Md5Checksum,
		Size:     created.Size,
		ModTime:  parseDriveTime(created.ModifiedTime),
		RemoteID: created.Id,
	}, nil
}

func (g *GDrive) Delete(ctx context.Context, remotePath string) error {
	svc, err := g.getService(ctx)
	if err != nil {
		return err
	}

	fileID, err := g.resolveFileID(ctx, remotePath)
	if err != nil {
		return err
	}

	if err := svc.Files.Delete(fileID).Do(); err != nil {
		if isNotFound(err) {
			return nil // already gone — not an error
		}
		return fmt.Errorf("%w: %v", styxErrors.ErrStoreUnavailable, err)
	}

	return nil
}

// [METADATA]

func (g *GDrive) Hash(ctx context.Context, remotePath string) (string, error) {
	svc, err := g.getService(ctx)
	if err != nil {
		return "", err
	}

	fileID, err := g.resolveFileID(ctx, remotePath)
	if err != nil {
		return "", err
	}

	meta, err := svc.Files.Get(fileID).Fields("md5Checksum").Do()
	if err != nil {
		if isNotFound(err) {
			return "", fmt.Errorf("%w: %s", styxErrors.ErrStoreNotFound, remotePath)
		}
		return "", fmt.Errorf("%w: %v", styxErrors.ErrStoreUnavailable, err)
	}

	return "md5:" + meta.Md5Checksum, nil
}

func (g *GDrive) List(ctx context.Context, remotePath string) ([]store.FileMeta, error) {
	svc, err := g.getService(ctx)
	if err != nil {
		return nil, err
	}

	parentID, err := g.resolveFileID(ctx, remotePath)
	if err != nil {
		if errors.Is(err, styxErrors.ErrStoreNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return g.listChildren(ctx, svc, parentID, remotePath)
}

func (g *GDrive) ListRecursive(ctx context.Context, remotePath string) ([]store.FileMeta, error) {
	svc, err := g.getService(ctx)
	if err != nil {
		return nil, err
	}

	parentID, err := g.resolveFileID(ctx, remotePath)
	if err != nil {
		if errors.Is(err, styxErrors.ErrStoreNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return g.listRecursive(ctx, svc, parentID, remotePath)
}

func (g *GDrive) listChildren(ctx context.Context, svc *drive.Service, parentID, prefix string) ([]store.FileMeta, error) {
	var result []store.FileMeta
	pageToken := ""

	for {
		call := svc.Files.List().
			Q(fmt.Sprintf("'%s' in parents and trashed = false", parentID)).
			Fields("nextPageToken, files(id, name, mimeType, md5Checksum, size, modifiedTime)").
			PageSize(200)

		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		list, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", styxErrors.ErrStoreUnavailable, err)
		}

		for _, f := range list.Files {
			relPath := prefix + "/" + f.Name
			result = append(result, store.FileMeta{
				Path:     relPath,
				Hash:     "md5:" + f.Md5Checksum,
				Size:     f.Size,
				ModTime:  parseDriveTime(f.ModifiedTime),
				IsDir:    f.MimeType == "application/vnd.google-apps.folder",
				RemoteID: f.Id,
			})
		}

		if list.NextPageToken == "" {
			break
		}
		pageToken = list.NextPageToken
	}

	return result, nil
}

func (g *GDrive) listRecursive(ctx context.Context, svc *drive.Service, parentID, prefix string) ([]store.FileMeta, error) {
	children, err := g.listChildren(ctx, svc, parentID, prefix)
	if err != nil {
		return nil, err
	}

	result := children
	for _, c := range children {
		if c.IsDir {
			sub, err := g.listRecursive(ctx, svc, c.RemoteID, c.Path)
			if err != nil {
				return nil, err
			}
			result = append(result, sub...)
		}
	}

	return result, nil
}

// [CHANGE DETECTION]

func (g *GDrive) ChangesSince(ctx context.Context, cursor string) ([]store.Change, string, error) {
	svc, err := g.getService(ctx)
	if err != nil {
		return nil, "", err
	}

	if cursor == "" {
		tok, err := svc.Changes.GetStartPageToken().Do()
		if err != nil {
			return nil, "", fmt.Errorf("%w: %v", styxErrors.ErrStoreUnavailable, err)
		}
		cursor = tok.StartPageToken
		return nil, cursor, nil // first call returns the starting token, no changes
	}

	var changes []store.Change
	pageToken := cursor

	for {
		list, err := svc.Changes.List(pageToken).
			Fields("nextPageToken, newStartPageToken, changes(fileId, removed, file(id, name, mimeType, md5Checksum, size, modifiedTime))").
			PageSize(100).Do()
		if err != nil {
			return nil, "", fmt.Errorf("%w: %v", styxErrors.ErrStoreUnavailable, err)
		}

		for _, c := range list.Changes {
			if c.Removed || c.File == nil {
				continue
			}

			path, err := g.filePath(ctx, c.File.Id)
			if err != nil {
				path = c.File.Name
			}

			changes = append(changes, store.Change{
				Path: path,
				FileMeta: &store.FileMeta{
					Path:     path,
					Hash:     "md5:" + c.File.Md5Checksum,
					Size:     c.File.Size,
					ModTime:  parseDriveTime(c.File.ModifiedTime),
					IsDir:    c.File.MimeType == "application/vnd.google-apps.folder",
					RemoteID: c.File.Id,
				},
			})
		}

		if list.NextPageToken == "" {
			cursor = list.NewStartPageToken
			break
		}
		pageToken = list.NextPageToken
	}

	return changes, cursor, nil
}

// [HELPERS]

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

func escapeQuery(s string) string {
	return strings.ReplaceAll(s, "'", "\\'")
}

func isNotFound(err error) bool {
	if gerr, ok := err.(*googleapi.Error); ok && gerr.Code == http.StatusNotFound {
		return true
	}
	return false
}

func parseDriveTime(t string) time.Time {
	parsed, err := time.Parse(time.RFC3339, t)
	if err != nil {
		return time.Time{}
	}
	return parsed
}
