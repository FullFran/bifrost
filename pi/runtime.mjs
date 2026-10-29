import { spawn, execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { createInterface } from 'node:readline';
import { readFileSync, mkdirSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';

const ZERO = { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 };
const CACHE = join(homedir(), '.cache', 'gentle-shell', 'agy-models.json');
const binary = () => process.env.BIFROST_BIN || 'bifrost';
const exec = promisify(execFile);
function cleanModels(models) {
  const seen = new Set(['default']);
  return Array.isArray(models) ? models.filter(m => {
    if (!m || typeof m.id !== 'string' || !m.id || typeof m.name !== 'string' || seen.has(m.id)) return false;
    seen.add(m.id); return true;
  }) : [];
}
function loadModels() {
  try { return cleanModels(JSON.parse(readFileSync(CACHE, 'utf8'))); } catch { return []; }
}
async function refreshModels() {
  try {
    const { stdout } = await exec(binary(), ['models', 'agy'], { timeout: 30_000 });
    const models = cleanModels(stdout.split('\n').filter(s => s.includes('\t')).map(s => {
      const [id, name] = s.split('\t'); return { id: id.trim(), name: name.trim() };
    }));
    if (!models.length) return;
    mkdirSync(join(homedir(), '.cache', 'gentle-shell'), { recursive: true });
    writeFileSync(CACHE, JSON.stringify(models));
  } catch { /* Best-effort catalog, retaining the previous cache on failure. */ }
}
function messageText(message) {
  if (typeof message.content === 'string') return message.content;
  return (message.content ?? []).filter(b => b.type === 'text').map(b => b.text).join('\n');
}
function buildPrompt(context, deps) {
  // The host replays named prompt sections and removals; never reconstruct that state here.
  const system = deps.getCurrentSystemPrompt(context.messages);
  const turns = context.messages.filter(m => ['user', 'assistant', 'toolResult'].includes(m.role));
  const users = turns.filter(m => m.role === 'user');
  const task = users.length <= 1 ? (users.length ? messageText(users[0]) : '') :
    `Conversation so far:\n\n${turns.map(m => `${m.role === 'user' ? 'User' : m.role === 'assistant' ? 'Assistant' : 'Tool result'}:\n${messageText(m)}`).join('\n\n')}\n\nContinue from the last user message.`;
  return [system ? `System:\n${system}` : '', task].filter(Boolean).join('\n\n');
}
function summary(name, input) {
  const detail = Object.values(input ?? {}).find(v => typeof v === 'string');
  return `${name || 'tool'}${detail ? `: ${detail.replace(/\s+/g, ' ').slice(0, 120)}` : ''}`;
}
function count(value) { return typeof value === 'number' && Number.isFinite(value) && value >= 0 ? value : 0; }
function nativeUsage(usage, out) {
  out.input = count(usage?.input_tokens); out.output = count(usage?.output_tokens);
  out.cacheRead = count(usage?.cache_read_input_tokens); out.cacheWrite = count(usage?.cache_creation_input_tokens);
}
// Hold partial blocks until their assistant snapshot supplies complete tool inputs.
// Message IDs scope block indexes; tool IDs identify tools within each message.
function claudeActivity(think) {
  const messages = new Map();
  let current;
  const fresh = () => ({ blocks: new Map() });
  const flush = message => {
    if (!message) return;
    for (const entry of message.blocks.values()) {
      if (entry.emitted) continue;
      const part = entry.part;
      if (part.type === 'tool_use') {
        let input = part.input;
        if (entry.json) { try { input = JSON.parse(entry.json); } catch { /* Retain native input on truncated JSON. */ } }
        think(`→ ${summary(part.name, input)}`);
      } else if (part.type === 'text') think(part.text);
      else if (part.type === 'thinking') think(part.thinking);
      entry.emitted = true;
    }
  };
  return event => {
    const native = event.type === 'stream_event' ? event.event : event;
    if (native?.type === 'message_start') {
      flush(current);
      const id = native.message?.id;
      current = (id && messages.get(id)) || fresh();
      if (id) messages.set(id, current);
    }
    if (native?.type === 'content_block_start' || native?.type === 'content_block_delta') {
      current ??= fresh();
      const index = native.index ?? 0;
      let entry = current.blocks.get(index);
      if (!entry) {
        entry = { part: { ...native.content_block } };
        current.blocks.set(index, entry);
      }
      const delta = native.delta;
      if (delta?.type === 'input_json_delta') entry.json = (entry.json ?? '') + delta.partial_json;
      for (const [type, key] of [['text_delta', 'text'], ['thinking_delta', 'thinking']]) {
        if (delta?.type === type) {
          entry.part.type = key;
          entry.part[key] = (entry.part[key] ?? '') + (delta[key] ?? '');
        }
      }
    }
    if (event.type === 'assistant' && Array.isArray(event.message?.content)) {
      const id = event.message.id;
      const message = (id && messages.get(id)) || current || fresh();
      if (id) messages.set(id, message);
      event.message.content.forEach((part, index) => {
        const entry = part.id ? [...message.blocks.values()].find(e => e.part.id === part.id) : message.blocks.get(index);
        if (entry) { entry.part = part; entry.json = ''; }
        else message.blocks.set(index, { part });
      });
      flush(message);
      // Without message_start, the next partial belongs to a new message.
      if (current === message) current = undefined;
    }
    if (event.type === 'result') { flush(current); current = undefined; }
  };
}
function streamAgent(realm, model, context, options, deps) {
  const stream = deps.createStream();
  const output = {
    role: 'assistant', api: model.api, provider: model.provider, model: model.id,
    content: [], timestamp: Date.now(), stopReason: 'pending',
    usage: { ...ZERO, totalTokens: 0, cost: { ...ZERO, total: 0 } },
  };
  (async () => {
    let child, closed = false, thinkingOpen = false, aborted = false;
    const abort = () => { if (!aborted && child && !closed) { aborted = true; child.kill('SIGTERM'); } };
    const endThinking = () => {
      if (!thinkingOpen) return;
      thinkingOpen = false;
      stream.push({ type: 'thinking_end', contentIndex: 0, content: output.content[0].thinking, partial: output });
    };
    try {
      if (options.signal?.aborted) throw new Error('Request was aborted');
      const cwd = deps.cwd();
      const effort = options.reasoning === 'minimal' ? 'low' : options.reasoning;
      let args = ['--json', '-r', realm, '-d', cwd];
      if (model.id !== 'default') args.push('-m', model.id);
      if (effort && effort !== 'off') args.push('-e', effort);
      args.push('--', buildPrompt(context, deps));
      const replacement = await options.onPayload?.({ args }, model);
      if (replacement !== undefined) {
        if (!Array.isArray(replacement?.args) || !replacement.args.every(s => typeof s === 'string')) throw new Error('Invalid bifrost payload');
        args = replacement.args;
      }
      if (options.signal?.aborted) throw new Error('Request was aborted');
      child = deps.spawn(deps.binary(), args, { cwd, stdio: ['ignore', 'pipe', 'pipe'], env: { ...process.env, ...options.env } });
      let stderr = '';
      child.stderr.on('data', chunk => { stderr = (stderr + chunk).slice(-8192); });
      // Resolve rather than reject, so process errors cannot become unhandled during stream parsing.
      const exited = new Promise(resolve => {
        child.once('error', error => { closed = true; resolve({ error }); });
        child.once('close', code => { closed = true; resolve({ code }); });
      });
      options.signal?.addEventListener('abort', abort, { once: true });
      if (options.signal?.aborted) abort();
      // CLI transport has no HTTP response; onResponse is intentionally not fabricated.
      stream.push({ type: 'start', partial: output });
      output.content.push({ type: 'thinking', thinking: '' }); thinkingOpen = true;
      stream.push({ type: 'thinking_start', contentIndex: 0, partial: output });
      const think = text => {
        if (!text) return;
        const block = output.content[0]; const delta = `${block.thinking ? '\n' : ''}${text}`;
        block.thinking += delta;
        stream.push({ type: 'thinking_delta', contentIndex: 0, delta, partial: output });
      };
      const claude = claudeActivity(think);
      let finalText, failure, pendingNarration = '';
      for await (const line of createInterface({ input: child.stdout })) {
        if (!line.trim()) continue;
        let event;
        try { event = JSON.parse(line); } catch { throw new Error('Invalid bifrost JSON'); }
        if (!event || typeof event !== 'object') throw new Error('Invalid bifrost JSON');
        await options.onProviderStreamEvent?.(event, model);
        if (realm === 'agy') {
          const step = event.step_update;
          if (step?.step_type === 'agent_response' && step.text_delta) pendingNarration += step.text_delta;
          if (step?.step_type === 'tool' && step.state === 'ACTIVE') {
            think(pendingNarration.trim()); pendingNarration = '';
            think(`→ ${summary(step.tool_name, step.tool_info?.parameters)}`);
          }
          if (step?.state === 'DONE' && step.usage) {
            output.usage.input += count(step.usage.input_tokens);
            output.usage.output += count(step.usage.output_tokens) + count(step.usage.thinking_tokens);
            output.usage.cacheRead += count(step.usage.cache_read_tokens);
          }
          if (event.result) {
            if (event.result.status === 'SUCCESS' && typeof event.result.response === 'string') finalText = event.result.response;
            else failure = event.result.error || event.result.status || 'Invalid agy result';
          }
        } else {
          claude(event);
          if (event.type === 'result') {
            nativeUsage(event.usage, output.usage);
            output.usage.cost.total = count(event.total_cost_usd);
            if (event.is_error || event.subtype !== 'success') failure = event.errors?.join('; ') || event.result || event.subtype || 'Claude failed';
            else if (typeof event.result !== 'string') failure = 'Invalid claude result';
            else finalText = event.result;
          }
        }
      }
      const exit = await exited;
      endThinking();
      if (options.signal?.aborted) throw new Error('Request was aborted');
      if (exit.error) throw exit.error;
      if (exit.code !== 0) throw new Error(`bifrost exited with code ${exit.code}${stderr.trim() ? `: ${stderr.trim()}` : ''}`);
      if (failure) throw new Error(failure);
      if (finalText === undefined) throw new Error(`Missing ${realm} result`);
      const index = output.content.length;
      output.content.push({ type: 'text', text: '' });
      stream.push({ type: 'text_start', contentIndex: index, partial: output });
      output.content[index].text = finalText;
      stream.push({ type: 'text_delta', contentIndex: index, delta: finalText, partial: output });
      stream.push({ type: 'text_end', contentIndex: index, content: finalText, partial: output });
      output.stopReason = 'stop';
    } catch (error) {
      abort(); endThinking();
      output.stopReason = options.signal?.aborted ? 'aborted' : 'error';
      output.errorMessage = error instanceof Error ? error.message : String(error);
    } finally {
      options.signal?.removeEventListener('abort', abort);
      output.usage.totalTokens = output.usage.input + output.usage.output + output.usage.cacheRead + output.usage.cacheWrite;
      if (output.stopReason === 'stop') stream.push({ type: 'done', reason: 'stop', message: output });
      else stream.push({ type: 'error', reason: output.stopReason, error: output });
      stream.end();
    }
  })();
  return stream;
}
function policies(stdout) {
  const config = JSON.parse(stdout);
  if (!config.realms || typeof config.realms !== 'object' || Array.isArray(config.realms)) throw new Error('Invalid realm config');
  return Object.entries(config.realms).map(([realm, policy]) => {
    if (!policy || typeof policy !== 'object') throw new Error('Invalid realm config');
    for (const key of ['enabled', 'auto']) if (policy[key] !== undefined && typeof policy[key] !== 'boolean') throw new Error('Invalid realm policy');
    return { realm, enabled: policy.enabled ?? true, auto: policy.auto ?? true };
  });
}
export function registerBifrost(pi, injected) {
  const deps = { spawn, exec, binary, loadModels, refreshModels, cwd: () => process.cwd(), ...injected };
  for (const realm of ['agy', 'claude']) {
    const provider = realm === 'claude' ? 'bifrost-claude' : 'agy';
    const catalog = realm === 'agy' ? cleanModels(deps.loadModels()) : [];
    pi.registerProvider(provider, {
      baseUrl: `bifrost://${realm}`, apiKey: 'local', api: `${provider}-bifrost`,
      models: [{ id: 'default', name: `${realm} (bifrost default model)` }, ...catalog].map(m => ({
        ...m, reasoning: true, input: ['text'], cost: { ...ZERO }, contextWindow: 1_000_000, maxTokens: 65_536,
      })),
      streamSimple: (model, context, options = {}) => streamAgent(realm, model, context, options, deps),
    });
  }
  pi.on('session_start', async () => { await deps.refreshModels(); });
  pi.registerCommand('bifrost', {
    description: 'Toggle Bifrost realm enabled/auto policy (saved by the CLI)',
    handler: async (_args, ctx) => {
      if (!ctx.hasUI) { ctx.ui.notify('/bifrost requires dialog-capable UI', 'error'); return; }
      const call = args => deps.exec(deps.binary(), args, { cwd: ctx.cwd, timeout: 30_000 });
      try {
        while (true) {
          const items = policies((await call(['config', '--json'])).stdout);
          const labels = items.map(p => `${p.realm} (enabled: ${p.enabled}, auto: ${p.auto})`);
          const selected = await ctx.ui.select('Bifrost realms', labels);
          const policy = items[labels.indexOf(selected)]; if (!policy) return;
          const keys = ['enabled', 'auto'];
          const options = keys.map(key => `${key} (${policy[key]}) → ${!policy[key]}`);
          const action = await ctx.ui.select(policy.realm, options);
          const key = keys[options.indexOf(action)]; if (!key) continue;
          await call(['config', 'set', policy.realm, key, String(!policy[key])]);
          ctx.ui.notify(`${policy.realm} ${key} = ${!policy[key]}`, 'info');
        }
      } catch (error) { ctx.ui.notify(`Bifrost: ${error.message}`, 'error'); }
    },
  });
}
