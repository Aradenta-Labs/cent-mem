import test from 'node:test';
import assert from 'node:assert/strict';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { buildSync } from '../node_modules/esbuild/lib/main.js';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const apiEntry = path.resolve(__dirname, '../src/services/api.ts');

const bundled = buildSync({
  entryPoints: [apiEntry],
  bundle: true,
  format: 'esm',
  write: false,
});

const apiMod = await import(
  'data:text/javascript;base64,' + Buffer.from(bundled.outputFiles[0].text).toString('base64')
);

const { streamBatchProposals, streamBatchSuggestedLinks } = apiMod;

function makeSSEStream(chunks) {
  const enc = new TextEncoder();
  return new ReadableStream({
    start(controller) {
      for (const ch of chunks) {
        controller.enqueue(enc.encode(ch));
      }
      controller.close();
    },
  });
}

test('streamBatchProposals parses split-chunk SSE frames with mixed errors', async () => {
  const progressEvents = [];
  const origFetch = globalThis.fetch;
  globalThis.fetch = async (url, init) => {
    assert.equal(url, '/api/proposals/batch/stream');
    assert.equal(init.method, 'POST');
    const body = JSON.parse(init.body);
    assert.deepEqual(body, { action: 'apply', ids: [10, 20, 30] });

    // Split JSON and CRLF lines across arbitrary chunk boundaries
    const stream = makeSSEStream([
      'event: start\r\ndata: {"total":3,"action":"apply"}\r\n\r\n',
      'event: progress\ndata: {"id":10,"proce',
      'ssed":1,"succeeded":1,"failed":0,"total":3}\n\nevent: progress\ndata: {"id":20,"processed":2,"succeeded":1,"failed":1,"total":3,"error":"proposal conflict"}\n\n',
      'event: progress\ndata: {"id":30,"processed":3,"succeeded":2,"failed":1,"total":3}\n\n',
      'event: done\ndata: {"processed":3,"succeeded":2,"failed":1,"cancelled":false}\n\n',
    ]);
    return new Response(stream, {
      status: 200,
      headers: { 'Content-Type': 'text/event-stream' },
    });
  };

  try {
    const res = await streamBatchProposals('apply', [10, 20, 30], {
      onProgress: (ev) => progressEvents.push(ev),
    });
    assert.equal(res.processed, 3);
    assert.equal(res.succeeded, 2);
    assert.equal(res.failed, 1);
    assert.equal(res.total, 3);
    assert.equal(res.cancelled, false);
    assert.deepEqual(res.errors, [{ id: 20, kind: 'proposal', error: 'proposal conflict' }]);
    assert.equal(progressEvents.length, 3);
    assert.deepEqual(progressEvents[1], {
      id: 20,
      processed: 2,
      succeeded: 1,
      failed: 1,
      total: 3,
      error: 'proposal conflict',
    });
  } finally {
    globalThis.fetch = origFetch;
  }
});

test('streamBatchProposals throws on truncated stream without event: done', async () => {
  const origFetch = globalThis.fetch;
  globalThis.fetch = async () => {
    const stream = makeSSEStream([
      'event: start\ndata: {"total":10,"action":"apply"}\n\n',
      'event: progress\ndata: {"id":1,"processed":1,"succeeded":1,"failed":0,"total":10}\n\n',
      'event: progress\ndata: {"id":2,"processed":2,"succeeded":2,"failed":0,"total":10}\n\n',
    ]);
    return new Response(stream, {
      status: 200,
      headers: { 'Content-Type': 'text/event-stream' },
    });
  };

  try {
    await assert.rejects(
      () => streamBatchProposals('apply', [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]),
      /ended unexpectedly before completion \(2\/10 processed\)/
    );
  } finally {
    globalThis.fetch = origFetch;
  }
});

test('streamBatchProposals chunks >500 IDs smoothly and supports abort between chunks', async () => {
  const origFetch = globalThis.fetch;
  const seenChunkSizes = [];
  const ac = new AbortController();

  globalThis.fetch = async (_url, init) => {
    const body = JSON.parse(init.body);
    seenChunkSizes.push(body.ids.length);
    const n = body.ids.length;
    const frames = [`event: start\ndata: {"total":${n},"action":"dismiss"}\n\n`];
    for (let i = 0; i < n; i++) {
      frames.push(
        `event: progress\ndata: {"id":${body.ids[i]},"processed":${i + 1},"succeeded":${i + 1},"failed":0,"total":${n}}\n\n`
      );
    }
    frames.push(`event: done\ndata: {"processed":${n},"succeeded":${n},"failed":0,"cancelled":false}\n\n`);
    if (seenChunkSizes.length === 1) {
      ac.abort();
    }
    return new Response(makeSSEStream(frames), {
      status: 200,
      headers: { 'Content-Type': 'text/event-stream' },
    });
  };

  try {
    const ids620 = Array.from({ length: 620 }, (_, i) => i + 1);
    const res = await streamBatchProposals('dismiss', ids620, { signal: ac.signal });
    assert.deepEqual(seenChunkSizes, [500]);
    assert.equal(res.processed, 500);
    assert.equal(res.succeeded, 500);
    assert.equal(res.total, 620);
    assert.equal(res.cancelled, true);
  } finally {
    globalThis.fetch = origFetch;
  }
});

test('streamBatchProposals does not mark cancelled=true when final chunk already completed 100%', async () => {
  const origFetch = globalThis.fetch;
  const ac = new AbortController();

  globalThis.fetch = async () => {
    const enc = new TextEncoder();
    const stream = new ReadableStream({
      start(c) {
        c.enqueue(enc.encode('event: start\ndata: {"total":1,"action":"apply"}\n\n'));
        c.enqueue(enc.encode('event: progress\ndata: {"id":1,"processed":1,"succeeded":1,"failed":0,"total":1}\n\n'));
        c.enqueue(enc.encode('event: done\ndata: {"processed":1,"succeeded":1,"failed":0,"cancelled":false}\n\n'));
        ac.abort();
        c.close();
      },
    });
    return new Response(stream, {
      status: 200,
      headers: { 'Content-Type': 'text/event-stream' },
    });
  };

  try {
    const res = await streamBatchProposals('apply', [1], { signal: ac.signal });
    assert.equal(res.processed, 1);
    assert.equal(res.succeeded, 1);
    assert.equal(res.cancelled, false);
  } finally {
    globalThis.fetch = origFetch;
  }
});

test('streamBatchProposals falls back on 404 and respects AbortSignal across 100-item fallback chunks', async () => {
  const origFetch = globalThis.fetch;
  const ac = new AbortController();
  const fallbackChunks = [];

  globalThis.fetch = async (url, init) => {
    if (String(url).includes('/stream')) {
      return new Response('Not Found', { status: 404 });
    }
    assert.equal(url, '/api/proposals/batch');
    const body = JSON.parse(init.body);
    fallbackChunks.push(body.ids.length);
    if (fallbackChunks.length === 1) {
      ac.abort();
    }
    return new Response(
      JSON.stringify({
        ok: true,
        action: body.action,
        total: body.ids.length,
        succeeded: body.ids.slice(0, -1),
        failed: [{ id: body.ids[body.ids.length - 1], error: 'missing' }],
      }),
      { status: 200, headers: { 'Content-Type': 'application/json' } }
    );
  };

  try {
    const ids250 = Array.from({ length: 250 }, (_, i) => i + 1);
    const res = await streamBatchProposals('apply', ids250, { signal: ac.signal });
    assert.deepEqual(fallbackChunks, [100]);
    assert.equal(res.processed, 100);
    assert.equal(res.succeeded, 99);
    assert.equal(res.failed, 1);
    assert.equal(res.total, 250);
    assert.equal(res.cancelled, true);
    assert.deepEqual(res.errors, [{ id: 100, kind: 'proposal', error: 'missing' }]);
  } finally {
    globalThis.fetch = origFetch;
  }
});

test('streamBatchSuggestedLinks parses string auth errors and tags link errors with kind="link"', async () => {
  const origFetch = globalThis.fetch;

  // 1. 401 string error payload
  globalThis.fetch = async () =>
    new Response(JSON.stringify({ ok: false, error: 'unauthorized: invalid or missing bearer token' }), {
      status: 401,
      headers: { 'Content-Type': 'application/json' },
    });

  try {
    await assert.rejects(
      () => streamBatchSuggestedLinks('confirm', [1]),
      /unauthorized: invalid or missing bearer token/
    );

    // 2. Link stream with error tagged kind: 'link'
    globalThis.fetch = async (url) => {
      assert.equal(url, '/api/links/suggested/batch/stream');
      return new Response(
        makeSSEStream([
          'event: start\ndata: {"total":2,"action":"confirm"}\n\n',
          'event: progress\ndata: {"id":7,"processed":1,"succeeded":1,"failed":0,"total":2}\n\n',
          'event: progress\ndata: {"id":8,"processed":2,"succeeded":1,"failed":1,"total":2,"error":"sql: no rows"}\n\n',
          'event: done\ndata: {"processed":2,"succeeded":1,"failed":1,"cancelled":false}\n\n',
        ]),
        { status: 200, headers: { 'Content-Type': 'text/event-stream' } }
      );
    };

    const res = await streamBatchSuggestedLinks('confirm', [7, 8]);
    assert.equal(res.processed, 2);
    assert.equal(res.succeeded, 1);
    assert.equal(res.failed, 1);
    assert.equal(res.cancelled, false);
    assert.deepEqual(res.errors, [{ id: 8, kind: 'link', error: 'sql: no rows' }]);
  } finally {
    globalThis.fetch = origFetch;
  }
});
