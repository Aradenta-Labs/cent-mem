# @aradenta.labs/centmem-skills

> **Frictionless installer for `cent-mem` skills & slash commands across AI agent harnesses.**

[![npm version](https://img.shields.io/npm/v/@aradenta.labs/centmem-skills.svg)](https://www.npmjs.com/package/@aradenta.labs/centmem-skills)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/aradenta-labs/cent-mem/blob/master/LICENSE)

`centmem-skills` is a zero-dependency, cross-platform installer that equips your AI coding agents with [cent-mem](https://github.com/aradenta-labs/cent-mem) shared memory and intuitive `/centmem` slash command capabilities.

---

## Prerequisites: Install `centmem` CLI

Before connecting your AI agents, ensure the `centmem` Go binary is installed and initialized on your machine.

### 1. Install `centmem`

**Option A: Pre-built Binary (macOS & Linux)**
Download the latest release for your OS and architecture from [GitHub Releases](https://github.com/aradenta-labs/cent-mem/releases/latest):

```bash
# Example for macOS (Apple Silicon):
curl -fsSL -o centmem https://github.com/aradenta-labs/cent-mem/releases/latest/download/centmem-darwin-arm64
chmod +x centmem
sudo mv centmem /usr/local/bin/
```

*(Pre-built binaries available for `darwin-arm64`, `darwin-amd64`, `linux-amd64`, and `linux-arm64`.)*

**Option B: Install with Go (Go 1.22+)**
```bash
go install -tags fts5 github.com/aradenta-labs/cent-mem/cmd/centmem@latest
```

### 2. Initialize `centmem` (Run once per machine)

```bash
centmem init
```
*This creates the local database (`~/.centmem/`) and downloads the local ONNX embedding model (~100 MB).*

---

## Quickstart: Install Agent Skills

Run the installer directly from your project root:

```bash
npx @aradenta.labs/centmem-skills
```

---

## What It Does

When you run `npx @aradenta.labs/centmem-skills`:

1. **Installs Detailed Skill Artifacts:** Auto-discovers and installs `SKILL.md`, comprehensive references, workflow examples, and helper scripts into standard AI skill locations:
   - **Project-Level Agents Skills** (`./.agents/skills/centmem`)
   - **User-Level Global Agents Skills** (`~/.agents/skills/centmem`)
   - **Claude Code** (`./.claude/skills/centmem`, `~/.claude/skills/centmem`)
   - **Cursor** (`~/.cursor/rules/centmem`)
   - **Google Antigravity** (`~/.gemini/antigravity/skills/centmem`)
   - **OpenAI Codex** (`~/.codex/skills/centmem`)
   - **Trae** (`~/.trae/skills/centmem`)
   - **Hermes Agent** (`~/.hermes/skills/centmem`)
   - **DeepSeek Harness** (`~/.deepseek/skills/centmem`)
   - **Global Config** (`~/.config/skills/centmem`)

2. **Automated Memory Loop Injection in `AGENTS.md`:**
   Guarantees that `AGENTS.md` is updated (or created) in your project root, and injects into any existing rule files (`CLAUDE.md`, `.cursorrules`, `.github/copilot-instructions.md`) to establish the standard memory loop:
   - **READ**: Run `centmem recall "<task context>"` to load prior context.
   - **DO**: Execute the task.
   - **UPDATE**: Run `centmem put` or `centmem set` to persist decisions and learnings.

3. **Smart `/centmem` Slash Command Support:**
   Registers routing instructions that allow you to interact with memory using simple prompts like `/centmem review recent architectural decisions` (triggers recall), `/centmem remember we use PostgreSQL for this service` (triggers put/set), or `/centmem configure llm backend to ollama` (triggers config get/set).

---

## CLI Usage & Options

```bash
# Default installation
npx @aradenta.labs/centmem-skills

# Install with explicit repository source
npx @aradenta.labs/centmem-skills add https://github.com/aradenta-labs/cent-mem --skill centmem

# Preview targets and planned actions without touching disk
npx @aradenta.labs/centmem-skills --dry-run

# List detected instruction files and target directories
npx @aradenta.labs/centmem-skills --list

# Cleanly remove installed skill directories and injected instructions
npx @aradenta.labs/centmem-skills --uninstall
```

### Options

| Option | Description |
|---|---|
| `--global` | Install globally to user-level harness directories (`~/.claude/`, `~/.cursor/`, etc.) |
| `--project` | Install locally to project-level harness directories (`./.agents/`, `./.claude/`, etc.) |
| `--yes, -y` | Skip interactive prompts and accept all detected defaults |
| `--skill <name>` | Custom skill name (default: `centmem`) |
| `--dry-run` | Simulate actions without modifying the filesystem |
| `--list` | Display target install paths and detected instruction files |
| `--uninstall` | Cleanly remove skill files and instruction blocks |
| `--cwd <path>` | Override target working directory |
| `--home <path>` | Override target home directory |
| `-h, --help` | Show command help |

---

## About cent-mem

`cent-mem` is a fast, local-first shared memory store for AI agents powered by SQLite, vector search, BM25 full-text search, and a built-in cognitive memory agent engine.

- **GitHub Repository:** [github.com/aradenta-labs/cent-mem](https://github.com/aradenta-labs/cent-mem)
- **Documentation:** [cent-mem Docs](https://github.com/aradenta-labs/cent-mem/tree/master/docs)
- **License:** MIT
