package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	styxErrors "github.com/AdityaTaggar05/styx/internal/errors"
	"github.com/AdityaTaggar05/styx/internal/store"
)

// ManifestEntry tracks a single synced file in the manifest.
type ManifestEntry struct {
	Hash     string    `json:"hash"`
	Size     int64     `json:"size"`
	ModTime  time.Time `json:"mod_time"`
	RemoteID string    `json:"remote_id"`
}

// Manifest is the per-directory sync state file.
type Manifest struct {
	Version      int                       `json:"version"`
	Store        string                    `json:"store"`
	RemoteRoot   string                    `json:"remote_root"`
	LastSync     string                    `json:"last_sync"`
	RemoteCursor string                    `json:"remote_cursor"`
	Files        map[string]ManifestEntry  `json:"files"`
}

// ActionType classifies a sync operation to perform.
type ActionType int

const (
	ActionNoOp          ActionType = iota
	ActionPullNew                  // remote exists, not tracked — download
	ActionPushNew                  // local exists, not tracked — upload
	ActionPullUpdate               // remote changed, local unchanged — download
	ActionPushUpdate               // local changed, remote unchanged — upload
	ActionConflict                 // both changed — save .conflict, pull remote
	ActionRestoreLocal             // file deleted locally, exists on remote — pull
	ActionDeleteLocal              // file deleted remotely, exists locally — delete
	ActionForgetManifest           // file gone from both sides — clean manifest
)

// Action describes a single sync operation for one file.
type Action struct {
	Type      ActionType
	Path      string
	LocalHash string // hash of the local file (for push actions)
	RemoteID  string // remote file ID (for pull/delete actions)
}

// FileInfo holds local file metadata without computing the hash.
type FileInfo struct {
	Path    string
	Size    int64
	ModTime time.Time
}

// NewManifest creates an empty manifest for a store/remote pair.
func NewManifest(store, remoteRoot string) *Manifest {
	return &Manifest{
		Version:    1,
		Store:      store,
		RemoteRoot: remoteRoot,
		Files:      make(map[string]ManifestEntry),
	}
}

// LoadManifest reads and parses manifest.json from path.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", styxErrors.ErrSyncManifestCorrupt, path)
		}
		return nil, fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
	}

	m := &Manifest{}
	if err := json.Unmarshal(data, m); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", styxErrors.ErrSyncManifestCorrupt, path, err)
	}

	if m.Files == nil {
		m.Files = make(map[string]ManifestEntry)
	}

	return m, nil
}

// Save writes the manifest atomically (write .tmp → rename).
func (m *Manifest) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
	}

	tmp := path + ".tmp"
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("%w: %v", styxErrors.ErrSyncManifestCorrupt, err)
	}

	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("%w: %v", styxErrors.ErrFilePermission, err)
	}

	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
	}

	return nil
}

// ScanLocal walks a directory and returns FileInfo for every non-ignored file.
// Hashing is deferred — only size and modTime are captured here.
func ScanLocal(dir string, matcher *Matcher) ([]FileInfo, error) {
	var files []FileInfo

	err := filepath.WalkDir(dir, func(abs string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(dir, abs)
		if err != nil {
			return err
		}

		if rel == "." {
			return nil
		}

		// Skip the .styx directory itself
		if rel == ".styx" {
			return filepath.SkipDir
		}

		if matcher.Match(rel, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			files = append(files, FileInfo{
				Path:    rel,
				Size:    info.Size(),
				ModTime: info.ModTime(),
			})
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
	}

	return files, nil
}

// Diff compares local, manifest, and remote state and returns the ordered list
// of sync actions. Local files are hashed lazily — only when size or modTime
// differ from the manifest entry.
//
// dir is the absolute path to the local directory root.
func Diff(dir string, local []FileInfo, manifest *Manifest, remote []store.FileMeta) ([]Action, error) {
	// Build lookup maps
	localMap := make(map[string]*FileInfo, len(local))
	for i := range local {
		localMap[local[i].Path] = &local[i]
	}

	remoteMap := make(map[string]*store.FileMeta, len(remote))
	for i := range remote {
		remoteMap[remote[i].Path] = &remote[i]
	}

	// Union of all known paths
	paths := make(map[string]bool)
	for p := range localMap {
		paths[p] = true
	}
	for p := range manifest.Files {
		paths[p] = true
	}
	for p := range remoteMap {
		paths[p] = true
	}

	var actions []Action

	for path := range paths {
		localFile := localMap[path]
		maniEntry, inManifest := manifest.Files[path]
		remoteFile := remoteMap[path]

		localExists := localFile != nil
		remoteExists := remoteFile != nil

		// Determine local hash (only compute when needed)
		var localHash string
		if localExists {
			if inManifest && localFile.Size == maniEntry.Size && localFile.ModTime.Equal(maniEntry.ModTime) {
				// Size and modTime match manifest — assume hash hasn't changed
				localHash = maniEntry.Hash
			} else {
				absPath := filepath.Join(dir, path)
				h, err := HashFile(absPath)
				if err != nil {
					continue // skip files we can't read
				}
				localHash = h
			}
		}

		// Decision matrix
		switch {
		// [NEW FILES]
		case !localExists && !inManifest && remoteExists:
			actions = append(actions, Action{
				Type:     ActionPullNew,
				Path:     path,
				RemoteID: remoteFile.RemoteID,
			})

		case localExists && !inManifest && !remoteExists:
			actions = append(actions, Action{
				Type:      ActionPushNew,
				Path:      path,
				LocalHash: localHash,
			})

		// [TRACKED FILES]
		case localExists && inManifest && remoteExists:
			localChanged := localHash != maniEntry.Hash
			remoteChanged := remoteFile.Hash != maniEntry.Hash

			switch {
			case !localChanged && !remoteChanged:
				actions = append(actions, Action{Type: ActionNoOp, Path: path})
			case localChanged && !remoteChanged:
				actions = append(actions, Action{
					Type:      ActionPushUpdate,
					Path:      path,
					LocalHash: localHash,
				})
			case !localChanged && remoteChanged:
				actions = append(actions, Action{
					Type:     ActionPullUpdate,
					Path:     path,
					RemoteID: remoteFile.RemoteID,
				})
			default: // both changed
				actions = append(actions, Action{
					Type:     ActionConflict,
					Path:     path,
					RemoteID: remoteFile.RemoteID,
				})
			}

		// [DELETIONS]
		case !localExists && inManifest && !remoteExists:
			actions = append(actions, Action{Type: ActionForgetManifest, Path: path})

		case !localExists && inManifest && remoteExists:
			actions = append(actions, Action{
				Type:     ActionRestoreLocal,
				Path:     path,
				RemoteID: remoteFile.RemoteID,
			})

		case localExists && inManifest && !remoteExists:
			actions = append(actions, Action{Type: ActionDeleteLocal, Path: path})

		default:
			// Unreachable (all 8 combos covered above)
		}
	}

	// Order: deletes first (DeleteLocal, ForgetManifest), then the rest
	sort.SliceStable(actions, func(i, j int) bool {
		rank := func(a ActionType) int {
			if a == ActionDeleteLocal || a == ActionForgetManifest {
				return 0
			}
			return 1
		}
		return rank(actions[i].Type) < rank(actions[j].Type)
	})

	return actions, nil
}
