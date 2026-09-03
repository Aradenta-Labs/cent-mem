# Implementation Plan: cent-mem v1.2.0 (Frictionless UX)

**Version:** 1.2.0
**Goal:** Drastically simplify skill installation via `npx`, introduce slash command support (`/centmem`), and enforce the memory loop by automatically injecting instructions into project-level agent docs.

---

## 1. Resolved Requirements

1. **Custom NPM Installer:** We will create and publish our own NPM package installer so users can run a command like `npx centmem-skills add <url>`. 
2. **Slash Command Targets:** The `/centmem` slash command integration will be built for the following harnesses:
   - Antigravity
   - Trae
   - Claude Code
   - Cursor
   - Codex
   - Deepseek Harness
   - Hermes
3. **Instruction Injection Fallback:** If the installer cannot find an existing `AGENTS.md`, `.cursorrules`, or `CLAUDE.md` in the project root, it will automatically generate a new `AGENTS.md` file and inject the cent-mem workflow into it.

---

## 2. Proposed Milestones

### Milestone 1: The `npx` Installer Package
*Move away from `curl | bash` to a standardized Node-based installer.*
- **[NEW]** `npm/package.json` and `npm/bin/install.js`: Create a custom npm package wrapper for the installer (e.g., `centmem-skills`).
- **[MODIFY]** `skill/install.sh`: Port the core logic of finding directories to the Node script for cross-platform support (Windows/macOS/Linux), allowing execution via `npx`.
- **Deliverable:** Users can run the command `npx centmem-skills add https://github.com/aradenta-labs/cent-mem --skill centmem`.

### Milestone 2: Automated Workflow Injection
*Enforce the Read -> Do -> Update loop.*
- **[MODIFY]** Node installer logic:
  1. Detect project-level instruction files in `$PWD` (`AGENTS.md`, `CLAUDE.md`, `.cursorrules`, `.github/copilot-instructions.md`).
  2. If none are found, **automatically create `AGENTS.md`** at the project root.
  3. Safely append the **cent-mem core workflow loop**:
     ```markdown
     ## cent-mem Workflow
     Whenever you start a task, follow this loop:
     1. **READ**: Run `centmem recall "<task context>"` to load prior context.
     2. **DO**: Execute the user's prompt.
     3. **UPDATE**: Run `centmem put` or `centmem set` to store new learnings, decisions, or endpoints before ending.
     ```
- **Deliverable:** The skill installer automatically updates (or creates) the repo's prompt files, ensuring the AI agent always uses the memory loop.

### Milestone 3: Slash Command Integrations
*Allow users to manage the shared memory via a single, smart `/centmem` command.*
- **[NEW]** Config payload templates in `skill/adapters/` for the target agents: Antigravity, Trae, Claude Code, Cursor, Codex, Deepseek Harness, and Hermes.
- **[MODIFY]** Installer logic to automatically register the slash command. Because `centmem` has multiple capabilities (reading, writing, maintenance), the command will use a **Smart Routing Prompt** rather than forcing a single action.
- **[MODIFY]** For closed AI platforms (like Cursor and Claude Code) lacking a programmatic global slash command API, the installer will simulate the command by injecting the routing instruction into their respective rule files (e.g., `.cursorrules`).
- **Prompt Payload Example:** *"When the user types `/centmem <input>`, act as the Memory Manager. Analyze the intent: if asking a question or looking for context, use `centmem recall` or `timeline`. If stating a decision or fact to save, use `centmem put` or `set`. If requesting maintenance, use `centmem compact` or `doctor`."*
- **Deliverable:** Users can intuitively invoke the agent with `/centmem check project progress` (read intent) or `/centmem save this API structure` (write intent), and the agent will dynamically select the correct CLI tool.

### Milestone 4: Documentation & Release
- **[MODIFY]** `README.md` and `docs/getting-started.md`: Update installation instructions to feature the new `npx` flow and the `/centmem` slash command.
- **[MODIFY]** `CHANGELOG.md`: Draft the v1.2.0 release notes.

---

## 3. Verification Plan

- **Automated Tests:** Add a Node.js test suite for the new `install.js` script to verify it correctly parses and appends to dummy `AGENTS.md` and `.cursorrules` files without destroying existing content.
- **Manual E2E Validation:** 
  1. Run the `npx` command in a fresh test repository.
  2. Verify `AGENTS.md` was modified with the workflow loop.
  3. Open the repository in Cursor / Claude Code and verify the `/centmem` slash command registers successfully.
