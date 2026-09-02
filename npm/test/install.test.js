const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');

const {
  WORKFLOW_BLOCK,
  COMMAND_BLOCK,
  CANDIDATE_INSTRUCTION_FILES,
  getHomeDir,
  getSkillTargets,
  detectInstructionFiles,
  injectSection,
  removeSection,
  injectWorkflowAndCommands,
  removeWorkflowAndCommands,
  parseArgs,
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
