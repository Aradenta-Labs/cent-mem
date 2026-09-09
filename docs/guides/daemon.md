# Daemon Guide: centmemd & Multi-Process Architecture

`centmemd` is the background daemon service for `cent-mem`. It coordinates multi-agent access to shared memory, eliminates SQLite write concurrency bottlenecks, manages background embedding workers, runs scheduled compaction, and powers low-latency local IPC.

---

## 1. Overview & Architecture

In single-process mode, each invocation of `centmem` opens SQLite, acquires locks, loads configuration, and runs queries directly. When multiple AI agents or IDE harnesses (e.g., Claude Code, Cursor, Windsurf, Trae, custom harnesses) write simultaneously, SQLite's single-writer lock can cause contention and `database is locked` errors.

`centmemd` solves this by introducing a client-daemon architecture:

```
┌─────────────────────────────────────────────────────────────┐
│                      Client Interfaces                      │
│   ┌──────────────┐   ┌──────────────┐   ┌──────────────┐   │
│   │ centmem CLI  │   │   MCP Tool   │   │ Python / TS  │   │
│   │ (Agent 1)    │   │ (Claude/IDE) │   │ SDKs (v2.0)  │   │
│   └───────┬──────┘   └───────┬──────┘   └───────┬──────┘   │
└───────────┼──────────────────┼──────────────────┼───────────┘
            │                  │                  │
            │  Unix Domain Socket (gRPC IPC <1.5ms)│
            ▼                  ▼                  ▼
┌─────────────────────────────────────────────────────────────┐
│                       centmemd                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │ Single-Writer Serialization (LockWrite Mutex)         │  │
│  └───────────────────────────┬───────────────────────────┘  │
│                              ▼                              │
│  ┌───────────────────────┐       ┌───────────────────────┐  │
│  │ SQLite + WAL Store    │       │ Background Embedder   │  │
│  │ (FTS5 + Vector + Link)│       │ (ONNX Queue Drainer)  │  │
│  └───────────────────────┘       └───────────────────────┘  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │ SyncService & Event Stream (Replication CDC)          │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### Key Capabilities
- **Zero-Contention Writes:** Mutex-governed write serialization eliminates SQLite lock errors under heavy concurrent agent load.
- **Microsecond IPC:** High-speed gRPC over Unix domain sockets delivers roundtrip latency comparable to native SQLite opens (<1.5 ms).
- **Transparent CLI Delegation:** The standard `centmem` CLI automatically detects a running daemon and forwards requests. If the daemon is not running, it falls back seamlessly to embedded SQLite.
- **Embedding Queue Offload:** Ingestion requests return immediately (`"status": "queued"`), while `centmemd`'s background worker continuously drains and generates embeddings.
- **Scheduled Compaction:** Periodic retention cleanup and compaction runs automatically in the background without blocking CLI commands.

---

## 2. Transparent Delegation & Fallback

You do not need to change how you invoke `centmem`. Every command checks for a live daemon socket (`~/.centmem/centmemd.sock` by default).

- **Daemon running:** `centmem` connects via Unix socket IPC and returns identical JSON output.
- **Daemon stopped or socket missing:** `centmem` transparently falls back to direct embedded SQLite with zero user-visible error.

### Explicit Bypass (`--direct`)
To force direct embedded SQLite access (bypassing the daemon socket entirely):

```bash
# Via command-line flag
centmem recall "search query" --direct

# Via environment variable
export CENTMEM_DIRECT=1
centmem put --scope project:myapp --type note --content "direct write"
```

---

## 3. Daemon Lifecycle Management

Use the `centmemd` binary to manage daemon execution:

### 3.1 Start in Background
Spawns the daemon as a detached background process and waits for socket readiness:

```bash
centmemd start
```

Output:
```json
{
  "ok": true,
  "status": "started",
  "pid": 48215,
  "socket": "/Users/user/.centmem/centmemd.sock",
  "port": 0
}
```

If already running, returns:
```json
{
  "ok": true,
  "status": "already_running",
  "pid": 48215,
  "socket": "/Users/user/.centmem/centmemd.sock",
  "port": 0
}
```

### 3.2 Foreground Execution
Runs the daemon in the foreground, logging to stdout/stderr. Recommended for process supervisors (`systemd`, `launchd`, Docker):

```bash
centmemd run
```

### 3.3 Check Status
Probes socket health and queries memory statistics via gRPC:

```bash
centmemd status
```

Output:
```json
{
  "ok": true,
  "status": "running",
  "pid": 48215,
  "socket": "/Users/user/.centmem/centmemd.sock",
  "port": 0,
  "stats": {
    "db_path": "/Users/user/.centmem/centmem.db",
    "db_size_mb": 2.14,
    "total_memories": 128,
    "by_type": {
      "note": 94,
      "fact": 34
    },
    "by_scope": {
      "project:cent-mem": 128
    },
    "pending_embedding": 0,
    "last_compact_at": "2026-09-09T08:00:00Z"
  }
}
```

### 3.4 Graceful Stop
Sends `SIGTERM` to the daemon PID, allows up to 5 seconds for active transactions and queue draining to complete, and cleans up socket and PID files:

```bash
centmemd stop
```

Output:
```json
{
  "ok": true,
  "status": "stopped",
  "pid": 48215
}
```

---

## 4. Configuration Options

Configure `centmemd` in `~/.centmem/config.toml` under the `[daemon]` table or via environment variables:

### `~/.centmem/config.toml`
```toml
[daemon]
socket_path = "~/.centmem/centmemd.sock"
pid_path = "~/.centmem/centmemd.pid"
port = 0                  # 0 = Unix domain socket only; >0 enables TCP gRPC
```

### Environment Variables
| Variable | Description | Default |
|---|---|---|
| `CENTMEM_DAEMON_SOCKET` | Path to Unix domain socket file | `~/.centmem/centmemd.sock` |
| `CENTMEM_DAEMON_PID_FILE`| Path to daemon PID file | `~/.centmem/centmemd.pid` |
| `CENTMEM_DAEMON_PORT` | Optional TCP port to listen on | `0` (disabled) |
| `CENTMEM_DIRECT` | If set to `1` or `true`, CLI bypasses daemon | `0` |

---

## 5. Process Supervision Setup

### macOS (`launchd`)
Create `~/Library/LaunchAgents/com.aradenta.centmemd.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.aradenta.centmemd</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/centmemd</string>
        <string>run</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/Users/youruser/.centmem/centmemd.log</string>
    <key>StandardErrorPath</key>
    <string>/Users/youruser/.centmem/centmemd.log</string>
</dict>
</plist>
```

Load and start the service:
```bash
launchctl load ~/Library/LaunchAgents/com.aradenta.centmemd.plist
```

### Linux (`systemd` user service)
Create `~/.config/systemd/user/centmemd.service`:

```ini
[Unit]
Description=centmemd memory daemon
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/centmemd run
Restart=always
RestartSec=3

[Install]
WantedBy=default.target
```

Enable and start the service:
```bash
systemctl --user enable --now centmemd
```

---

## 6. Replication Foundation (CDC Preview)

`centmemd` exposes `SyncService.StreamEvents` defined in `proto/centmem/v1/centmem.proto`.
Every mutation to memories, facts, and links is recorded in the SQLite `events` table with microsecond timestamps and Lamport sequence numbers.

Replication and multi-machine sync operate on the following convergence rules:
- **Facts:** Last-Write-Wins (LWW) based on event timestamps.
- **Notes & Logs:** Append-only with deduplication on content hashes.
- **Links:** Conflict-free set union.
