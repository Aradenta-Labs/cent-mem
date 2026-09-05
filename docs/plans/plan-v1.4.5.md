# v1.4.5 — Smart Skills Installer

**Status:** Decisions resolved — ready for implementation  
**Scope:** `npm/bin/install.js` + `npm/test/install.test.js` only. `skill/install.sh` is unchanged.

The current skills installer (`@aradenta.labs/centmem-skills`) has three problems:

1. **Scatter-shot targeting**: It installs to every known harness directory regardless of whether that harness is actually installed on the machine. A user with only Cursor ends up with skill files in `.claude/`, `.gemini/`, `.codex/`, etc. — directories that don't exist and don't help.

2. **No scope awareness**: It always installs to both project-level and global directories simultaneously. Users who want project-only or global-only installation have no clean way to express that.

3. **No interactivity**: When something is ambiguous (multiple harnesses found, or none found), it silently installs everywhere or nowhere. The `skills.sh` ecosystem (reference: `npx skills add <owner/repo>`) shows users a clear, interactive install experience.
**v1.4.5** fixes all three by reworking `npm/bin/install.js` with:
- **Harness detection** (filesystem config dir presence)
- **Scope inference** (project if inside a Git repo, global via `--global`)
- **Interactive disambiguation** using Node.js built-in `readline` — TTY-safe, zero new deps
**What we are NOT doing:**
- Fetching skills from GitHub repos (our package only installs centmem's bundled skill)
- Changing `skill/install.sh` (bash installer stays as a simple dumb fallback)
- Rewriting existing passing tests (extend, don't replace)

## 1. Comparison: skills.sh vs. ours (current vs. target)
| Capability | skills.sh | Ours (current) | Ours (v1.4.5 target) |
|---|---|---|---|
| Detect installed harnesses | ✅ | ❌ installs everywhere | ✅ config dir detection |
| Project scope install | ✅ default when in repo | ❌ always both | ✅ default in Git repo |
| Global scope install | ✅ flag | ❌ always both | ✅ `--global` flag |
| Interactive harness picker | ✅ | ❌ | ✅ (when ambiguous) |
| Fetch from GitHub repo | ✅ core feature | ❌ | ❌ (out of scope) |
| CI / non-TTY safe | ✅ | ✅ | ✅ silent default fallback |
| Zero new deps | ✅ | ✅ | ✅ `readline` built-in only |
---

## 2. Design Decisions

All resolved during design interview.
| # | Question | Decision | Rationale |
|---|---|---|---|
| 1 | Install targeting | Detect-and-install: only target harnesses whose config dir exists | Avoids litter in directories the user doesn't use |
| 2 | Default scope | Project scope when inside a Git repo; global when outside | Matches how npm/pip work; predictable context-aware default |
| 3 | `--global` flag | Explicit flag forces global scope from any directory | Clean escape hatch for users who want machine-wide installation |
| 4 | Interactive prompts | Only when ambiguous (multiple harnesses found, or none found) | Silent success is fast; interactive is a fallback not a tax |
| 5 | Prompt library | Node.js `readline` built-in + TTY detection | Zero new dependencies; CI-safe |
| 6 | GitHub fetch | Not implemented | Our package is self-contained; not a skills.sh clone |
| 7 | `install.sh` | Unchanged | Out of scope for patch release |
| 8 | Tests | Keep 9 existing, add unit tests for new code paths | Extend, don't rewrite |
---

## 3. New Architecture

### 3.1 Harness Registry
A declarative registry maps each supported harness to its detection signal
and install target path template:

```js
// npm/bin/harnesses.js  [NEW]
const HARNESSES = [
  {
    id:       'antigravity',
    label:    'Antigravity (AGY)',
    detect:   home => fs.existsSync(path.join(home, '.gemini', 'antigravity')),
    targets:  { global: home => path.join(home, '.gemini', 'antigravity', 'skills') },
  },
  {
    id:       'claude',
    label:    'Claude Code',
    detect:   home => fs.existsSync(path.join(home, '.claude')),
    targets:  {
      global:  home => path.join(home, '.claude', 'skills'),
      project: cwd  => path.join(cwd, '.claude', 'skills'),
    },
  },
  {
    id:       'cursor',
    label:    'Cursor',
    detect:   home => fs.existsSync(path.join(home, '.cursor')),
    targets:  { global: home => path.join(home, '.cursor', 'rules') },
  },
  {
    id:       'codex',
    label:    'OpenAI Codex',
    detect:   home => fs.existsSync(path.join(home, '.codex')),
    targets:  { global: home => path.join(home, '.codex', 'skills') },
  },
  {
    id:       'agents',
    label:    'Generic (.agents/skills)',
    detect:   (home, cwd) =>
                fs.existsSync(path.join(home, '.agents')) ||
                fs.existsSync(path.join(cwd,  '.agents')),
    targets:  {
      global:  home => path.join(home, '.agents', 'skills'),
      project: cwd  => path.join(cwd,  '.agents', 'skills'),
    },
  },
  {
    id:       'trae',
    label:    'Trae',
    detect:   home => fs.existsSync(path.join(home, '.trae')),
    targets:  { global: home => path.join(home, '.trae', 'skills') },
  },
  {
    id:       'hermes',
    label:    'Hermes',
    detect:   home => fs.existsSync(path.join(home, '.hermes')),
    targets:  { global: home => path.join(home, '.hermes', 'skills') },
  },
];
module.exports = { HARNESSES };
```
`detectHarnesses(home, cwd)` → returns only entries whose `detect()` returns true.

---

### 3.2 Scope Inference

```js
// In install.js — resolveScope(options, cwd)
function isInsideGitRepo(cwd) {
  // Walk up from cwd looking for a .git directory
  let dir = cwd;
  while (true) {
    if (fs.existsSync(path.join(dir, '.git'))) return true;
    const parent = path.dirname(dir);
    if (parent === dir) return false;
    dir = parent;
  }
}
function resolveScope(options) {
  if (options.global) return 'global';
  if (options.project) return 'project';
  return isInsideGitRepo(options.cwd) ? 'project' : 'global';
}
```

Scope `'project'` → use `targets.project(cwd)` from the harness.
Scope `'global'` → use `targets.global(home)` from the harness.
If a harness has no `targets.project`, it falls back to `targets.global`.

---

### 3.3 Interactive Disambiguation

Triggered only in these cases:
1. **Multiple harnesses detected** → show checklist, let user deselect
2. **No harness detected** → show full list, prompt user to pick
3. **Non-TTY (CI/pipe)** → skip prompts, use all detected (case 1) or none (case 2)

```js
// In install.js — promptHarnessSelection(detected)
async function promptHarnessSelection(detected) {
  if (!process.stdout.isTTY) {
    // CI mode: use all detected silently
    return detected;
  }

  if (detected.length === 0) {
    console.log('No supported AI harnesses detected on this machine.');
    console.log('Available harnesses:');
    HARNESSES.forEach((h, i) => console.log(`  ${i + 1}. ${h.label}`));
    const answer = await prompt('Enter numbers to install to (e.g. 1,3) or Enter to skip: ');
    return parseSelection(answer, HARNESSES);
  }

  if (detected.length === 1) {
    // Exactly one: install silently, no prompt
    return detected;
  }

  // Multiple detected: confirm selection
  console.log('Detected harnesses:');
  detected.forEach((h, i) => console.log(`  [${i + 1}] ${h.label}`));
  const answer = await prompt(`Install to all ${detected.length}? [Y/n/list of numbers]: `);
  return parseConfirmation(answer, detected);
}
```

`prompt()` is a thin wrapper over `readline.createInterface`.

---

### 3.4 Updated `run()` Flow

```
run(argv)
  → parseArgs()               // new flags: --global, --project, --yes
  → resolveScope()            // infer from Git root or flags
  → detectHarnesses()         // filter by config dir presence
  → promptHarnessSelection()  // interactive only when needed; --yes skips
  → buildTargets()            // construct install paths from scope + harness registry
  → install / uninstall       // existing copyDirRecursive / rmSync logic
  → injectWorkflowAndCommands() // unchanged
```

---

## 4. CLI Changes

### New flags

| Flag | Description |
|---|---|
| `--global` | Force global scope (`~/<harness>/...`). |
| `--project` | Force project scope (`<cwd>/<harness>/...`). Useful when run outside a Git repo. |
| `--yes`, `-y` | Skip all interactive prompts; accept all detected harnesses automatically. |

### Updated help text
```
USAGE:
  npx @aradenta.labs/centmem-skills [command] [options]

COMMANDS:
  install (default)   Install skill to detected AI harnesses
  uninstall           Remove skill files and injected workflow instructions
  list, --list        Show detected harnesses and install targets

OPTIONS:
  --global            Install to global harness dirs (~/.claude/, ~/.cursor/, etc.)
  --project           Install to project harness dirs (./.claude/, ./.agents/, etc.)
  --yes, -y           Skip interactive prompts, accept defaults
  --skill <name>      Skill name (default: "centmem")
  --dry-run           Show actions without modifying disk
  --cwd <path>        Override working directory
  --home <path>       Override home directory
  -h, --help          Show this help

EXAMPLES:
  # Install to this project (auto-detects harnesses):
  npx @aradenta.labs/centmem-skills

  # Install globally to all detected harnesses:
  npx @aradenta.labs/centmem-skills --global

  # Install globally, no prompts (CI/script):
  npx @aradenta.labs/centmem-skills --global --yes

  # Preview what would be installed:
  npx @aradenta.labs/centmem-skills --dry-run

  # Uninstall from project:
  npx @aradenta.labs/centmem-skills uninstall
```

---

## 5. Files Affected

| File | Change |
|---|---|
| [`npm/bin/install.js`](../../npm/bin/install.js) | Add `resolveScope()`, `isInsideGitRepo()`, `promptHarnessSelection()`, `buildTargets()`, new flag parsing (`--global`, `--project`, `--yes`). Refactor `getSkillTargets()` to use harness registry. |
| [`npm/bin/harnesses.js`](../../npm/bin/harnesses.js) | **[NEW]** Harness registry (`HARNESSES` array + `detectHarnesses()`) |
| [`npm/test/install.test.js`](../../npm/test/install.test.js) | Keep all 9 existing tests. Add 10 new unit tests (see §6). |
| [`npm/package.json`](../../npm/package.json) | Bump version to `1.4.5` |
| [`cmd/centmem/main.go`](../../cmd/centmem/main.go) | Bump version to `1.4.5` |
| [`CHANGELOG.md`](../../CHANGELOG.md) | Add `[1.4.5]` section |

---

## 6. Tests

### Keep (9 existing, must stay green)

All tests in `npm/test/install.test.js` — no modifications.

### New tests to add

| Test | What it proves |
|---|---|
| `detectHarnesses_returnsOnlyPresentDirs` | Only harnesses whose config dir exists are returned |
| `detectHarnesses_returnsEmptyWhenNoneFound` | Returns `[]` when no harness config dirs exist |
| `resolveScope_projectWhenInsideGitRepo` | Returns `'project'` when `.git` is found walking up |
| `resolveScope_globalWhenOutsideGitRepo` | Returns `'global'` when no `.git` found |
| `resolveScope_globalFlagOverridesGitRepo` | `--global` forces `'global'` even inside a repo |
| `resolveScope_projectFlagOverridesOutsideRepo` | `--project` forces `'project'` even outside a repo |
| `buildTargets_projectScope_usesProjectPaths` | Targets are `cwd/.agents/skills/centmem`, etc. |
| `buildTargets_globalScope_usesHomePaths` | Targets are `~/.agents/skills/centmem`, etc. |
| `promptHarnessSelection_nonTTY_returnsDetected` | In CI (non-TTY), returns all detected without prompting |
| `promptHarnessSelection_singleHarness_returnsWithoutPrompt` | Exactly one harness → silent install, no readline |

---

## 7. Example UX Flows

### Flow A — Project install, one harness detected (Antigravity)

```
$ npx @aradenta.labs/centmem-skills

Detected harnesses: Antigravity (AGY)
Scope: project (inside Git repo)

Installing skill 'centmem'...
  installed → .agents/skills/centmem
  installed → .gemini/antigravity/skills/centmem
  injected  → AGENTS.md

  Done. Restart your agent to pick up the new skill.
```

### Flow B — Project install, multiple harnesses detected

```
$ npx @aradenta.labs/centmem-skills

Detected harnesses:
  [1] Antigravity (AGY)
  [2] Claude Code
  [3] Cursor

Install to all 3? [Y/n/list of numbers]: 1,2
Scope: project (inside Git repo)

Installing skill 'centmem'...
  installed → .agents/skills/centmem
  installed → .gemini/antigravity/skills/centmem
  installed → .claude/skills/centmem
  injected  → AGENTS.md

Done. Restart your agent to pick up the new skill.
```

### Flow C — Global install, no harnesses auto-detected

```
$ npx @aradenta.labs/centmem-skills --global

No supported AI harnesses detected on this machine.
Available harnesses:
  1. Antigravity (AGY)
  2. Claude Code
  3. Cursor
  4. OpenAI Codex
  5. Generic (.agents/skills)
  6. Trae
  7. Hermes

Enter numbers to install to (e.g. 1,3) or Enter to skip: 1,3
Scope: global

Installing skill 'centmem'...
  installed → ~/.gemini/antigravity/skills/centmem
  installed → ~/.cursor/rules/centmem
  injected  → ~/AGENTS.md

Done. Restart your agent to pick up the new skill.
```

### Flow D — CI / scripted install (non-TTY, `--yes`)
```
$ npx @aradenta.labs/centmem-skills --global --yes
# No prompts. Installs to all detected harnesses silently.
# If none detected, exits 0 with a warning on stderr.
```

## 8. Milestone Checklist

**Implementation**
- [ ] `npm/bin/harnesses.js` — write `HARNESSES` registry and `detectHarnesses(home, cwd)`
- [ ] `npm/bin/install.js` — add `isInsideGitRepo()` (walk-up `.git` detection)
- [ ] `npm/bin/install.js` — add `resolveScope(options)` using Git detection + flags
- [ ] `npm/bin/install.js` — update `parseArgs()` for `--global`, `--project`, `--yes`
- [ ] `npm/bin/install.js` — refactor `getSkillTargets()` to use harness registry + resolved scope
- [ ] `npm/bin/install.js` — implement `promptHarnessSelection()` with `readline` + TTY guard
- [ ] `npm/bin/install.js` — implement `buildTargets(harnesses, scope, home, cwd, skillName)`
- [ ] `npm/bin/install.js` — update `printHelp()` with new flags and example flows
- [ ] `npm/bin/install.js` — update `list` command to show detected harnesses and resolved scope

**Tests**
- [ ] All 9 existing tests still pass (`node npm/test/install.test.js`)
- [ ] 10 new unit tests added and passing

**Release**
- [ ] `npm/package.json` version bumped to `1.4.5`
- [ ] `cmd/centmem/main.go` version bumped to `1.4.5`
- [ ] `CHANGELOG.md` updated with `[1.4.5]` section
- [ ] Tagged `v1.4.5` and pushed; GitHub Actions release runs
- [ ] npm publish: `cd npm && npm publish --access public`
- [ ] `graphify update .` run after all code changes


