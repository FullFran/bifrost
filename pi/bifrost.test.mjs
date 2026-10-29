import { test } from 'node:test';
import assert from 'node:assert/strict';
import { EventEmitter } from 'node:events';
import { PassThrough } from 'node:stream';
import { registerBifrost } from './runtime.mjs';

// Implements the public stream surface without loading or installing the Pi host.
function createStream() {
  const events = [];
  let finish;
  const complete = new Promise(resolve => { finish = resolve; });
  return {
    push(event) { events.push(structuredClone(event)); },
    end() { finish(); },
    async *[Symbol.asyncIterator]() { await complete; yield* events; },
  };
}
function harness({ lines = [], code = 0, spawnError, hang = false, execError, failSet = false, systemPromptFn, config = { realms: { agy: {}, claude: { enabled: false, auto: true }, opencode: {} } } } = {}) {
  const providers = new Map(), commands = new Map(), hooks = new Map(), calls = [], children = [];
  const pi = {
    registerProvider(name, value) { providers.set(name, value); },
    registerCommand(name, value) { commands.set(name, value); },
    on(name, fn) { hooks.set(name, fn); },
  };
  const deps = {
    createStream,
    // Host-owned transcript replay is injected, like the stream factory.
    getCurrentSystemPrompt: systemPromptFn ?? (messages => messages.filter(m => m.role === 'system').map(m => typeof m.content === 'string' ? m.content : m.content.map(b => b.text).join('\n')).join('\n\n')),
    binary: () => '/fake/bifrost', loadModels: () => [{ id: 'gemini-test', name: 'Test' }],
    refreshModels: async () => {}, cwd: () => '/workspace',
    exec: async (binary, args, options) => {
      calls.push({ binary, args, options });
      if (execError) throw new Error(execError);
      if (failSet && args[1] === 'set') throw new Error('write denied');
      return { stdout: JSON.stringify(config), stderr: '' };
    },
    spawn(binary, args, options) {
      calls.push({ binary, args, options });
      if (spawnError === 'sync') throw new Error('spawn failed');
      const child = new EventEmitter();
      child.stdout = new PassThrough(); child.stderr = new PassThrough(); child.kills = [];
      child.kill = signal => {
        child.kills.push(signal);
        child.stdout.end();
        queueMicrotask(() => child.emit('close', null));
      };
      children.push(child);
      setImmediate(() => {
        if (spawnError) { child.emit('error', new Error('ENOENT')); child.stdout.end(); return; }
        if (hang) return;
        const data = lines.map(line => typeof line === 'string' ? line : JSON.stringify(line)).join('\n');
        // Split UTF-8 across chunks to exercise readline decoding.
        const bytes = Buffer.from(data + '\n');
        for (const byte of bytes) child.stdout.write(Buffer.from([byte]));
        child.stdout.end(); child.emit('close', code);
      });
      return child;
    },
  };
  registerBifrost(pi, deps);
  return { providers, commands, hooks, calls, children };
}
async function run(h, realm = 'claude', options = {}, messages = [{ role: 'user', content: 'task' }], id = 'default') {
  const providerName = realm === 'claude' ? 'bifrost-claude' : realm;
  const provider = h.providers.get(providerName);
  const events = [];
  for await (const event of provider.streamSimple({ id, provider: providerName, api: provider.api }, { messages }, options)) events.push(event);
  const last = events.at(-1);
  return { events, last, message: last.message ?? last.error };
}
const success = (result = 'Answer 🌈', usage) => ({ type: 'result', subtype: 'success', is_error: false, result, usage });

test('registers agy/default, cached agy models and bifrost-claude/default, with no startup process', () => {
  const h = harness();
  assert.deepEqual([...h.providers.keys()], ['agy', 'bifrost-claude']);
  assert.deepEqual(h.providers.get('agy').models.map(m => m.id), ['default', 'gemini-test']);
  assert.deepEqual(h.providers.get('bifrost-claude').models.map(m => m.id), ['default']);
  assert.equal(h.calls.length, 0);
});
test('Claude final/usage and tool activity are text/thinking only; CLI defaults and cwd survive', async () => {
  const h = harness({ lines: [
    { type: 'assistant', message: { content: [{ type: 'tool_use', name: 'Read', input: { file_path: 'a.ts' } }] } },
    { type: 'stream_event', event: { type: 'content_block_delta', delta: { type: 'thinking_delta', thinking: 'Planning' } } },
    success('Answer 🌈', { input_tokens: 10, output_tokens: 4, cache_read_input_tokens: 3, cache_creation_input_tokens: 2 }),
  ] });
  const { events, message, last } = await run(h);
  assert.equal(last.type, 'done');
  assert.equal(message.stopReason, 'stop');
  assert.equal(message.content.find(b => b.type === 'text').text, 'Answer 🌈');
  assert.match(message.content.find(b => b.type === 'thinking').thinking, /Read.*a.ts/);
  assert.match(message.content[0].thinking, /Planning/);
  assert.ok(!events.some(e => e.type.startsWith('toolcall')));
  assert.deepEqual(message.usage, { input: 10, output: 4, cacheRead: 3, cacheWrite: 2, totalTokens: 19, cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, total: 0 } });
  assert.deepEqual(h.calls[0].args, ['--json', '-r', 'claude', '-d', '/workspace', '--', 'task']);
  assert.equal(h.calls[0].options.shell, undefined);
});
test('Claude reconciles partial blocks with completed snapshots without duplicate activity', async () => {
  const partial = event => ({ type: 'stream_event', event });
  const lines = [
    partial({ type: 'message_start', message: { id: 'm1' } }),
    partial({ type: 'content_block_start', index: 0, content_block: { type: 'tool_use', id: 't1', name: 'Read', input: {} } }),
    partial({ type: 'content_block_delta', index: 0, delta: { type: 'input_json_delta', partial_json: '{"file_path":"a.ts"}' } }),
    partial({ type: 'content_block_stop', index: 0 }),
    partial({ type: 'content_block_start', index: 1, content_block: { type: 'text', text: '' } }),
    partial({ type: 'content_block_delta', index: 1, delta: { type: 'text_delta', text: 'hel' } }),
    partial({ type: 'content_block_delta', index: 1, delta: { type: 'text_delta', text: 'lo' } }),
    partial({ type: 'message_stop' }),
    { type: 'assistant', message: { id: 'm1', content: [
      { type: 'tool_use', id: 't1', name: 'Read', input: { file_path: 'a.ts' } }, { type: 'text', text: 'hello' },
      { type: 'thinking', thinking: 'snapshot-only thought' },
    ] } },
    // A different message can reuse block indexes and contain identical text/tools.
    partial({ type: 'message_start', message: { id: 'm2' } }),
    partial({ type: 'content_block_start', index: 0, content_block: { type: 'tool_use', id: 't2', name: 'Read', input: {} } }),
    partial({ type: 'content_block_stop', index: 0 }),
    { type: 'assistant', message: { id: 'm2', content: [
      { type: 'tool_use', id: 't2', name: 'Read', input: { file_path: 'a.ts' } }, { type: 'text', text: 'hello' },
      { type: 'tool_use', id: 't3', name: 'Read', input: { file_path: 'b.ts' } },
    ] } },
    success('hello'),
  ];
  const { message, events, last } = await run(harness({ lines }));
  assert.equal(last.type, 'done');
  const expected = '→ Read: a.ts\nhello\nsnapshot-only thought\n→ Read: a.ts\nhello\n→ Read: b.ts';
  assert.equal(message.content[0].thinking, expected);
  assert.equal(events.filter(e => e.type === 'thinking_delta').map(e => e.delta).join(''), expected);
  assert.deepEqual(message.content.filter(b => b.type === 'text'), [{ type: 'text', text: 'hello' }]);
  assert.deepEqual(events.filter(e => e.type === 'text_delta').map(e => e.delta), ['hello']);
});
test('Claude reconciles snapshots when the transport omits message_start', async () => {
  const { message, events, last } = await run(harness({ lines: [
    { type: 'stream_event', event: { type: 'content_block_start', index: 0, content_block: { type: 'tool_use', id: 'tool-1', name: 'Read', input: {} } } },
    { type: 'assistant', message: { id: 'msg-1', content: [{ type: 'tool_use', id: 'tool-1', name: 'Read', input: { path: 'a.ts' } }] } },
    { type: 'stream_event', event: { type: 'content_block_delta', index: 0, delta: { type: 'text_delta', text: 'hello' } } },
    { type: 'assistant', message: { id: 'msg-2', content: [{ type: 'text', text: 'hello' }] } },
    success('hello'),
  ] }));
  assert.equal(last.type, 'done');
  assert.equal(message.content[0].thinking, '→ Read: a.ts\nhello');
  assert.deepEqual(events.filter(e => e.type === 'text_delta').map(e => e.delta), ['hello']);
});
test('Claude preserves native non-stream content and partial-only tools and text', async () => {
  const lines = [
    { type: 'assistant', message: { id: 'native', content: [{ type: 'text', text: 'same' }, { type: 'text', text: 'same' }, { type: 'tool_use', id: 'n1', name: 'Read', input: { path: 'native.ts' } }] } },
    { type: 'stream_event', event: { type: 'message_start', message: { id: 'partial' } } },
    { type: 'stream_event', event: { type: 'content_block_start', index: 0, content_block: { type: 'tool_use', id: 'p1', name: 'Read', input: {} } } },
    { type: 'stream_event', event: { type: 'content_block_delta', index: 0, delta: { type: 'input_json_delta', partial_json: '{"path":"partial.ts"}' } } },
    { type: 'stream_event', event: { type: 'content_block_start', index: 1, content_block: { type: 'text', text: 'start' } } },
    { type: 'stream_event', event: { type: 'content_block_delta', index: 1, delta: { type: 'text_delta', text: ' end' } } },
    success('final'),
  ];
  const { message, last } = await run(harness({ lines }));
  assert.equal(last.type, 'done');
  assert.equal(message.content[0].thinking, 'same\nsame\n→ Read: native.ts\n→ Read: partial.ts\nstart end');
  assert.equal(message.content.at(-1).text, 'final');
});
test('system/subagent instructions survive dispatch without Pi tool declarations', async () => {
  const h = harness({ lines: [success()] });
  await run(h, 'claude', {}, [{ role: 'system', content: [{ type: 'text', text: 'Worker instructions' }], tools: [{ name: 'danger' }] }, { role: 'user', content: 'task' }]);
  assert.equal(h.calls[0].args.at(-1), 'System:\nWorker instructions\n\ntask');
});
test('catalog refresh is deferred to session_start', async () => {
  const h = harness();
  assert.ok(h.hooks.has('session_start'));
  await h.hooks.get('session_start')({}, { cwd: '/project' });
  assert.equal(h.calls.length, 0);
});
test('delegates current named system section replay to the host helper', async () => {
  const messages = [{ role: 'system', content: 'base', sections: { worker: 'old' }, timestamp: 0 }, { role: 'system', content: '', sections: { worker: 'new' }, timestamp: 1 }, { role: 'user', content: 'task' }];
  const h = harness({ lines: [success()], systemPromptFn: received => { assert.equal(received, messages); return 'base\n\nnew'; } });
  await run(h, 'claude', {}, messages);
  assert.equal(h.calls[0].args.at(-1), 'System:\nbase\n\nnew\n\ntask');
});
test('payload replacement and native event instrumentation run before normalization', async () => {
  const observed = [], h = harness({ lines: [success('ok')] });
  const { last } = await run(h, 'claude', {
    onPayload: payload => { assert.equal(payload.args.at(-1), 'task'); return { args: ['replacement'] }; },
    onProviderStreamEvent: event => observed.push(event.type),
  });
  assert.deepEqual(h.calls[0].args, ['replacement']);
  assert.deepEqual(observed, ['result']); assert.equal(last.type, 'done');
});
test('empty successful native result is valid and does not use narration as final', async () => {
  const h = harness({ lines: [{ type: 'assistant', message: { content: [{ type: 'text', text: 'not final' }] } }, success('')] });
  assert.equal((await run(h)).message.content.find(b => b.type === 'text').text, '');
});
for (const [name, lines, code, error] of [
  ['missing result', [], 0, 'Missing claude result'],
  ['malformed JSON', ['not json'], 0, 'Invalid bifrost JSON'],
  ['truncated result', [{ type: 'result', subtype: 'success' }], 0, 'Invalid claude result'],
  ['native error', [{ type: 'result', subtype: 'error_during_execution', is_error: true, errors: ['denied'] }], 0, 'denied'],
  ['nonzero despite result', [success()], 7, 'bifrost exited with code 7'],
  ['signal exit', [success()], null, 'bifrost exited with code null'],
]) test(`fails closed: ${name}`, async () => {
  const { last, message, events } = await run(harness({ lines, code }));
  assert.equal(last.type, 'error'); assert.equal(message.stopReason, 'error');
  assert.equal(message.errorMessage, error);
  assert.ok(!message.content.some(b => b.type === 'text'));
  assert.equal(events.filter(e => ['done', 'error'].includes(e.type)).length, 1);
});
for (const spawnError of ['sync', 'async']) test(`spawn ${spawnError} failure terminates`, async () => {
  assert.equal((await run(harness({ spawnError }))).last.type, 'error');
});
test('cancellation before dispatch spawns nothing', async () => {
  const h = harness(); const abort = new AbortController(); abort.abort();
  assert.equal((await run(h, 'claude', { signal: abort.signal })).message.stopReason, 'aborted');
  assert.equal(h.calls.length, 0);
});
test('cancellation during dispatch kills child and ends aborted', async () => {
  const h = harness({ hang: true }); const abort = new AbortController();
  const result = run(h, 'claude', { signal: abort.signal });
  await new Promise(resolve => setImmediate(resolve)); abort.abort();
  assert.equal((await result).message.stopReason, 'aborted');
  assert.deepEqual(h.children[0].kills, ['SIGTERM']);
});
test('preserves agy streaming, usage, model/effort and follow-up transcript', async () => {
  const h = harness({ lines: [
    { step_update: { step_type: 'tool', state: 'ACTIVE', tool_name: 'read', tool_info: { parameters: { path: 'a' } } } },
    { step_update: { state: 'DONE', usage: { input_tokens: 2, output_tokens: 3, thinking_tokens: 4, cache_read_tokens: 5 } } },
    { result: { status: 'SUCCESS', response: 'agy answer' } },
  ] });
  const { message } = await run(h, 'agy', { reasoning: 'minimal' }, [{ role: 'user', content: 'one' }, { role: 'assistant', content: [{ type: 'text', text: 'two' }] }, { role: 'user', content: 'three' }], 'gemini-test');
  assert.equal(message.content.at(-1).text, 'agy answer');
  assert.equal(message.usage.totalTokens, 14);
  assert.deepEqual(h.calls[0].args.slice(0, 11), ['--json', '-r', 'agy', '-d', '/workspace', '-m', 'gemini-test', '-e', 'low', '--', h.calls[0].args.at(-1)]);
  assert.match(h.calls[0].args.at(-1), /User:\none.*Assistant:\ntwo.*User:\nthree/s);
});
test('agy/default still delegates defaults and fails closed on nonzero after result', async () => {
  const h = harness({ lines: [{ result: { status: 'SUCCESS', response: 'answer' } }], code: 2 });
  const { message } = await run(h, 'agy');
  assert.deepEqual(h.calls[0].args, ['--json', '-r', 'agy', '-d', '/workspace', '--', 'task']);
  assert.equal(message.stopReason, 'error');
  assert.equal(message.errorMessage, 'bifrost exited with code 2');
});
function ui(choices) {
  const shown = [], notices = [];
  return { shown, notices, select: async (title, options) => { shown.push({ title, options }); return choices.shift()?.(options); }, notify: (text, level) => notices.push({ text, level }) };
}
test('/bifrost reads effective CLI policy and writes only chosen realm option then reloads', async () => {
  const h = harness();
  const view = ui([opts => opts.find(s => s.startsWith('claude ')), opts => opts.find(s => s.startsWith('enabled ')), opts => opts.find(s => s.startsWith('claude ')), opts => opts.find(s => s.startsWith('auto '))]);
  await h.commands.get('bifrost').handler('', { cwd: '/project', hasUI: true, ui: view });
  assert.deepEqual(h.calls.map(c => c.args), [['config', '--json'], ['config', 'set', 'claude', 'enabled', 'true'], ['config', '--json'], ['config', 'set', 'claude', 'auto', 'false'], ['config', '--json']]);
  assert.ok(view.shown[0].options.includes('agy (enabled: true, auto: true)'));
  assert.ok(h.calls.every(c => c.options.cwd === '/project'));
});
test('/bifrost cancel and unavailable UI never mutate state', async () => {
  const h = harness();
  await h.commands.get('bifrost').handler('', { cwd: '/p', hasUI: true, ui: ui([]) });
  assert.deepEqual(h.calls.map(c => c.args), [['config', '--json']]);
  const h2 = harness();
  await h2.commands.get('bifrost').handler('', { hasUI: false, ui: ui([]) });
  assert.equal(h2.calls.length, 0);
});
test('/bifrost failed write reports no success and preserves CLI state', async () => {
  const config = { realms: { claude: { enabled: false, auto: true } } };
  const h = harness({ config, failSet: true });
  const view = ui([opts => opts[0], opts => opts[0]]);
  await h.commands.get('bifrost').handler('', { cwd: '/p', hasUI: true, ui: view });
  assert.deepEqual(config, { realms: { claude: { enabled: false, auto: true } } });
  assert.deepEqual(h.calls.map(c => c.args), [['config', '--json'], ['config', 'set', 'claude', 'enabled', 'true']]);
  assert.deepEqual(view.notices, [{ text: 'Bifrost: write denied', level: 'error' }]);
});
for (const settings of [{ execError: 'config failed' }, { config: { realms: { claude: { enabled: 'false' } } } }]) test('/bifrost config failures notify without mutation', async () => {
  const h = harness(settings); const view = ui([]);
  await h.commands.get('bifrost').handler('', { cwd: '/p', hasUI: true, ui: view });
  assert.equal(h.calls.length, 1); assert.equal(view.notices.at(-1).level, 'error');
});
