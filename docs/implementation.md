# Styx — Implementation Summary

## Build Status

```
go build ./...          ✓ passes
go vet ./...            ✓ passes
```

Both `cmd/styx` and `cmd/styxd` compile.

---

## Files Implemented (30 total)

### Phase 1 — Foundation (complete)

| File | Lines | Purpose |
|------|-------|---------|
| `internal/errors/errors.go` | 64 | 30 sentinel errors across 7 categories |
| `internal/config/paths.go` | 61 | Path constants + `ExpandPath()` for `~` |
| `internal/config/global.go` | 141 | `GlobalConfig` TOML CRUD + validate + directory add/remove |
| `internal/config/local.go` | 87 | `LocalConfig` per-directory TOML |
| `internal/log/logger.go` | 56 | `slog.NewMultiHandler` dual logging setup |
| `internal/store/store.go` | 57 | `DataStore` interface + `FileMeta`/`Change` types |
| `internal/store/registry.go` | 55 | Register/Get/List factory pattern |
| `internal/cli/root.go` | 118 | Full cobra tree, persistent flags |
| `cmd/styx/main.go` | 14 | Entry point + gdrive side-effect import |
| `cmd/styxd/main.go` | 13 | Daemon entry point + gdrive import |

### Phase 2 — CLI + GDrive Auth (complete)

| File | Lines | Purpose |
|------|-------|---------|
| `internal/store/gdrive/gdrive.go` | — | GDrive struct, bundled creds, `New()`, `init()` registration, full CRUD (replaced stubs in Phase 3) |
| `internal/store/gdrive/auth.go` | 124 | OAuth2 flow: AuthURL, Authenticate, IsAuthenticated (auto-refresh), Logout |
| `internal/store/gdrive/crypto.go` | 68 | AES-256-GCM encrypt/decrypt, key from `/etc/machine-id` |
| `internal/cli/auth.go` | 182 | auth login/logout/status commands, localhost callback server |
| `internal/cli/setup.go` | 75 | Guided first-run bootstrap wizard |
| `internal/cli/init.go` | 104 | init: prompt for remote path, create .styx/, write config + manifest |
| `internal/cli/remove.go` | 45 | remove: unregister directory (supports `.`) |
| `internal/cli/list.go` | 34 | list: print registered directories |

### Phase 3 — Sync Engine (complete)

| File | Lines | Purpose |
|------|-------|---------|
| `internal/sync/hasher.go` | 28 | `HashFile()` — SHA-256 streaming hash via `io.Copy` |
| `internal/sync/matcher.go` | 57 | Pattern matcher for ignore rules (matches against path segments) |
| `internal/sync/manifest.go` | 245 | Manifest struct (dual hash), atomic save, `ScanLocal`, `Diff` (lazy hashing) |
| `internal/sync/conflict.go` | 16 | `SaveConflict()` — rename local file before remote overwrite |
| `internal/sync/engine.go` | 136 | `Sync()` — full cycle: scan → diff → execute → save manifest |
| `internal/cli/sync.go` | 178 | `styx sync` and `styx status` commands (supports `.`) |

### Phase 4 — Daemon (complete)

| File | Purpose |
|------|---------|
| `internal/daemon/daemon.go` | `Daemon.Run` — per-dir workers, watcher + debouncers + periodic poll, PID guard, graceful shutdown |
| `internal/daemon/watcher.go` | Recursive fsnotify wrapper; maps events to owning root; ignores `.styx/` and `*.tmp` |
| `internal/daemon/debounce.go` | Timer-reset debouncer with `running`/`pending` guard; `Stop`+`Wait` for shutdown |
| `internal/daemon/pid.go` | PID file write/read/remove + `IsRunning` via signal 0 |
| `internal/daemon/systemd.go` | systemd user unit generation + `systemctl --user` install/uninstall |
| `internal/cli/daemon.go` | `daemon start/stop/status/install/uninstall`; locates `styxd` next to `styx` |
| `internal/cli/log.go` | `styx log [--follow]` with partial-line buffering |
| `cmd/styxd/main.go` | Daemon entry: `--config`, `SetupDaemon`, signal context, `Daemon.Run` |

### Phase 5 — S3 Backend (complete)

| File | Purpose |
|------|---------|
| `internal/store/s3/s3.go` | S3 struct, `New()`, `init()` registration, full CRUD via AWS SDK v2; configurable endpoint (path-style); `md5:<ETag>` hashes |

Also refactored:
- `internal/config/global.go` — `Stores` is now `map[string]map[string]string` (generic per-backend settings).
- `internal/cli/auth.go`, `internal/cli/sync.go`, `internal/daemon/daemon.go` — pass the config map directly; `auth login` skips the browser for key-based stores (empty `AuthURL`).

---

## Key Implementation Details

### Credential Bundling

OAuth `client_id` and `client_secret` are package-level vars in `store/gdrive`:

```go
var (
    ClientID     string
    ClientSecret string
)
```

Set at build time via `-ldflags`. `New()` falls back from config map → build-time vars. Empty in source (never committed).

### Error System

No `errors.New()` or `fmt.Errorf()` outside `internal/errors/errors.go`. All other packages wrap sentinels:

```go
return fmt.Errorf("%w: %v", styxErrors.ErrAuthInvalid, err)
```

Callers check with `errors.Is()`.

### Store Registry

Backends self-register in `init()`:

```go
func init() {
    store.Register("gdrive", func(cfg map[string]string) (store.DataStore, error) {
        return New(cfg)
    })
}
```

CLI and daemon import packages via side-effect (`_ "..."`) to trigger registration.

### Token Encryption

AES-256-GCM with key derived from `/etc/machine-id` (SHA-256). Falls back to hardcoded key if machine-id is unavailable. Nonce prepended to ciphertext.

### Config TOML vs Manifest JSON

- **Config files** use TOML — human-editable, section-based, supports comments
- **Manifest** stays JSON — machine-managed, efficient for large `files` maps

### Directory → Store Binding

Each directory's `.styx/config.toml` declares which store and remote path. Multiple directories can use the same store with different `remote_root` values. Changing a directory's store doesn't touch other directories or global config.

### CLI OAuth Flow

1. `auth login` loads store, gets `AuthURL()`
2. Starts `localhost:8972` HTTP server in a goroutine
3. Opens browser to Google consent URL
4. Blocks on channel — either receives code or error
5. On code: calls `Authenticate()`, saves token path to config
6. On error/timeout: shuts down server, returns error

### Cobra Command Constructors

No `new` prefix: `RootCommand()`, `initCmd()`, `authLoginCmd()`, etc.

### `.` Path Support

`styx sync`, `styx status`, and `styx remove` resolve `.` to the current working directory. Relative paths are converted to absolute before matching against the registry.

### Comment Headers

`// [HEADING]` style for section markers.

### Phase 3 — Sync Engine

### Lazy Hashing

`ScanLocal()` uses `filepath.WalkDir` and captures only `Size` and `ModTime` from `os.FileInfo` — no hashing. `Diff()` compares size + modTime against the manifest first and only invokes `HashFile()` when they differ. This avoids expensive SHA-256 on unchanged files.

### Dual Hash System

The manifest entry stores two hashes:

```go
type ManifestEntry struct {
    Hash       string    // local SHA-256 at time of sync
    RemoteHash string    // Drive checksum (md5:...) at time of sync
    Size       int64
    ModTime    time.Time
    RemoteID   string
}
```

- `localHash != manifest.Hash` → local file changed → push or conflict
- `remoteFile.Hash != manifest.RemoteHash` → remote file changed → pull or conflict
- Both differ → conflict (remote wins, local renamed to `.conflict`)

### Pattern Matcher

`Matcher` matches ignore patterns against every path **segment** — `node_modules/` catches that directory at any depth without `**`. Supports trailing `/` for directory-only matching. `**/` prefix is accepted for gitignore compat but not required.

### GDrive CRUD

`Push` checks if the file already exists (update vs create) to avoid duplicates. Missing remote folders are auto-created via `resolveOrCreateFolder`. `ListRecursive` walks subfolders depth-first. `ChangesSince` uses Drive's `Changes.list` API with page tokens. All path operations go through `resolveFileID` which walks Drive's parent references segment by segment.

### Manifest Atomic Writes

`Save()` writes to `<path>.tmp` then `os.Rename` to the real path. On POSIX, `rename` is atomic — readers never see a half-written file.

### Conflict File Auto-Ignore

`DefaultLocalConfig` includes `"*.conflict"` in the `ignore` list. Users can remove it to track conflicts remotely across devices.

### Phase 4 — Daemon

### Engine Logging

`sync.Sync()` takes a `*slog.Logger`. The CLI passes `log.SetupCLI` (text → stderr); the daemon passes `log.SetupDaemon` (text → stderr, JSON → file). No `fmt.Printf` remains in the engine.

### Recursive Watching

fsnotify has no recursive mode. On startup, `Watcher.addRecursive` walks each registered root and calls `Add` for every subdirectory. When a `Create` event names a directory, it is walked and added too (there is a small race for files created inside a brand-new dir before the watch lands — the periodic poll is the backstop). Events under `.styx/` and `*.tmp` are dropped to avoid feedback from manifest writes.

### Debouncing

Each directory owns a `Debouncer`: a single `time.Timer` that is reset on every trigger. When it fires, `Sync` runs. A `running` flag means a trigger during an in-flight sync just sets `pending`, so runs never overlap. `Stop` prevents new runs and `Wait` blocks until the in-flight one finishes (used for graceful shutdown).

### Detached Start & PID

`styx daemon start` locates `styxd` next to the running `styx` binary (`os.Executable`), opens the log file, and starts the child with `SysProcAttr{Setsid: true}` so it survives the parent shell. It then waits for the daemon to write its PID file. `IsRunning` uses signal `0`; `stop` sends `SIGTERM` and polls until exit; a stale PID file is cleaned up.

### systemd User Unit

`daemon.Install` writes `~/.config/systemd/user/styxd.service` (`Type=simple`, `Restart=on-failure`, absolute `ExecStart`) then runs `systemctl --user daemon-reload` and `enable --now`. `Uninstall` disables, removes the file, and reloads. Honors `XDG_CONFIG_HOME`.

### Phase 5 — S3 Backend

`S3` implements `DataStore` with AWS SDK v2. `getClient` builds the client lazily: region defaults to `us-east-1`; explicit `access_key_id`/`secret_access_key` use a static credentials provider, otherwise the default AWS chain; a custom `endpoint` enables path-style addressing (MinIO/R2/B2).

- **Path mapping**: `objectKey` strips the leading `/` (`/styx/projects/a.txt` → `styx/projects/a.txt`); `prefixFor` appends `/` for listings.
- **CRUD**: `Push` = single `PutObject`; `Pull` = `GetObject`; `Delete` = `DeleteObject` (idempotent); `Hash` = `HeadObject` ETag.
- **Listing**: `ListRecursive` paginates `ListObjectsV2` over the prefix and skips zero-byte `dir/` markers; `List` uses delimiter `/` for one level.
- **Hashes**: `md5:<ETag>` — the ETag of a single-part upload is the object MD5, so it fits the dual-hash manifest unchanged.
- **Auth**: `AuthURL()` is empty; `auth login` detects this and calls `Authenticate` (a `HeadBucket` check) directly; `Logout` is a no-op.
- **Changes**: `ChangesSince` returns nothing — S3 has no change feed, so the periodic full sync covers remote changes.

---

## Dependencies

| Package | Version | Use |
|---------|---------|-----|
| `github.com/spf13/cobra` | v1.10.2 | CLI framework |
| `github.com/BurntSushi/toml` | v1.6.0 | TOML config parsing |
| `github.com/fsnotify/fsnotify` | v1.10.1 | Recursive filesystem watching (daemon) |
| `golang.org/x/oauth2` | v0.36.0 | OAuth2 client |
| `google.golang.org/api/drive/v3` | v0.281.0 | GDrive API (full CRUD, listing, changes) |
| `github.com/aws/aws-sdk-go-v2` | v1.47.8 | AWS SDK core |
| `github.com/aws/aws-sdk-go-v2/config` | v1.33.17 | AWS config + credential chain |
| `github.com/aws/aws-sdk-go-v2/credentials` | v1.20.17 | Static credentials provider |
| `github.com/aws/aws-sdk-go-v2/service/s3` | v1.114.9 | S3 client |
| `github.com/aws/smithy-go` | v1.28.8 | API error inspection |

---

## Next

All five phases are complete: foundation, CLI + auth, sync engine, daemon, S3 backend.

Possible follow-ups:
- **Tests** — the repo still has none; unit tests for `sync.Diff`, `Matcher`, `Debouncer`, `pid`, and S3 path mapping are the highest value.
- **S3 setup UX** — `setup`/`init` don't scaffold `[stores.s3]`; users hand-edit the config.
- **Incremental remote polling** — wire gdrive's `ChangesSince` + the manifest `remote_cursor` to replace the periodic full sync (S3 has no change feed).
- **More backends** — Dropbox/OneDrive via the `store.Register` factory.
- **Config hot-reload** — the daemon reads config once at startup; a reload would avoid restarts.
