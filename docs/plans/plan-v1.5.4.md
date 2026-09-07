# v1.5.4 — Sync: `centmemd` Daemon & Multi-Process Architecture

**Version:** 1.5.4 (target)  
**Owner:** Aradenta Labs  
**Status:** Approved — Ready for implementation  
**Depends on:** v1.5.0–v1.5.3; non-breaking server/client architectural evolution  
**Roadmap Reference:** [docs/plans/roadmap-v1.5.x.md](roadmap-v1.5.x.md)

---

## 0. Executive Summary & Objective

Throughout **v1.0–v1.5.3**, centmem operates as a direct-access embedded database library: every CLI invocation opens the SQLite database file, applies WAL pragmas, performs reads and writes, and closes the handle.

While ideal for zero-configuration, single-agent workflows, this embedded process model faces concurrency bottlenecks as multi-agent collaboration grows:
1. **WAL Lock Contention:** Multiple autonomous agents running concurrently (e.g. 5 parallel subagents writing memories simultaneously) risk `busy_timeout` errors and lock waiting.
2. **Duplicated Model Memory:** Each concurrent CLI process must initialize its own ONNX runtime or stub embedder in memory.
3. **Absence of Shared Event Streaming:** Other processes cannot subscribe to real-time memory write streams.

**v1.5.4 (Sync: `centmemd` Daemon)** establishes a client/daemon architecture:
- **`centmemd` Daemon:** A long-running service that exclusively holds the SQLite database connection, runs the ONNX embedding queue drainer, and manages memory compaction.
- **High-Performance IPC:** Communicates with CLI processes over Unix Domain Sockets (`~/.centmem/centmemd.sock`) on Unix/macOS and Named Pipes on Windows, with optional gRPC over TCP (`:50051`).
- **Transparent CLI Delegation:** The existing `centmem` CLI automatically checks for a running daemon. If found, it delegates operations over IPC; if absent, it falls back instantly to direct SQLite access with zero behavioral deviation.
- **Replication Foundation:** Exposes gRPC server-side event streaming from the `events` table for future multi-machine synchronization.

---

## 1. Problem Statement & Root Causes

| # | Current Limitation | Root Cause in Code | v1.5.4 Solution |
|---|---|---|---|
| 1 | SQLite write lock contention under load | `store/store.go:53`: Multiple processes write directly to `db.sqlite`. Under heavy parallel agent runs, writes block on SQLite transaction locks. | Single-writer daemon pattern: `centmemd` holds the sole write connection, queueing requests in memory. |
| 2 | Redundant model initialization | `embed/embed.go`: Each CLI invocation loads embedder models into memory or queries disk cache independently. | `centmemd` maintains a persistent warm embedder pool in memory. |
| 3 | Inability to stream live updates | `store/events.go`: The `events` table records changes, but no daemon streams them to listening clients. | gRPC `SyncService.StreamEvents` server-streaming API. |
| 4 | Risk of breaking standalone simplicity | If a daemon is strictly required, users without background service setups cannot use centmem. | 100% transparent auto-detection: daemon is optional; CLI falls back seamlessly to direct SQLite mode. |

---

## 2. Technical Architecture & Process Model

```
 ┌─────────────────┐       ┌─────────────────┐       ┌─────────────────┐
 │  Agent Session  │       │  Claude / MCP   │       │   Web UI / REST │
 │ (centmem recall)│       │ (centmem serve) │       │   (centmem ui)  │
 └────────┬────────┘       └────────┬────────┘       └────────┬────────┘
          │                         │                         │
          └────────────────┬────────┴─────────────────────────┘
                           │
             Check if socket exists (~/.centmem/centmemd.sock)
                           │
             ┌─────────────┴─────────────┐
             │                           │
      (Socket Found)              (Socket Missing)
             │                           │
             ▼                           ▼
  ┌───────────────────────┐   ┌─────────────────────────────────────┐
  │  gRPC / IPC Client    │   │      Direct Embedded Store          │
  │  (Zero-overhead IPC)  │   │  (Legacy Single-Process Fallback)   │
  └──────────┬────────────┘   └─────────────────────────────────────┘
             │
             ▼
  ┌─────────────────────────────────────────────────────────────────┐
  │                        centmemd Daemon                          │
  │                                                                 │
  │   - Exclusive SQLite Connection Pool (1 Writer, N Readers)      │
  │   - Continuous Embedder Queue Drainer                           │
  │   - Background Retention & Compaction Engine                    │
  │   - gRPC Service Endpoints (Recall, Put, Set, Sync)             │
  │                                                                 │
  │   Database File: ~/.centmem/data.db                             │
  └─────────────────────────────────────────────────────────────────┘
```

---

## 3. gRPC & Protobuf Service Contracts (`proto/centmem.proto`)

```protobuf
syntax = "proto3";

package centmem.v1;

option go_package = "github.com/aradenta-labs/cent-mem/internal/gen/centmem/v1";

service MemoryService {
  rpc Recall (RecallRequest) returns (RecallResponse);
  rpc Put (PutRequest) returns (PutResponse);
  rpc Set (SetRequest) returns (SetResponse);
  rpc Get (GetRequest) returns (GetResponse);
  rpc Timeline (TimelineRequest) returns (TimelineResponse);
  rpc Forget (ForgetRequest) returns (ForgetResponse);
  rpc Stats (StatsRequest) returns (StatsResponse);
  rpc Compact (CompactRequest) returns (CompactResponse);
}

service SyncService {
  rpc StreamEvents (StreamEventsRequest) returns (stream Event);
  rpc PushEvents (stream Event) returns (PushSummary);
}

message RecallRequest {
  string text = 1;
  string scope = 2;
  int32 top = 3;
  string type = 4;
  repeated string tags = 5;
  bool inherit = 6;
  string caller_agent = 7;
}

message RecallResponse {
  bool ok = 1;
  repeated RankedMemory results = 2;
}

message RankedMemory {
  int64 id = 1;
  string type = 2;
  string scope = 3;
  string content = 4;
  repeated string tags = 5;
  double score = 6;
  int64 access_count = 7;
  repeated string matched_by = 8;
  int64 created_at = 9;
}

message Event {
  int64 id = 1;
  int64 memory_id = 2;
  string op = 3; // "put" | "set" | "forget"
  string scope = 4;
  string payload_json = 5;
  int64 timestamp = 6;
}
```

---

## 4. Transparent CLI Delegation Protocol

In `cmd/centmem/commands.go` / `runCommand`:

```go
func getStoreClient(cfg config.Config) (StoreInterface, func(), error) {
    sockPath := getDaemonSocketPath(cfg)
    
    // Check if daemon socket is active with low-latency probe (5ms timeout)
    if conn, err := net.DialTimeout("unix", sockPath, 5*time.Millisecond); err == nil {
        conn.Close()
        // Daemon is healthy -> return gRPC client wrapper
        client, cleanup, err := grpcclient.New(sockPath)
        if err == nil {
            return client, cleanup, nil
        }
    }

    // Daemon not running -> fallback directly to local SQLite
    st, err := store.Open(cfg)
    if err != nil {
        return nil, nil, err
    }
    return st, func() { st.Close() }, nil
}
```

### Direct Mode Escape Hatch
To bypass daemon delegation explicitly:
```bash
centmem recall "query" --direct
# OR via environment variable:
export CENTMEM_DIRECT=1
```

---

## 5. Daemon Lifecycle Management

### 5.1 CLI Commands

```bash
# Start daemon in background
centmemd start [--socket <path>] [--port <port>] [--pid-file <path>]

# Start in foreground (for launchd/systemd/docker)
centmemd run

# Check daemon health and active connection count
centmemd status

# Gracefully terminate daemon
centmemd stop
```

### 5.2 Signal Handling & Clean Shutdown
- Traps `SIGTERM` and `SIGINT`.
- Stops accepting new IPC connections.
- Waits up to 5 seconds for in-flight database transactions to commit.
- Flushes the embedding queue.
- Deletes the Unix domain socket file (`~/.centmem/centmemd.sock`) and PID file.

---

## 6. Replication Foundation (Multi-Machine Sync Preview)

`centmemd` leverages the existing `events` table (created in `m0001_init.sql`) to provide change-data-capture:
1. Every write (`put`, `set`, `forget`, `link`) logs an immutable row to `events`.
2. `SyncService.StreamEvents` streams events sequentially to replica instances.
3. Conflict Resolution Principles:
   - **Facts:** Last-Write-Wins (LWW) based on microsecond timestamps.
   - **Notes & Logs:** Append-only with deduplication on `content_hash`.
   - **Links:** Union of non-conflicting links.

---

## 7. Testing & Verification Plan

### Test Suites:
1. **Socket IPC Latency Benchmark (`internal/daemon/ipc_bench_test.go`):**
   - Measure roundtrip latency between CLI client and `centmemd` over Unix domain socket.
   - Target: $< 1.5$ ms per IPC query (comparable to native SQLite open).
2. **Concurrent Write Stress Test (`internal/daemon/concurrency_test.go`):**
   - Run 100 concurrent workers sending simultaneous `Put` and `Set` requests.
   - Verify 0 SQLite `database is locked` errors under daemon operation.
3. **Failover & Graceful Fallback (`cmd/centmem/delegation_test.go`):**
   - Run command while daemon is active $\to$ verify request routed to daemon.
   - Kill daemon process (`kill -9`) $\to$ verify subsequent command succeeds immediately via direct SQLite mode.
4. **Socket Cleanup Test:**
   - Terminate daemon via SIGTERM $\to$ verify socket file and PID file are cleanly unlinked.

---

## 8. Files Affected & Implementation Checklist

- [ ] **Protobuf Definitions:** `proto/centmem/v1/centmem.proto` [NEW]
- [ ] **Generated Code:** `internal/gen/centmem/v1/` (protoc Go & gRPC output) [NEW]
- [ ] **Daemon Core:** `cmd/centmemd/main.go`, `server.go`, `lifecycle.go` [NEW]
- [ ] **gRPC Service Handlers:** `internal/daemon/service.go` [NEW]
- [ ] **Client Delegation Wrapper:** `internal/daemon/client.go` [NEW]
- [ ] **CLI Transparent Hook:** `cmd/centmem/main.go` (socket probe & delegation) [MODIFY]
- [ ] **Config:** `internal/config/config.go` (daemon socket & port settings) [MODIFY]
- [ ] **Documentation:** `docs/guides/daemon.md` & `docs/cli-contract.md` [NEW/MODIFY]
- [ ] **Knowledge Graph:** Update with `graphify update .` after completion
