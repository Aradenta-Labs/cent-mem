# v2.1.1 — `centmem recall --interactive` Interactive TUI

**Version:** 2.1.1 (target)  
**Owner:** Aradenta Labs  
**Status:** Completed / Implemented (Phases 1–6 verified with race detector)  
**Depends on:** v2.0.0 (hybrid search engine, `searcher.Recall()`, daemon client)  
**Source:** [docs/v2-nice-to-have.md](../v2-nice-to-have.md) — Quick Win #2

---

## 0. Executive Summary

`centmem recall --interactive` adds an `fzf`-style fuzzy memory browser to the terminal. Developers type a query, results update live via the existing hybrid search engine, and they navigate, preview, and open memories without leaving the shell.

**Why this version (v2.1.1)?** The hybrid search backend (`searcher.Recall()`) already exists and is battle-tested. The only new work is:
1. A new Go package `internal/tui` implementing the bubbletea model.
2. A `--interactive` flag added to the existing `recall` command in `cmd/centmem/handlers.go`.
3. Two new Go module dependencies: `charmbracelet/bubbletea` + `charmbracelet/lipgloss`.

No schema changes. No new CLI commands. No backend API changes.

### Design decisions (locked via `/grill-me`)

| Question | Decision | Rationale |
|----------|----------|-----------|
| Command surface | **`recall --interactive`** | Extends the existing `recall` handler; no new entry in `commands.go` needed; consistent with `ask --interactive` pattern |
| TUI library | **bubbletea** (`charmbracelet/bubbletea` + `charmbracelet/lipgloss`) | Named explicitly in v2-nice-to-have; leading Go TUI ecosystem; bubbles/textinput + bubbles/list ready to use |
| Build tag | **Always compiled** | No build tag complexity; TTY detection guards runtime behavior |
| Search backend | **Full hybrid search** (`searcher.Recall()`) | Same quality as `centmem recall`; debounced keystroke trigger |
| Layout | **Split pane** (left = result list, right = full content preview) | Best UX for content-heavy memories; j/k navigation updates preview in real-time |
| Actions | **Open/preview** (core), **copy to clipboard** (`c`), **delete/forget** (`d` + confirm) | Covers the full use case of navigate-preview-open from the nice-to-have doc |
| TTY guard | **Fallback to JSON when stdout is not a TTY** | Makes the flag safe in scripts; CI and pipes get normal JSON output |

---

## 1. Scope & Anti-goals

### In scope
- `--interactive` flag on `centmem recall`.
- TTY detection: if not a TTY, run the existing non-interactive recall path (JSON output).
- Split-pane TUI: query input bar, scrollable result list (left), full-content preview (right).
- Live search: debounce keystrokes, re-run `searcher.Recall()` on each query change.
- Navigation: `j`/`k` (or arrow keys) to move through results; preview updates in real-time.
- Actions: Enter/`o` to open full detail (print to stdout + exit TUI); `c` to copy content to clipboard; `d` to forget with an inline confirmation prompt; `Esc`/`q` to quit.
- New package: `internal/tui/` — self-contained bubbletea model.
- Two new `go.mod` dependencies: `charmbracelet/bubbletea`, `charmbracelet/lipgloss`.
- Optional companion: `charmbracelet/bubbles` (textinput + list components).

### Anti-goals (explicit)
- No new top-level command (`centmem search`, `centmem tui`).
- No `--edit` / in-TUI editing of memory content.
- No TUI for `centmem timeline` or `centmem list` in this version.
- No mouse support (keyboard-only).
- No Windows ConPTY support tested (best-effort; bubbletea handles most cases).
- No daemon-mode TUI (always connects to local store directly, same as non-interactive recall).
- No change to existing `centmem recall` non-interactive behavior.
- No change to `commands.go` registered command metadata (the `--interactive` flag is added to the existing `recall` handler without changing the contract test entry because the contract test only validates known flags; `--interactive` should be added to the flag list in `commands.go`).

---

## 2. Architecture

```
cmd/centmem/handlers.go  (recall handler)
  │
  ├── isatty(os.Stdout) == false  ──►  existing JSON recall path (unchanged)
  │
  └── isatty(os.Stdout) == true
        └── internal/tui.Run(ctx, TUIConfig{ ... })
              │
              ├── bubbletea.Program (event loop)
              │     ├── textinput model  (query bar)
              │     ├── list model       (result list, left pane)
              │     └── viewport model   (content preview, right pane)
              │
              └── on query change (debounced 200ms)
                    └── searcher.Recall(ctx, search.Query{ Text: query, ... })
                          └── returns []search.Ranked
```

### Package: `internal/tui`

A self-contained package exposing a single entry point:

```go
package tui

type Config struct {
    Searcher    *search.Searcher
    Store       *store.Store
    Scope       string
    Top         int       // max results (default: 20 for TUI)
    InitQuery   string    // pre-populate from CLI positional arg if any
}

// Run starts the interactive TUI. Blocks until the user quits.
// Returns the memory the user "opened" (if any) so the caller can
// print it to stdout after the TUI exits.
func Run(ctx context.Context, cfg Config) (*store.Memory, error)
```

The caller in `handlers.go` prints the returned memory as JSON to stdout after `tui.Run()` returns, so the output can still be piped: `centmem recall --interactive | jq .id`.

---

## 3. TUI Layout & Key Map

### 3.1 Screen Layout

```
┌─ centmem recall ──────────────────────────────────────────────┐
│ > hybrid search fusion________                  project:cent-mem │
├───────────────────────┬───────────────────────────────────────┤
│ [1] architecture note  │  ID: 142  Type: note  Scope: project:│
│     project:cent-mem   │  cent-mem  Tags: [architecture,search]│
│ [2] RRF decision       │                                       │
│     project:cent-mem   │  Decided to use RRF with k=60 for    │
│ [3] embedding model    │  hybrid search fusion after benchmark │
│     global             │  testing. The 60-constant provides    │
│ ...                    │  stable ranking regardless of         │
│                        │  individual lane score magnitude.     │
│                        │                                       │
│                        │  Created: 2026-09-10  Access: 3x     │
├───────────────────────┴───────────────────────────────────────┤
│ j/k navigate  Enter open  c copy  d delete  / focus  q quit   │
└───────────────────────────────────────────────────────────────┘
```

- **Top bar:** query input (bubbletea `textinput`). Scope shown on the right.
- **Left pane (40% width):** result list (bubbletea `list`). Each item shows: title (first 40 chars of content), scope, type badge.
- **Right pane (60% width):** content preview (bubbletea `viewport`). Full content, ID, type, scope, tags, created_at, access_count.
- **Status bar:** key map hint, updated dynamically based on current state (e.g. `"Confirm delete? y/n"` during delete confirmation).

### 3.2 Key Map

| Key | Action |
|-----|--------|
| Any printable char | Appended to query; triggers debounced search |
| Backspace | Remove last query char |
| `j` / `↓` | Move selection down |
| `k` / `↑` | Move selection up |
| Enter / `o` | **Open**: exit TUI, print selected memory JSON to stdout |
| `c` | **Copy**: copy selected memory content to clipboard (pbcopy/xclip/wl-copy auto-detected) |
| `d` | **Delete**: show inline confirmation (`Confirm delete? [y/N]`); `y` calls `store.Forget()`, `n` cancels |
| `Esc` / `q` | Quit without selecting |
| `?` | Toggle key map help overlay |
| `Ctrl+C` | Quit (bubbletea default) |

### 3.3 Debounce Strategy

After each character input, start a 200ms timer. If another character arrives before the timer fires, reset. When the timer fires, call `searcher.Recall()` in a goroutine and send the results back to the bubbletea model via a `tea.Msg`. Show a "searching…" spinner in the query bar during in-flight requests.

---

## 4. CLI Integration

### 4.1 Flag addition to `recall` handler

In [cmd/centmem/handlers.go](../../cmd/centmem/handlers.go) inside `cmdRecall`:

```go
fs.Bool("interactive", false, "launch interactive TUI browser")
```

At the start of the `runCommandQuery` callback (before the daemon client check):

```go
if fs.Lookup("interactive").Value.String() == "true" {
    if !isatty.IsTerminal(os.Stdout.Fd()) {
        // Fallback: run normal non-interactive recall with the query
        // (proceed with the existing recall path below)
    } else {
        return runInteractiveTUI(cfg, fs, query)
    }
}
```

### 4.2 `runInteractiveTUI` function

Defined in a new file `cmd/centmem/handlers_tui.go`:

```go
func runInteractiveTUI(cfg config.Config, fs *flag.FlagSet, initQuery string) error {
    s, err := store.Open(cfg)
    if err != nil {
        return cli.Internalf("recall --interactive: %v", err)
    }
    defer s.Close()

    searcher := search.New(s)
    scopePath := fs.Lookup("scope").Value.String()
    top := intFlag(fs, "top", 20)

    ctx, cancel := signalContext()
    defer cancel()

    selected, err := tui.Run(ctx, tui.Config{
        Searcher:  searcher,
        Store:     s,
        Scope:     scopePath,
        Top:       top,
        InitQuery: initQuery,
    })
    if err != nil {
        return cli.Internalf("tui: %v", err)
    }
    if selected != nil {
        // Print the selected memory as JSON so it can be piped
        return prettyPrint(fs, map[string]any{
            "ok":     true,
            "memory": selected,
        })
    }
    return nil
}
```

### 4.3 `commands.go` update

Add `"--interactive"` to the `recall` entry's flags slice:

```go
"recall": {cmdRecall, "recall <query> --scope <scope> [--top N] ... [--interactive]",
    []string{"--scope", "--top", ..., "--interactive"}},
```

This keeps the contract test (`contract_test.go`) in sync.

### 4.4 TTY Detection

Use `github.com/mattn/go-isatty` — **already in `go.mod`** as a transitive dependency. Import it directly:

```go
import "github.com/mattn/go-isatty"
```

No new dependency needed for TTY detection.

---

## 5. New Package: `internal/tui`

### 5.1 File structure

```
internal/tui/
├── tui.go          — Run() entry point, bubbletea program setup
├── model.go        — bubbletea Model struct, Init/Update/View
├── search.go       — debounced search goroutine, searchMsg type
├── clipboard.go    — cross-platform clipboard write (pbcopy/xclip/wl-copy)
├── keys.go         — key binding constants
└── tui_test.go     — unit tests (headless model update tests)
```

### 5.2 Model state

```go
type model struct {
    // input
    query     textinput.Model
    scope     string
    top       int

    // results
    list      list.Model
    results   []search.Ranked
    searching bool

    // preview
    viewport  viewport.Model
    selected  *store.Memory   // full memory fetched on selection change

    // actions
    confirmDelete bool
    err           error

    // dependencies
    searcher  *search.Searcher
    store     *store.Store
    ctx       context.Context

    // output
    chosen    *store.Memory   // set when user presses Enter
}
```

### 5.3 Message types

```go
type searchResultMsg struct{ results []search.Ranked; err error }
type memoryLoadedMsg  struct{ memory *store.Memory; err error }
type clipboardDoneMsg struct{ err error }
type forgetDoneMsg    struct{ id int64; err error }
```

### 5.4 Search flow (debounce)

```go
// In Update(), on key input:
case tea.KeyMsg:
    m.query, cmd = m.query.Update(msg)
    return m, tea.Batch(cmd, m.scheduleSearch())

// scheduleSearch returns a Cmd that:
// 1. Waits 200ms (tea.Tick)
// 2. If query hasn't changed, calls searcher.Recall() in a goroutine
// 3. Returns searchResultMsg
```

### 5.5 Clipboard detection

`clipboard.go` tries the following in order, using `os/exec`:

1. `pbcopy` (macOS)
2. `wl-copy` (Wayland)
3. `xclip -selection clipboard` (X11)
4. `xsel --clipboard --input` (X11 fallback)

If none found, shows an inline error in the status bar: `"clipboard: no supported tool found (pbcopy, xclip, wl-copy)"`.

No new Go dependency for clipboard (shell-out approach avoids CGo clipboard libs).

---

## 6. New Go Module Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| `github.com/charmbracelet/bubbletea` | `v1.x` (latest stable) | TUI event loop, Model/View/Update pattern |
| `github.com/charmbracelet/lipgloss` | `v1.x` (latest stable) | Style primitives (colors, borders, layout) |
| `github.com/charmbracelet/bubbles` | `v0.20+` | `textinput`, `list`, `viewport`, `spinner` components |

All three are from the same maintainer and work as a unit. They have no CGo dependencies and compile cleanly on macOS, Linux, and Windows.

**Add to `go.mod`:**
```bash
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/lipgloss@latest
go get github.com/charmbracelet/bubbles@latest
```

---

## 7. Phased Implementation Plan

### Phase 1 — Dependencies & package scaffold

**Steps:**

1. **Add Go dependencies**
   ```bash
   go get github.com/charmbracelet/bubbletea@latest
   go get github.com/charmbracelet/lipgloss@latest
   go get github.com/charmbracelet/bubbles@latest
   go mod tidy
   ```

2. **Create `internal/tui/` package skeleton**  
   Files: `tui.go` (stub `Run()`), `model.go` (stub model), `keys.go`, `clipboard.go`, `search.go`.  
   Ensure `go build -tags fts5 ./...` passes with stubs returning `nil, nil`.

3. **Verify build is clean**
   ```bash
   go build -tags fts5 ./...
   go vet -tags fts5 ./...
   ```

---

### Phase 2 — CLI wiring (`--interactive` flag + TTY fallback)

**Steps:**

1. **Create `cmd/centmem/handlers_tui.go`**  
   Add `runInteractiveTUI()` function (see §4.2). Import `internal/tui`.

2. **Patch `cmdRecall` in `handlers.go`**  
   Add `fs.Bool("interactive", false, ...)` flag declaration.  
   Insert TTY check + `runInteractiveTUI()` call at the top of the `runCommandQuery` callback, before daemon client check.

3. **Update `commands.go`**  
   Add `"--interactive"` to the `recall` flags slice.

4. **Verify contract test still passes**
   ```bash
   go test -tags fts5 ./cmd/centmem/ -run TestContract -race
   ```

5. **Manual smoke test (non-TUI path)**  
   Confirm that `centmem recall "query" --scope global` still works identically (TTY fallback is automatic).

---

### Phase 3 — Core TUI model (query + list)

**Steps:**

1. **Implement `model.go`**  
   - `Init()`: focus `textinput`, fire initial search if `InitQuery != ""`.
   - `Update()`: handle `tea.KeyMsg` for query editing and navigation; handle `searchResultMsg` to populate list.
   - `View()`: render query bar + result list (left pane only for now; preview pane placeholder).

2. **Implement `search.go`**  
   - `scheduleSearch()` Cmd: 200ms debounce using `time.AfterFunc` wrapped in a `tea.Cmd`.
   - Search goroutine: calls `searcher.Recall(ctx, search.Query{Text: q.query.Value(), Scope: m.scope, Top: m.top, Inherit: true})`.
   - Returns `searchResultMsg`.

3. **Implement result list item**  
   Implement `list.Item` interface for `search.Ranked`:
   - `Title()`: first 40 chars of `Content`.
   - `Description()`: `scope + " · " + type`.
   - `FilterValue()`: full `Content` (for bubbletea list's built-in filter, disabled in favor of server-side search).

4. **Smoke test: run TUI and verify list populates**
   ```bash
   go build -tags fts5 ./cmd/centmem/
   ./centmem recall "architecture" --scope global --interactive
   ```

---

### Phase 4 — Preview pane + full memory fetch

**Steps:**

1. **Split-pane layout in `View()`**  
   Use `lipgloss.HorizontalJoin()` (or equivalent) to split left (40%) and right (60%).  
   Right pane is a `viewport.Model` initialized to terminal height minus header/footer rows.

2. **Memory fetch on selection change**  
   In `Update()`, when list selection changes, dispatch a `tea.Cmd` that calls `store.GetMemory(ctx, selectedID)` and returns `memoryLoadedMsg`.  
   On receiving `memoryLoadedMsg`, populate `m.selected` and refresh `m.viewport.SetContent(renderMemory(m.selected))`.

3. **`renderMemory()` function**  
   Returns a formatted multi-line string:
   ```
   ID: 142  ·  type: note  ·  scope: project:cent-mem
   Tags: [architecture, search]
   Created: 2026-09-10 14:32  ·  Accessed: 3 times

   Decided to use RRF with k=60 for hybrid search fusion after
   benchmark testing. The 60-constant provides stable ranking
   regardless of individual lane score magnitude.
   ```
   Apply `lipgloss` styles for field labels vs values.

4. **Terminal resize handling**  
   Handle `tea.WindowSizeMsg` in `Update()` to recalculate pane widths and viewport height.

---

### Phase 5 — Actions (open, copy, delete)

**Steps:**

1. **Open (Enter / `o`)**  
   Set `m.chosen = m.selected`; return `tea.Quit`.  
   In `tui.Run()`, the bubbletea program exit returns `m.chosen` to the caller.  
   Caller (`runInteractiveTUI`) prints it as JSON to stdout.

2. **Copy (`c`)**  
   Implement `clipboard.go` with the shell-out chain (§5.5).  
   In `Update()`, on `c` key: dispatch `tea.Cmd` that writes `m.selected.Content` to clipboard and returns `clipboardDoneMsg`.  
   Show status bar message `"Copied to clipboard"` or error for 2 seconds (use a `tea.Tick` to clear).

3. **Delete (`d` + confirm)**  
   On `d`: set `m.confirmDelete = true`; update status bar to `"Delete memory #142? [y/N]"`.  
   On `y`: dispatch `tea.Cmd` calling `m.store.Forget(ctx, []int64{selectedID}, nil, nil, nil)`, return `forgetDoneMsg`.  
   On `n` or `Esc`: clear `m.confirmDelete`.  
   On `forgetDoneMsg` success: remove item from list; clear preview.

4. **Key help overlay (`?`)**  
   Toggle a full-screen overlay rendered over the main view showing the complete key map.

---

### Phase 6 — Polish & testing

**Steps:**

1. **Write `internal/tui/tui_test.go`**  
   Headless bubbletea tests using `bubbletea.NewProgram` with `WithInput(bytes.NewReader(...))` and `WithOutput(io.Discard)`:
   - Test: typing query characters updates `m.query.Value()`.
   - Test: receiving `searchResultMsg` populates the list.
   - Test: pressing `j` moves selection down; `memoryLoadedMsg` follows.
   - Test: pressing `Enter` sets `m.chosen`.
   - Test: pressing `d` → `y` dispatches `forgetDoneMsg`.
   - Test: TTY guard — when `os.Stdout` is not a TTY, `runInteractiveTUI` falls through to JSON recall.

2. **Run full test suite**
   ```bash
   go test -tags fts5 ./... -race
   go build -tags fts5 ./...
   ```

3. **`graphify update .`**
   ```bash
   graphify update .
   ```

4. **Update `docs/cli-contract.md`**  
   Add `--interactive` to the `recall` command's flag table. No version bump required (additive flag, no breaking change).

---

## 8. Test Strategy

### Unit tests (in `internal/tui/tui_test.go`)

| Test | What to assert |
|------|---------------|
| `TestModelInit` | `Init()` returns a focus Cmd; query value is `InitQuery` |
| `TestQueryTyping` | Successive `KeyMsg` characters build query correctly |
| `TestSearchResult` | `searchResultMsg` populates `m.results` and `list` items |
| `TestNavigation` | `j` increments selected index; `k` decrements |
| `TestOpenAction` | Enter sets `m.chosen` and triggers `tea.Quit` |
| `TestDeleteConfirm` | `d` sets `confirmDelete`; `y` dispatches forget Cmd; `n` clears flag |
| `TestClipboard` | `c` key dispatches clipboard Cmd; result updates status bar |
| `TestWindowResize` | `WindowSizeMsg` recalculates pane widths without panic |

### Integration tests (in `cmd/centmem/`)

| Test | What to assert |
|------|---------------|
| `TestRecallInteractiveFallback` | When stdout is redirected (not TTY), `--interactive` produces standard JSON recall output |
| `TestRecallInteractiveFlag` | Flag is registered and contract test passes |

---

## 9. Dependencies & Risks

| Item | Risk | Mitigation |
|------|------|-----------|
| bubbletea adds ~3 new transitive deps | Low — pure Go, no CGo | Audit with `go mod tidy`; pin version in go.mod |
| `mattn/go-isatty` already in `go.mod` | None | Already present as transitive dep; import directly |
| Clipboard shell-out may fail in some envs | Low — graceful error shown in status bar | Status bar error message; no crash |
| Terminal resize not handled | Medium — layout breaks on resize | Phase 4 explicitly handles `tea.WindowSizeMsg` |
| Slow hybrid search on keystroke | Medium — noticeable lag on large stores | 200ms debounce; spinner feedback during search; TUI `top` defaults to 20 (not 5) |
| Windows ConPTY terminal compatibility | Low-medium — bubbletea has known Windows quirks | Mark as best-effort; test on macOS/Linux; document caveat |
| Contract test breaks on flag addition | Low | Add `"--interactive"` to `commands.go` flags slice before running contract test |

---

## 10. Acceptance Criteria (Done Definition)

Phase 1–3 done when:
- [x] `go build -tags fts5 ./...` passes with new dependencies.
- [x] `centmem recall "query" --interactive` launches a TUI with a live-updating result list.
- [x] `centmem recall "query" --interactive | cat` (non-TTY) produces standard JSON output.

Phase 4–5 done when:
- [x] Right pane shows full memory content when navigating the list.
- [x] Enter exits TUI and prints the selected memory as JSON to stdout.
- [x] `c` copies content to clipboard (or shows a clear error on unsupported systems).
- [x] `d` + `y` deletes the memory and removes it from the list.

Phase 6 done when:
- [x] All unit tests pass: `go test -tags fts5 ./internal/tui/... -race`.
- [x] Full test suite passes: `go test -tags fts5 ./... -race`.
- [x] Contract test passes: `go test -tags fts5 ./cmd/centmem/ -run TestContract -race`.
- [x] `docs/cli-contract.md` updated with `--interactive` flag.

---

## 11. Future Enhancements (out of scope for v2.1.1)

- **Tag filter bar** — add a tag chip row in the TUI (type `#tag` in query to activate).
- **Type filter** — a tab row (All / Notes / Logs / Facts) that narrows results.
- **In-TUI edit** — press `e` to open the memory content in `$EDITOR`, then update on save.
- **Yank JSON** (`y`) — print selected memory JSON to stdout without exiting (like `fzf --print-query`).
- **TUI for `centmem timeline`** — a chronological feed mode triggered by `centmem timeline --interactive`.
- **Mouse support** — click to select; scroll in preview pane.

---

## 12. Changelog

| Date | Change |
|------|--------|
| 2026-09-14 | Initial plan created via `/grill-me` session. Design locked. |
| 2026-09-14 | Implemented Phases 1–6: `internal/tui` package, CLI `--interactive` flag wiring with TTY fallback, race-hardened preview viewport with scrolling, 18 unit tests, CLI integration tests, cli-contract.md and skill docs updated. |
