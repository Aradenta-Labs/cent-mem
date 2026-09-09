#!/usr/bin/env node
/**
 * centmem-skills installer
 * Cross-platform installer for cent-mem shared memory skills and slash commands.
 * Zero-dependency: works with pure Node.js built-in modules.
 */

const fs = require('fs');
const path = require('path');
const os = require('os');
const readline = require('readline');
const { HARNESSES, detectHarnesses } = require('./harnesses');

const WORKFLOW_BLOCK = `<!-- centmem:start -->
## cent-mem Workflow
All agents working on this project share persistent memory via \`centmem\`. Detailed skill artifacts and reference guides are located in \`.agents/skills/centmem/\`.

Whenever you start a task, follow this 3-step loop:
1. **READ**: Run \`centmem recall "<task context>"\` to load prior decisions, conventions, and learnings.
2. **DO**: Execute the user's prompt.
3. **UPDATE**: Run \`centmem put\` or \`centmem set\` to store new learnings, decisions, conventions, or checkpoints before finishing.

### Essential Commands
- **Recall Context**: \`centmem recall "<query>" --scope "project:$CENTMEM_PROJ" --top 5\`
- **Save Decision / Note**: \`centmem put --scope "project:$CENTMEM_PROJ" --type note --content "<decision>" --tags decision\`
- **Save Key/Value Fact**: \`centmem set --scope "project:$CENTMEM_PROJ" --key "<key>" --value '<json>'\`
- **Recent Activity Timeline**: \`centmem timeline --scope "project:$CENTMEM_PROJ" --since 24h\`
- **Manage Relationships**: \`centmem link <from_id> <to_id> --relation supersedes\` or \`centmem links <id>\`
- **Check Health**: \`centmem doctor\`
<!-- centmem:end -->`;

const COMMAND_BLOCK = `<!-- centmem-command:start -->
## /centmem Command
When the user types \`/centmem <input>\`, act as the Memory Manager. Analyze the intent:
- **Recall / Context**: If asking a question or looking for context, run \`centmem recall "<input>" --scope "project:$CENTMEM_PROJ" --top 5\` or \`centmem timeline\`.
- **Save Decisions / Facts**: If stating a decision, convention, preference, or learning to save, run \`centmem put --scope "project:$CENTMEM_PROJ" --type note --content "<input>"\` or \`centmem set --scope "project:$CENTMEM_PROJ" --key "<key>" --value '<json>'\`.
- **Relationships / Links**: If asking to link memories, inspect relationships, trace dependencies/contradictions, or confirm/dismiss auto-suggestions, run centmem link, centmem links, or centmem unlink.
- **Ingest / Capture**: If asking to ingest repository knowledge, commits, docs, shell history, or code annotations, run \`centmem capture git\`, \`centmem capture docs\`, \`centmem capture shell\`, or \`centmem capture comments\`.
- **Configuration / Agent Engine**: If asking to configure LLM backend, agent reasoning limits, or inspect settings, run \`centmem config get llm\`, \`centmem config get agent\`, or \`centmem config set <key> <value>\`.
- **Maintenance / Health**: If requesting maintenance, health checks, or statistics, run \`centmem doctor\`, \`centmem stats\`, or \`centmem compact\`.
- **Visual Dashboard / UI**: If asking to view, browse, inspect, or manage memories in a browser interface, run \`centmem ui\`.

Always verify execution results from stdout JSON and report them clearly to the user.
<!-- centmem-command:end -->`;

const CANDIDATE_INSTRUCTION_FILES = [
  'AGENTS.md',
  'CLAUDE.md',
  '.cursorrules',
  '.github/copilot-instructions.md'
];

function getHomeDir(customHome) {
  if (customHome) return customHome;
  return process.env.HOME || process.env.USERPROFILE || os.homedir();
}

function getSkillSourceDir() {
  // Check local npm/templates directory first (packaged standalone)
  const templateDir = path.resolve(__dirname, '..', 'templates');
  if (fs.existsSync(templateDir) && fs.existsSync(path.join(templateDir, 'SKILL.md'))) {
    return templateDir;
  }
  // Fallback to repo root skill/ directory if running from repo
  const repoSkillDir = path.resolve(__dirname, '..', '..', 'skill');
  if (fs.existsSync(repoSkillDir) && fs.existsSync(path.join(repoSkillDir, 'SKILL.md'))) {
    return repoSkillDir;
  }
  return templateDir;
}

function isInsideGitRepo(cwd) {
  if (!cwd) return false;
  let dir = path.resolve(cwd);
  while (true) {
    try {
      if (fs.existsSync(path.join(dir, '.git'))) {
        return true;
      }
    } catch (_) {
      return false;
    }
    const parent = path.dirname(dir);
    if (parent === dir) {
      return false;
    }
    dir = parent;
  }
}

function resolveScope(options = {}) {
  if (options.global) return 'global';
  if (options.project) return 'project';
  const targetDir = options.cwd || process.cwd();
  return isInsideGitRepo(targetDir) ? 'project' : 'global';
}

function buildTargets(harnesses, scope = 'global', home, cwd, skillName = 'centmem') {
  const targets = [];
  const list = Array.isArray(harnesses) ? harnesses : [];

  // In project scope, always include standard .agents/skills project directory
  if (scope === 'project' && cwd) {
    targets.push(path.join(cwd, '.agents', 'skills', skillName));
  }

  for (const h of list) {
    if (scope === 'project') {
      if (h.targets && h.targets.project && cwd) {
        targets.push(path.join(h.targets.project(cwd), skillName));
      } else if (h.targets && h.targets.global && home) {
        targets.push(path.join(h.targets.global(home), skillName));
      }
    } else {
      if (h.targets && h.targets.global && home) {
        targets.push(path.join(h.targets.global(home), skillName));
      } else if (h.targets && h.targets.project && cwd) {
        targets.push(path.join(h.targets.project(cwd), skillName));
      }
    }
  }

  return Array.from(new Set(targets));
}

function getSkillTargets(homeDir, cwd, skillName = 'centmem', scope, harnesses) {
  if (harnesses && Array.isArray(harnesses)) {
    return buildTargets(harnesses, scope || (cwd ? resolveScope({ cwd }) : 'global'), homeDir, cwd, skillName);
  }
  // Backwards-compatible / all targets across registry
  const targets = [];
  for (const h of HARNESSES) {
    if (h.targets && h.targets.project && cwd) {
      targets.push(path.join(h.targets.project(cwd), skillName));
    }
    if (h.targets && h.targets.global && homeDir) {
      targets.push(path.join(h.targets.global(homeDir), skillName));
    }
  }
  return Array.from(new Set(targets));
}

function detectInstructionFiles(cwd) {
  const detected = [];
  const primaryAgentsPath = path.join(cwd, 'AGENTS.md');
  const lowerAgentsPath = path.join(cwd, 'agents.md');

  // Check actual directory entries to preserve exact case on case-insensitive filesystems
  let agentsTarget = primaryAgentsPath;
  if (fs.existsSync(cwd)) {
    try {
      const dirEntries = fs.readdirSync(cwd);
      if (dirEntries.includes('agents.md')) {
        agentsTarget = lowerAgentsPath;
      }
    } catch (_) {}
  }
  detected.push(agentsTarget);

  for (const relPath of CANDIDATE_INSTRUCTION_FILES) {
    if (relPath.toLowerCase() === 'agents.md') continue;
    const fullPath = path.join(cwd, relPath);
    if (fs.existsSync(fullPath)) {
      detected.push(fullPath);
    }
  }
  return detected;
}

function injectSection(content, startTag, endTag, newBlock) {
  const startIndex = content.indexOf(startTag);
  const endIndex = content.indexOf(endTag);

  if (startIndex !== -1 && endIndex !== -1 && endIndex >= startIndex) {
    const before = content.slice(0, startIndex).trimEnd();
    const after = content.slice(endIndex + endTag.length).trimStart();
    const joined = [before, newBlock, after].filter(Boolean).join('\n\n');
    return joined.endsWith('\n') ? joined : joined + '\n';
  }

  const trimmed = content.trimEnd();
  if (!trimmed) {
    return newBlock + '\n';
  }
  return trimmed + '\n\n' + newBlock + '\n';
}

function removeSection(content, startTag, endTag) {
  const startIndex = content.indexOf(startTag);
  const endIndex = content.indexOf(endTag);

  if (startIndex !== -1 && endIndex !== -1 && endIndex >= startIndex) {
    const before = content.slice(0, startIndex).trimEnd();
    const after = content.slice(endIndex + endTag.length).trimStart();
    const joined = [before, after].filter(Boolean).join('\n\n');
    return joined ? (joined.endsWith('\n') ? joined : joined + '\n') : '';
  }
  return content;
}

function injectWorkflowAndCommands(filePath, dryRun = false) {
  let existingContent = '';
  const fileExists = fs.existsSync(filePath);
  if (fileExists) {
    existingContent = fs.readFileSync(filePath, 'utf8');
  }

  let updated = existingContent;
  updated = injectSection(updated, '<!-- centmem:start -->', '<!-- centmem:end -->', WORKFLOW_BLOCK);
  updated = injectSection(updated, '<!-- centmem-command:start -->', '<!-- centmem-command:end -->', COMMAND_BLOCK);

  if (!dryRun) {
    const dir = path.dirname(filePath);
    if (!fs.existsSync(dir)) {
      fs.mkdirSync(dir, { recursive: true });
    }
    fs.writeFileSync(filePath, updated, 'utf8');
  }
  return { updated, created: !fileExists };
}

function removeWorkflowAndCommands(filePath, dryRun = false) {
  if (!fs.existsSync(filePath)) {
    return { modified: false, removed: false };
  }
  const content = fs.readFileSync(filePath, 'utf8');
  let updated = content;
  updated = removeSection(updated, '<!-- centmem:start -->', '<!-- centmem:end -->');
  updated = removeSection(updated, '<!-- centmem-command:start -->', '<!-- centmem-command:end -->');

  const changed = updated !== content;
  if (!dryRun && changed) {
    if (updated.trim() === '') {
      fs.unlinkSync(filePath);
      return { modified: true, removed: true };
    } else {
      fs.writeFileSync(filePath, updated, 'utf8');
      return { modified: true, removed: false };
    }
  }
  return { modified: changed, removed: false };
}

function copyDirRecursive(src, dest, dryRun = false) {
  if (!fs.existsSync(src)) return;
  if (!dryRun && !fs.existsSync(dest)) {
    fs.mkdirSync(dest, { recursive: true });
  }

  const entries = fs.readdirSync(src, { withFileTypes: true });
  for (const entry of entries) {
    const srcPath = path.join(src, entry.name);
    const destPath = path.join(dest, entry.name);

    if (entry.isDirectory()) {
      copyDirRecursive(srcPath, destPath, dryRun);
    } else {
      if (!dryRun) {
        fs.copyFileSync(srcPath, destPath);
        if (entry.name.endsWith('.sh') || entry.name.endsWith('.js') || entry.name.endsWith('.mjs')) {
          try {
            fs.chmodSync(destPath, 0o755);
          } catch (_) {
            // Ignore permission errors on filesystems that do not support chmod
          }
        }
      }
    }
  }
}

function prompt(question, options = {}) {
  return new Promise(resolve => {
    const rl = readline.createInterface({
      input: options.input || process.stdin,
      output: options.output || process.stdout
    });
    rl.question(question, answer => {
      rl.close();
      resolve(answer);
    });
  });
}

function parseSelection(answer, harnesses) {
  const trimmed = (answer || '').trim();
  if (!trimmed) return [];
  const parts = trimmed.split(/[\s,]+/).map(s => parseInt(s.trim(), 10)).filter(n => !isNaN(n) && n >= 1 && n <= harnesses.length);
  const selected = [];
  for (const idx of parts) {
    const h = harnesses[idx - 1];
    if (h && !selected.includes(h)) {
      selected.push(h);
    }
  }
  return selected;
}

function parseConfirmation(answer, detected) {
  const trimmed = (answer || '').trim().toLowerCase();
  if (!trimmed || trimmed === 'y' || trimmed === 'yes') {
    return detected;
  }
  if (trimmed === 'n' || trimmed === 'no') {
    return [];
  }
  return parseSelection(answer, detected);
}

function promptHarnessSelection(detected, allHarnesses = HARNESSES, options = {}) {
  const isTTY = options.isTTY !== undefined
    ? Boolean(options.isTTY)
    : Boolean(process.stdout && process.stdout.isTTY && process.stdin && process.stdin.isTTY);

  // Non-TTY / CI mode or --yes flag:
  if (!isTTY || options.yes) {
    return detected;
  }

  // Exactly one detected: silent install, no prompt
  if (detected.length === 1) {
    return detected;
  }

  // If none detected: prompt user to pick from all available harnesses
  if (detected.length === 0) {
    console.log('No supported AI harnesses detected on this machine.');
    console.log('Available harnesses:');
    allHarnesses.forEach((h, i) => console.log(`  ${i + 1}. ${h.label}`));
    return prompt('Enter numbers to install to (e.g. 1,3) or Enter to skip: ', options).then(answer => {
      return parseSelection(answer, allHarnesses);
    });
  }

  // Multiple detected: confirm selection
  console.log('Detected harnesses:');
  detected.forEach((h, i) => console.log(`  [${i + 1}] ${h.label}`));
  return prompt(`Install to all ${detected.length}? [Y/n/list of numbers]: `, options).then(answer => {
    return parseConfirmation(answer, detected);
  });
}

function parseArgs(argv = process.argv) {
  const options = {
    command: 'install', // install | uninstall | list
    skillName: 'centmem',
    dryRun: false,
    list: false,
    uninstall: false,
    global: false,
    project: false,
    yes: false,
    url: null,
    cwd: process.cwd(),
    home: null,
    help: false
  };

  const args = argv.slice(2);
  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    if (arg === '--help' || arg === '-h') {
      options.help = true;
    } else if (arg === '--list') {
      options.list = true;
      options.command = 'list';
    } else if (arg === '--dry-run') {
      options.dryRun = true;
    } else if (arg === '--uninstall') {
      options.uninstall = true;
      options.command = 'uninstall';
    } else if (arg === '--global') {
      options.global = true;
    } else if (arg === '--project') {
      options.project = true;
    } else if (arg === '--yes' || arg === '-y') {
      options.yes = true;
    } else if (arg === '--skill' && i + 1 < args.length) {
      options.skillName = args[++i];
    } else if (arg === '--cwd' && i + 1 < args.length) {
      options.cwd = path.resolve(args[++i]);
    } else if (arg === '--home' && i + 1 < args.length) {
      options.home = path.resolve(args[++i]);
    } else if (arg === 'add') {
      options.command = 'install';
      if (i + 1 < args.length && !args[i + 1].startsWith('-')) {
        options.url = args[++i];
      }
    } else if (arg === 'install') {
      options.command = 'install';
    } else if (arg === 'list') {
      options.list = true;
      options.command = 'list';
    } else if (arg === 'uninstall') {
      options.uninstall = true;
      options.command = 'uninstall';
    } else if (!arg.startsWith('-') && !options.url) {
      options.url = arg;
    }
  }

  return options;
}

function printHelp() {
  console.log(`@aradenta.labs/centmem-skills — Installer for cent-mem AI agent skills & slash commands

USAGE:
  npx @aradenta.labs/centmem-skills [command] [options]
  npx @aradenta.labs/centmem-skills add [url] [--skill <name>]

COMMANDS:
  install (default)   Install skill to detected AI harnesses
  add <url>           Install skill from repository or source
  uninstall           Remove skill files and injected workflow instructions
  list, --list        Show detected harnesses and install targets

OPTIONS:
  --global            Install to global harness dirs (~/.claude/, ~/.cursor/, etc.)
  --project           Install to project harness dirs (./.claude/, ./.agents/, etc.)
  --yes, -y           Skip interactive prompts, accept defaults
  --skill <name>      Skill name (default: "centmem")
  --dry-run           Show actions without modifying disk
  --list              Display target locations and exit
  --uninstall         Remove installed files and instructions
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
`);
}

function run(argv = process.argv, runOptions = {}) {
  const options = Object.assign(parseArgs(argv), runOptions);

  if (options.help) {
    printHelp();
    return 0;
  }

  const homeDir = getHomeDir(options.home);
  if (!homeDir) {
    console.error('ERROR: Could not determine user home directory.');
    console.error('Set HOME or USERPROFILE, or use --home <path>.');
    return 1;
  }

  const cwd = options.cwd;
  const scope = resolveScope(options);
  const detectedHarnesses = detectHarnesses(homeDir, cwd);
  const skillSrc = getSkillSourceDir();
  const instructionDir = options.global ? homeDir : cwd;
  const detectedFiles = detectInstructionFiles(instructionDir);

  if (options.list || options.command === 'list') {
    console.log(`Skill source: ${skillSrc}`);
    console.log(`Resolved scope: ${scope}${options.global ? ' (forced by --global)' : options.project ? ' (forced by --project)' : isInsideGitRepo(cwd) ? ' (inside Git repo)' : ' (outside Git repo)'}`);
    console.log(`Detected harnesses:`);
    if (detectedHarnesses.length > 0) {
      for (const h of detectedHarnesses) {
        console.log(`  - ${h.label} (${h.id})`);
      }
    } else {
      console.log(`  (none detected)`);
    }

    const previewTargets = detectedHarnesses.length > 0
      ? buildTargets(detectedHarnesses, scope, homeDir, cwd, options.skillName)
      : [];

    console.log(`\nTarget destinations (${options.skillName}):`);
    if (previewTargets.length > 0) {
      for (const target of previewTargets) {
        console.log(`  ${target}`);
      }
    } else {
      console.log(`  (none — no harnesses detected)`);
    }
    console.log(`\nInstruction files in ${options.global ? `user home (${homeDir})` : `project (${cwd})`}:`);
    if (detectedFiles.length > 0) {
      for (const file of detectedFiles) {
        console.log(`  ${file}`);
      }
    } else {
      console.log(`  (none found — will default to ${path.join(instructionDir, 'AGENTS.md')})`);
    }
    return 0;
  }

  if (options.uninstall || options.command === 'uninstall') {
    console.log(`Uninstalling skill '${options.skillName}'...`);
    const targets = getSkillTargets(homeDir, cwd, options.skillName);
    for (const target of targets) {
      if (options.dryRun) {
        console.log(`  [dry-run] would remove -> ${target}`);
      } else {
        if (fs.existsSync(target)) {
          fs.rmSync(target, { recursive: true, force: true });
          console.log(`  removed -> ${target}`);
        } else {
          console.log(`  (not present) ${target}`);
        }
      }
    }

    console.log(`Cleaning injected workflow instructions...`);
    const filesToClean = detectedFiles.length > 0 ? detectedFiles : [path.join(instructionDir, 'AGENTS.md')];
    for (const file of filesToClean) {
      if (fs.existsSync(file)) {
        if (options.dryRun) {
          console.log(`  [dry-run] would clean instructions in -> ${file}`);
        } else {
          const res = removeWorkflowAndCommands(file, false);
          if (res.removed) {
            console.log(`  removed empty instruction file -> ${file}`);
          } else if (res.modified) {
            console.log(`  cleaned instructions in -> ${file}`);
          }
        }
      }
    }

    console.log('Uninstall done.');
    return 0;
  }

  const isTTY = options.isTTY !== undefined
    ? Boolean(options.isTTY)
    : Boolean(process.stdout && process.stdout.isTTY && process.stdin && process.stdin.isTTY);

  function finishInstall(selectedHarnesses, promptShown = false) {
    let targets;
    if (selectedHarnesses === null) {
      targets = getSkillTargets(homeDir, cwd, options.skillName);
    } else if (selectedHarnesses.length === 0) {
      if (options.yes || (!isTTY)) {
        console.error('Warning: No supported AI harnesses detected. Nothing installed.');
        return 0;
      }
      console.log('No harnesses selected. Exiting without installation.');
      return 0;
    } else {
      targets = buildTargets(selectedHarnesses, scope, homeDir, cwd, options.skillName);
    }

    if (!promptShown && detectedHarnesses.length > 0) {
      console.log(`Detected harnesses: ${detectedHarnesses.map(h => h.label).join(', ')}`);
    }
    console.log(`Scope: ${scope}${scope === 'project' && isInsideGitRepo(cwd) ? ' (inside Git repo)' : ''}`);
    console.log(`Installing skill '${options.skillName}' from ${skillSrc}...`);
    let installedCount = 0;
    for (const target of targets) {
      if (options.dryRun) {
        console.log(`  [dry-run] would install -> ${target}`);
      } else {
        copyDirRecursive(skillSrc, target, false);
        console.log(`  installed -> ${target}`);
        installedCount++;
      }
    }

    // Workflow & Slash Command Injection
    console.log(`Injecting cent-mem workflow and /centmem slash command...`);
    if (detectedFiles.length === 0) {
      const defaultFile = path.join(instructionDir, 'AGENTS.md');
      if (options.dryRun) {
        console.log(`  [dry-run] would create ${defaultFile} with workflow and /centmem command`);
      } else {
        injectWorkflowAndCommands(defaultFile, false);
        console.log(`  created and injected -> ${defaultFile}`);
      }
    } else {
      for (const file of detectedFiles) {
        if (options.dryRun) {
          console.log(`  [dry-run] would inject workflow and /centmem command into -> ${file}`);
        } else {
          const { created } = injectWorkflowAndCommands(file, false);
          console.log(`  ${created ? 'created and injected' : 'injected'} -> ${file}`);
        }
      }
    }

    if (options.dryRun) {
      console.log('\nDry run complete (no files were changed).');
      return 0;
    }

    console.log(`\nDone. Installed skill to ${installedCount} location(s) and configured workflow.`);
    console.log('Restart your AI agent or editor to pick up the new skill and /centmem slash command.');
    return 0;
  }

  // Non-TTY / CI mode or --yes flag
  if (!isTTY || options.yes) {
    if (detectedHarnesses.length > 0) {
      return finishInstall(detectedHarnesses, false);
    }
    if (options.yes) {
      console.error('Warning: No supported AI harnesses detected. Nothing installed.');
      return 0;
    }
    // Silent default fallback when non-TTY, no harnesses detected, and no --yes flag
    return finishInstall(null, false);
  }

  // Interactive mode (TTY)
  const promptShown = detectedHarnesses.length !== 1;
  const selectionResult = promptHarnessSelection(detectedHarnesses, HARNESSES, options);
  if (selectionResult instanceof Promise) {
    return selectionResult.then(selected => finishInstall(selected, promptShown));
  }
  return finishInstall(selectionResult, promptShown);
}

if (require.main === module) {
  const result = run();
  if (result instanceof Promise) {
    result.then(code => process.exit(code || 0)).catch(err => {
      console.error(err);
      process.exit(1);
    });
  } else {
    process.exit(result || 0);
  }
}

module.exports = {
  WORKFLOW_BLOCK,
  COMMAND_BLOCK,
  CANDIDATE_INSTRUCTION_FILES,
  HARNESSES,
  detectHarnesses,
  isInsideGitRepo,
  resolveScope,
  buildTargets,
  promptHarnessSelection,
  parseSelection,
  parseConfirmation,
  getHomeDir,
  getSkillSourceDir,
  getSkillTargets,
  detectInstructionFiles,
  injectSection,
  removeSection,
  injectWorkflowAndCommands,
  removeWorkflowAndCommands,
  parseArgs,
  printHelp,
  run
};
