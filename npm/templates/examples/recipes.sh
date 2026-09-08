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

echo -e "\n=== 7. Capturing developer artifacts ==="
# Ingest git commits and dependency changes
centmem capture git --scope "project:$PROJ" --max-commits 20

# Ingest project documentation
centmem capture docs --scope "project:$PROJ" --dir docs

# Ingest shell workflows and recurring command patterns
centmem capture shell --scope "project:$PROJ" --top 10

# Scan code annotations (TODOs, FIXMEs)
centmem capture comments --scope "project:$PROJ" --ext go,ts,js,py

echo -e "\n=== 8. Managing memory relationships (Link Graph) ==="
# Explicitly link two memories with a directional relationship
centmem link 42 15 --relation supersedes

# List relationship graph edges for a memory (including auto-suggested)
centmem links 42 --all

# Recall context expanded with 1-hop relationship graph edges
centmem recall "logging architecture" --scope "project:$PROJ" --include-links

# Confirm an auto-suggested link
centmem link confirm 12

# Dismiss an auto-suggested link
centmem link dismiss 13

# Remove relationship links between memories
centmem unlink 42 15


