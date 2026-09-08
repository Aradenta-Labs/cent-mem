# Model Context Protocol (MCP) Setup Guide for centmem

centmem exposes a native, zero-dependency stdio Model Context Protocol (MCP) server via `centmem serve` (or `centmem serve --mcp`). Any MCP-compliant agent harness can launch `centmem serve` as a child process and execute centmem tools natively.

---

## 1. Quick Verification

You can verify that the MCP server is working using standard stdio JSON-RPC:

```bash
echo '{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2024-11-05"}}' | centmem serve
```

Expected output:
```json
{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{}},"serverInfo":{"name":"centmem","version":"1.5.3"}}}
```

---

## 2. Agent Harness Configurations

### Claude Code

Add the following to `~/.claude/mcp.json` or `.claude.json` in your repository root:

```json
{
  "mcpServers": {
    "centmem": {
      "command": "centmem",
      "args": ["serve"]
    }
  }
}
```

### Cursor

Add to `~/.cursor/mcp.json` or project-level `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "centmem": {
      "command": "centmem",
      "args": ["serve"]
    }
  }
}
```

### Windsurf

Add to `~/.codeium/windsurf/mcp_config.json`:

```json
{
  "mcpServers": {
    "centmem": {
      "command": "centmem",
      "args": ["serve"]
    }
  }
}
```

### Zed

Add to `~/.config/zed/settings.json`:

```json
{
  "context_servers": {
    "centmem": {
      "command": "centmem",
      "args": ["serve"]
    }
  }
}
```

### Google Antigravity

In Antigravity agent tool definitions or plugin configuration:

```json
{
  "mcpServers": {
    "centmem": {
      "command": "centmem",
      "args": ["serve"]
    }
  }
}
```

---

## 3. Exposed MCP Tools

When connected, centmem provides the following 7 tools:

| Tool | Purpose | Key Parameters |
|---|---|---|
| `centmem_recall` | Hybrid search (semantic + keyword + facts + timeline) | `query` (required), `scope`, `top`, `type`, `tags`, `since`, `until` |
| `centmem_put` | Store a note or chronological log | `content` (required), `scope` (required), `type`, `tags` |
| `centmem_set` | Upsert a structured fact | `scope` (required), `key` (required), `value` (required), `tags` |
| `centmem_get` | Fetch a memory by ID or fact key | `id` or `scope` + `key`, `inherit` |
| `centmem_timeline` | Retrieve chronological memory stream | `scope`, `since`, `until`, `limit` |
| `centmem_stats` | Memory count & health metrics | `scope` |
| `centmem_forget` | Archive or delete a memory | `id` or `scope` + `key` or `scope` + `tag` |

---

## 4. Environment Variables

- `CENTMEM_HOME`: Path to data directory (defaults to `~/.centmem`).
- `CENTMEM_AGENT`: Default agent identifier for affinity boosting.
