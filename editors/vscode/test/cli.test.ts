import * as assert from 'assert';
import { CentmemCLI, buildRecallArgs, buildPutArgs, parseCLIError } from '../src/cli';
import { MemoryItem, RecallResult } from '../src/types';

function testTypesAndShapes() {
  const item: MemoryItem = {
    id: 1,
    scope: 'project:cent-mem',
    type: 'note',
    content: 'test note content',
    score: 0.95,
    tags: ['code', 'test'],
  };

  assert.strictEqual(item.id, 1);
  assert.strictEqual(item.type, 'note');

  const result: RecallResult = {
    ok: true,
    query: 'test',
    results: [item],
  };

  assert.strictEqual(result.ok, true);
  assert.strictEqual(result.results.length, 1);
  assert.strictEqual(result.results[0].content, 'test note content');
  console.log('✔ Types and shapes tests passed.');
}

function testBuildRecallArgs() {
  const basic = buildRecallArgs('vector search');
  assert.deepStrictEqual(basic, ['recall', 'vector search']);

  const full = buildRecallArgs('vector search', {
    scope: 'project:cent-mem',
    top: 10,
    type: 'note',
    tags: ['ai', 'memory'],
  });
  assert.deepStrictEqual(full, [
    'recall',
    'vector search',
    '--scope',
    'project:cent-mem',
    '--top',
    '10',
    '--type',
    'note',
    '--tags',
    'ai,memory',
  ]);
  console.log('✔ buildRecallArgs tests passed.');
}

function testBuildPutArgs() {
  const basic = buildPutArgs('sqlite-vec chosen');
  assert.deepStrictEqual(basic, [
    'put',
    '--content',
    'sqlite-vec chosen',
    '--scope',
    'global',
    '--type',
    'note',
    '--tags',
    'code',
  ]);

  const custom = buildPutArgs('pipeline restarted', {
    scope: 'project:cent-mem',
    type: 'log',
    tags: ['deploy', 'infra'],
  });
  assert.deepStrictEqual(custom, [
    'put',
    '--content',
    'pipeline restarted',
    '--scope',
    'project:cent-mem',
    '--type',
    'log',
    '--tags',
    'deploy,infra',
  ]);
  console.log('✔ buildPutArgs tests passed.');
}

function testParseCLIError() {
  const jsonErr = JSON.stringify({
    ok: false,
    error: {
      code: 'ERR_INVALID_FLAG',
      message: 'Unknown flag --invalid',
    },
  });
  const err1 = parseCLIError(jsonErr, 'fallback');
  assert.strictEqual(err1.message, 'Unknown flag --invalid');

  const rawErr = 'Fatal error: database locked\n';
  const err2 = parseCLIError(rawErr, 'fallback');
  assert.strictEqual(err2.message, 'Fatal error: database locked');

  const emptyErr = parseCLIError('', 'default fallback error');
  assert.strictEqual(emptyErr.message, 'default fallback error');
  console.log('✔ parseCLIError tests passed.');
}

function testBinaryPathDefault() {
  const bin = CentmemCLI.getBinaryPath();
  assert.strictEqual(bin, 'centmem');
  console.log('✔ CentmemCLI defaults passed.');
}

testTypesAndShapes();
testBuildRecallArgs();
testBuildPutArgs();
testParseCLIError();
testBinaryPathDefault();
console.log('✔ All VS Code extension unit tests passed.');

