const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');

const {
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
  getSkillTargets,
  detectInstructionFiles,
  injectSection,
  removeSection,
  injectWorkflowAndCommands,
  removeWorkflowAndCommands,
  parseArgs,
  printHelp,
  run
} = require('../bin/install.js');

function createTempDir(prefix = 'centmem-test-') {
  return fs.mkdtempSync(path.join(os.tmpdir(), prefix));
}

test('parseArgs correctly parses flags and commands', () => {
  assert.strictEqual(parseArgs(['node', 'install.js']).command, 'install');
  assert.strictEqual(parseArgs(['node', 'install.js', '--list']).list, true);
  assert.strictEqual(parseArgs(['node', 'install.js', '--dry-run']).dryRun, true);
  assert.strictEqual(parseArgs(['node', 'install.js', '--uninstall']).uninstall, true);

  const customSkill = parseArgs(['node', 'install.js', '--skill', 'custom-mem']);
  assert.strictEqual(customSkill.skillName, 'custom-mem');

  const addCmd = parseArgs(['node', 'install.js', 'add', 'https://github.com/aradenta-labs/cent-mem', '--skill', 'centmem']);
  assert.strictEqual(addCmd.command, 'install');
  assert.strictEqual(addCmd.url, 'https://github.com/aradenta-labs/cent-mem');
  assert.strictEqual(addCmd.skillName, 'centmem');

  // v1.4.5 new flags
  const globalOpts = parseArgs(['node', 'install.js', '--global']);
  assert.strictEqual(globalOpts.global, true);

  const projectOpts = parseArgs(['node', 'install.js', '--project']);
  assert.strictEqual(projectOpts.project, true);

  const yesOpts = parseArgs(['node', 'install.js', '--yes']);
  assert.strictEqual(yesOpts.yes, true);

  const yShortOpts = parseArgs(['node', 'install.js', '-y']);
  assert.strictEqual(yShortOpts.yes, true);
});

test('injectSection appends when tag is missing and replaces when tag exists', () => {
  const startTag = '<!-- test:start -->';
  const endTag = '<!-- test:end -->';
  const block = `${startTag}\nHello World\n${endTag}`;

  // Case 1: Empty initial content
  const res1 = injectSection('', startTag, endTag, block);
  assert.strictEqual(res1.trim(), block);

  // Case 2: Append to existing text
  const initial = '# My Project\nSome description.';
  const res2 = injectSection(initial, startTag, endTag, block);
  assert(res2.includes('# My Project'));
  assert(res2.includes(block));

  // Case 3: Replace existing block idempotently
  const modifiedBlock = `${startTag}\nUpdated Content\n${endTag}`;
  const res3 = injectSection(res2, startTag, endTag, modifiedBlock);
  assert.strictEqual((res3.match(/<!-- test:start -->/g) || []).length, 1);
  assert(res3.includes('Updated Content'));
  assert(!res3.includes('Hello World'));
});

test('removeSection removes tagged block cleanly', () => {
  const startTag = '<!-- test:start -->';
  const endTag = '<!-- test:end -->';
  const content = `# Title\n\n${startTag}\nContent to remove\n${endTag}\n\nFooter note.`;

  const cleaned = removeSection(content, startTag, endTag);
  assert(!cleaned.includes('Content to remove'));
  assert(cleaned.includes('# Title'));
  assert(cleaned.includes('Footer note.'));
});

test('detectInstructionFiles finds present instruction files', () => {
  const tmpDir = createTempDir();
  try {
    fs.writeFileSync(path.join(tmpDir, 'AGENTS.md'), '# Agents');
    fs.writeFileSync(path.join(tmpDir, '.cursorrules'), '# Cursor rules');

    const found = detectInstructionFiles(tmpDir);
    assert.strictEqual(found.length, 2);
    assert(found.some(f => f.endsWith('AGENTS.md')));
    assert(found.some(f => f.endsWith('.cursorrules')));
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
});

test('injectWorkflowAndCommands creates AGENTS.md if missing', () => {
  const tmpDir = createTempDir();
  try {
    const agentsPath = path.join(tmpDir, 'AGENTS.md');
    const res = injectWorkflowAndCommands(agentsPath);
    assert.strictEqual(res.created, true);
    assert(fs.existsSync(agentsPath));

    const content = fs.readFileSync(agentsPath, 'utf8');
    assert(content.includes('## cent-mem Workflow'));
    assert(content.includes('## /centmem Command'));
    assert(content.includes('<!-- centmem:start -->'));
    assert(content.includes('<!-- centmem-command:start -->'));
    assert(content.includes('Ingest / Capture'));
    assert(content.includes('centmem capture git'));
    assert(content.includes('Manage Relationships'));
    assert(content.includes('Relationships / Links'));
    assert(content.includes('centmem link'));
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
});

test('injectWorkflowAndCommands preserves existing content and is idempotent', () => {
  const tmpDir = createTempDir();
  try {
    const cursorrulesPath = path.join(tmpDir, '.cursorrules');
    fs.writeFileSync(cursorrulesPath, '# Existing rules\nRule 1: Be fast.\n');

    // First injection
    injectWorkflowAndCommands(cursorrulesPath);
    const firstContent = fs.readFileSync(cursorrulesPath, 'utf8');
    assert(firstContent.startsWith('# Existing rules'));
    assert(firstContent.includes('## cent-mem Workflow'));
    assert(firstContent.includes('## /centmem Command'));

    // Second injection (idempotency check)
    injectWorkflowAndCommands(cursorrulesPath);
    const secondContent = fs.readFileSync(cursorrulesPath, 'utf8');
    assert.strictEqual(firstContent, secondContent);
    assert.strictEqual((secondContent.match(/<!-- centmem:start -->/g) || []).length, 1);
    assert.strictEqual((secondContent.match(/<!-- centmem-command:start -->/g) || []).length, 1);
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
});

test('detectInstructionFiles always includes AGENTS.md even if missing', () => {
  const tmpDir = createTempDir();
  try {
    fs.writeFileSync(path.join(tmpDir, '.cursorrules'), '# Cursor rules');

    const found = detectInstructionFiles(tmpDir);
    assert.strictEqual(found.length, 2);
    assert(found.some(f => f.endsWith('AGENTS.md')));
    assert(found.some(f => f.endsWith('.cursorrules')));
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
});

test('getSkillTargets includes .agents/skills in project cwd and user home', () => {
  const tmpHome = createTempDir('home-');
  const tmpCwd = createTempDir('cwd-');
  try {
    const targets = getSkillTargets(tmpHome, tmpCwd, 'centmem');
    assert(targets.includes(path.join(tmpCwd, '.agents', 'skills', 'centmem')));
    assert(targets.includes(path.join(tmpHome, '.agents', 'skills', 'centmem')));
  } finally {
    fs.rmSync(tmpHome, { recursive: true, force: true });
    fs.rmSync(tmpCwd, { recursive: true, force: true });
  }
});

test('Full E2E install and uninstall workflow', () => {
  const tmpHome = createTempDir('centmem-home-');
  const tmpCwd = createTempDir('centmem-cwd-');

  try {
    // 1. Run install in fresh directory
    const exitCode = run(['node', 'install.js', '--home', tmpHome, '--cwd', tmpCwd]);
    assert.strictEqual(exitCode, 0);

    // Verify skill targets exist
    const targets = getSkillTargets(tmpHome, tmpCwd, 'centmem');
    for (const target of targets) {
      assert(fs.existsSync(path.join(target, 'SKILL.md')), `Missing SKILL.md at ${target}`);
    }

    // Verify detailed skill artifacts in project .agents/skills/centmem
    const agentsSkillDir = path.join(tmpCwd, '.agents', 'skills', 'centmem');
    assert(fs.existsSync(path.join(agentsSkillDir, 'SKILL.md')), 'Missing SKILL.md in .agents/skills/centmem');
    assert(fs.existsSync(path.join(agentsSkillDir, 'references', 'cli-commands.md')), 'Missing cli-commands.md');
    assert(fs.existsSync(path.join(agentsSkillDir, 'references', 'architecture-and-scoping.md')), 'Missing architecture-and-scoping.md');
    assert(fs.existsSync(path.join(agentsSkillDir, 'references', 'capture-hooks.md')), 'Missing capture-hooks.md');
    assert(fs.existsSync(path.join(agentsSkillDir, 'examples', 'agent-workflow-examples.md')), 'Missing agent-workflow-examples.md');
    assert(fs.existsSync(path.join(agentsSkillDir, 'examples', 'recipes.sh')), 'Missing recipes.sh');
    assert(fs.existsSync(path.join(agentsSkillDir, 'scripts', 'centmem-helper.sh')), 'Missing centmem-helper.sh');

    // Verify AGENTS.md was created in project root with enriched instructions
    const agentsPath = path.join(tmpCwd, 'AGENTS.md');
    assert(fs.existsSync(agentsPath));
    const agentsContent = fs.readFileSync(agentsPath, 'utf8');
    assert(agentsContent.includes('## cent-mem Workflow'));
    assert(agentsContent.includes('.agents/skills/centmem/'));
    assert(agentsContent.includes('## /centmem Command'));
    assert(agentsContent.includes('centmem recall'));
    assert(agentsContent.includes('Ingest / Capture'));
    assert(agentsContent.includes('centmem capture git'));
    assert(agentsContent.includes('Manage Relationships'));
    assert(agentsContent.includes('Relationships / Links'));
    assert(agentsContent.includes('centmem link'));

    // 2. Run uninstall
    const uninstExit = run(['node', 'install.js', 'uninstall', '--home', tmpHome, '--cwd', tmpCwd]);
    assert.strictEqual(uninstExit, 0);

    // Verify skill targets are removed
    for (const target of targets) {
      assert(!fs.existsSync(target), `Target directory was not removed: ${target}`);
    }
  } finally {
    fs.rmSync(tmpHome, { recursive: true, force: true });
    fs.rmSync(tmpCwd, { recursive: true, force: true });
  }
});

// =========================================================================
// v1.4.5 New Unit Tests
// =========================================================================

test('detectHarnesses_returnsOnlyPresentDirs', () => {
  const tmpHome = createTempDir('home-detect-');
  const tmpCwd = createTempDir('cwd-detect-');
  try {
    fs.mkdirSync(path.join(tmpHome, '.cursor'), { recursive: true });
    fs.mkdirSync(path.join(tmpHome, '.gemini', 'antigravity'), { recursive: true });

    const detected = detectHarnesses(tmpHome, tmpCwd);
    const ids = detected.map(h => h.id);
    assert.strictEqual(detected.length, 2);
    assert(ids.includes('cursor'));
    assert(ids.includes('antigravity'));
    assert(!ids.includes('claude'));
  } finally {
    fs.rmSync(tmpHome, { recursive: true, force: true });
    fs.rmSync(tmpCwd, { recursive: true, force: true });
  }
});

test('detectHarnesses_returnsEmptyWhenNoneFound', () => {
  const tmpHome = createTempDir('home-empty-');
  const tmpCwd = createTempDir('cwd-empty-');
  try {
    const detected = detectHarnesses(tmpHome, tmpCwd);
    assert.strictEqual(detected.length, 0);
    assert.deepStrictEqual(detected, []);
  } finally {
    fs.rmSync(tmpHome, { recursive: true, force: true });
    fs.rmSync(tmpCwd, { recursive: true, force: true });
  }
});

test('resolveScope_projectWhenInsideGitRepo', () => {
  const tmpGitRoot = createTempDir('git-root-');
  try {
    fs.mkdirSync(path.join(tmpGitRoot, '.git'), { recursive: true });
    const subDir = path.join(tmpGitRoot, 'packages', 'app');
    fs.mkdirSync(subDir, { recursive: true });

    assert.strictEqual(isInsideGitRepo(subDir), true);
    assert.strictEqual(resolveScope({ cwd: subDir }), 'project');
  } finally {
    fs.rmSync(tmpGitRoot, { recursive: true, force: true });
  }
});

test('resolveScope_globalWhenOutsideGitRepo', () => {
  const tmpDir = createTempDir('non-git-');
  try {
    assert.strictEqual(isInsideGitRepo(tmpDir), false);
    assert.strictEqual(resolveScope({ cwd: tmpDir }), 'global');
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
});

test('resolveScope_globalFlagOverridesGitRepo', () => {
  const tmpGitRoot = createTempDir('git-root-');
  try {
    fs.mkdirSync(path.join(tmpGitRoot, '.git'), { recursive: true });
    assert.strictEqual(resolveScope({ cwd: tmpGitRoot, global: true }), 'global');
  } finally {
    fs.rmSync(tmpGitRoot, { recursive: true, force: true });
  }
});

test('resolveScope_projectFlagOverridesOutsideRepo', () => {
  const tmpDir = createTempDir('non-git-');
  try {
    assert.strictEqual(resolveScope({ cwd: tmpDir, project: true }), 'project');
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
});

test('buildTargets_projectScope_usesProjectPaths', () => {
  const tmpHome = createTempDir('home-targets-');
  const tmpCwd = createTempDir('cwd-targets-');
  try {
    const claudeHarness = HARNESSES.find(h => h.id === 'claude');
    const targets = buildTargets([claudeHarness], 'project', tmpHome, tmpCwd, 'centmem');

    assert(targets.includes(path.join(tmpCwd, '.claude', 'skills', 'centmem')));
    assert(targets.includes(path.join(tmpCwd, '.agents', 'skills', 'centmem')));
    assert(!targets.includes(path.join(tmpHome, '.claude', 'skills', 'centmem')));
  } finally {
    fs.rmSync(tmpHome, { recursive: true, force: true });
    fs.rmSync(tmpCwd, { recursive: true, force: true });
  }
});

test('buildTargets_globalScope_usesHomePaths', () => {
  const tmpHome = createTempDir('home-targets-');
  const tmpCwd = createTempDir('cwd-targets-');
  try {
    const claudeHarness = HARNESSES.find(h => h.id === 'claude');
    const cursorHarness = HARNESSES.find(h => h.id === 'cursor');
    const targets = buildTargets([claudeHarness, cursorHarness], 'global', tmpHome, tmpCwd, 'centmem');

    assert(targets.includes(path.join(tmpHome, '.claude', 'skills', 'centmem')));
    assert(targets.includes(path.join(tmpHome, '.cursor', 'rules', 'centmem')));
    assert(!targets.includes(path.join(tmpCwd, '.claude', 'skills', 'centmem')));
  } finally {
    fs.rmSync(tmpHome, { recursive: true, force: true });
    fs.rmSync(tmpCwd, { recursive: true, force: true });
  }
});

test('promptHarnessSelection_nonTTY_returnsDetected', async () => {
  const claudeHarness = HARNESSES.find(h => h.id === 'claude');
  const cursorHarness = HARNESSES.find(h => h.id === 'cursor');
  const detected = [claudeHarness, cursorHarness];

  // In test environment, process.stdout.isTTY is falsy (non-TTY)
  const result = await promptHarnessSelection(detected, HARNESSES, {});
  assert.deepStrictEqual(result, detected);
});

test('promptHarnessSelection_singleHarness_returnsWithoutPrompt', async () => {
  const antigravityHarness = HARNESSES.find(h => h.id === 'antigravity');
  const detected = [antigravityHarness];

  const result = await promptHarnessSelection(detected, HARNESSES, {});
  assert.deepStrictEqual(result, detected);
});

test('parseSelection and parseConfirmation correctly handle interactive answers', () => {
  const h1 = { id: 'one' };
  const h2 = { id: 'two' };
  const list = [h1, h2];

  // parseSelection
  assert.deepStrictEqual(parseSelection('', list), []);
  assert.deepStrictEqual(parseSelection('   ', list), []);
  assert.deepStrictEqual(parseSelection('abc', list), []);
  assert.deepStrictEqual(parseSelection('1', list), [h1]);
  assert.deepStrictEqual(parseSelection('2', list), [h2]);
  assert.deepStrictEqual(parseSelection('1, 2', list), [h1, h2]);
  assert.deepStrictEqual(parseSelection('1 2', list), [h1, h2]);
  assert.deepStrictEqual(parseSelection('1,1,2', list), [h1, h2]);
  assert.deepStrictEqual(parseSelection('0, 3', list), []);

  // parseConfirmation
  assert.deepStrictEqual(parseConfirmation('', list), list);
  assert.deepStrictEqual(parseConfirmation('y', list), list);
  assert.deepStrictEqual(parseConfirmation('yes', list), list);
  assert.deepStrictEqual(parseConfirmation('YES', list), list);
  assert.deepStrictEqual(parseConfirmation('n', list), []);
  assert.deepStrictEqual(parseConfirmation('no', list), []);
  assert.deepStrictEqual(parseConfirmation('2', list), [h2]);
});

test('run with --list, --help, and --dry-run exits 0', () => {
  const tmpHome = createTempDir('home-cli-');
  const tmpCwd = createTempDir('cwd-cli-');
  try {
    assert.strictEqual(run(['node', 'install.js', '--help']), 0);
    assert.strictEqual(run(['node', 'install.js', '--list', '--home', tmpHome, '--cwd', tmpCwd]), 0);
    assert.strictEqual(run(['node', 'install.js', '--dry-run', '--home', tmpHome, '--cwd', tmpCwd]), 0);
  } finally {
    fs.rmSync(tmpHome, { recursive: true, force: true });
    fs.rmSync(tmpCwd, { recursive: true, force: true });
  }
});

test('run with --global --yes exits 0 with warning when no harnesses detected', () => {
  const tmpHome = createTempDir('home-empty-');
  const tmpCwd = createTempDir('cwd-empty-');
  try {
    const exitCode = run(['node', 'install.js', '--global', '--yes', '--home', tmpHome, '--cwd', tmpCwd]);
    assert.strictEqual(exitCode, 0);
  } finally {
    fs.rmSync(tmpHome, { recursive: true, force: true });
    fs.rmSync(tmpCwd, { recursive: true, force: true });
  }
});

test('run installs only to detected harnesses when harness exists', () => {
  const tmpHome = createTempDir('home-single-');
  const tmpCwd = createTempDir('cwd-single-');
  try {
    // Simulate only cursor installed
    fs.mkdirSync(path.join(tmpHome, '.cursor'), { recursive: true });

    const exitCode = run(['node', 'install.js', '--home', tmpHome, '--cwd', tmpCwd]);
    assert.strictEqual(exitCode, 0);

    // Cursor rule exists
    assert(fs.existsSync(path.join(tmpHome, '.cursor', 'rules', 'centmem', 'SKILL.md')));

    // Claude and Trae should NOT be installed
    assert(!fs.existsSync(path.join(tmpHome, '.claude', 'skills', 'centmem')));
    assert(!fs.existsSync(path.join(tmpHome, '.trae', 'skills', 'centmem')));
  } finally {
    fs.rmSync(tmpHome, { recursive: true, force: true });
    fs.rmSync(tmpCwd, { recursive: true, force: true });
  }
});

test('run with --global installs workflow and commands to homeDir/AGENTS.md and uninstalls cleanly', () => {
  const tmpHome = createTempDir('home-global-');
  const tmpCwd = createTempDir('cwd-global-');
  try {
    fs.mkdirSync(path.join(tmpHome, '.gemini', 'antigravity'), { recursive: true });

    // Install with --global
    const exitCode = run(['node', 'install.js', '--global', '--home', tmpHome, '--cwd', tmpCwd]);
    assert.strictEqual(exitCode, 0);

    // AGENTS.md must be created in homeDir, NOT cwd
    assert(fs.existsSync(path.join(tmpHome, 'AGENTS.md')), 'AGENTS.md should exist in homeDir for --global');
    assert(!fs.existsSync(path.join(tmpCwd, 'AGENTS.md')), 'AGENTS.md should NOT exist in cwd for --global');

    const homeContent = fs.readFileSync(path.join(tmpHome, 'AGENTS.md'), 'utf8');
    assert(homeContent.includes('## cent-mem Workflow'));

    // Uninstall with --global
    const uninstExit = run(['node', 'install.js', 'uninstall', '--global', '--home', tmpHome, '--cwd', tmpCwd]);
    assert.strictEqual(uninstExit, 0);
    assert(!fs.existsSync(path.join(tmpHome, 'AGENTS.md')), 'Empty AGENTS.md should be cleaned up from homeDir');
  } finally {
    fs.rmSync(tmpHome, { recursive: true, force: true });
    fs.rmSync(tmpCwd, { recursive: true, force: true });
  }
});

test('promptHarnessSelection interactive selection with readline streams', async () => {
  const { Readable, Writable } = require('node:stream');

  const antigravityHarness = HARNESSES.find(h => h.id === 'antigravity');
  const claudeHarness = HARNESSES.find(h => h.id === 'claude');
  const cursorHarness = HARNESSES.find(h => h.id === 'cursor');
  const detected = [antigravityHarness, claudeHarness, cursorHarness];

  // Helper to create mocked readline I/O streams
  function makeMockStreams(inputText) {
    const input = new Readable({
      read() {
        this.push(inputText);
        this.push(null);
      }
    });
    const output = new Writable({
      write(chunk, encoding, callback) {
        callback();
      }
    });
    return { input, output, isTTY: true };
  }

  // 1. User picks '1,2' from detected
  const res1 = await promptHarnessSelection(detected, HARNESSES, makeMockStreams('1,2\n'));
  assert.strictEqual(res1.length, 2);
  assert.strictEqual(res1[0].id, 'antigravity');
  assert.strictEqual(res1[1].id, 'claude');

  // 2. User answers 'n' to reject all
  const res2 = await promptHarnessSelection(detected, HARNESSES, makeMockStreams('n\n'));
  assert.deepStrictEqual(res2, []);

  // 3. User answers 'y' to accept all
  const res3 = await promptHarnessSelection(detected, HARNESSES, makeMockStreams('y\n'));
  assert.strictEqual(res3.length, 3);

  // 4. No harnesses detected: user picks '1,3' from all available harnesses
  const res4 = await promptHarnessSelection([], HARNESSES, makeMockStreams('1,3\n'));
  assert.strictEqual(res4.length, 2);
  assert.strictEqual(res4[0].id, HARNESSES[0].id);
  assert.strictEqual(res4[1].id, HARNESSES[2].id);

  // 5. No harnesses detected: user presses Enter to skip
  const res5 = await promptHarnessSelection([], HARNESSES, makeMockStreams('\n'));
  assert.deepStrictEqual(res5, []);
});

test('run end-to-end interactive TTY disambiguation where user selects 1 of 2 detected harnesses', async () => {
  const { Readable, Writable } = require('node:stream');

  const tmpHome = createTempDir('home-interactive-');
  const tmpCwd = createTempDir('cwd-interactive-');
  try {
    // Simulate both cursor and antigravity present
    fs.mkdirSync(path.join(tmpHome, '.cursor'), { recursive: true });
    fs.mkdirSync(path.join(tmpHome, '.gemini', 'antigravity'), { recursive: true });

    // Stream inputs '2\n' (Cursor is item 2 after Antigravity)
    const input = new Readable({
      read() {
        this.push('2\n');
        this.push(null);
      }
    });
    const output = new Writable({
      write(chunk, encoding, callback) {
        callback();
      }
    });

    const exitCode = await run(
      ['node', 'install.js', '--home', tmpHome, '--cwd', tmpCwd],
      { isTTY: true, input, output }
    );
    assert.strictEqual(exitCode, 0);

    // Cursor rules should be installed (item 2)
    assert(fs.existsSync(path.join(tmpHome, '.cursor', 'rules', 'centmem', 'SKILL.md')));

    // Antigravity should NOT be installed (not chosen)
    assert(!fs.existsSync(path.join(tmpHome, '.gemini', 'antigravity', 'skills', 'centmem')));
  } finally {
    fs.rmSync(tmpHome, { recursive: true, force: true });
    fs.rmSync(tmpCwd, { recursive: true, force: true });
  }
});

