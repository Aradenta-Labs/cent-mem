#!/usr/bin/env bash
# Useful copy-pasteable recipes for interacting with centmem

set -euo pipefail

PROJ="${CENTMEM_PROJ:-my-app}"

echo "=== 1. Recalling context ==="
centmem recall "authentication and database architecture" --scope "project:$PROJ" --top 3

echo -e "\n=== 2. Setting key-value configuration facts ==="
centmem set --scope "project:$PROJ" --key "server.port" --value '8080' --tags config
centmem set --scope "project:$PROJ" --key "database.url" --value '"postgres://localhost:5432/myapp"' --tags config

echo -e "\n=== 3. Storing an architectural decision ==="
centmem put --scope "project:$PROJ" --type note \
  --content "We use structured logging with slog; all logs must output JSON." \
  --tags decision,logging

echo -e "\n=== 4. Fetching fact ==="
centmem get --scope "project:$PROJ" --key "server.port"

echo -e "\n=== 5. Listing recent memories ==="
centmem list --scope "project:$PROJ" --limit 5

echo -e "\n=== 6. Health check ==="
centmem doctor
