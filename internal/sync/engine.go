package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AdityaTaggar05/styx/internal/config"
	"github.com/AdityaTaggar05/styx/internal/store"
)

// Sync runs a full sync cycle on a single registered directory.
func Sync(ctx context.Context, rootDir string, cfg *config.LocalConfig, st store.DataStore) error {
	manifestPath := filepath.Join(rootDir, ".styx", "manifest.json")

	// Load or create manifest
	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		manifest = NewManifest(st.Name(), cfg.RemoteRoot)
	}

	// Scan local filesystem (with ignore patterns from config)
	matcher := NewMatcher(cfg.Ignore)
	localFiles, err := ScanLocal(rootDir, matcher)
	if err != nil {
		return fmt.Errorf("scanning local: %w", err)
	}

	// List remote files under the configured remote root
	remoteFiles, err := st.ListRecursive(ctx, cfg.RemoteRoot)
	if err != nil {
		return fmt.Errorf("listing remote: %w", err)
	}

	// Strip remote root prefix so remote paths match local relative paths
	remoteRootPrefix := strings.TrimRight(cfg.RemoteRoot, "/") + "/"
	normalizedRemote := make([]store.FileMeta, 0, len(remoteFiles))
	for _, rf := range remoteFiles {
		if strings.HasPrefix(rf.Path, remoteRootPrefix) {
			rf.Path = rf.Path[len(remoteRootPrefix):]
		}
		normalizedRemote = append(normalizedRemote, rf)
	}

	// Compute diff
	actions, err := Diff(rootDir, localFiles, manifest, normalizedRemote)
	if err != nil {
		return fmt.Errorf("computing diff: %w", err)
	}

	remotePath := func(rel string) string {
		return strings.TrimRight(cfg.RemoteRoot, "/") + "/" + rel
	}

	// Execute actions
	for _, a := range actions {
		localPath := filepath.Join(rootDir, a.Path)
		rPath := remotePath(a.Path)

		switch a.Type {
		case ActionNoOp:
			continue

		case ActionPullNew, ActionPullUpdate, ActionRestoreLocal:
			if err := st.Pull(ctx, rPath, localPath); err != nil {
				return fmt.Errorf("pull %s: %w", a.Path, err)
			}
			// Update manifest from remote metadata
			meta, err := st.Hash(ctx, rPath)
			if err != nil {
				return fmt.Errorf("hash remote %s: %w", a.Path, err)
			}
			entry := manifest.Files[a.Path]
			entry.RemoteHash = meta
			entry.RemoteID = a.RemoteID
			// Rehash local to capture the new file
			localHash, _ := HashFile(localPath)
			entry.Hash = localHash
			info, _ := os.Stat(localPath)
			if info != nil {
				entry.Size = info.Size()
				entry.ModTime = info.ModTime()
			}
			manifest.Files[a.Path] = entry
			fmt.Printf("  pulled → %s\n", a.Path)

		case ActionPushNew, ActionPushUpdate:
			meta, err := st.Push(ctx, localPath, rPath)
			if err != nil {
				return fmt.Errorf("push %s: %w", a.Path, err)
			}
			manifest.Files[a.Path] = ManifestEntry{
				Hash:       a.LocalHash,
				RemoteHash: meta.Hash,
				Size:       meta.Size,
				ModTime:    meta.ModTime,
				RemoteID:   meta.RemoteID,
			}
			fmt.Printf("  pushed → %s\n", a.Path)

		case ActionConflict:
			conflictPath, err := SaveConflict(localPath, cfg.ConflictSuffix)
			if err != nil {
				return fmt.Errorf("saving conflict for %s: %w", a.Path, err)
			}
			if err := st.Pull(ctx, rPath, localPath); err != nil {
				return fmt.Errorf("pull conflict winner %s: %w", a.Path, err)
			}
			meta, _ := st.Hash(ctx, rPath)
			localHash, _ := HashFile(localPath)
			info, _ := os.Stat(localPath)
			entry := ManifestEntry{
				RemoteHash: meta,
				RemoteID:   a.RemoteID,
			}
			if info != nil {
				entry.Size = info.Size()
				entry.ModTime = info.ModTime()
			}
			entry.Hash = localHash
			manifest.Files[a.Path] = entry
			fmt.Printf("  conflict → %s (local saved to %s)\n", a.Path, conflictPath)

		case ActionDeleteLocal:
			if err := os.Remove(localPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("deleting %s: %w", a.Path, err)
			}
			delete(manifest.Files, a.Path)
			fmt.Printf("  deleted → %s\n", a.Path)

		case ActionForgetManifest:
			delete(manifest.Files, a.Path)
			fmt.Printf("  cleaned → %s\n", a.Path)
		}
	}

	manifest.LastSync = time.Now().UTC().Format(time.RFC3339)
	if err := manifest.Save(manifestPath); err != nil {
		return fmt.Errorf("saving manifest: %w", err)
	}

	fmt.Printf("Sync complete. %d file(s) processed.\n", len(actions))
	return nil
}
