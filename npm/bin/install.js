#!/usr/bin/env node
/**
 * centmem-skills installer
 * Cross-platform installer for cent-mem shared memory skills and slash commands.
 * Zero-dependency: works with pure Node.js built-in modules.
 */

const fs = require('fs');
const path = require('path');
const os = require('os');

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
- **Check Health**: \`centmem doctor\`
<!-- centmem:end -->`;

const COMMAND_BLOCK = `<!-- centmem-command:start -->
## /centmem Command
When the user types \`/centmem <input>\`, act as the Memory Manager. Analyze the intent:
- **Recall / Context**: If asking a question or looking for context, run \`centmem recall "<input>" --scope "project:$CENTMEM_PROJ" --top 5\` or \`centmem timeline\`.
- **Save Decisions / Facts**: If stating a decision, convention, preference, or learning to save, run \`centmem put --scope "project:$CENTMEM_PROJ" --type note --content "<input>"\` or \`centmem set --scope "project:$CENTMEM_PROJ" --key "<key>" --value '<json>'\`.
- **Maintenance / Health**: If requesting maintenance, health checks, or statistics, run \`centmem doctor\`, \`centmem stats\`, or \`centmem compact\`.

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

function getSkillTargets(homeDir, cwd, skillName = 'centmem') {
  const targets = [
    // Standard agent skills directories (project-level & user-level)
    path.join(cwd, '.agents', 'skills', skillName),
    path.join(homeDir, '.agents', 'skills', skillName),

    // Claude Code skills
    path.join(cwd, '.claude', 'skills', skillName),
    path.join(homeDir, '.claude', 'skills', skillName),

    // Harness-specific locations
    path.join(homeDir, '.cursor', 'rules', skillName),
    path.join(homeDir, '.codex', 'skills', skillName),
    path.join(homeDir, '.config', 'skills', skillName),
    path.join(homeDir, '.gemini', 'antigravity', 'skills', skillName),
    path.join(homeDir, '.trae', 'skills', skillName),
    path.join(homeDir, '.hermes', 'skills', skillName),
    path.join(homeDir, '.deepseek', 'skills', skillName)
  ];

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

function parseArgs(argv) {
  const options = {
    command: 'install', // install | uninstall | list
    skillName: 'centmem',
    dryRun: false,
    list: false,
    uninstall: false,
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
  install (default)  Install skill & inject workflow and /centmem command
  add <url>          Install skill from repository or source
  uninstall          Remove skill files and injected workflow instructions
  list, --list       List install targets and detected instruction files

OPTIONS:
  --skill <name>     Name of the skill (default: "centmem")
  --dry-run          Simulate actions without modifying disk
  --list             Display target locations and exit
  --uninstall        Remove installed files and instructions
  --cwd <path>       Override target working directory
  --home <path>      Override home directory
  -h, --help         Show this help message

EXAMPLES:
  npx @aradenta.labs/centmem-skills
  npx @aradenta.labs/centmem-skills add https://github.com/aradenta-labs/cent-mem --skill centmem
  npx @aradenta.labs/centmem-skills --dry-run
  npx @aradenta.labs/centmem-skills --uninstall
`);
}

function run(argv = process.argv) {
  const options = parseArgs(argv);

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
  const skillSrc = getSkillSourceDir();
  const targets = getSkillTargets(homeDir, cwd, options.skillName);
  let detectedFiles = detectInstructionFiles(cwd);

  if (options.list || options.command === 'list') {
    console.log(`Skill source: ${skillSrc}`);
    console.log(`Target destinations (${options.skillName}):`);
    for (const target of targets) {
      console.log(`  ${target}`);
    }
    console.log(`\nInstruction files in project (${cwd}):`);
    if (detectedFiles.length > 0) {
      for (const file of detectedFiles) {
        console.log(`  ${file}`);
      }
    } else {
      console.log(`  (none found — will default to ${path.join(cwd, 'AGENTS.md')})`);
    }
    return 0;
  }

  if (options.uninstall || options.command === 'uninstall') {
    console.log(`Uninstalling skill '${options.skillName}'...`);
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
    const filesToClean = detectedFiles.length > 0 ? detectedFiles : [path.join(cwd, 'AGENTS.md')];
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

  // Install mode
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
    const defaultFile = path.join(cwd, 'AGENTS.md');
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

if (require.main === module) {
  const exitCode = run();
  process.exit(exitCode);
}

module.exports = {
  WORKFLOW_BLOCK,
  COMMAND_BLOCK,
  CANDIDATE_INSTRUCTION_FILES,
  getHomeDir,
  getSkillSourceDir,
  getSkillTargets,
  detectInstructionFiles,
  injectSection,
  removeSection,
  injectWorkflowAndCommands,
  removeWorkflowAndCommands,
  parseArgs,
  run
};
