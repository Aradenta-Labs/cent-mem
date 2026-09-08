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
# Store two related decisions and capture their IDs
MEM_A=$(centmem put --scope "project:$PROJ" --type note --content "Decision A: Use Postgres with pgvector." --tags decision,vector | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
MEM_B=$(centmem put --scope "project:$PROJ" --type note --content "Decision B: Switch to sqlite-vec; pgvector is deprecated." --tags decision,vector | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)

# Explicitly link memories with a directional relationship (MEM_B supersedes MEM_A)
centmem link "$MEM_B" "$MEM_A" --relation supersedes

# List relationship graph edges for a memory (including auto-suggested)
centmem links "$MEM_B" --all

# Recall context expanded with 1-hop relationship graph edges
centmem recall "vector database" --scope "project:$PROJ" --include-links

# Confirm or dismiss auto-suggested links when suggested_links are returned:
# centmem link confirm <link_id>
# centmem link dismiss <link_id>

# Remove relationship links between memories
centmem unlink "$MEM_B" "$MEM_A" --relation supersedes
