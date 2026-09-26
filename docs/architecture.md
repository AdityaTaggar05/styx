# Styx — Architecture Reference

## Overview

Styx syncs local directories to cloud data stores (GDrive first, extensible to Dropbox/OneDrive). Two binaries: the `styx` CLI and the `styxd` systemd daemon.

---

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Hashing | SHA-256 (local), MD5 (remote) | Built into `crypto/sha256`; Drive returns MD5 via API metadata |
| Conflict resolution | Remote wins, local → `.conflict` | Safest for multi-device; preserves local changes |
| Change detection | Size + mtime first, hash on mismatch | Avoids expensive SHA-256 on unchanged files |
| Config format | TOML (config) + JSON (manifest) | TOML is human-editable; JSON suits machine-managed maps |
| Bundled credentials | Client ID/secret embedded at build time via `-ldflags` | No GCP project setup required for users (rclone-style) |
| Comment style | `// [HEADING]` | Brief, scannable section markers |
| Cobra constructors | No `new` prefix (e.g., `RootCommand()`, `initCmd()`) | Cleaner call sites |
| Daemon debounce | 10000ms default | Balances responsiveness vs noise reduction |
| Daemon remote detection | Periodic full sync (300000ms default) | Reuses the sync engine; no cursor bookkeeping |
| Daemon start mode | Detached background (`Setsid`) | `styx daemon start` returns to the shell; stop/status via PID file |
| File watching | fsnotify, recursive by walking + adding on create | fsnotify has no native recursion |
| Backend registry | Factory pattern: `store.Register()` in `init()` | Adding a backend = one package, no changes to core code |
| Conflict file handling | `*.conflict` auto-added to default ignore list | Users can remove from config to track conflicts remotely |

---

## Project Structure

```
styx/
├── cmd/
│   ├── styx/main.go          # CLI entry point + gdrive side-effect import
│   └── styxd/main.go         # Daemon entry point + gdrive side-effect import
├── internal/
│   ├── cli/                  # Cobra command tree
│   │   ├── root.go           # RootCommand(), persistent flags, subcommand wiring
│   │   ├── setup.go          # Guided bootstrap wizard
│   │   ├── auth.go           # auth login, logout, status
│   │   ├── init.go           # Initialize directory for syncing
│   │   ├── remove.go         # Remove from registry (supports . for cwd)
│   │   ├── list.go           # List registered directories
│   │   ├── sync.go           # sync + status commands (Phase 3)
│   │   ├── daemon.go         # daemon start/stop/status/install/uninstall
│   │   └── log.go            # log [--follow]
│   ├── config/               # Configuration loading
│   │   ├── paths.go          # Well-known paths, ExpandPath() for ~
│   │   ├── global.go         # GlobalConfig (TOML): stores, directories, daemon
│   │   └── local.go          # LocalConfig (TOML): per-directory settings
│   ├── errors/               # Centralized sentinel errors
│   │   └── errors.go         # 7 categories, 30 errors total
│   ├── log/                  # Slog setup
│   │   └── logger.go         # SetupCLI, SetupDaemon (multi-handler via slog.NewMultiHandler)
│   ├── store/                # Data store abstraction
│   │   ├── store.go          # DataStore interface + FileMeta, Change types
│   │   ├── registry.go       # Register/Get/List — factory pattern
│   │   └── gdrive/           # Google Drive implementation
│   │       ├── gdrive.go     # GDrive struct, New(), init() registration, full CRUD
│   │       ├── auth.go       # OAuth2: AuthURL, Authenticate, IsAuthenticated (auto-refresh), Logout
│   │       └── crypto.go     # AES-256-GCM token encryption (key from /etc/machine-id)
│   ├── sync/                 # Sync engine (Phase 3 — complete)
│   │   ├── hasher.go         # SHA-256 streaming file hasher
│   │   ├── matcher.go        # .gitignore-style ignore pattern matcher
│   │   ├── manifest.go       # Manifest load/save (atomic) + Diff (lazy hashing)
│   │   ├── conflict.go       # SaveConflict (rename local before remote overwrite)
│   │   └── engine.go         # Sync() — full cycle: scan → diff → execute → save
│   └── daemon/               # Daemon process (Phase 4 — complete)
│       ├── daemon.go         # Daemon.Run — watcher + debouncers + periodic poll
│       ├── watcher.go        # Recursive fsnotify wrapper (event → owning root)
│       ├── debounce.go       # Timer-reset debouncer with running guard
│       ├── pid.go            # PID file write/read/remove/liveness
│       └── systemd.go        # systemd user unit install/uninstall
└── docs/
    ├── architecture.md       # This file
    └── implementation.md     # Build status and implementation notes
```

---

## Config Files

### `~/.styx/config.toml` — Global Config

```toml
version = 1

directories = [
    "/home/alice/projects",
]

[daemon]
log_level = "info"
log_file = "~/.styx/styxd.log"
debounce_ms = 10000
poll_interval_ms = 300000
pid_file = "~/.styx/styxd.pid"
```

The `[stores.gdrive]` section is optional — if missing, bundled credentials are used. It can be populated to override the bundled client ID/secret or set a custom `token_file` path.

### `<dir>/.styx/config.toml` — Per-Directory Config

```toml
version = 1
store = "gdrive"
remote_root = "/styx/projects"
conflict_suffix = ".conflict"

ignore = [
    ".git/",
    "node_modules/",
    "*.tmp",
    "*.conflict",
]
```

Each directory has its own `remote_root`, even when using the same store.

### `<dir>/.styx/manifest.json` — Per-Directory Manifest

```json
{
  "version": 1,
  "store": "gdrive",
  "remote_root": "/styx/projects",
  "last_sync": "2026-01-15T10:30:00Z",
  "remote_cursor": "CkRCNjgzNTc4NhIGc3R5eC1z",
  "files": {
    "src/main.go": {
      "hash": "sha256:e3b0c44...",
      "remote_hash": "md5:d41d8cd...",
      "size": 2048,
      "mod_time": "2026-01-15T10:00:00Z",
      "remote_id": "1aBcDef..."
    }
  }
}
```

Saved atomically (write `.tmp` → rename). `hash` is local SHA-256; `remote_hash` is the Drive checksum
(md5 or sha256 depending on backend) — used to independently detect local vs remote changes.

---

## DataStore Interface

```go
type DataStore interface {
    Name() string

    // Auth
    AuthURL() string
    Authenticate(ctx context.Context, code string) error
    IsAuthenticated(ctx context.Context) (bool, error)
    Logout(ctx context.Context) error

    // CRUD
    Pull(ctx context.Context, remotePath, localPath string) error
    Push(ctx context.Context, localPath, remotePath string) (*FileMeta, error)
    Delete(ctx context.Context, remotePath string) error

    // Metadata
    Hash(ctx context.Context, remotePath string) (string, error)
    List(ctx context.Context, remotePath string) ([]FileMeta, error)
    ListRecursive(ctx context.Context, remotePath string) ([]FileMeta, error)

    // Incremental sync
    ChangesSince(ctx context.Context, cursor string) ([]Change, string, error)
}
```

### Backend Registration

```go
// In gdrive.go init():
func init() {
    store.Register("gdrive", func(cfg map[string]string) (store.DataStore, error) {
        return New(cfg)
    })
}
```

Adding Dropbox: create `internal/store/dropbox/`, implement `DataStore`, call `store.Register("dropbox", ...)` in `init()`.

---

## Sync Decision Matrix

For each file in the union of (local FS, manifest, remote):

| Local State | Manifest State | Remote State | Action |
|-------------|---------------|--------------|--------|
| — | — | exists | Pull new remote file |
| exists | — | — | Push new local file |
| exists (hash=man) | exists | exists (rem_hash=man) | No-op |
| exists (hash≠man) | exists | exists (rem_hash=man) | Push local changes |
| exists (hash=man) | exists | exists (rem_hash≠man) | Pull remote changes |
| exists (hash≠man) | exists | exists (rem_hash≠man) | **Conflict** — pull remote, local → `.conflict` |
| — | exists | — | Delete manifest entry |
| — | exists | exists | Pull (restore local) |
| exists | exists | — | Delete locally |

### Lazy Hashing

To avoid expensive SHA-256 on unchanged files:
1. `ScanLocal` captures **size** and **modTime** (cheap stat call) for every local file
2. `Diff` compares size/modTime against the manifest entry first
3. Only when size or modTime **differs** from the manifest is the file hashed
4. Remote hashes (MD5 from Drive) are returned by `ListRecursive` — no extra API calls

### Dual Hash System

The manifest stores two hashes per file:
- `hash` — local SHA-256 at time of last sync (detects **local** modifications)
- `remote_hash` — Drive checksum at time of last sync (detects **remote** modifications)

These are compared independently: `hash ≠ manifest.hash` means local changed; `remote.hash ≠ manifest.remote_hash` means remote changed.

---

## OAuth2 Flow

1. `styx auth login --store gdrive` loads the store, gets `AuthURL()`
2. Starts `localhost:8972` HTTP server, opens browser to Google consent URL
3. User grants access → Google redirects to `localhost:8972/callback?code=...`
4. Server receives code → calls `Authenticate(ctx, code)` → exchanges for token
5. Token encrypted with AES-256-GCM (key from `/etc/machine-id`) → saved to `~/.styx/gdrive-token.enc`
6. `IsAuthenticated()` auto-refreshes expired tokens via `oauth2.TokenSource`

### Bundled Credentials

```go
// Set at build time, empty in source
var (
    ClientID     string
    ClientSecret string
)
```

Build command:
```
go build -ldflags "-X github.com/AdityaTaggar05/styx/internal/store/gdrive.ClientID=xxx \
                   -X github.com/AdityaTaggar05/styx/internal/store/gdrive.ClientSecret=yyy" \
          -o styx ./cmd/styx/
go build -ldflags "..." -o styxd ./cmd/styxd/
```

Both binaries need the same ldflags since both reference gdrive.

---

## CLI Command Reference

| Command | Status | Description |
|---------|--------|-------------|
| `styx setup` | Done | Bootstrap config and `~/.styx/` directory |
| `styx auth login --store <name>` | Done | OAuth2 browser flow |
| `styx auth logout --store <name>` | Done | Delete token |
| `styx auth status --store <name>` | Done | Check if authenticated |
| `styx init [--store gdrive] [--root <path>]` | Done | Register directory, prompt for remote path |
| `styx remove <path>` | Done | Unregister directory (supports `.`) |
| `styx list` | Done | Show all registered directories |
| `styx sync [path]` | Done | Force immediate sync of directory (or all) |
| `styx status [path]` | Done | Dry-run: show pending pushes, pulls, conflicts |
| `styx daemon start` | Done | Spawn styxd detached; writes PID file |
| `styx daemon stop` | Done | SIGTERM the daemon; waits for exit |
| `styx daemon status` | Done | Report running/not running via PID file |
| `styx daemon install` | Done | Install + enable systemd user unit |
| `styx daemon uninstall` | Done | Disable + remove systemd user unit |
| `styx log [--follow]` | Done | View daemon log (JSON+text); follow with `-f` |

---

## Daemon Workflow

### `styxd` (the daemon binary)

```
1. Load ~/.styx/config.toml
2. Set up logging: text → stderr, JSON → ~/.styx/styxd.log
3. Acquire PID file (refuse if another pid is alive); defer removal
4. For each registered directory: load .styx/config.toml, build the store,
   verify auth — skip with a warning if either fails
5. Start recursive fsnotify watcher over every registered root
6. Start one debouncer per directory (DebounceMS, default 10s)
7. Start a periodic ticker (PollIntervalMS, default 5m) → triggers a sync per dir
8. On SIGINT/SIGTERM: stop watchers, drain debouncers, remove PID
```

- **Local changes** → watcher maps the event to its owning root → that dir's debouncer
  fires after the quiet interval → `sync.Sync`.
- **Remote changes** → the periodic ticker triggers a full `sync.Sync` per directory.
- **Overlap protection**: while a sync runs, further triggers queue a single re-run
  instead of starting a second sync.
- **Ignored**: everything under `.styx/` and `*.tmp` scratch files (no feedback loop).

### Two ways to run it

| Path | Command | Behaviour |
|------|---------|-----------|
| Manual | `styx daemon start` | Spawns `styxd` detached (`Setsid`), returns to the shell |
| systemd | `styx daemon install` | Writes `~/.config/systemd/user/styxd.service`, `enable --now` |

Both converge on the same `styxd` process and PID file, so `stop`/`status` work either way.
