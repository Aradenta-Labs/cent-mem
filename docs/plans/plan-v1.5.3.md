# v1.5.3 — Integrations: MCP Server & Client, Remote REST, VS Code

**Version:** 1.5.3 (target)  
**Owner:** Aradenta Labs  
**Status:** Approved — Ready for implementation  
**Depends on:** v1.5.0; operates orthogonally to schema versions  
**Roadmap Reference:** [docs/plans/roadmap-v1.5.x.md](roadmap-v1.5.x.md)

---

## 0. Executive Summary & Objective

Up to **v1.5.2**, centmem primarily interfaces with AI agents through shell invocation (`centmem recall ...`) wrapped by agent skills installed in local file directories. While effective for CLI-first agents, modern AI coding environments require native protocols:
- **Model Context Protocol (MCP):** Supported natively by Claude Code, Cursor, Windsurf, Zed, and Google Antigravity.
- **Remote REST APIs:** Required for team deployments, shared development servers, and centralized browser dashboards.
- **IDE Native Extension:** Seamless developer memory recall inside the editor without context switching.

**v1.5.3 (Integrations)** delivers a unified integration suite across four key pillars:
1. **MCP Server (`centmem serve --mcp`):** A zero-dependency, stdio-based Model Context Protocol server exposing centmem tools directly to any MCP-compliant agent harness.
2. **MCP Client Enrichment:** Enables the capture classifier pipeline to query external MCP tools (documentation search, code lookup) to enrich context before saving memories.
3. **Remote REST API with Bearer Token Auth:** Allows `centmem ui` to bind to `0.0.0.0` securely guarded by mandatory token validation.
4. **VS Code Sidebar Extension (`centmem-vscode`):** A lightweight TypeScript extension providing real-time selection context recall and one-click memory capture from the editor.

---

## 1. Problem Statement & Root Causes

| # | Current Limitation | Root Cause in Code | v1.5.3 Solution |
|---|---|---|---|
| 1 | Agent skill installation overhead | Agents require shell access and filesystem permission to execute `centmem` CLI binaries and npm scripts. | `centmem serve --mcp` exposes standard JSON-RPC 2.0 stdio MCP interface. |
| 2 | Inability to access centmem on remote devboxes | `cmd/centmem/handlers_ui.go:51`: Web server binds exclusively to `127.0.0.1` and has no authentication layer. | Support `--host 0.0.0.0` with strict requirement for `--token <secret>` or `CENTMEM_UI_TOKEN`. |
| 3 | Lack of IDE workflow integration | Developers must switch to terminal or open a browser to query centmem while writing code. | VS Code extension automatically surfaces relevant memories based on active file and cursor selection. |
| 4 | Capture classification missing external context | `internal/capture/classifier.go`: Classifier only sees raw commit text or transcript lines; cannot inspect referenced URLs or docs. | MCP client allows classifier to invoke external MCP tools for contextual enrichment. |

---

## 2. Pillar 1: Model Context Protocol (MCP) Server

### 2.1 Protocol & Transport Design
- **Transport:** Standard input / standard output (`stdio`).
- **Framing:** Line-delimited JSON-RPC 2.0 messages.
- **Process Model:** Launched as a child process directly by agent harnesses (e.g. Claude Code, Cursor). Runs standalone and connects to the local SQLite database directly without requiring background daemons.
- **Dependencies:** Built entirely using Go standard library (`encoding/json`, `bufio`, `os`).

### 2.2 Exposed MCP Tools

| MCP Tool Name | Target CLI Command | Input Schema Highlights | Output Highlights |
|---|---|---|---|
| `centmem_recall` | `centmem recall` | `query` (str), `scope` (str), `top` (int), `type` (str), `tags` (array) | Scored memory list with access counts & matched rankers |
| `centmem_put` | `centmem put` | `content` (str), `scope` (str), `type` (note/log), `tags` (array) | Stored memory ID, content hash, suggested links |
| `centmem_set` | `centmem set` | `key` (str), `value` (str/json), `scope` (str), `tags` (array) | Updated fact memory ID |
| `centmem_get` | `centmem get` | `id` (int) | Complete memory object |
| `centmem_timeline` | `centmem timeline` | `scope` (str), `since` (str), `limit` (int) | Chronological memory stream |
| `centmem_stats` | `centmem stats` | `scope` (str) | Memory counts, types breakdown, importance histogram |
| `centmem_forget` | `centmem forget` | `id` (int) | Confirmation of archive/tombstone |

### 2.3 Agent Configuration Example

#### Claude Code (`~/.claude/mcp.json` or `.claude.json`):
```json
{
  "mcpServers": {
    "centmem": {
      "command": "centmem",
      "args": ["serve", "--mcp"]
    }
  }
}
```

#### Cursor (`~/.cursor/mcp.json`):
```json
{
  "mcpServers": {
    "centmem": {
      "command": "centmem",
      "args": ["serve", "--mcp"]
    }
  }
}
```

### 2.4 Internal Architecture (`internal/mcp/`)

```go
package mcp

type Server struct {
    store    *store.Store
    searcher *search.Searcher
    in       *bufio.Reader
    out      *json.Encoder
    mu       sync.Mutex
}

func (s *Server) Run(ctx context.Context) error {
    // Read JSON-RPC requests from os.Stdin
    // Dispatch initialize, tools/list, tools/call
    // Write JSON-RPC responses to os.Stdout
}
```

---

## 3. Pillar 2: MCP Client (Capture Enrichment)

When ingesting ambiguous commits or docs in `internal/capture`, the classifier can invoke external MCP servers configured in `config.toml`:

```toml
[capture.mcp]
enabled = false
servers = [
  { name = "docs-search", command = "npx", args = ["-y", "@modelcontextprotocol/server-everything"] }
]
tools = ["search_docs", "fetch_url"]
```

If enabled, before classifying a complex commit message, the classifier can run `tools/call` to fetch contextual summaries, feeding richer text into the 3-tier classification prompt.

---

## 4. Pillar 3: Remote REST API & Token Authentication

### 4.1 CLI Invocations

```bash
# Start remote server on all interfaces with bearer token
centmem ui --host 0.0.0.0 --port 4231 --token "sec_9a8f2bc0e194871da938b"

# Alternatively, pass via environment variable:
export CENTMEM_UI_TOKEN="sec_9a8f2bc0e194871da938b"
centmem ui --host 0.0.0.0
```

### 4.2 Security Constraints
1. **Binding Guard:** If `--host` is set to any non-loopback address (e.g. `0.0.0.0`, `192.168.x.x`, public IP) AND neither `--token` nor `CENTMEM_UI_TOKEN` is supplied:
   - Server **MUST refuse to start** and exit with code 1:
     `{"error": {"code": "ERR_INVALID_FLAG", "message": "Refusing to bind to non-loopback host without authentication token. Pass --token or set CENTMEM_UI_TOKEN."}}`
2. **Minimum Entropy:** The token must be at least 16 characters long.
3. **API Protection:** All `/api/*` routes validate:
   ```
   Authorization: Bearer <token>
   ```
   Missing or mismatched tokens return `HTTP 401 Unauthorized` with JSON:
   `{"ok": false, "error": "unauthorized: invalid or missing bearer token"}`.
4. **CORS:** Restrict allowed origins or allow configurable `allowed_origins` in `[server]` config.

---

## 5. Pillar 4: VS Code Extension (`editors/vscode`)

### 5.1 Architecture & Flow

```
┌────────────────────────────────────────────────────────┐
│                   VS Code Editor                       │
│  Active File: internal/store/store.go                  │
│  Selected: "sqlite_vec.Auto()"                         │
└───────────────────────────┬────────────────────────────┘
                            │
              (debounced 300ms event trigger)
                            │
                            ▼
┌────────────────────────────────────────────────────────┐
│             centmem-vscode Extension Host              │
│  Executes: centmem recall "store.go sqlite_vec.Auto()" │
│            --top 5 --inherit true                      │
└───────────────────────────┬────────────────────────────┘
                            │
                            ▼
┌────────────────────────────────────────────────────────┐
│               Sidebar Webview Provider                 │
│  - Displays matching memory cards                      │
│  - Shows scores, tags, and access frequency            │
│  - Quick Action: "Save Selection as Memory"            │
└────────────────────────────────────────────────────────┘
```

### 5.2 Extension Features
- **Contextual Recall Sidebar:** Updates automatically when switching tabs or selecting code blocks.
- **One-Click Save:** Right-click context menu: `Centmem: Save Selection as Memory` (triggers `centmem put --content "<selection>" --tags "code"`).
- **Settings:**
  - `centmem.binaryPath`: Path to `centmem` CLI executable (defaults to `PATH`).
  - `centmem.autoRecallOnSelection`: Boolean (default `true`).
  - `centmem.scope`: Default recall scope.

---

## 6. Testing & Verification Plan

### Test Suites:
1. **MCP Stdio Server Tests (`internal/mcp/server_test.go`):**
   - Mock stdin/stdout pipes.
   - Send `initialize` request $\to$ verify protocol version `2024-11-05` and capabilities.
   - Send `tools/list` $\to$ verify all 7 tools with compliant schemas.
   - Send `tools/call` for `centmem_put` and `centmem_recall` $\to$ verify valid tool results.
2. **Remote REST Security Tests (`internal/ui/auth_test.go`):**
   - Test non-loopback binding without token $\to$ verify startup failure.
   - Test API requests without `Authorization` header $\to$ verify 401 Unauthorized.
   - Test API requests with valid token $\to$ verify 200 OK.
3. **MCP Client Enrichment Tests (`internal/capture/mcp_test.go`):**
   - Mock external MCP server responding to `search_docs` tool calls.
4. **VS Code Extension Test (`editors/vscode/test/`):**
   - Extension integration tests using `@vscode/test-electron`.

---

## 7. Files Affected & Implementation Checklist

- [x] **MCP Server Engine:** `internal/mcp/server.go`, `tools.go`, `protocol.go` [NEW]
- [x] **MCP CLI Command:** `cmd/centmem/handlers_serve.go` (`centmem serve --mcp`) [NEW]
- [x] **MCP Client Subsystem:** `internal/capture/mcp_client.go` [NEW]
- [x] **Remote Token Auth:** `internal/ui/server.go` (middleware & token validation) [MODIFY]
- [x] **CLI UI Flags:** `cmd/centmem/handlers_ui.go` (add `--token` flag and security guard) [MODIFY]
- [x] **VS Code Extension:** `editors/vscode/package.json`, `src/extension.ts`, `src/sidebar.ts` [NEW]
- [x] **Docs:** Update `docs/cli-contract.md` and create `docs/guides/mcp-setup.md` [NEW/MODIFY]
- [x] **Knowledge Graph:** Update with `graphify update .` after completion
