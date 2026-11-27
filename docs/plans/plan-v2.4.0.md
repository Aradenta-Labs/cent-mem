# v2.4.0 — Cross-Session Working Memory

**Version:** 2.4.0 (target)  
**Owner:** Aradenta Labs  
**Status:** Planned  
**Schema Migration:** `m0009_session_memory.sql` (Schema v9)  
**Source:** [docs/v2-nice-to-have.md](../v2-nice-to-have.md) — High Effort #9

---

## 0. Goal & Context

Schema v6 (`m0006_agent_proposals.sql`) already created `agent_conversations` and `agent_messages`, which power Web UI Assistant Chat threads. However:
1. CLI `centmem ask` starts cold on every invocation — there is no `--session <id>` flag to resume a thread across commands or agent restarts.
2. Long conversations eventually overflow the LLM context window because there is no rolling working-memory summary.
3. Sessions cannot be inspected, promoted to durable memories, or shared seamlessly between CLI and Web UI.

This release extends `agent_conversations` into a full **Cross-Session Working Memory** layer shared by CLI and Web UI.

---

## 1. Schema Migration (`m0009_session_memory.sql`)

```sql
ALTER TABLE agent_conversations ADD COLUMN summary TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_conversations ADD COLUMN summarized_up_to_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE agent_conversations ADD COLUMN agent_name TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_conversations ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0;
ALTER TABLE agent_conversations ADD COLUMN archived_at INTEGER; -- unix microseconds

CREATE INDEX IF NOT EXISTS idx_conversations_scope_active
    ON agent_conversations(scope_id, archived_at, updated_at DESC);

INSERT OR REPLACE INTO meta(key, value) VALUES ('schema_version', '9');
```

---

## 2. Phased Implementation Plan

### Phase 1 — Conversation Store Enhancements & Rolling Context Compaction

**Scope:**
1. Add `internal/store/migrations/m0009_session_memory.sql` and update `docs/data-model.md`.
2. Extend `internal/store/agent.go` with:
   - `GetOrCreateConversation(ctx, id, scopeID, title, agentName)` (allows human-readable session IDs like `auth-refactor` or auto-generated UUIDs).
   - `UpdateConversationSummary(ctx, convID, summary, upToMessageID)`.
   - `PinConversation` / `ArchiveConversation` / `ListConversations` with active/archived filter.
3. In `internal/agent/engine.go`:
   - Add `SessionID string` to `AskRequest`.
   - On `Ask()`, load `summary` (injected as a condensed system/context block) + recent unsummarized `agent_messages` up to a configurable token budget (`[agent].session_context_budget`, default 6000 tokens).
   - When unsummarized turns exceed the budget, run a rolling compaction step (LLM summary when online; deterministic extractive fallback when offline) and advance `summarized_up_to_id`.
   - Persist both user prompt and assistant response (with citations/tool calls) back to `agent_messages`.
4. Unit tests in `internal/agent/session_test.go` covering multi-turn replay, rolling compaction trigger, and offline fallback.

**Exit criteria:** `go test -tags fts5 ./internal/agent/... ./internal/store/... -race` passes.

---

### Phase 2 — CLI Session Continuity (`centmem ask --session`, `centmem sessions`)

**Scope:**
1. Extend `centmem ask`:
   - `--session <id>` — resume (or create) a named session thread; returns `session_id` in the JSON output envelope.
   - `--new-session` — force a fresh session and print its generated `session_id`.
2. Add `centmem sessions` subcommand group in `cmd/centmem/`:
   - `centmem sessions list --scope <scope> [--archived]`
   - `centmem sessions show <id>` (metadata, rolling summary, message history)
   - `centmem sessions promote <id>` (saves the distilled session summary as a durable `note` memory in the scope)
   - `centmem sessions archive <id>`
3. Golden-file CLI contract tests in `cmd/centmem/`.

**Exit criteria:** Two separate `centmem ask --session sprint-42` processes share working memory across restarts and `centmem sessions promote` creates a recallable memory.

---

### Phase 3 — Web UI Thread Continuity, Pinning & Release (`v2.4.0`)

**Scope:**
1. Expose session summary, pin/archive, and promote actions via REST (`PATCH /api/agent/conversations/{id}`, `POST /api/agent/conversations/{id}/promote`).
2. In `AssistantTab.tsx`:
   - Display CLI-created and Web-UI-created sessions in one unified thread list (with agent badge and pinned section).
   - Show a collapsible **Working Memory Summary** banner when a thread has been compacted, plus a **"Save to Memory"** (promote) button.
3. Update `docs/cli-contract.md`, `docs/ui.md`, `docs/data-model.md`, `skill/SKILL.md` + mirrors, and mark #9 complete in `docs/v2-nice-to-have.md`.
4. Bump version to `2.4.0`, update `CHANGELOG.md`, run full test suite + `scripts/check_contract.sh` + `graphify update .`.
