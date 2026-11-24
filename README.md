# Styx

Sync local directories to Google Drive or S3 (including S3-compatible services).

## Requirements

- Go (see `go.mod`)
- Linux with a systemd user session (for the daemon)

## Build from source

```sh
git clone <repo-url> styx
cd styx
go mod download
go build -o styx  ./cmd/styx
go build -o styxd ./cmd/styxd
```

## Create your own Google Drive credentials

1. Create a project at <https://console.cloud.google.com>.
2. Enable **Google Drive API** (APIs & Services → Library).
3. Configure the **OAuth consent screen** (External), add scope
   `https://www.googleapis.com/auth/drive`, and add your account as a test user.
4. Credentials → Create credentials → **OAuth client ID** → **Desktop app**.
   Copy the Client ID and Client Secret.

Add them to `~/.styx/config.toml` (run `styx setup` once to create it):

```toml
[stores.gdrive]
client_id = "YOUR_ID.apps.googleusercontent.com"
client_secret = "YOUR_SECRET"
```

Or bake them into the binaries at build time (never commit the values):

```sh
# build.env (gitignored)
export STYX_CLIENT_ID="YOUR_ID.apps.googleusercontent.com"
export STYX_CLIENT_SECRET="YOUR_SECRET"

. ./build.env && bash build.sh
```

## Install

Keep both binaries in the same directory (the daemon commands look for `styxd`
next to `styx`):

```sh
install -m755 styx styxd ~/.local/bin/
```

## First run

```sh
styx setup
styx auth login --store gdrive
styx init --store gdrive --root "/Styx/notes"   # remote folder path
styx status
styx sync
```

## Daemon

```sh
styx daemon install
systemctl --user status styxd
styx log -f
```

## S3 (optional)

```toml
[stores.s3]
bucket = "my-bucket"
region = "us-east-1"
# endpoint = "http://localhost:9000"   # MinIO / R2 / B2
# access_key_id = "..."                # else the AWS credential chain
# secret_access_key = "..."
```

```sh
styx init --store s3 --root styx/notes
styx auth login --store s3
```

## Ignore files

`<dir>/.styx/config.toml`:

```toml
ignore = [".trash/", "node_modules/", "*.tmp", "*.conflict"]
```

Patterns without `/` match any path segment; patterns with `/` are anchored to
the directory root; a trailing `/` matches directories only.

## Notes

- Never commit credentials; `build.env` is gitignored.
- The daemon uses a **systemd user unit** (`~/.config/systemd/user/styxd.service`).
- OAuth apps in **Testing** mode get 7-day refresh tokens; publish the app or
  re-run `styx auth login` when needed.
