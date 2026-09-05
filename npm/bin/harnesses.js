/**
 * centmem-skills harness registry
 * Declarative registry mapping supported AI harnesses to detection signals
 * and installation target directories.
 */

const fs = require('fs');
const path = require('path');

const HARNESSES = [
  {
    id: 'antigravity',
    label: 'Antigravity (AGY)',
    detect: (home, cwd) => home ? fs.existsSync(path.join(home, '.gemini', 'antigravity')) : false,
    targets: {
      global: home => path.join(home, '.gemini', 'antigravity', 'skills')
    }
  },
  {
    id: 'claude',
    label: 'Claude Code',
    detect: (home, cwd) => (home && fs.existsSync(path.join(home, '.claude'))) || (cwd && fs.existsSync(path.join(cwd, '.claude'))),
    targets: {
      global: home => path.join(home, '.claude', 'skills'),
      project: cwd => path.join(cwd, '.claude', 'skills')
    }
  },
  {
    id: 'cursor',
    label: 'Cursor',
    detect: (home, cwd) => home ? fs.existsSync(path.join(home, '.cursor')) : false,
    targets: {
      global: home => path.join(home, '.cursor', 'rules')
    }
  },
  {
    id: 'codex',
    label: 'OpenAI Codex',
    detect: (home, cwd) => home ? fs.existsSync(path.join(home, '.codex')) : false,
    targets: {
      global: home => path.join(home, '.codex', 'skills')
    }
  },
  {
    id: 'agents',
    label: 'Generic (.agents/skills)',
    detect: (home, cwd) =>
      (home && fs.existsSync(path.join(home, '.agents'))) ||
      (cwd && fs.existsSync(path.join(cwd, '.agents'))),
    targets: {
      global: home => path.join(home, '.agents', 'skills'),
      project: cwd => path.join(cwd, '.agents', 'skills')
    }
  },
  {
    id: 'trae',
    label: 'Trae',
    detect: (home, cwd) => home ? fs.existsSync(path.join(home, '.trae')) : false,
    targets: {
      global: home => path.join(home, '.trae', 'skills')
    }
  },
  {
    id: 'hermes',
    label: 'Hermes',
    detect: (home, cwd) => home ? fs.existsSync(path.join(home, '.hermes')) : false,
    targets: {
      global: home => path.join(home, '.hermes', 'skills')
    }
  }
];

function detectHarnesses(home, cwd) {
  if (!home && !cwd) return [];
  return HARNESSES.filter(h => {
    try {
      return Boolean(h.detect && h.detect(home, cwd));
    } catch (_) {
      return false;
    }
  });
}

module.exports = {
  HARNESSES,
  detectHarnesses
};
