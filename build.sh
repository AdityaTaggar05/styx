#!/bin/bash
set -e

# Credentials come from the environment (see build.env, which is gitignored).
# Example: source build.env && bash build.sh
CLIENT_ID="${STYX_CLIENT_ID:-}"
CLIENT_SECRET="${STYX_CLIENT_SECRET:-}"

cd "$(dirname "$0")"

LDFLAGS="-X github.com/AdityaTaggar05/styx/internal/store/gdrive.ClientID=${CLIENT_ID} -X github.com/AdityaTaggar05/styx/internal/store/gdrive.ClientSecret=${CLIENT_SECRET}"

go build -ldflags "${LDFLAGS}" -o styx ./cmd/styx/
go build -ldflags "${LDFLAGS}" -o styxd ./cmd/styxd/

echo "Built styx and styxd"
