# Auto-Capture Hooks Reference — centmem

This reference describes the auto-capture architecture introduced in cent-mem v1.3.0.

## Overview

Auto-capture monitors agent transcripts to extract durable knowledge without manual user prompting.

```
Agent Conversation
       │
  (Trigger: watcher | exit trap | on-demand)
       ▼
centmem capture run
       ▼
 1. Transcript Ingestion (JSONL / plain text)
 2. Three-Tier Classifier (Local LLM -> Heuristic -> BYOK Cloud)
 3. Novelty Judgment (Recall-before-write)
 4. Store Persistence (put / set)
 5. Session Telemetry (summary report)
```

---

## 1. Triggers

- **Per-message Watcher**: A continuous background file watcher (`centmem capture run --watch`) tracking new lines in the agent transcript.
- **Session-end Trap**: Shell exit trap in agent harness scripts triggering a one-shot extraction on exit.
- **On-demand**: Executed directly via `centmem capture run --transcript <file>`.

---

## 2. Three-Tier Classification Backend

The classifier attempts backends in order:
1. **Local LLM (`backend = "local-llm"`)**:
   - Calls local OpenAI-compatible endpoint (e.g. Ollama at `http://localhost:11434/v1`).
   - Private and 100% on-device.
2. **Deterministic Heuristic (`backend = "heuristic"`)**:
   - Zero-dependency regex pattern matcher.
   - Triggers on indicators like "we decided", "chose X over Y", version strings, and error resolutions.
   - Always succeeds as a fallback if other backends fail.
3. **Bring-Your-Own-Key (`backend = "openai-compatible"`)**:
   - User-provided endpoint (e.g. `https://api.openai.com/v1`).
   - Secret key is read from an environment variable name (e.g. `$OPENAI_API_KEY`), never written to disk.

---

## 3. Recall-Before-Write Deduplication

To prevent duplicate memories:
1. Before saving a candidate memory, `centmem` performs a high-threshold `recall` query.
2. If similar content exists in the project scope, the item is marked as skipped.
3. A session-level dedup lock file (`~/.centmem/session-<id>.dedup.json`) prevents reprocessing within the same session.

---

## 4. Category Whitelist

Default categories:
- `decision`: Architectural and design choices.
- `fact`: Concrete configuration values, ports, endpoints.
- `preference`: User coding style and workflow preferences.
- `code`: Reusable snippets, commands, and patterns.
- `log`: Session checkpoints and progress notes.
- `error`: Bugs, root causes, and resolutions.
- `dependency`: Newly introduced packages and external services.

Manage categories via:
```bash
centmem capture categories --list
centmem capture categories --add "custom-category"
centmem capture categories --remove "preference"
```
