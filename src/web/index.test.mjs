import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { createHash } from 'node:crypto';
import { access, mkdtemp, readFile, rm } from 'node:fs/promises';
import { createServer, request as httpRequest } from 'node:http';
import { tmpdir } from 'node:os';
import path from 'node:path';
import test from 'node:test';
import vm from 'node:vm';

async function installedChromium() {
  const candidates = [
    process.env.CHROME_PATH,
    process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH,
    'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
    'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe',
    '/usr/bin/google-chrome',
    '/usr/bin/chromium',
    '/usr/bin/chromium-browser',
    '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  ].filter(Boolean);
  for (const candidate of candidates) {
    try {
      await access(candidate);
      return candidate;
    } catch {
      // Keep looking for an installed browser.
    }
  }
  return '';
}

const wait = (milliseconds) => new Promise((resolve) => setTimeout(resolve, milliseconds));

function requestWithLiteralHost(url, host, timeout = 2_000) {
  return new Promise((resolve, reject) => {
    const target = new URL(url);
    let response;
    let timeoutID;
    let settled = false;
    const onResponseEnd = () => finish(null, { status: Number(response?.statusCode || 0) });
    const onResponseError = (error) => finish(error);
    const onRequestError = (error) => finish(error);
    const onRequestClose = () => request.removeListener('error', onRequestError);
    const finish = (error, result) => {
      if (settled) return;
      settled = true;
      clearTimeout(timeoutID);
      response?.removeListener('end', onResponseEnd);
      response?.removeListener('error', onResponseError);
      if (error) {
        response?.destroy();
        if (!request.destroyed) request.destroy();
        reject(error);
        return;
      }
      resolve(result);
    };
    const request = httpRequest({
      protocol: target.protocol,
      hostname: target.hostname,
      port: target.port || undefined,
      path: `${target.pathname}${target.search}`,
      method: 'GET',
      headers: { Host: host, Connection: 'close' },
    }, (incoming) => {
      response = incoming;
      response.once('end', onResponseEnd);
      response.once('error', onResponseError);
      response.resume();
    });
    request.on('error', onRequestError);
    request.once('close', onRequestClose);
    timeoutID = setTimeout(() => {
      finish(new Error(`literal Host request timed out after ${timeout} ms`));
    }, timeout);
    request.end();
  });
}

async function within(promise, label, timeout = 5_000) {
  let timeoutID;
  try {
    return await Promise.race([
      promise,
      new Promise((_, reject) => {
        timeoutID = setTimeout(() => reject(new Error(`${label} timed out`)), timeout);
      }),
    ]);
  } finally {
    clearTimeout(timeoutID);
  }
}

async function waitFor(check, label, timeout = 8_000) {
  const deadline = Date.now() + timeout;
  let lastError;
  while (Date.now() < deadline) {
    try {
      const value = await within(Promise.resolve().then(check), `${label} check`, 1_000);
      if (value) return value;
    } catch (error) {
      lastError = error;
    }
    await wait(40);
  }
  throw new Error(`${label} timed out${lastError ? `: ${lastError.message}` : ''}`);
}

class DevToolsClient {
  constructor(socket, closureDetails = () => '') {
    this.socket = socket;
    this.sequence = 0;
    this.pending = new Map();
    this.listeners = new Map();
    this.closed = false;
    this.shutdownExpected = false;
    this.closureDetails = closureDetails;
    socket.addEventListener('message', (event) => {
      const message = JSON.parse(String(event.data));
      if (message.id) {
        const pending = this.pending.get(message.id);
        this.pending.delete(message.id);
        if (message.error) pending?.reject(new Error(message.error.message));
        else pending?.resolve(message.result || {});
        return;
      }
      for (const listener of this.listeners.get(message.method) || []) listener(message.params || {});
    });
    const rejectPending = () => {
      if (this.closed) return;
      this.closed = true;
      for (const pending of this.pending.values()) {
        if (this.shutdownExpected && pending.method === 'Browser.close') pending.resolve({});
        else pending.reject(new Error(`Chromium DevTools connection closed while ${pending.method} was pending${this.closureDetails()}`));
      }
      this.pending.clear();
    };
    socket.addEventListener('close', rejectPending, { once: true });
    socket.addEventListener('error', rejectPending, { once: true });
  }

  static async connect(url, closureDetails) {
    assert.equal(typeof WebSocket, 'function', 'Node with built-in WebSocket support is required for the Chromium regression');
    const socket = new WebSocket(url);
    await new Promise((resolve, reject) => {
      socket.addEventListener('open', resolve, { once: true });
      socket.addEventListener('error', () => reject(new Error('Could not connect to Chromium DevTools')), { once: true });
    });
    return new DevToolsClient(socket, closureDetails);
  }

  on(method, listener) {
    if (!this.listeners.has(method)) this.listeners.set(method, new Set());
    this.listeners.get(method).add(listener);
  }

  expectShutdown() {
    this.shutdownExpected = true;
  }

  send(method, params = {}, sessionId) {
    if (this.closed) return Promise.reject(new Error('Chromium DevTools connection is closed'));
    const id = ++this.sequence;
    return new Promise((resolve, reject) => {
      this.pending.set(id, { method, resolve, reject });
      try {
        this.socket.send(JSON.stringify({ id, method, params, ...(sessionId ? { sessionId } : {}) }));
      } catch (error) {
        this.pending.delete(id);
        reject(error);
      }
    });
  }

  close() {
    if (!this.closed) {
      this.closed = true;
      for (const pending of this.pending.values()) pending.reject(new Error('Chromium DevTools client closed'));
      this.pending.clear();
      this.socket.close();
    }
  }
}

async function closeChromium(chromium, label) {
  if (!chromium) return;
  chromium.client.expectShutdown();
  try { await within(chromium.client.send('Browser.close'), `${label} browser close`, 1_000); } catch {}
  chromium.client.close();
  const waitForExit = async () => {
    if (chromium.processHandle.exitCode !== null) return;
    await Promise.race([
      new Promise((resolve) => chromium.processHandle.once('exit', resolve)),
      wait(1_000),
    ]);
  };
  await waitForExit();
  if (chromium.processHandle.exitCode === null) {
    chromium.processHandle.kill();
    await waitForExit();
  }
}

async function evaluate(client, expression, contextId, sessionId) {
  const response = await client.send('Runtime.evaluate', {
    expression,
    contextId,
    awaitPromise: true,
    returnByValue: true,
  }, sessionId);
  if (response.exceptionDetails) {
    const detail = response.exceptionDetails.exception?.description || response.exceptionDetails.text || 'browser evaluation failed';
    throw new Error(detail);
  }
  return response.result?.value;
}

async function launchChromium(executable, profileDirectory) {
  const args = [
    '--headless=new',
    '--disable-background-networking',
    '--disable-breakpad',
    '--disable-crash-reporter',
    '--disable-default-apps',
    '--disable-gpu',
    '--disable-gpu-shader-disk-cache',
    '--no-first-run',
    '--no-default-browser-check',
    '--remote-debugging-address=127.0.0.1',
    '--remote-debugging-port=0',
    `--user-data-dir=${profileDirectory}`,
    'about:blank',
  ];
  if (process.getuid?.() === 0) args.unshift('--no-sandbox');
  const processHandle = spawn(executable, args, { stdio: ['ignore', 'ignore', 'pipe'], windowsHide: true });
  let stderr = '';
  processHandle.stderr.on('data', (chunk) => { stderr += String(chunk); });
  const portFile = path.join(profileDirectory, 'DevToolsActivePort');
  const port = await waitFor(async () => {
    if (processHandle.exitCode !== null) throw new Error(stderr.trim() || `Chromium exited ${processHandle.exitCode}`);
    try {
      return Number((await readFile(portFile, 'utf8')).split(/\r?\n/u)[0]) || 0;
    } catch {
      return 0;
    }
  }, 'Chromium DevTools');
  const target = await waitFor(async () => {
    const response = await fetch(`http://127.0.0.1:${port}/json/list`);
    const targets = await response.json();
    return targets.find((item) => item.type === 'page' && item.webSocketDebuggerUrl);
  }, 'Chromium page target');
  const closureDetails = () => ` (exit=${processHandle.exitCode ?? 'running'}, signal=${processHandle.signalCode ?? 'none'}, stderr=${stderr.trim().slice(-800) || 'empty'})`;
  return { processHandle, client: await DevToolsClient.connect(target.webSocketDebuggerUrl, closureDetails) };
}

async function pressKey(client, key, code, windowsVirtualKeyCode) {
  const params = { key, code, windowsVirtualKeyCode, nativeVirtualKeyCode: windowsVirtualKeyCode };
  await client.send('Input.dispatchKeyEvent', { type: 'keyDown', ...params });
  await client.send('Input.dispatchKeyEvent', { type: 'keyUp', ...params });
}

test('cockpit inline modules parse before they are embedded in the server binary', async () => {
  const html = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const modules = [...html.matchAll(/<script type="module">([\s\S]*?)<\/script>/g)];

  assert.equal(modules.length, 1, 'the cockpit must contain one executable module');
  assert.doesNotThrow(() => new vm.Script(modules[0][1], { filename: 'index.html:inline-module' }));
});

test('public landing exposes aligned, truthful search metadata and crawl guidance', async () => {
  const html = await readFile(new URL('./landing.html', import.meta.url), 'utf8');
  const account = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const robots = await readFile(new URL('./robots.txt', import.meta.url), 'utf8');
  const sitemap = await readFile(new URL('./sitemap.xml', import.meta.url), 'utf8');
  const title = html.match(/<title>([^<]+)<\/title>/u)?.[1] || '';
  const description = html.match(/<meta name="description" content="([^"]+)">/u)?.[1] || '';
  const structuredSource = html.match(/<script type="application\/ld\+json">([\s\S]*?)<\/script>/u)?.[1] || '';
  const structured = JSON.parse(structuredSource);

  assert.equal(title, 'Community App Builder for Muslim Organizations | Taawun');
  assert.ok(title.length <= 60, `title should remain concise: ${title.length}`);
  assert.ok(description.length > 100 && description.length <= 160, `description should be useful and concise: ${description.length}`);
  assert.match(description, /mosques, charities and Muslim groups/iu);
  assert.match(html, /<link rel="canonical" href="https:\/\/taawun-production\.up\.railway\.app\/">/u);
  assert.match(html, /<meta property="og:title" content="Community App Builder for Muslim Organizations \| Taawun">/u);
  assert.match(html, /<meta name="twitter:card" content="summary">/u);
  assert.doesNotMatch(html, /id="authPanel"|id="loginForm"|id="appView"|Authorization/u);
  assert.match(account, /<meta name="robots" content="noindex,nofollow,noarchive">/u);
  assert.match(account, /id="authPanel"[^>]*data-nosnippet/u);
  assert.match(account, /id="appView"[\s\S]*?hidden[\s\S]*?data-nosnippet/u);
  assert.equal(structured['@type'], 'WebSite');
  assert.equal(structured.name, 'Taawun');
  assert.equal(structured.url, 'https://taawun-production.up.railway.app/');
  assert.match(structured.description || '', /private-beta community app builder/iu);
  assert.doesNotMatch(structuredSource, /"(?:aggregateRating|review|offers?|price(?:Currency)?)"\s*:/iu);
  assert.match(robots, /Disallow: \/api\//u);
  assert.match(robots, /Disallow: \/account/u);
  assert.match(robots, /Sitemap: https:\/\/taawun-production\.up\.railway\.app\/sitemap\.xml/u);
  assert.match(sitemap, /<loc>https:\/\/taawun-production\.up\.railway\.app\/<\/loc>/u);
  assert.doesNotMatch(html, /<script[^>]+src="https?:\/\/|<link[^>]+rel="stylesheet"[^>]+href="https?:\/\//iu);
  assert.match(html, /private preview/iu);
  assert.match(html, /does not hold funds or settle payments/iu);
  assert.match(html, /not religious rulings/iu);
  assert.match(html, /Sūrat al-Māʾidah · 5:2 \(excerpt\)/u);
  assert.match(html, /<blockquote lang="ar" dir="rtl">وَتَعَاوَنُواْ عَلَى ٱلۡبِرِّ وَٱلتَّقۡوَىٰۖ وَلَا تَعَاوَنُواْ عَلَى ٱلۡإِثۡمِ وَٱلۡعُدۡوَٰنِۚ<\/blockquote>/u);
  assert.doesNotMatch(html, /24:35|visual language|visual inspiration/iu);
  assert.match(html, /href="\/account#register"/u);
  assert.match(html, /href="\/account#login"/u);
  assert.doesNotMatch(html, /Taawun is Shariah[- ]certified|scholar[- ]approved (?:platform|software)|launch your app today|start free/iu);
});

test('geometric enhancement is surface-level, optional, and scroll-driven', async () => {
  const html = await readFile(new URL('./landing.html', import.meta.url), 'utf8');
  const loader = await readFile(new URL('./geometric-landing.js', import.meta.url), 'utf8');
  const renderer = await readFile(new URL('./geometric-renderer.js', import.meta.url), 'utf8');

  assert.match(html, /class="geometry-stage"[^>]*aria-hidden="true"/u);
  assert.match(html, /\.geometry-stage \{ position: fixed; inset: 0;[^}]*background: transparent;/u);
  assert.match(html, /id="motionToggle"[^>]*type="button"[^>]*aria-pressed="true"/u);
  assert.match(html, /id="motionToggle"[^>]*aria-label="Decorative motion"/u);
  assert.match(html, /id="menuToggle"[^>]*aria-expanded="false"[^>]*aria-controls="primaryNavigation"/u);
  assert.deepEqual([...html.matchAll(/data-geometry-state="\d+" data-geometry-side="(right|left)"/gu)].map((match) => match[1]), ['right', 'right', 'left', 'right', 'right', 'left']);
  assert.match(html, /prefers-reduced-motion: reduce/u);
  assert.match(html, /forced-colors: active/u);
  assert.match(loader, /prefers-reduced-motion: reduce/u);
  assert.match(loader, /prefers-contrast: more/u);
  assert.match(loader, /prefers-reduced-transparency: reduce/u);
  assert.match(loader, /connection && connection\.saveData/u);
  assert.match(loader, /connection\?\.addEventListener\?\.\('change', reconcilePreference\)/u);
  assert.match(loader, /window\.innerWidth > 840/u);
  assert.match(loader, /event\.key === 'Escape'/u);
  assert.match(loader, /setAttribute\('aria-pressed', String\(active\)\)/u);
  assert.match(loader, /progress > 0\.12 && progress < 0\.88/u);
  assert.ok(loader.indexOf('if (!capable()') < loader.indexOf("import('/geometric-renderer.js')"), 'capability checks must precede the renderer request');
  assert.match(renderer, /powerPreference: 'low-power'/u);
  assert.match(renderer, /adapter\.isFallbackAdapter/u);
  assert.match(renderer, /1_500_000/u);
  assert.match(renderer, /window\.addEventListener\('scroll', onScroll, \{ passive: true \}\)/u);
  assert.doesNotMatch(renderer, /requestAnimationFrame\(render\)|preventDefault\(\)|setInterval\(/u);
  assert.match(renderer, /device\.lost/u);
  assert.match(renderer, /refractionEdge/u);
  assert.match(renderer, /1\.0 - smoothstep\(0\.004, 0\.055, line\)/u);
  assert.match(renderer, /1\.0 - smoothstep\(0\.04, 0\.66, lensDistance\)/u);
  assert.match(renderer, /clearValue: \{ r: 0, g: 0, b: 0, a: 0 \}/u);
  assert.match(renderer, /color \* alpha, alpha/u);
});

test('promoted browser proves WebGPU enhancement, reversible pause, failure fallback, and reduced motion', { timeout: 90_000 }, async () => {
  const browser = await installedChromium();
  assert.ok(browser, 'Chromium is required; the WebGPU lifecycle regression cannot be skipped');
  const landingHTML = await readFile(new URL('./landing.html', import.meta.url), 'utf8');
  const loader = await readFile(new URL('./geometric-landing.js', import.meta.url), 'utf8');
  const renderer = await readFile(new URL('./geometric-renderer.js', import.meta.url), 'utf8');
  const server = createServer((request, response) => {
    const requestPath = new URL(request.url, 'http://localhost').pathname;
    const sources = new Map([
      ['/', ['text/html; charset=utf-8', landingHTML]],
      ['/geometric-landing.js', ['text/javascript; charset=utf-8', loader]],
      ['/geometric-renderer.js', ['text/javascript; charset=utf-8', renderer]],
    ]);
    const source = sources.get(requestPath);
    if (request.method === 'GET' && source) {
      response.writeHead(200, { 'Content-Type': source[0], 'Cache-Control': 'no-store' });
      response.end(source[1]);
      return;
    }
    response.writeHead(404, { 'Content-Type': 'text/plain; charset=utf-8' });
    response.end('Not found.');
  });
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
  const origin = `http://127.0.0.1:${server.address().port}`;
  const tempDirectory = await mkdtemp(path.join(tmpdir(), 'taawun-webgpu-browser-'));
  let chromium;
  try {
    chromium = await launchChromium(browser, path.join(tempDirectory, 'profile'));
    const { client } = chromium;
    await client.send('Page.enable');
    await client.send('Runtime.enable');
    await client.send('Emulation.setDeviceMetricsOverride', { width: 1_200, height: 900, deviceScaleFactor: 1, mobile: false });
    await client.send('Page.addScriptToEvaluateOnNewDocument', { source: `(() => {
      let loseDevice;
      const lost = new Promise((resolve) => { loseDevice = resolve; });
      window.__gpuQA = { adapterRequests: 0, submissions: 0, destroys: 0, nullAdapter: false, lose: () => loseDevice({ reason: 'destroyed' }) };
      Object.defineProperty(navigator, 'hardwareConcurrency', { configurable: true, value: 8 });
      Object.defineProperty(navigator, 'deviceMemory', { configurable: true, value: 8 });
      const connectionListeners = [];
      const connection = { saveData: false, addEventListener(type, listener) { if (type === 'change') connectionListeners.push(listener); } };
      window.__setSaveData = (value) => { connection.saveData = value; connectionListeners.forEach((listener) => listener()); };
      Object.defineProperty(navigator, 'connection', { configurable: true, value: connection });
      const pass = { setPipeline() {}, setBindGroup() {}, draw() {}, end() {} };
      const device = {
        lost,
        queue: { writeBuffer() {}, submit() { window.__gpuQA.submissions += 1; } },
        createShaderModule() { return { getCompilationInfo: async () => ({ messages: [] }) }; },
        createRenderPipeline() { return { getBindGroupLayout() { return {}; } }; },
        createBuffer() { return { destroy() {} }; },
        createBindGroup() { return {}; },
        createCommandEncoder() { return { beginRenderPass() { return pass; }, finish() { return {}; } }; },
        destroy() { window.__gpuQA.destroys += 1; },
      };
      Object.defineProperty(navigator, 'gpu', { configurable: true, value: {
        async requestAdapter() { window.__gpuQA.adapterRequests += 1; if (window.__gpuQA.nullAdapter) return null; return { isFallbackAdapter: false, async requestDevice() { return device; } }; },
        getPreferredCanvasFormat() { return 'bgra8unorm'; },
      } });
      Object.defineProperty(window, 'GPUBufferUsage', { configurable: true, value: { UNIFORM: 1, COPY_DST: 2 } });
      const original = HTMLCanvasElement.prototype.getContext;
      HTMLCanvasElement.prototype.getContext = function(type, ...args) {
        if (type === 'webgpu') return { configure() {}, getCurrentTexture() { return { createView() { return {}; } }; } };
        return original.call(this, type, ...args);
      };
      window.requestIdleCallback = (callback) => window.setTimeout(() => callback({ didTimeout: false, timeRemaining: () => 50 }), 0);
      window.cancelIdleCallback = (id) => window.clearTimeout(id);
    })();` });
    await client.send('Page.navigate', { url: origin });
    await waitFor(() => evaluate(client, `document.querySelector('.geometry-stage')?.dataset.enhanced === 'true' && !document.querySelector('#motionToggle').hidden`), 'mocked WebGPU enhancement');
    const initial = await evaluate(client, `({ requests: window.__gpuQA.adapterRequests, submissions: window.__gpuQA.submissions, renderer: document.querySelector('#geometryCanvas').dataset.renderer || '', controlHeight: Math.round(document.querySelector('#motionToggle').getBoundingClientRect().height), state: document.querySelector('.geometry-stage').dataset.state, side: document.querySelector('.geometry-stage').dataset.side })`);
    assert.equal(initial.requests, 1);
    assert.ok(initial.submissions >= 1);
    assert.equal(initial.renderer, 'webgpu');
    assert.ok(initial.controlHeight >= 44);
    assert.equal(initial.state, '0');
    assert.equal(initial.side, 'right');

    await evaluate(client, `document.querySelector('#motionToggle').click()`);
    await waitFor(() => evaluate(client, `!document.querySelector('.geometry-stage').hasAttribute('data-enhanced') && !document.querySelector('#motionToggle').hidden && document.querySelector('#motionToggle').getAttribute('aria-pressed') === 'false'`), 'reversible user pause');
    assert.equal(await evaluate(client, `localStorage.getItem('taawun-decorative-motion')`), 'off');
    await evaluate(client, `document.querySelector('#motionToggle').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('.geometry-stage')?.dataset.enhanced === 'true' && document.querySelector('#motionToggle').getAttribute('aria-pressed') === 'true'`), 'motion resume');
    assert.equal(await evaluate(client, `localStorage.getItem('taawun-decorative-motion')`), null);

    const beforeScroll = await evaluate(client, `window.__gpuQA.submissions`);
    await evaluate(client, `window.scrollTo({ top: window.innerHeight * 1.2, behavior: 'instant' })`);
    await waitFor(() => evaluate(client, `window.__gpuQA.submissions > ${beforeScroll}`), 'scroll-threshold redraw');
    await waitFor(() => evaluate(client, `document.querySelector('.geometry-stage').dataset.state !== '0'`), 'approaching-section state transition');
    await evaluate(client, `window.__gpuQA.lose()`);
    await waitFor(() => evaluate(client, `!document.querySelector('.geometry-stage').hasAttribute('data-enhanced') && !document.querySelector('#motionToggle').hidden && document.querySelector('#motionToggle').getAttribute('aria-pressed') === 'true'`), 'device-loss static fallback');
    await evaluate(client, `window.__gpuQA.nullAdapter = true; document.querySelector('#motionToggle').click(); document.querySelector('#motionToggle').click()`);
    await waitFor(() => evaluate(client, `!document.querySelector('.geometry-stage').hasAttribute('data-enhanced') && !document.querySelector('#motionToggle').hidden && document.querySelector('#motionToggle').getAttribute('aria-pressed') === 'true' && window.__gpuQA.adapterRequests >= 3`), 'null-adapter static fallback keeps its motion control');

    await client.send('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-reduced-motion', value: 'reduce' }] });
    await client.send('Page.navigate', { url: origin });
    await waitFor(() => evaluate(client, `document.readyState === 'complete'`), 'reduced-motion landing');
    await wait(200);
    const reduced = await evaluate(client, `({ requests: window.__gpuQA.adapterRequests, canvasDisplay: getComputedStyle(document.querySelector('#geometryCanvas')).display, controlHidden: document.querySelector('#motionToggle').hidden })`);
    assert.equal(reduced.requests, 0, 'reduced motion must not request a GPU adapter');
    assert.equal(reduced.canvasDisplay, 'none');
    assert.equal(reduced.controlHidden, true);

    await client.send('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-reduced-motion', value: 'no-preference' }, { name: 'forced-colors', value: 'active' }] });
    await client.send('Page.navigate', { url: origin });
    await waitFor(() => evaluate(client, `document.readyState === 'complete'`), 'forced-colors landing');
    await wait(200);
    const forced = await evaluate(client, `({ requests: window.__gpuQA.adapterRequests, stageDisplay: getComputedStyle(document.querySelector('.geometry-stage')).display })`);
    assert.equal(forced.requests, 0, 'forced colors must not request a GPU adapter');
    assert.equal(forced.stageDisplay, 'none');

    await client.send('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-reduced-motion', value: 'no-preference' }, { name: 'forced-colors', value: 'none' }] });
    await client.send('Page.navigate', { url: origin });
    await waitFor(() => evaluate(client, `document.querySelector('.geometry-stage')?.dataset.enhanced === 'true'`), 'save-data live gate setup');
    await evaluate(client, `window.__setSaveData(true)`);
    await waitFor(() => evaluate(client, `!document.querySelector('.geometry-stage').hasAttribute('data-enhanced') && !document.querySelector('#motionToggle').hidden && document.querySelector('#motionToggle').getAttribute('aria-pressed') === 'true'`), 'save-data live fallback');
  } finally {
    await closeChromium(chromium, 'webgpu-lifecycle');
    server.closeAllConnections?.();
    await new Promise((resolve) => server.close(resolve));
    await rm(tempDirectory, { recursive: true, force: true, maxRetries: 10, retryDelay: 100 });
  }
});

test('account hash registration stays on the account document and signs out to login', { timeout: 90_000 }, async () => {
  const browser = await installedChromium();
  assert.ok(browser, 'Chromium is required; account-route continuity cannot be skipped');
  const cockpitHTML = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const user = { id: 901, username: 'Route QA', email: 'route@example.test', role: 'user' };
  const requests = [];
  const server = createServer(async (request, response) => {
    const chunks = [];
    for await (const chunk of request) chunks.push(chunk);
    const requestPath = new URL(request.url, 'http://localhost').pathname;
    requests.push(`${request.method} ${requestPath}`);
    const send = (status, type, payload) => { response.writeHead(status, { 'Content-Type': type, 'Cache-Control': 'no-store' }); response.end(payload); };
    const json = (status, payload) => send(status, 'application/json; charset=utf-8', JSON.stringify(payload));
    if (request.method === 'GET' && requestPath === '/account') return send(200, 'text/html; charset=utf-8', cockpitHTML);
    if (request.method === 'POST' && requestPath === '/api/register') return json(201, { user });
    if (request.method === 'POST' && requestPath === '/api/login') return json(200, { token: 'route-token', user });
    if (request.method === 'GET' && requestPath === '/api/profile') return json(200, user);
    if (request.method === 'GET' && requestPath === '/api/workspaces') return json(200, { workspaces: [] });
    if (request.method === 'GET' && requestPath === '/api/templates') return json(200, { templates: [] });
    if (request.method === 'GET' && requestPath === '/api/modules') return json(200, { modules: [], componentDocumentPolicy: { version: 2, fields: { maxComponents: 12 } } });
    if (request.method === 'GET' && requestPath === '/api/bazaar/listings') return json(200, { listings: [] });
    return json(404, { error: { code: 'not_found', message: 'Not found.' } });
  });
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
  const origin = `http://127.0.0.1:${server.address().port}`;
  const tempDirectory = await mkdtemp(path.join(tmpdir(), 'taawun-account-route-browser-'));
  let chromium;
  try {
    chromium = await launchChromium(browser, path.join(tempDirectory, 'profile'));
    const { client } = chromium;
    await client.send('Page.enable');
    await client.send('Runtime.enable');
    await client.send('Page.navigate', { url: `${origin}/account#register` });
    await waitFor(() => evaluate(client, `document.readyState === 'complete' && !document.querySelector('#registerForm').hidden`), 'register hash selects registration');
    await evaluate(client, `(() => {
      const set = (id, value) => { const input = document.getElementById(id); input.value = value; input.dispatchEvent(new Event('input', { bubbles: true })); };
      set('registerUsername', 'Route QA'); set('registerEmail', 'route@example.test'); set('registerPassword', 'correct horse battery staple'); document.querySelector('#registerForm').requestSubmit();
    })()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#appView').hidden && location.pathname === '/account' && location.hash === ''`), 'registration auto-login preserves account document');
    assert.deepEqual(requests.filter((entry) => entry === 'POST /api/register' || entry === 'POST /api/login'), ['POST /api/register', 'POST /api/login']);
    await evaluate(client, `document.querySelector('#logoutButton').click()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#authView').hidden && !document.querySelector('#loginForm').hidden && document.querySelector('#registerForm').hidden`), 'sign-out normalizes to login');
  } finally {
    await closeChromium(chromium, 'account-route-continuity');
    server.closeAllConnections?.();
    await new Promise((resolve) => server.close(resolve));
    await rm(tempDirectory, { recursive: true, force: true, maxRetries: 10, retryDelay: 100 });
  }
});

test('publication expiry and successor trust stay server-clock and principal bound', async () => {
  const html = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const publicationClock = html.match(/function normalizePublicationContextEnvelope[\s\S]*?(?=\n\s*function resetPublicationContextUI)/u)?.[0] || '';
  const historyLoader = html.match(/async function loadDomainPublications[\s\S]*?(?=\n\s*async function selectDomainPublication)/u)?.[0] || '';
  const successorGuard = html.match(/function requireReplacementSuccessorBinding[\s\S]*?(?=\n\s*function startPublicationAction)/u)?.[0] || '';

  assert.match(publicationClock, /value\.serverTime/u);
  assert.match(publicationClock, /envelope\.requestStartedAt \+ Math\.max\(0, signedRemaining\)/u);
  assert.match(publicationClock, /performance\.now\(\)/u);
  assert.doesNotMatch(publicationClock, /Date\.now/u);
  assert.match(publicationClock, /active\.length > 1/u);
  assert.match(publicationClock, /append && !appendedRows\.some/u);
  assert.match(publicationClock, /function revalidatePublicationDeadlineAfterResume/u);
  assert.doesNotMatch(historyLoader, /clearPublicationDeadline\(/u);
  assert.match(historyLoader, /reconcilePublicationDeadline\(publications, envelope, \{ append, appendedRows \}\)/u);
  assert.match(html, /visibilityState === 'visible'\) revalidatePublicationDeadlineAfterResume\(\)/u);
  assert.match(successorGuard, /Number\(newTrack\.createdBy\) !== principalID/u);
  assert.match(successorGuard, /String\(signedSubject\.id \|\| ''\) !== subjectID/u);
  assert.match(successorGuard, /exactOriginPolicyMatch\(newTrack\.request\.requestedOrigins, requestedPolicy\)/u);
  assert.match(successorGuard, /exactOriginPolicyMatch\(preview\.allowedOrigins, requestedPolicy\)/u);
  assert.match(successorGuard, /exactOriginPolicyMatch\(authorization\.allowedOrigins, requestedPolicy\)/u);
});

test('registration enforces the server password minimum', async () => {
  const html = await readFile(new URL('./index.html', import.meta.url), 'utf8');

  assert.match(html, /id="registerPassword"[^>]*minlength="12"/u);
  assert.match(html, /id="registerPassword"[^>]*aria-describedby="registerPasswordHelp"/u);
  assert.match(html, /Use at least 12 characters\./u);
  assert.doesNotMatch(html, /Use at least 8 characters\.|id="registerPassword"[^>]*minlength="8"/u);
});

test('bounded UI safety mutations stay self/workspace scoped and version guarded', async () => {
  const html = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const memberRemoval = html.match(/async function removeSelectedMember[\s\S]*?(?=\n\s*const RESUMABLE_TRACK_STATUSES)/u)?.[0] || '';
  const invitationRevoke = html.match(/async function revokeSelectedInvitation[\s\S]*?(?=\n\s*async function createInvitation)/u)?.[0] || '';
  const domainRevoke = html.match(/async function revokeSelectedDomain[\s\S]*?(?=\n\s*function selectedModules)/u)?.[0] || '';
  const accountLifecycle = html.match(/function accountMarker[\s\S]*?(?=\n\s*const authTabs)/u)?.[0] || '';

  assert.doesNotMatch(html, /Enter the builder/u);
  assert.match(html, /Events and registrations/u);
  assert.match(html, /Community information/u);
  assert.match(html, /People and decisions/u);
  assert.match(html, /Cooperative planning/u);
  assert.match(html, /id="revokeDomainButton"[^>]*>Revoke domain</u);
  assert.match(memberRemoval, /\/workspaces\/\$\{action\.marker\.workspaceID\}\/users\/\$\{action\.userID\}/u);
  assert.match(html, /Number\(member\.user_id\) !== principalID\(\)/u);
  assert.match(html, /String\(member\.role \|\| ''\)\.toLowerCase\(\) !== 'owner'/u);
  assert.match(invitationRevoke, /body: \{ expected_version: action\.version \}/u);
  assert.match(invitationRevoke, /String\(revoked\.status \|\| ''\)\.toUpperCase\(\) !== 'REVOKED'/u);
  assert.ok(domainRevoke.indexOf("{ method: 'DELETE' }") < domainRevoke.indexOf('const readback = await api(path)'), 'domain revoke must issue DELETE before exact claim readback');
  assert.equal((domainRevoke.match(/await api\(path, \{ method: 'DELETE' \}\)/gu) || []).length, 1, 'domain revoke issues one DELETE');
  assert.doesNotMatch(accountLifecycle, /\/admin\//u);
  assert.match(accountLifecycle, /api\(`\/users\/\$\{marker\.userID\}`.*method: 'PUT'/u);
  assert.match(accountLifecycle, /api\(`\/users\/\$\{marker\.userID\}`.*method: 'DELETE'/u);
  assert.match(accountLifecycle, /error\.code === 'owned_workspaces_remaining'/u);
  assert.match(html, /\.button\.tertiary \{ min-height: 44px; min-width: 44px;/u);
  assert.match(html, /details summary \{ min-height: 44px; min-width: 44px;/u);
});

test('successful registration signs in with ephemeral local credentials', async () => {
  const html = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const handler = html.match(/element\('registerForm'\)\.addEventListener\('submit',[\s\S]*?(?=\n\s*element\('logoutButton'\))/u)?.[0];
  const showAuth = html.match(/function showAuth\([\s\S]*?(?=\n\s*async function enterBuilder)/u)?.[0] || '';

  assert.ok(handler, 'registration submit handler must exist');
  assert.match(handler, /const email = element\('registerEmail'\)\.value\.trim\(\);/u);
  assert.match(handler, /let password = element\('registerPassword'\)\.value;/u);
  assert.match(handler, /body: \{ username, email, password \}/u);
  assert.match(handler, /body: \{ email, password \}/u);
  assert.ok(handler.indexOf("await api('/register'") < handler.indexOf("await api('/login'"), 'registration must complete before automatic sign-in');
  assert.ok(handler.indexOf("element('registerForm').reset()") < handler.indexOf("await api('/login'"), 'the password field must be cleared before automatic sign-in');
  assert.ok(handler.indexOf("password = ''") > handler.indexOf("await api('/login'"), 'the local password must be cleared after automatic sign-in');
  assert.match(handler, /if \(accountCreated\) \{[\s\S]*?showAuth\(\);[\s\S]*?switchAuthTab\('login'\);[\s\S]*?element\('loginPassword'\)\.value = '';/u);
  assert.match(handler, /finally \{\s*password = '';/u);
  assert.doesNotMatch(handler, /localStorage|sessionStorage|document\.cookie/u);
  assert.ok(showAuth.indexOf("switchAuthTab('login')") < showAuth.indexOf("setNotice(element('authAlert')"), 'all sign-out and expiry paths must select login before preserving their notice');
});

test('public landing and account shell remain distinct, responsive, and keyboard accessible', async () => {
  const browser = await installedChromium();
  assert.ok(browser, 'Chromium is required; the mobile accessibility regression cannot be skipped');

  const cockpitHTML = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const landingHTML = await readFile(new URL('./landing.html', import.meta.url), 'utf8');
  const landingScript = await readFile(new URL('./geometric-landing.js', import.meta.url), 'utf8');
  const server = createServer((request, response) => {
    const requestPath = new URL(request.url, 'http://localhost').pathname;
    if (request.method === 'GET' && requestPath === '/') {
      response.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8', 'Cache-Control': 'no-store' });
      response.end(landingHTML);
      return;
    }
    if (request.method === 'GET' && requestPath === '/geometric-landing.js') {
      response.writeHead(200, { 'Content-Type': 'text/javascript; charset=utf-8', 'Cache-Control': 'no-store' });
      response.end(landingScript);
      return;
    }
    if (request.method === 'GET' && requestPath === '/account') {
      response.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8', 'Cache-Control': 'no-store', 'X-Robots-Tag': 'noindex, nofollow, noarchive' });
      response.end(cockpitHTML);
      return;
    }
    response.writeHead(404, { 'Content-Type': 'text/plain; charset=utf-8' });
    response.end('Not found.');
  });
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const origin = `http://127.0.0.1:${server.address().port}`;
  const tempDirectory = await mkdtemp(path.join(tmpdir(), 'taawun-auth-browser-'));
  let chromium;
  try {
    chromium = await launchChromium(browser, path.join(tempDirectory, 'profile'));
    const { client } = chromium;
    await client.send('Page.enable');
    await client.send('Runtime.enable');
    await client.send('Emulation.setDeviceMetricsOverride', {
      width: 400,
      height: 1_000,
      deviceScaleFactor: 1,
      mobile: true,
    });
    await client.send('Page.navigate', { url: origin });
    await waitFor(() => evaluate(client, `document.readyState === 'complete' && Boolean(document.querySelector('#heroTitle'))`), 'mobile public landing');
    const landingSemantics = await evaluate(client, `(() => ({
      h1: document.querySelector('#heroTitle')?.innerText.trim(),
      visibleH1s: [...document.querySelectorAll('h1')].filter((heading) => getComputedStyle(heading).display !== 'none').length,
      registerHref: document.querySelector('a[href="/account#register"]')?.getAttribute('href'),
      loginHref: document.querySelector('a[href="/account#login"]')?.getAttribute('href'),
      canvasHidden: document.querySelector('#geometryCanvas')?.parentElement?.getAttribute('aria-hidden'),
      geometryStates: document.querySelectorAll('[data-geometry-state]').length,
      geometrySides: [...document.querySelectorAll('[data-geometry-state]')].map((section) => section.dataset.geometrySide),
      hasAuthForm: Boolean(document.querySelector('#loginForm, #registerForm, #appView')),
      copy: document.querySelector('main').innerText.replace(/\\s+/g, ' ').trim(),
      overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
      controlHeights: [...document.querySelectorAll('a.button, .nav-account, .nav-control:not([hidden])')].filter((item) => item.getClientRects().length > 0).map((item) => Math.round(item.getBoundingClientRect().height)),
      menuHidden: document.querySelector('#menuToggle').hidden,
      menuExpanded: document.querySelector('#menuToggle').getAttribute('aria-expanded'),
      motionHidden: document.querySelector('#motionToggle').hidden,
      motionPressed: document.querySelector('#motionToggle').getAttribute('aria-pressed'),
      staticEdgePeek: (() => { const rect = document.querySelector('.geometry-static').getBoundingClientRect(); return rect.left < 0 || rect.right > innerWidth; })(),
    }))()`);
    assert.equal(landingSemantics.h1, 'Many hands. One clear community effort.');
    assert.equal(landingSemantics.visibleH1s, 1);
    assert.equal(landingSemantics.registerHref, '/account#register');
    assert.equal(landingSemantics.loginHref, '/account#login');
    assert.equal(landingSemantics.canvasHidden, 'true');
    assert.equal(landingSemantics.geometryStates, 6);
    assert.deepEqual(landingSemantics.geometrySides, ['right', 'right', 'left', 'right', 'right', 'left']);
    assert.equal(landingSemantics.hasAuthForm, false);
    assert.match(landingSemantics.copy, /does not hold funds or settle payments/iu);
    assert.match(landingSemantics.copy, /community-owned web address/iu);
    assert.equal(landingSemantics.overflow, false);
    assert.equal(landingSemantics.menuHidden, false);
    assert.equal(landingSemantics.menuExpanded, 'false');
    assert.equal(landingSemantics.motionHidden, false);
    assert.equal(landingSemantics.motionPressed, 'true');
    assert.equal(landingSemantics.staticEdgePeek, true);
    assert.ok(landingSemantics.controlHeights.every((height) => height >= 44), `landing controls must be at least 44 CSS px: ${landingSemantics.controlHeights}`);

    await evaluate(client, `document.querySelector('#menuToggle').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#menuToggle').getAttribute('aria-expanded') === 'true' && getComputedStyle(document.querySelector('#primaryNavigation')).display !== 'none' && document.activeElement === document.querySelector('#primaryNavigation a')`), 'mobile navigation opens into forward focus order');
    await pressKey(client, 'Escape', 'Escape', 27);
    await waitFor(() => evaluate(client, `document.querySelector('#menuToggle').getAttribute('aria-expanded') === 'false' && document.activeElement === document.querySelector('#menuToggle')`), 'Escape closes mobile navigation and restores focus');

    for (const layoutWidth of [160, 200, 320, 400]) {
      await client.send('Emulation.setDeviceMetricsOverride', { width: layoutWidth, height: 1_000, deviceScaleFactor: 1, mobile: false });
      await waitFor(() => evaluate(client, `window.innerWidth === ${layoutWidth}`), `${layoutWidth}px public landing`);
      const layout = await evaluate(client, `({
        overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
        visibleH1s: [...document.querySelectorAll('h1')].filter((heading) => getComputedStyle(heading).display !== 'none').length,
        canvasEnhanced: document.querySelector('.geometry-stage').hasAttribute('data-enhanced'),
        navOverlap: document.querySelector('.brand').getBoundingClientRect().right > document.querySelector('.nav-actions').getBoundingClientRect().left,
        targets: [...document.querySelectorAll('a.button, .nav-account, .nav-control:not([hidden])')].filter((item) => item.getClientRects().length > 0).map((item) => Math.round(item.getBoundingClientRect().height)),
      })`);
      assert.equal(layout.overflow, false, `${layoutWidth}px public landing must not overflow horizontally`);
      assert.equal(layout.visibleH1s, 1, `${layoutWidth}px public landing must keep one H1`);
      assert.equal(layout.canvasEnhanced, false, `${layoutWidth}px public landing must remain static`);
      assert.equal(layout.navOverlap, false, `${layoutWidth}px public brand and controls must not overlap`);
      assert.ok(layout.targets.every((height) => height >= 44), `${layoutWidth}px public targets must remain at least 44px: ${layout.targets}`);
    }

    await client.send('Page.navigate', { url: `${origin}/account#login` });
    await waitFor(() => evaluate(client, `document.readyState === 'complete' && !document.querySelector('#authView').hidden`), 'mobile auth shell');

    const semantics = await evaluate(client, `(() => {
      const skip = document.querySelector('#skipLink');
      const target = document.querySelector(skip.hash);
      return {
        skipHref: skip.getAttribute('href'),
        skipLabel: skip.textContent.trim(),
        targetVisible: Boolean(target && !target.closest('[hidden]') && getComputedStyle(target).display !== 'none'),
        loginTabIndex: document.querySelector('#loginTab').tabIndex,
        registerTabIndex: document.querySelector('#registerTab').tabIndex,
        loginPanelRole: document.querySelector('#loginForm').getAttribute('role'),
        registerPanelRole: document.querySelector('#registerForm').getAttribute('role'),
        passwordDescription: document.querySelector('#registerPassword').getAttribute('aria-describedby'),
        h1Text: document.querySelector('#authView h1')?.innerText.trim(),
        navTargetsValid: [...document.querySelectorAll('.landing-nav a[href^="#"]')].every((link) => Boolean(document.querySelector(link.hash))),
        mainSectionCount: document.querySelectorAll('#authView > section').length,
        authNoSnippet: document.querySelector('#authPanel').hasAttribute('data-nosnippet'),
      };
    })()`);
    assert.deepEqual(semantics, {
      skipHref: '#authPanel',
      skipLabel: 'Skip to account access',
      targetVisible: true,
      loginTabIndex: 0,
      registerTabIndex: -1,
      loginPanelRole: 'tabpanel',
      registerPanelRole: 'tabpanel',
      passwordDescription: 'registerPasswordHelp',
      h1Text: 'Continue your community work.',
      navTargetsValid: true,
      mainSectionCount: 9,
      authNoSnippet: true,
    });

    await evaluate(client, `document.querySelector('#skipLink').focus()`);
    await pressKey(client, 'Enter', 'Enter', 13);
    await waitFor(() => evaluate(client, `document.activeElement?.id === 'authPanel'`), 'logged-out skip destination');

    await evaluate(client, `document.querySelector('#loginTab').focus()`);
    await pressKey(client, 'ArrowRight', 'ArrowRight', 39);
    const registerState = await evaluate(client, `(() => ({
      active: document.activeElement?.id,
      loginSelected: document.querySelector('#loginTab').getAttribute('aria-selected'),
      registerSelected: document.querySelector('#registerTab').getAttribute('aria-selected'),
      loginTabIndex: document.querySelector('#loginTab').tabIndex,
      registerTabIndex: document.querySelector('#registerTab').tabIndex,
      loginHidden: document.querySelector('#loginForm').hidden,
      registerHidden: document.querySelector('#registerForm').hidden,
    }))()`);
    assert.deepEqual(registerState, {
      active: 'registerTab',
      loginSelected: 'false',
      registerSelected: 'true',
      loginTabIndex: -1,
      registerTabIndex: 0,
      loginHidden: true,
      registerHidden: false,
    });
    await pressKey(client, 'ArrowLeft', 'ArrowLeft', 37);
    assert.equal(await evaluate(client, `document.activeElement?.id`), 'loginTab');
    assert.equal(await evaluate(client, `document.querySelector('#loginForm').hidden`), false);

    for (const width of [320, 400]) {
      for (const scale of [1, 2]) {
        const layoutWidth = Math.round(width / scale);
        await client.send('Emulation.setDeviceMetricsOverride', {
          width: layoutWidth,
          height: 1_000,
          deviceScaleFactor: 1,
          mobile: false,
        });
        await client.send('Emulation.setPageScaleFactor', { pageScaleFactor: 1 });
        await waitFor(() => evaluate(client, `window.innerWidth === ${layoutWidth}`), `${width}px viewport at ${scale * 100}% reflow`);
        const authLayout = await evaluate(client, `(() => {
          document.querySelector('#authView').hidden = false;
          document.querySelector('#appView').hidden = true;
          const copy = document.querySelector('#authPanel');
          const style = getComputedStyle(copy);
          const rect = copy.getBoundingClientRect();
          return {
            copy: copy.innerText.replace(/\\s+/g, ' ').trim(),
            visible: style.display !== 'none' && style.visibility !== 'hidden' && rect.width > 0 && rect.height > 0,
            overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
            overflowElements: [...document.querySelectorAll('body *')].filter((candidate) => {
              const candidateRect = candidate.getBoundingClientRect();
              return candidateRect.right > document.documentElement.clientWidth + 0.5 || candidateRect.left < -0.5;
            }).slice(0, 8).map((candidate) => candidate.tagName.toLowerCase() + '#' + candidate.id + '.' + candidate.className),
            layoutWidth: window.innerWidth,
            visibleH1s: [...document.querySelectorAll('h1')].filter((heading) => !heading.closest('[hidden]') && getComputedStyle(heading).display !== 'none').length,
            controlHeights: ['skipLink', 'landingBrand', 'loginTab', 'registerTab', 'loginEmail', 'loginPassword'].map((id) => Math.round(document.getElementById(id).getBoundingClientRect().height)),
          };
        })()`);
        assert.match(authLayout.copy, /Sign in or create an account/u);
        assert.match(authLayout.copy, /session stays in this browser tab/u);
        assert.equal(authLayout.visible, true, `${width}px at ${scale * 100}% must show account access`);
        assert.equal(authLayout.overflow, false, `${width}px at ${scale * 100}% must reflow without horizontal document overflow: ${authLayout.overflowElements.join(', ')}`);
        assert.equal(authLayout.layoutWidth, layoutWidth, `${width}px at ${scale * 100}% must use the expected reflow width`);
        assert.equal(authLayout.visibleH1s, 1, `${width}px at ${scale * 100}% must expose one visible account H1`);
        assert.ok(authLayout.controlHeights.every((height) => height >= 44), `${width}px at ${scale * 100}% account controls must be at least 44 CSS px: ${authLayout.controlHeights}`);

        const railLayout = await evaluate(client, `(() => {
          document.querySelector('#authView').hidden = true;
          document.querySelector('#appView').hidden = false;
          const boundary = document.querySelector('.rail-boundary');
          const style = getComputedStyle(boundary);
          const rect = boundary.getBoundingClientRect();
          return {
            text: boundary.innerText.replace(/\\s+/g, ' ').trim(),
            visible: style.display !== 'none' && style.visibility !== 'hidden' && rect.width > 0 && rect.height > 0,
            overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
            overflowElements: [...document.querySelectorAll('body *')].filter((candidate) => {
              const candidateRect = candidate.getBoundingClientRect();
              return candidateRect.right > document.documentElement.clientWidth + 0.5 || candidateRect.left < -0.5;
            }).slice(0, 8).map((candidate) => candidate.tagName.toLowerCase() + '#' + candidate.id + '.' + candidate.className),
          };
        })()`);
        assert.match(railLayout.text, /Amanah boundary/iu);
        assert.match(railLayout.text, /browser-owned app/u);
        assert.equal(railLayout.visible, true, `${width}px at ${scale * 100}% must keep the Amanah boundary visible`);
        assert.equal(railLayout.overflow, false, `${width}px app shell at ${scale * 100}% must avoid horizontal document overflow: ${railLayout.overflowElements.join(', ')}`);
      }
    }
  } finally {
    await closeChromium(chromium, 'responsive-journey');
    server.closeAllConnections?.();
    await new Promise((resolve) => server.close(resolve));
    await rm(tempDirectory, { recursive: true, force: true, maxRetries: 10, retryDelay: 100 });
  }
});

test('customer cockpit renders an authenticated signed preview in the exact sandbox', { timeout: 90_000 }, async () => {
  const browser = await installedChromium();
  assert.ok(browser, 'Chromium is required; the cockpit browser regression cannot be skipped');

  const cockpitHTML = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const runtimeSource = await readFile(new URL('./assets/datastar-v1.0.2.js', import.meta.url), 'utf8');
  const signedRuntime = runtimeSource.replaceAll('\r\n', '\n');
  const publicRuntime = signedRuntime.replaceAll('\n', '\r\n');
  assert.match(signedRuntime, /\$&/u, 'the exact pinned runtime fixture must retain its literal $&');

  const token = 'browser-signed-preview-token';
  const secondToken = 'browser-second-principal-token';
  const removedMemberToken = 'browser-removed-member-token';
  const documentFields = [
    { key: 'title', label: 'Card heading', valueType: 'string', description: 'Curated heading.', required: true, maxLength: 120 },
    { key: 'summary', label: 'Summary', valueType: 'string', description: 'Curated plain-text summary.', required: true, maxLength: 600 },
  ];
  const reservedKeyParts = ['proto', 'prototype', 'constructor', 'script', 'html', 'css', 'style', 'endpoint', 'url', 'uri', 'origin', 'surface', 'embedder', 'connection', 'resource', 'authority', 'permission', 'capability', 'workspace', 'principal', 'subject', 'user', 'actor', 'signer', 'signature', 'auth', 'lifecycle', 'expiry', 'expires', 'shura', 'finance', 'payment', 'settlement', 'escrow', 'artifactid', 'contenthash', 'manifestdigest', 'contractversion', 'templateid', 'moduleid', 'componentid', 'token', 'password', 'credential', 'secret', 'cookie', 'apikey'];
  const componentPolicy = { contractVersion: 'taawun.artifact/v2', rootType: 'object', stableIdRule: 'one instance per allowed module; id equals type', keyGrammar: 'ASCII letter first', numberFormat: 'canonical base-10 JSON', allowedValueTypes: ['null', 'boolean', 'number', 'string', 'array', 'object'], reservedKeyParts, maxComponentBytes: 8192, maxTotalBytes: 32768, maxDepth: 6, maxKeyBytes: 64, maxObjectFields: 32, maxArrayItems: 32, maxTotalKeys: 128, maxStringRunes: 2048, maxNumberBytes: 64 };
  const previewPrefix = '/api/conductor/tracks/track_browser/preview/files/';
  const runtimeBundlePath = 'assets/datastar-v1.0.2.js';
  const signedRuntimePath = `${previewPrefix}${runtimeBundlePath}`;
  const runtimeDigest = createHash('sha256').update(Buffer.from(signedRuntime, 'utf8')).digest('hex');
  assert.notEqual(createHash('sha256').update(Buffer.from(publicRuntime, 'utf8')).digest('hex'), runtimeDigest, 'the CRLF public fixture must not match the canonical LF signed runtime digest');
  const previewDocument = `<!doctype html>
<html lang="en"><head>
  <meta charset="utf-8">
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'self' 'unsafe-eval'; style-src 'self'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'">
  <link rel="stylesheet" href="./theme.css">
  <link rel="stylesheet" href="./app.css">
  <script type="module" src="/assets/datastar-v1.0.2.js" integrity="sha256-browser-fixture"></script>
</head><body data-signals="{_notice: 'Signed community card ready'}">
  <main id="signed-card" data-artifact-signature="fixture-ed25519">
    <h1>Signed community card</h1>
    <p id="notice" data-text="$_notice">Fallback notice</p>
    <button id="card-control" type="button" data-on:click="$_notice = 'Card control activated'">Activate card</button>
  </main>
</body></html>`;
  const previewFiles = new Map([
    [`${previewPrefix}index.html`, ['text/html; charset=utf-8', previewDocument]],
    [`${previewPrefix}theme.css`, ['text/css; charset=utf-8', ':root{--taawun-color-accent:#57a68e}']],
    [`${previewPrefix}app.css`, ['text/css; charset=utf-8', 'body{margin:0}main{padding:2rem}']],
    [signedRuntimePath, ['text/javascript; charset=utf-8', signedRuntime]],
  ]);
  const manifestFiles = [...previewFiles.entries()].map(([url, [, contents]]) => ({
    path: url.slice(previewPrefix.length),
    sha256: createHash('sha256').update(Buffer.from(contents, 'utf8')).digest('hex'),
    bytes: Buffer.byteLength(contents, 'utf8'),
  }));
  const requests = [];
  const receiptVariants = [
    'active',
    'missing-attestation',
    'missing-authorization-state',
    'authorization-state-mismatch',
    'missing-digest',
    'digest-mismatch',
    'artifact',
    'content-hash',
    'workspace',
    'signature',
    'key',
    'algorithm',
    'subject-workspace',
    'authorization-key',
    'lifecycle',
    'expiry',
    'elapsed-expiry',
    'surfaces',
    'embedders',
    'connections',
    'resources',
    'component-data',
    'component-digest',
    'component-substitution-attested',
    'component-missing',
    'missing-fields',
    'track-missing',
    'track-id-missing',
    'track-workspace',
    'track-artifact',
    'track-hash',
    'track-preview-missing',
    'track-preview-artifact',
    'track-preview-hash',
    'track-preview-workspace',
    'track-preview-subject',
    'track-preview-origins',
    'track-preview-expiry',
    'track-preview-public',
  ];
  let previewBuilds = 0;
  let activeTrackResponse = null;
  let initialPeopleResponseBarrier = null;
  let delayPeopleResponse = false;
  let failPeopleNext = false;
  let historyFailNext = false;
  let domainFailNext = false;
  let delayDomainResponse = false;
  let delayPublicationExclusionDomainResponse = false;
  let delayWorkspaceCreateResponse = false;
  let delayScopeClaimResponse = false;
  let delayInvitationResponse = false;
  let delayMemberDeleteResponse = false;
  let memberDeleteAmbiguousNext = false;
  let delayInvitationRevokeResponse = false;
  let invitationRevokeFailNext = false;
  let delayAcceptanceResponse = false;
  let delayScopePublicationResponse = false;
  let releaseScopePublicationResponse = null;
  let includeInapplicableNewest = false;
  let delayHistoryResponse = false;
  let delaySecondPrincipalHistory = false;
  let resumeConflictOnce = true;
  let activationFailNext = true;
  let delayClaimResponse = false;
  let delayVerifyResponse = false;
  let delayPublicationRequest = false;
  let delayRollbackActivationResponse = false;
  let delayWorkspaceDeleteResponse = false;
  let workspaceDeleteAmbiguousNext = false;
  const trackEvents = new Map();
  const extraTracks = new Map();
  let workspaces = [{ id: 41, name: 'QA Community' }, { id: 42, name: 'QA Other Workspace' }, { id: 81, name: 'Second Principal Workspace' }, { id: 91, name: 'Retained Unrelated Workspace' }];
  const workspaceOwners = new Map([[41, new Set([7, 8])], [42, new Set([7, 8])], [81, new Set([8])], [91, new Set([99])]]);
  const workspaceMembers = new Map([
    [41, [
      { user_id: 7, username: 'QA Architect', role: 'owner', joined_at: '2026-08-18T00:00:00Z' },
      { user_id: 8, username: 'Removable Viewer', role: 'owner', joined_at: '2026-08-18T00:00:00Z' },
      { user_id: 9, username: 'Removable Viewer', role: 'viewer', joined_at: '2026-08-18T00:00:00Z' },
      { user_id: 10, username: 'Ambiguous Viewer', role: 'viewer', joined_at: '2026-08-18T00:00:00Z' },
    ]],
    [42, [
      { user_id: 7, username: 'QA Architect', role: 'owner', joined_at: '2026-08-18T00:00:00Z' },
      { user_id: 8, username: 'Second Architect', role: 'owner', joined_at: '2026-08-18T00:00:00Z' },
    ]],
    [81, [
      { user_id: 8, username: 'Second Architect', role: 'owner', joined_at: '2026-08-18T00:00:00Z' },
    ]],
  ]);
  let removedMemberActive = true;
  let invitationSequence = 0;
  const currentInvitations = new Map();
  let invitationAcceptRaceNext = false;
  let accountTokenInvalid = false;
  let accountDeleted = false;
  let secondAccountDeleted = false;
  let currentAccountPassword = 'correct horse battery staple';
  let delayPasswordUpdateResponse = false;
  let passwordUpdateAmbiguousNext = false;
  let delayAccountDeleteResponse = false;
  let accountDeleteCommitAmbiguousNext = false;
  const principalForAuthorization = (authorization) => authorization === `Bearer ${token}` ? 7 : authorization === `Bearer ${secondToken}` ? 8 : 0;
  const accountHasOwnedWorkspaces = (userID) => workspaces.some((workspace) => workspaceOwners.get(workspace.id)?.has(userID));
  let domainRevokeFailNext = false;
  let delayDomainRevokeResponse = false;
  let nextWorkspaceID = 43;
  let domainClaims = [
    { id: 'claim_browser', workspaceId: 41, origin: 'https://app.community.example', host: 'app.community.example', status: 'verified', challengeExpiresAt: '2099-08-17T00:00:00Z', verifiedAt: '2026-08-18T00:00:00Z', verificationExpiresAt: '2099-08-18T00:00:00Z', createdAt: '2026-08-18T00:00:00Z', updatedAt: '2026-08-18T00:00:00Z' },
    { id: 'claim_pending', workspaceId: 41, origin: 'https://pending.community.example', host: 'pending.community.example', status: 'pending', challengeExpiresAt: '2099-08-18T12:00:00Z', createdAt: '2026-08-18T00:10:00Z', updatedAt: '2026-08-18T00:10:00Z' },
    { id: 'claim_pending_revoke', workspaceId: 41, origin: 'https://pending-revoke.community.example', host: 'pending-revoke.community.example', status: 'pending', challengeExpiresAt: '2099-08-18T12:00:00Z', createdAt: '2026-08-18T00:11:00Z', updatedAt: '2026-08-18T00:11:00Z' },
    { id: 'claim_revoked', workspaceId: 41, origin: 'https://revoked.community.example', host: 'revoked.community.example', status: 'revoked', revocationReason: 'user', challengeExpiresAt: '2026-08-18T00:00:00Z', createdAt: '2026-08-18T00:20:00Z', updatedAt: '2026-08-18T00:20:00Z' },
    { id: 'claim_expired', workspaceId: 41, origin: 'https://expired.community.example', host: 'expired.community.example', status: 'revoked', revocationReason: 'expired', challengeExpiresAt: '2026-08-18T00:00:00Z', createdAt: '2026-08-18T00:30:00Z', updatedAt: '2026-08-18T00:30:00Z' },
  ];
  let domainPublications = [];
  let replacementTrackResponse = null;
  let delayPublicationContextResponse = false;
  let publicationContextFailNext = false;
  let replacementPreviewFailAfterCommit = false;
  let delayReplacementSuccessResponse = false;
  let replacementSuccessorVariant = '';
  let publicationServerTime = '2026-08-18T06:02:00Z';
  let automaticPublicationExpiry = false;
  let automaticPublicationExactReads = 0;
  const publicationContextRow = (publication, exact = false) => ({
    id: publication.id,
    workspaceId: publication.workspaceId,
    claimId: publication.claimId,
    origin: publication.origin,
    contentHash: publication.contentHash,
    artifactId: publication.artifactId,
    ...(publication.sourcePublicationId ? { sourcePublicationId: publication.sourcePublicationId } : {}),
    activatedAt: publication.activatedAt,
    ...(publication.deactivatedAt ? { deactivatedAt: publication.deactivatedAt } : {}),
    active: publication.active,
    authorizationExpiresAt: publication.active || exact ? publication.authorizationExpiresAt : null,
    manifestDigest: publication.active || exact ? publication.manifestDigest : null,
    servingState: publication.active || exact ? publication.servingState : 'inactive',
    trackBinding: publication.trackBinding || null,
  });
  let failNextPreview = false;
  let delayTrackResponse = false;
  let incompleteCatalogNext = false;
  let delayPreviewResponse = false;
  let delaySignedRuntimeResponse = false;
  let corruptSignedFileNext = '';
  let lengthMismatchSignedFileNext = '';
  let missingSignedFileNext = '';
  let signedFileVariantNext = '';
  const server = createServer(async (request, response) => {
    const chunks = [];
    for await (const chunk of request) chunks.push(chunk);
    const rawBody = Buffer.concat(chunks).toString('utf8');
    const requestURL = new URL(request.url, 'http://localhost');
    const record = {
      method: request.method,
      path: requestURL.pathname,
      search: requestURL.search,
      authorization: request.headers.authorization || '',
      cacheControl: request.headers['cache-control'] || '',
      body: rawBody ? JSON.parse(rawBody) : null,
    };
    requests.push(record);
    const send = (status, contentType, body) => {
      response.writeHead(status, { 'Content-Type': contentType, 'Cache-Control': 'no-store' });
      response.end(body);
    };
    const json = (status, body) => send(status, 'application/json; charset=utf-8', JSON.stringify(body));

    if (record.method === 'GET' && record.path === '/account') return send(200, 'text/html; charset=utf-8', cockpitHTML);
    if (record.method === 'GET' && record.path === '/public-host') {
      const serving = request.headers.host === 'app.community.example'
        && domainPublications.some((publication) => publication.claimId === 'claim_browser' && publication.active && publication.servingState === 'serving');
      return serving ? send(200, 'text/html; charset=utf-8', '<h1>Public community app</h1>') : json(404, { error: { code: 'public_host_not_found' } });
    }
    if (record.method === 'GET' && record.path === '/assets/datastar-v1.0.2.js') {
      return send(200, 'text/javascript; charset=utf-8', publicRuntime);
    }
    if (record.method === 'POST' && record.path === '/api/login') {
      const second = record.body?.email === 'second@example.test';
      const valid = second
        ? record.body?.password === 'correct horse battery staple' && !secondAccountDeleted
        : record.body?.email === 'qa@example.test' && record.body?.password === currentAccountPassword && !accountDeleted;
      if (!valid) return json(401, { error: { code: 'unauthorized', message: 'Authentication failed.' } });
      if (!second) accountTokenInvalid = false;
      return json(200, { token: second ? secondToken : token, user: { id: second ? 8 : 7, username: second ? 'Second Architect' : 'QA Architect', email: record.body?.email } });
    }
    if (accountTokenInvalid && record.authorization === `Bearer ${token}` && record.path.startsWith('/api/')) {
      return json(401, { error: { code: 'unauthorized', message: 'Authentication required.' } });
    }
    if (secondAccountDeleted && record.authorization === `Bearer ${secondToken}` && record.path.startsWith('/api/')) {
      return json(401, { error: { code: 'unauthorized', message: 'Authentication required.' } });
    }
    if (!removedMemberActive && record.authorization === `Bearer ${removedMemberToken}` && record.path.startsWith('/api/conductor/tracks')) {
      return json(403, { error: { code: 'workspace_forbidden', message: 'Workspace access forbidden.' } });
    }
    if (record.method === 'GET' && record.path === '/api/workspaces/41/people' && record.authorization === `Bearer ${removedMemberToken}`) {
      return removedMemberActive
        ? json(200, { members: workspaceMembers.get(41) })
        : json(403, { error: { code: 'workspace_forbidden', message: 'Workspace access forbidden.' } });
    }
    if (previewFiles.has(record.path)) {
      if (![token, secondToken].some((value) => record.authorization === `Bearer ${value}`)) return json(401, { error: { code: 'unauthorized', message: 'Authentication required.' } });
      if (record.path === missingSignedFileNext) {
        missingSignedFileNext = '';
        return json(404, { error: { code: 'preview_file_not_found', message: 'Signed preview file not found.' } });
      }
      if (record.path === signedRuntimePath && delaySignedRuntimeResponse) {
        delaySignedRuntimeResponse = false;
        await wait(180);
      }
      const [contentType, storedBody] = previewFiles.get(record.path);
      const corrupt = record.path === corruptSignedFileNext;
      const lengthMismatch = record.path === lengthMismatchSignedFileNext;
      if (corrupt) corruptSignedFileNext = '';
      if (lengthMismatch) lengthMismatchSignedFileNext = '';
      let body = storedBody;
      if (corrupt) {
        body = Buffer.from(storedBody, 'utf8');
        body[body.byteLength - 1] = body[body.byteLength - 1] === 0x20 ? 0x21 : 0x20;
      }
      if (lengthMismatch) body = Buffer.concat([Buffer.from(storedBody, 'utf8'), Buffer.from([0x20])]);
      return send(200, contentType, body);
    }
    const authenticated = ['/api/profile', '/api/workspaces', '/api/workspaces/41/people', '/api/templates', '/api/modules', '/api/artifacts/preview'];
    if (authenticated.includes(record.path) && ![token, secondToken].some((value) => record.authorization === `Bearer ${value}`)) {
      return json(401, { error: { code: 'unauthorized', message: 'Authentication required.' } });
    }
    if (record.method === 'GET' && record.path === '/api/profile') {
      const second = record.authorization === `Bearer ${secondToken}`;
      return json(200, { id: second ? 8 : 7, username: second ? 'Second Architect' : 'QA Architect', email: second ? 'second@example.test' : 'qa@example.test' });
    }
    if (record.method === 'GET' && record.path === '/api/workspaces') {
      const userID = principalForAuthorization(record.authorization);
      return json(200, { workspaces: workspaces.filter((workspace) => workspaceOwners.get(workspace.id)?.has(userID)) });
    }
    if (record.method === 'POST' && record.path === '/api/workspaces') {
      const workspace = { id: nextWorkspaceID++, name: String(record.body?.name || 'Created workspace') };
      workspaces = [...workspaces, workspace];
      workspaceOwners.set(workspace.id, new Set([principalForAuthorization(record.authorization)]));
      if (delayWorkspaceCreateResponse) {
        delayWorkspaceCreateResponse = false;
        await wait(300);
      }
      return json(201, workspace);
    }
    if (record.method === 'GET' && /^\/api\/workspaces\/\d+\/people$/u.test(record.path)) {
      const barrier = initialPeopleResponseBarrier;
      initialPeopleResponseBarrier = null;
      if (barrier) await barrier;
      if (delayPeopleResponse) {
        delayPeopleResponse = false;
        await wait(300);
      }
    }
    if (record.method === 'GET' && record.path === '/api/workspaces/41/people') {
      if (record.authorization === `Bearer ${removedMemberToken}` && !removedMemberActive) return json(403, { error: { code: 'workspace_forbidden', message: 'Workspace access forbidden.' } });
      if (failPeopleNext) {
        failPeopleNext = false;
        return json(503, { error: { code: 'people_unavailable', message: 'Synthetic People interruption.' } });
      }
      return json(200, { members: workspaceMembers.get(41) });
    }
    if (record.method === 'GET' && record.path === '/api/workspaces/42/people') {
      return json(200, { members: workspaceMembers.get(42) });
    }
    if (record.method === 'DELETE' && /^\/api\/workspaces\/41\/users\/(9|10)$/u.test(record.path)) {
      if (![token, secondToken].some((value) => record.authorization === `Bearer ${value}`)) return json(403, { error: { code: 'workspace_forbidden', message: 'Workspace access forbidden.' } });
      if (delayMemberDeleteResponse) {
        delayMemberDeleteResponse = false;
        await wait(180);
      }
      const userID = Number(record.path.split('/').at(-1));
      workspaceMembers.set(41, workspaceMembers.get(41).filter((member) => member.user_id !== userID));
      if (userID === 9) removedMemberActive = false;
      if (memberDeleteAmbiguousNext) {
        memberDeleteAmbiguousNext = false;
        return json(503, { error: { code: 'membership_write_unavailable', message: 'Synthetic ambiguous member response.' } });
      }
      return send(204, 'application/json; charset=utf-8', '');
    }
    if (record.method === 'DELETE' && /^\/api\/workspaces\/\d+$/u.test(record.path)) {
      const workspaceID = Number(record.path.split('/').at(-1));
      const userID = principalForAuthorization(record.authorization);
      if (!userID || !workspaceOwners.get(workspaceID)?.has(userID)) return json(403, { error: { code: 'workspace_forbidden' } });
      workspaces = workspaces.filter((workspace) => workspace.id !== workspaceID);
      workspaceOwners.delete(workspaceID);
      workspaceMembers.delete(workspaceID);
      if (delayWorkspaceDeleteResponse) {
        delayWorkspaceDeleteResponse = false;
        await wait(180);
      }
      if (workspaceDeleteAmbiguousNext) {
        workspaceDeleteAmbiguousNext = false;
        return json(503, { error: { code: 'workspace_write_unavailable', message: 'Synthetic ambiguous workspace response.' } });
      }
      return send(204, 'application/json; charset=utf-8', '');
    }
    if (record.method === 'GET' && /^\/api\/workspaces\/\d+\/people$/u.test(record.path)) {
      if (![token, secondToken].some((value) => record.authorization === `Bearer ${value}`)) return json(401, { error: { code: 'unauthorized', message: 'Authentication required.' } });
      const second = record.authorization === `Bearer ${secondToken}`;
      return json(200, { members: [
        { user_id: second ? 8 : 7, username: second ? 'Second Architect' : 'QA Architect', role: 'owner', joined_at: '2026-08-18T00:00:00Z' },
      ] });
    }
    if (record.method === 'GET' && record.path === '/api/templates') {
      return json(200, { templates: [
        { id: 'community-iftar', title: 'Community Iftar', version: '1.0.0', description: 'Signed private-beta card.', allowedModules: ['iftar-registration', 'announcements', 'donation-campaign'] },
        { id: 'bazaar-cooperative', title: 'Bazaar cooperative', version: '1.0.0', description: 'Curated cooperative market card.', allowedModules: ['announcements', 'donation-campaign'] },
      ] });
    }
    if (record.method === 'GET' && record.path === '/api/modules') {
      if (incompleteCatalogNext) {
        incompleteCatalogNext = false;
        return json(200, { modules: [{ id: 'announcements', title: 'Incomplete', defaultDocument: {} }], componentDocumentPolicy: {} });
      }
      return json(200, { componentDocumentPolicy: componentPolicy, modules: [
        { id: 'iftar-registration', title: 'Iftar registration', dataClassifications: ['community-registrations'], documentFields, defaultDocument: { title: 'Iftar registration', summary: 'Register locally.' } },
        { id: 'announcements', title: 'Community announcements', dataClassifications: ['community-announcements'], documentFields, defaultDocument: { title: 'Community announcements', summary: 'Share community updates.' } },
        { id: 'donation-campaign', title: 'Donation campaign', dataClassifications: ['donation-intents'], allowedServerSignals: ['taawun_donation_status'], documentFields, defaultDocument: { title: 'Donation campaign', summary: 'Sandbox intent only.' } },
      ] });
    }
    if (record.method === 'GET' && record.path === '/api/conductor/tracks') {
      if (historyFailNext) {
        historyFailNext = false;
        return json(503, { error: { code: 'composition_dependency_unavailable', message: 'Synthetic history interruption.' } });
      }
      if (delayHistoryResponse) {
        delayHistoryResponse = false;
        await wait(180);
      }
      if (delaySecondPrincipalHistory && record.authorization === `Bearer ${secondToken}`) {
        delaySecondPrincipalHistory = false;
        await wait(420);
      }
      const workspaceID = Number(requestURL.searchParams.get('workspaceId'));
      const inapplicableNewest = includeInapplicableNewest ? {
        id: 'track_newest_draft', workspaceId: 41, status: 'DRAFT', version: 1, updatedAt: '2099-08-18T05:00:00Z',
        request: { templateId: 'community-iftar' },
      } : null;
      const all = [inapplicableNewest, activeTrackResponse?.track, ...extraTracks.values()].filter((track) => track && Number(track.workspaceId) === workspaceID);
      const tracks = all.map((track) => ({ id: track.id, templateId: track.request?.templateId || '', status: track.status, version: track.version, updatedAt: track.updatedAt, previewPresent: Boolean(track.preview), artifactPresent: Boolean(track.artifact), publicationPresent: Boolean(track.publication), authorizationExpiresAt: track.preview?.authorizationExpiresAt }));
      return json(200, { tracks });
    }
    if (record.method === 'GET' && /^\/api\/workspaces\/\d+\/domains$/u.test(record.path) && delayDomainResponse) {
      delayDomainResponse = false;
      await wait(300);
    }
    if (record.method === 'GET' && record.path === '/api/workspaces/41/domains') {
      if (![token, secondToken].some((value) => record.authorization === `Bearer ${value}`)) return json(403, { error: { code: 'workspace_forbidden' } });
      if (delayPublicationExclusionDomainResponse) {
        delayPublicationExclusionDomainResponse = false;
        await wait(180);
      }
      if (domainFailNext) {
        domainFailNext = false;
        return json(503, { error: { code: 'domain_dependency_unavailable', message: 'Synthetic domain interruption.' } });
      }
      return json(200, { claims: domainClaims });
    }
    if (record.method === 'GET' && record.path === '/api/workspaces/42/domains') return json(200, { claims: [] });
    if (record.method === 'GET' && /^\/api\/workspaces\/\d+\/domains$/u.test(record.path)) {
      if (![token, secondToken].some((value) => record.authorization === `Bearer ${value}`)) return json(401, { error: { code: 'unauthorized', message: 'Authentication required.' } });
      return json(200, { claims: [] });
    }
    if (record.method === 'GET' && /^\/api\/workspaces\/41\/domains\/[^/]+\/publications$/u.test(record.path)) {
      if (![token, secondToken].some((value) => record.authorization === `Bearer ${value}`)) return json(403, { error: { code: 'workspace_forbidden' } });
      if (publicationContextFailNext) {
        publicationContextFailNext = false;
        return json(503, { error: { code: 'publication_context_unavailable', message: 'Synthetic publication context interruption.' } });
      }
      if (delayPublicationContextResponse) {
        delayPublicationContextResponse = false;
        await wait(300);
      }
      const claimID = record.path.split('/').at(-2);
      const claimPublications = domainPublications.filter((publication) => publication.claimId === claimID);
      const publicationID = requestURL.searchParams.get('publicationId');
      if (publicationID) {
        const selected = claimPublications.find((publication) => publication.id === publicationID);
        if (selected && automaticPublicationExpiry && publicationID === 'publication_browser') {
          automaticPublicationExactReads += 1;
          if (automaticPublicationExactReads > 1) {
            selected.servingState = 'expired';
            publicationServerTime = selected.authorizationExpiresAt;
          }
        }
        return json(200, { publications: selected ? [publicationContextRow(selected, true)] : [], serverTime: publicationServerTime });
      }
      const cursor = requestURL.searchParams.get('cursor');
      const page = cursor ? claimPublications.slice(1) : claimPublications.slice(0, claimPublications.length > 1 ? 1 : 20);
      return json(200, { publications: page.map((publication) => publicationContextRow(publication)), serverTime: publicationServerTime, ...(claimPublications.length > 1 && !cursor ? { nextCursor: 'publication-page-2' } : {}) });
    }
    if (record.method === 'POST' && record.path === '/api/workspaces/41/domains') {
      const claim = { id: 'claim_rotated', workspaceId: 41, origin: record.body.origin, host: new URL(record.body.origin).host, status: 'pending', challengeExpiresAt: '2099-08-18T12:00:00Z', createdAt: '2026-08-18T01:00:00Z', updatedAt: '2026-08-18T01:00:00Z' };
      domainClaims = [claim, ...domainClaims.filter((entry) => entry.origin !== claim.origin)];
      if (delayScopeClaimResponse) {
        delayScopeClaimResponse = false;
        await wait(800);
      } else if (delayClaimResponse) {
        delayClaimResponse = false;
        await wait(180);
      }
      return json(201, { claim, verification: { recordType: 'TXT', recordName: `_taawun.${claim.host}`, value: 'taawun-verify=synthetic-new-proof', expiresAt: claim.challengeExpiresAt } });
    }
    if (record.method === 'POST' && record.path === '/api/shura/v1/invitations') {
      if (![token, secondToken].some((value) => record.authorization === `Bearer ${value}`)) return json(401, { error: { code: 'unauthorized', message: 'Authentication required.' } });
      const invitation = { id: `invitation_scope_race_${++invitationSequence}`, workspace_id: Number(record.body?.workspace_id), invitee: String(record.body?.invitee || ''), role: String(record.body?.role || 'Viewer'), status: 'PENDING', version: 1 };
      currentInvitations.set(invitation.id, invitation);
      if (delayInvitationResponse) {
        delayInvitationResponse = false;
        await wait(800);
      }
      return json(201, { invitation, token: 'session-only-scope-race-token' });
    }
    if (record.method === 'POST' && /^\/api\/shura\/v1\/invitations\/[^/]+\/revoke$/u.test(record.path)) {
      if (![token, secondToken].some((value) => record.authorization === `Bearer ${value}`)) return json(403, { error: { code: 'workspace_forbidden' } });
      const invitationID = record.path.split('/').at(-2);
      const invitation = currentInvitations.get(invitationID);
      if (!invitation || invitation.status !== 'PENDING' || Number(record.body?.expected_version) !== invitation.version) return json(409, { error: 'Conflict', message: 'governance state conflict' });
      if (delayInvitationRevokeResponse) {
        delayInvitationRevokeResponse = false;
        await wait(180);
      }
      if (invitationRevokeFailNext) {
        invitationRevokeFailNext = false;
        return json(503, { error: { code: 'invitation_dependency_unavailable', message: 'Synthetic invitation interruption.' } });
      }
      if (invitationAcceptRaceNext) {
        invitationAcceptRaceNext = false;
        Object.assign(invitation, { status: 'ACCEPTED', version: invitation.version + 1 });
        return json(409, { error: 'Conflict', message: 'governance state conflict' });
      }
      Object.assign(invitation, { status: 'REVOKED', version: invitation.version + 1 });
      return json(200, invitation);
    }
    if (record.method === 'POST' && record.path === '/api/shura/v1/invitations/accept') {
      if (![token, secondToken].some((value) => record.authorization === `Bearer ${value}`)) return json(401, { error: { code: 'unauthorized', message: 'Authentication required.' } });
      if (delayAcceptanceResponse) {
        delayAcceptanceResponse = false;
        await wait(800);
      }
      return json(200, { id: 'invitation_accept_scope_race', workspace_id: 42, role: 'Viewer', status: 'ACCEPTED' });
    }
    if (record.method === 'GET' && /^\/api\/workspaces\/41\/domains\/[^/]+$/u.test(record.path)) {
      if (![token, secondToken].some((value) => record.authorization === `Bearer ${value}`)) return json(403, { error: { code: 'workspace_forbidden' } });
      const claimID = record.path.split('/').at(-1);
      const claim = domainClaims.find((entry) => entry.id === claimID);
      return claim ? json(200, claim) : json(404, { error: { code: 'domain_claim_not_found', message: 'Domain claim not found.' } });
    }
    if (record.method === 'DELETE' && /^\/api\/workspaces\/41\/domains\/[^/]+$/u.test(record.path)) {
      if (![token, secondToken].some((value) => record.authorization === `Bearer ${value}`)) return json(403, { error: { code: 'workspace_forbidden' } });
      if (delayDomainRevokeResponse) {
        delayDomainRevokeResponse = false;
        await wait(180);
      }
      if (domainRevokeFailNext) {
        domainRevokeFailNext = false;
        return json(503, { error: { code: 'domain_dependency_unavailable', message: 'Synthetic ambiguous domain response.' } });
      }
      const claimID = record.path.split('/').at(-1);
      const claim = domainClaims.find((entry) => entry.id === claimID);
      if (!claim || !['pending', 'verified'].includes(claim.status)) return json(404, { error: { code: 'domain_claim_not_found', message: 'Domain claim not found.' } });
      Object.assign(claim, { status: 'revoked', revocationReason: 'user', revokedAt: '2026-08-18T08:00:00Z', updatedAt: '2026-08-18T08:00:00Z' });
      domainPublications = domainPublications.map((publication) => publication.claimId === claimID && publication.active
        ? { ...publication, servingState: 'claim_unavailable', authorizationExpiresAt: null, manifestDigest: null }
        : publication);
      return json(200, claim);
    }
    if (record.method === 'PUT' && record.path === '/api/users/7') {
      if (record.authorization !== `Bearer ${token}`) return json(record.authorization ? 403 : 401, { error: { code: 'user_forbidden' } });
      if (typeof record.body?.password !== 'string' || record.body.password.length < 12) return json(400, { error: 'password too short' });
      if (delayPasswordUpdateResponse) {
        delayPasswordUpdateResponse = false;
        await wait(180);
      }
      currentAccountPassword = record.body.password;
      accountTokenInvalid = true;
      if (passwordUpdateAmbiguousNext) {
        passwordUpdateAmbiguousNext = false;
        return json(503, { error: { code: 'user_write_unavailable', message: 'Synthetic ambiguous password response.' } });
      }
      return json(200, { id: 7, username: 'QA Architect', email: 'qa@example.test' });
    }
    if (record.method === 'DELETE' && /^\/api\/users\/(7|8)$/u.test(record.path)) {
      const userID = Number(record.path.split('/').at(-1));
      const callerID = principalForAuthorization(record.authorization);
      if (!callerID || callerID !== userID) return json(record.authorization ? 403 : 401, { error: { code: 'user_forbidden' } });
      if (delayAccountDeleteResponse) {
        delayAccountDeleteResponse = false;
        await wait(180);
      }
      if (accountHasOwnedWorkspaces(userID)) return json(409, { error: { code: 'owned_workspaces_remaining', message: 'Owned workspaces must be deleted before this account can be deleted.' } });
      if (accountDeleteCommitAmbiguousNext) {
        accountDeleteCommitAmbiguousNext = false;
        if (userID === 7) {
          accountDeleted = true;
          accountTokenInvalid = true;
        } else {
          secondAccountDeleted = true;
        }
        return json(503, { error: { code: 'user_write_unavailable', message: 'Synthetic ambiguous account response after commit.' } });
      }
      if (userID === 7) {
        accountDeleted = true;
        accountTokenInvalid = true;
      } else {
        secondAccountDeleted = true;
      }
      return send(204, 'application/json; charset=utf-8', '');
    }
    if (record.method === 'POST' && /^\/api\/workspaces\/41\/domains\/[^/]+\/verify$/u.test(record.path)) {
      const claimID = record.path.split('/').at(-2);
      const claim = domainClaims.find((entry) => entry.id === claimID);
      Object.assign(claim, { status: 'verified', verifiedAt: '2026-08-18T02:00:00Z', verificationExpiresAt: '2099-08-19T00:00:00Z', updatedAt: '2026-08-18T02:00:00Z' });
      if (delayVerifyResponse) {
        delayVerifyResponse = false;
        await wait(180);
      }
      return json(200, claim);
    }
    if (record.method === 'GET' && record.path === '/api/conductor/tracks/track_browser' && activeTrackResponse) {
      if (delayTrackResponse) {
        delayTrackResponse = false;
        await wait(180);
      }
      if (requestURL.searchParams.get('includeVerifiedPreview') !== 'true') return json(200, activeTrackResponse.track);
      const result = structuredClone(activeTrackResponse);
      if (signedFileVariantNext === 'missing-runtime') result.manifest.files = result.manifest.files.filter((file) => file.path !== runtimeBundlePath);
      if (signedFileVariantNext === 'runtime-descriptor-mismatch') result.manifest.runtime.sha256 = '0'.repeat(64);
      if (signedFileVariantNext === 'duplicate-descriptor') result.manifest.files.push(structuredClone(result.manifest.files[0]));
      if (signedFileVariantNext === 'malformed-descriptor-path') result.manifest.files.push({ path: '../runtime.js', sha256: '0'.repeat(64), bytes: 1 });
      if (signedFileVariantNext === 'theme-url-query') result.preview.themeUrl = `${previewPrefix}theme.css?untrusted=1`;
      if (signedFileVariantNext === 'document-wrong-track') result.preview.documentUrl = '/api/conductor/tracks/track_other/preview/files/index.html';
      if (signedFileVariantNext) {
        signedFileVariantNext = '';
        result.verification.manifestJson = JSON.stringify(result.manifest);
        result.verification.manifestDigest = createHash('sha256').update(result.verification.manifestJson).digest('hex');
      }
      return json(200, result);
    }
    if (record.method === 'GET' && record.path === '/api/conductor/tracks/track_replacement' && replacementTrackResponse) {
      return json(200, requestURL.searchParams.get('includeVerifiedPreview') === 'true' ? replacementTrackResponse : replacementTrackResponse.track);
    }
    if (record.method === 'GET' && /^\/api\/conductor\/tracks\/[^/]+$/u.test(record.path)) {
      const trackID = record.path.split('/').at(-1);
      const track = extraTracks.get(trackID);
      return track ? json(200, track) : json(404, { error: { code: 'track_not_found', message: 'Composition track was not found.' } });
    }
    if (record.method === 'GET' && /^\/api\/conductor\/tracks\/[^/]+\/events$/u.test(record.path)) {
      const trackID = record.path.split('/').at(-2);
      return json(200, { events: trackEvents.get(trackID) || [] });
    }
    if (record.method === 'POST' && /^\/api\/conductor\/tracks\/[^/]+\/resume$/u.test(record.path)) {
      const trackID = record.path.split('/').at(-2);
      const track = extraTracks.get(trackID);
      if (!track) return json(404, { error: { code: 'track_not_found', message: 'Composition track was not found.' } });
      if (resumeConflictOnce) {
        resumeConflictOnce = false;
        track.version += 1;
        track.updatedAt = '2026-08-18T05:00:00Z';
        return json(409, { error: { code: 'composition_conflict', message: 'Composition track version conflict.' } });
      }
      Object.assign(track, { status: 'VALIDATED', version: track.version + 1, updatedAt: '2026-08-18T05:01:00Z' });
      return json(200, track);
    }
    if (record.method === 'POST' && record.path === '/api/conductor/tracks/track_browser/publication' && activeTrackResponse) {
      Object.assign(activeTrackResponse.track, { status: 'PUBLICATION_REQUESTED', version: activeTrackResponse.track.version + 1, claimId: record.body.claimId, updatedAt: '2026-08-18T06:00:00Z' });
      trackEvents.set('track_browser', [...(trackEvents.get('track_browser') || []), { type: 'PUBLICATION_REQUESTED', toStatus: 'PUBLICATION_REQUESTED', trackVersion: activeTrackResponse.track.version, createdAt: '2026-08-18T06:00:00Z', detail: { secret: 'never-render-this-detail' } }]);
      if (delayScopePublicationResponse) {
        delayScopePublicationResponse = false;
        await new Promise((resolve) => { releaseScopePublicationResponse = resolve; });
        releaseScopePublicationResponse = null;
      } else if (delayPublicationRequest) {
        delayPublicationRequest = false;
        await wait(180);
      }
      return json(200, activeTrackResponse.track);
    }
    if (record.method === 'POST' && record.path === '/api/conductor/tracks/track_browser/activate' && activeTrackResponse) {
      if (activationFailNext) {
        activationFailNext = false;
        activeTrackResponse.track.version += 2;
        activeTrackResponse.track.updatedAt = '2026-08-18T06:01:00Z';
        trackEvents.set('track_browser', [...(trackEvents.get('track_browser') || []), { type: 'PUBLICATION_ACTIVATION_BLOCKED', toStatus: 'PUBLICATION_REQUESTED', trackVersion: activeTrackResponse.track.version, createdAt: '2026-08-18T06:01:00Z', detail: { token: 'never-render-this-token' } }]);
        return json(502, { error: { code: 'composition_dependency_unavailable', message: 'Synthetic activation interruption.' } });
      }
      const publication = {
        id: 'publication_browser', workspaceId: 41, claimId: 'claim_browser', origin: 'https://app.community.example', artifactId: 'browser-signed-artifact', contentHash: 'a'.repeat(64),
        active: true, activatedAt: '2026-08-18T06:02:00Z', authorizationExpiresAt: activeTrackResponse.manifest.authorization.expiresAt,
        manifestDigest: activeTrackResponse.verification.manifestDigest, servingState: 'serving',
        trackBinding: { trackId: 'track_browser', status: 'PUBLISHED', version: activeTrackResponse.track.version + 1 },
      };
      Object.assign(activeTrackResponse.track, { status: 'PUBLISHED', version: activeTrackResponse.track.version + 1, publication, updatedAt: publication.activatedAt });
      domainPublications = [publication];
      return json(201, activeTrackResponse.track);
    }
    if (record.method === 'POST' && record.path === '/api/conductor/tracks/track_replacement/publication' && replacementTrackResponse) {
      Object.assign(replacementTrackResponse.track, { status: 'PUBLICATION_REQUESTED', version: replacementTrackResponse.track.version + 1, claimId: record.body.claimId, updatedAt: '2026-08-18T08:01:00Z' });
      return json(200, replacementTrackResponse.track);
    }
    if (record.method === 'POST' && record.path === '/api/conductor/tracks/track_replacement/activate' && replacementTrackResponse) {
      const predecessor = domainPublications.find((publication) => publication.active);
      if (predecessor) Object.assign(predecessor, { active: false, deactivatedAt: '2026-08-18T08:02:00Z', servingState: 'inactive' });
      const publication = {
        id: 'publication_replacement', workspaceId: 41, claimId: 'claim_browser', origin: 'https://app.community.example', artifactId: 'browser-replacement-artifact', contentHash: 'c'.repeat(64),
        sourcePublicationId: predecessor && predecessor.id, active: true, activatedAt: '2026-08-18T08:02:00Z',
        authorizationExpiresAt: replacementTrackResponse.manifest.authorization.expiresAt, manifestDigest: replacementTrackResponse.verification.manifestDigest,
        servingState: 'serving', trackBinding: { trackId: 'track_replacement', status: 'PUBLISHED', version: replacementTrackResponse.track.version + 1 },
      };
      Object.assign(replacementTrackResponse.track, { status: 'PUBLISHED', version: replacementTrackResponse.track.version + 1, publication, updatedAt: publication.activatedAt });
      domainPublications = [publication, ...domainPublications];
      return json(201, replacementTrackResponse.track);
    }
    if (record.method === 'POST' && /^\/api\/workspaces\/41\/domains\/claim_browser\/publications\/[^/]+\/activate$/u.test(record.path)) {
      if (delayRollbackActivationResponse) {
        delayRollbackActivationResponse = false;
        await wait(180);
      }
      const sourceID = record.path.split('/').at(-2);
      const source = domainPublications.find((publication) => publication.id === sourceID);
      if (!source || source.servingState !== 'inactive') return json(422, { error: { code: 'artifact_not_publishable', message: 'Historical artifact is not currently eligible.' } });
      const active = domainPublications.find((publication) => publication.active);
      if (active) Object.assign(active, { active: false, deactivatedAt: '2026-08-18T09:00:00Z', servingState: 'inactive' });
      const publication = {
        ...structuredClone(source), id: 'publication_rollback', sourcePublicationId: source.id, active: true,
        activatedAt: '2026-08-18T09:00:00Z', deactivatedAt: null, servingState: 'serving',
      };
      domainPublications = [publication, ...domainPublications];
      return json(201, publication);
    }
    if (record.method === 'POST' && record.path === '/api/artifacts/preview') {
      if (failNextPreview) {
        failNextPreview = false;
        return json(503, { error: { code: 'composition_unavailable', message: 'Synthetic network interruption.' } });
      }
      const variant = receiptVariants[previewBuilds] || 'active';
      const replacement = Number(record.body.ttlHours) === 2160;
      const responseTrackID = replacement ? 'track_replacement' : 'track_browser';
      const responseArtifactID = replacement ? 'browser-replacement-artifact' : 'browser-signed-artifact';
      const responseContentHash = (replacement ? 'c' : 'a').repeat(64);
      const responseCreatedAt = replacement ? '2026-08-18T08:00:00Z' : '2026-08-18T04:00:00Z';
      const responseExpiresAt = replacement ? '2026-11-16T08:00:00Z' : '2099-08-18T04:23:28Z';
      const responsePreviewPrefix = replacement ? '/api/conductor/tracks/track_replacement/preview/files/' : previewPrefix;
      if (replacement) {
        for (const descriptor of manifestFiles) {
          const stored = previewFiles.get(`${previewPrefix}${descriptor.path}`);
          previewFiles.set(`${responsePreviewPrefix}${descriptor.path}`, stored);
        }
      }
      if (delayPreviewResponse) {
        delayPreviewResponse = false;
        await wait(180);
      }
      const attestedManifest = {
        contractVersion: 'taawun.artifact/v2',
        artifactId: responseArtifactID,
        contentHash: responseContentHash,
        createdAt: responseCreatedAt,
        workspaceId: record.body.workspaceId,
        template: { id: record.body.templateId, version: '1.0.0' },
        modules: record.body.modules,
        components: record.body.components.map((component) => ({
          ...component,
          documentPath: `components/${component.id}.json`,
          documentSha256: createHash('sha256').update(JSON.stringify(component.data)).digest('hex'),
        })),
        renderModes: ['standalone'],
        compliance: {
          status: 'reference-only-pending-qualified-review',
          references: [{ id: 'ref-iftar-1', title: 'Iftar reference', status: 'pending-qualified-review' }],
        },
        financial: { status: 'sandbox' },
        runtime: {
          name: 'Datastar', version: '1.0.2', path: '/assets/datastar-v1.0.2.js', sha256: runtimeDigest,
          requiredBy: ['standalone'], bundled: true, packagingRequirement: 'served from the signed artifact bundle',
        },
        theme: { stylesheet: 'theme.css', tokenNames: [] },
        files: structuredClone(manifestFiles),
        authorization: {
          subject: { id: record.authorization === `Bearer ${secondToken}` ? 'taawun:user:8' : 'taawun:user:7', userId: record.authorization === `Bearer ${secondToken}` ? 8 : 7, workspaceId: record.body.workspaceId },
          allowedOrigins: {
            surfaces: record.body.requestedOrigins?.surfaces?.length ? record.body.requestedOrigins.surfaces : [origin],
            embedders: replacement ? (record.body.requestedOrigins?.embedders || []) : ['https://community.example'],
            connections: replacement ? (record.body.requestedOrigins?.connections || []) : ['https://relay.example'],
            resources: replacement ? (record.body.requestedOrigins?.resources || []) : ['https://assets.example'],
          },
          expiresAt: responseExpiresAt,
          signerKeyId: 'railway-artifact-v1',
          lifecycle: 'preview',
        },
        signature: {
          algorithm: 'Ed25519',
          keyId: 'railway-artifact-v1',
          value: 'browser-signature-value',
        },
      };
      if (replacement && replacementSuccessorVariant === 'wrong-current-subject') {
        attestedManifest.authorization.subject = { id: 'taawun:user:99', userId: 99, workspaceId: record.body.workspaceId };
      }
      if (replacement && replacementSuccessorVariant === 'wrong-origin-policy') {
        attestedManifest.authorization.allowedOrigins = { surfaces: [origin], embedders: [], connections: [], resources: [] };
      }
      if (variant === 'elapsed-expiry') attestedManifest.authorization.expiresAt = '2000-08-18T04:23:28Z';
      if (variant === 'subject-workspace') attestedManifest.authorization.subject.workspaceId = 42;
      if (variant === 'authorization-key') attestedManifest.authorization.signerKeyId = 'tampered-key';
      if (variant === 'component-digest') attestedManifest.components[0].documentSha256 = '0'.repeat(64);
      if (variant === 'component-substitution-attested') {
        attestedManifest.components[0].data.summary = 'Server-substituted component copy';
        attestedManifest.components[0].documentSha256 = createHash('sha256').update(JSON.stringify(attestedManifest.components[0].data)).digest('hex');
      }
      if (variant === 'component-missing') delete attestedManifest.components;
      let responseManifest = structuredClone(attestedManifest);
      if (variant === 'lifecycle') responseManifest.authorization.lifecycle = 'published';
      if (variant === 'expiry') responseManifest.authorization.expiresAt = '2098-08-18T04:23:28Z';
      if (variant === 'surfaces') responseManifest.authorization.allowedOrigins.surfaces = ['https://tampered-surface.example'];
      if (variant === 'embedders') responseManifest.authorization.allowedOrigins.embedders = ['https://tampered-embedder.example'];
      if (variant === 'connections') responseManifest.authorization.allowedOrigins.connections = ['https://tampered-connection.example'];
      if (variant === 'resources') responseManifest.authorization.allowedOrigins.resources = ['https://tampered-resource.example'];
      if (variant === 'component-data') responseManifest.components[0].data.summary = 'Tampered component copy';
      if (variant === 'missing-fields') responseManifest = { artifactId: 'missing-optional-fields' };
      const manifestJson = JSON.stringify(attestedManifest);
      const verification = {
        status: 'verified',
        verified: true,
        authorizationState: variant === 'elapsed-expiry' ? 'expired' : 'active',
        serverTime: replacement ? '2026-08-18T08:00:00Z' : '2026-08-18T04:23:28Z',
        artifactId: responseArtifactID,
        contentHash: responseContentHash,
        workspaceId: record.body.workspaceId,
        signatureAlgorithm: 'Ed25519',
        signerKeyId: 'railway-artifact-v1',
        signatureValue: 'browser-signature-value',
        manifestDigest: createHash('sha256').update(manifestJson).digest('hex'),
        manifestJson,
      };
      if (variant === 'missing-authorization-state') delete verification.authorizationState;
      if (variant === 'authorization-state-mismatch') verification.authorizationState = 'expired';
      if (variant === 'missing-digest') delete verification.manifestDigest;
      if (variant === 'digest-mismatch') verification.manifestDigest = '0'.repeat(64);
      if (variant === 'artifact') verification.artifactId = 'tampered-artifact';
      if (variant === 'content-hash') verification.contentHash = 'b'.repeat(64);
      if (variant === 'workspace') verification.workspaceId = 42;
      if (variant === 'signature') verification.signatureValue = 'tampered-signature';
      if (variant === 'key') verification.signerKeyId = 'tampered-key';
      if (variant === 'algorithm') verification.signatureAlgorithm = 'tampered-algorithm';
      previewBuilds += 1;
      const result = {
        manifest: responseManifest,
        verification: variant === 'missing-attestation' ? undefined : verification,
        previewUrl: `${responsePreviewPrefix}index.html`,
        preview: {
          documentUrl: `${responsePreviewPrefix}index.html`,
          themeUrl: `${responsePreviewPrefix}theme.css`,
          stylesUrl: `${responsePreviewPrefix}app.css`,
        },
        track: {
          id: responseTrackID, workspaceId: record.body.workspaceId, request: structuredClone(record.body), status: 'PREVIEW_READY', version: 6,
          createdBy: record.authorization === `Bearer ${secondToken}` ? 8 : 7, createdAt: responseCreatedAt, updatedAt: replacement ? '2026-08-18T08:00:00Z' : '2026-08-18T04:23:28Z',
          artifact: { artifactId: responseArtifactID, contentHash: responseContentHash },
          preview: {
            artifactId: responseArtifactID, contentHash: responseContentHash, workspaceId: record.body.workspaceId,
            subject: { id: attestedManifest.authorization.subject.id, userId: attestedManifest.authorization.subject.userId }, allowedOrigins: structuredClone(attestedManifest.authorization.allowedOrigins),
            authorizationExpiresAt: attestedManifest.authorization.expiresAt, authenticationRequired: true,
          },
        },
      };
      if (replacement && replacementSuccessorVariant === 'wrong-created-by') result.track.createdBy = 99;
      if (variant === 'track-missing') delete result.track;
      if (variant === 'track-id-missing') result.track.id = '';
      if (variant === 'track-workspace') result.track.workspaceId = 42;
      if (variant === 'track-artifact') result.track.artifact.artifactId = 'tampered-track-artifact';
      if (variant === 'track-hash') result.track.artifact.contentHash = 'b'.repeat(64);
      if (variant === 'track-preview-missing') delete result.track.preview;
      if (variant === 'track-preview-artifact') result.track.preview.artifactId = 'tampered-preview-artifact';
      if (variant === 'track-preview-hash') result.track.preview.contentHash = 'b'.repeat(64);
      if (variant === 'track-preview-workspace') result.track.preview.workspaceId = 42;
      if (variant === 'track-preview-subject') result.track.preview.subject.userId = 99;
      if (variant === 'track-preview-origins') result.track.preview.allowedOrigins.surfaces = ['https://tampered-track-origin.example'];
      if (variant === 'track-preview-expiry') result.track.preview.authorizationExpiresAt = '2098-08-18T04:23:28Z';
      if (variant === 'track-preview-public') result.track.preview.authenticationRequired = false;
      if (variant === 'active') {
        if (replacement) {
          replacementTrackResponse = structuredClone(result);
          trackEvents.set('track_replacement', [{ type: 'PREVIEW_READY', toStatus: 'PREVIEW_READY', trackVersion: 6, createdAt: responseCreatedAt }]);
        } else {
          activeTrackResponse = structuredClone(result);
          trackEvents.set('track_browser', [
            { type: 'TRACK_CREATED', toStatus: 'DRAFT', trackVersion: 1, createdAt: '2026-08-18T04:00:00Z', detail: { principal: 'never-render-this-principal' } },
            { type: 'PREVIEW_READY', toStatus: 'PREVIEW_READY', trackVersion: 6, createdAt: '2026-08-18T04:23:28Z', detail: { raw: 'never-render-this-detail' } },
          ]);
        }
      }
      if (replacement && delayReplacementSuccessResponse) {
        delayReplacementSuccessResponse = false;
        await wait(180);
      }
      if (replacement && replacementPreviewFailAfterCommit) {
        replacementPreviewFailAfterCommit = false;
        return json(503, { error: { code: 'composition_dependency_unavailable', message: 'Synthetic committed replacement response interruption.' } });
      }
      return json(200, result);
    }
    return json(404, { error: { code: 'not_found', message: 'Not found.' } });
  });
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const address = server.address();
  const origin = `http://127.0.0.1:${address.port}`;
  const unauthenticatedPreview = await fetch(`${origin}${previewPrefix}index.html`);
  await unauthenticatedPreview.text();
  assert.equal(unauthenticatedPreview.status, 401, 'signed preview files must reject unauthenticated reads');
  const unauthenticatedRuntime = await fetch(`${origin}${signedRuntimePath}`);
  await unauthenticatedRuntime.text();
  assert.equal(unauthenticatedRuntime.status, 401, 'the manifest-listed signed runtime must reject anonymous reads');
  const authenticatedRuntime = await fetch(`${origin}${signedRuntimePath}`, { headers: { Authorization: `Bearer ${token}` } });
  const authenticatedRuntimeBytes = Buffer.from(await authenticatedRuntime.arrayBuffer());
  assert.equal(authenticatedRuntime.status, 200, 'the manifest-listed signed runtime must be available to an authorized track reader');
  assert.equal(createHash('sha256').update(authenticatedRuntimeBytes).digest('hex'), runtimeDigest, 'authorized signed runtime bytes must match the manifest SHA-256 fixture');
  requests.length = 0;

  const tempDirectory = await mkdtemp(path.join(tmpdir(), 'taawun-cockpit-browser-'));
  let chromium;
  try {
    chromium = await launchChromium(browser, path.join(tempDirectory, 'profile'));
    const { client } = chromium;
    await client.send('Page.enable');
    await client.send('Runtime.enable');
    await client.send('Page.navigate', { url: `${origin}/account#login` });
    await waitFor(() => evaluate(client, `document.readyState === 'complete' && !document.querySelector('#loginForm').hidden`), 'cockpit login');

    let releaseInitialPeopleResponse;
    initialPeopleResponseBarrier = new Promise((resolve) => { releaseInitialPeopleResponse = resolve; });
    try {
      await evaluate(client, `(() => {
        const set = (id, value) => {
          const input = document.getElementById(id);
          input.value = value;
          input.dispatchEvent(new Event('input', { bubbles: true }));
        };
        set('loginEmail', 'qa@example.test');
        set('loginPassword', 'correct horse battery staple');
        document.getElementById('loginForm').requestSubmit();
        return true;
      })()`);
      await waitFor(() => evaluate(client, `!document.querySelector('#signedStarterPath').hidden
        && document.querySelector('#buildHistoryState').textContent.includes('No workspace build records yet')
        && document.querySelector('#domainClaimSelect').value === 'claim_browser'
        && document.querySelector('#starterPrimaryButton').disabled
        && document.querySelector('#starterInviteButton').disabled
        && document.querySelector('#templateSelect').disabled
        && document.querySelector('#workspaceRole').textContent === 'Checking access'`), 'history and domain evidence independently load a role-gated starter while People is still loading');
    } finally {
      releaseInitialPeopleResponse?.();
      initialPeopleResponseBarrier = null;
    }
    await waitFor(() => evaluate(client, `!document.querySelector('#appView').hidden
      && document.querySelector('#workspaceSelect').value === '41'
      && document.querySelector('#workspaceRole').textContent === 'Architect'
      && document.querySelector('#templateSelect').options.length === 3
      && document.querySelector('#templateSelect').value === ''
      && document.querySelector('#previewButton').disabled`), 'workspace and explicit catalog choice');
    await waitFor(() => evaluate(client, `document.querySelector('#buildHistoryState').textContent.includes('No workspace build records yet') && document.querySelector('#domainClaimSelect').value === 'claim_browser'`), 'honest empty build history and recovered verified domain readiness');
    assert.equal(await evaluate(client, `document.querySelector('#signedStarterPath').hidden`), false, 'starter is visible only after an authorized exact-empty history response');
    assert.match(await evaluate(client, `document.querySelector('#signedStarterPath').innerText`), /no starter data is invented or persisted/iu);
    for (const layoutWidth of [160, 200, 320, 400]) {
      await client.send('Emulation.setDeviceMetricsOverride', { width: layoutWidth, height: 1_000, deviceScaleFactor: 1, mobile: false });
      await waitFor(() => evaluate(client, `window.innerWidth === ${layoutWidth}`), `${layoutWidth}px signed starter`);
      const layout = await evaluate(client, `({ overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth, path: document.querySelector('#signedStarterPath').getBoundingClientRect().width, viewport: document.documentElement.clientWidth, targets: [...document.querySelectorAll('#starterPrimaryButton, #starterInviteButton')].map((button) => button.getBoundingClientRect().height) })`);
      assert.equal(layout.overflow, false, `${layoutWidth}px signed starter must not overflow horizontally`);
      assert.ok(layout.path <= layout.viewport, `${layoutWidth}px signed starter remains inside the viewport`);
      assert.ok(layout.targets.every((height) => height >= 44), `${layoutWidth}px signed starter actions must be at least 44px`);
    }
    await client.send('Emulation.setDeviceMetricsOverride', { width: 1_280, height: 1_000, deviceScaleFactor: 1, mobile: false });
    await waitFor(() => evaluate(client, `window.innerWidth === 1280`), 'signed starter desktop reset');
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceRole').textContent === 'Architect'
      && !document.querySelector('#starterPrimaryButton').disabled
      && document.querySelector('#starterPrimaryButton').textContent === 'Choose a template'
      && !document.querySelector('#templateSelect').disabled`), 'resolved Architect authority before the measured keyboard path');

    const starterActivations = [];
    await evaluate(client, `document.querySelector('#starterPrimaryButton').focus()`);
    await pressKey(client, 'Enter', 'Enter', 13);
    starterActivations.push('focus explicit template choice');
    assert.equal(await evaluate(client, `document.activeElement?.id`), 'templateSelect', 'starter keyboard action focuses the explicit real-template selector');
    await pressKey(client, 'ArrowDown', 'ArrowDown', 40);
    await pressKey(client, 'Enter', 'Enter', 13);
    starterActivations.push('choose Community Iftar template');
    await waitFor(() => evaluate(client, `document.querySelector('#starterPrimaryButton').textContent === 'Choose components' && document.querySelector('#starterPathStatus').textContent.includes('1 of 5')`), 'starter live progress after explicit template');
    for (const [componentID, label] of [['iftar-registration', 'choose Iftar registration'], ['announcements', 'choose announcements'], ['donation-campaign', 'choose donation campaign']]) {
      await evaluate(client, `document.querySelector('#moduleList input[name="selectedModule"][value="${componentID}"]').focus()`);
      await pressKey(client, ' ', 'Space', 32);
      starterActivations.push(label);
      await waitFor(() => evaluate(client, `document.querySelector('#moduleList input[name="selectedModule"][value="${componentID}"]').checked`), `${componentID} keyboard selection`);
    }
    await waitFor(() => evaluate(client, `document.querySelectorAll('#moduleList input[name="selectedModule"]:checked').length === 3 && document.querySelector('#starterPathStatus').textContent.includes('2 of 5') && document.querySelector('#starterPrimaryButton').textContent === 'Customize component content'`), 'starter live progress after explicit components');
    const previewRequestsBeforeCustomization = requests.filter((request) => request.method === 'POST' && request.path === '/api/artifacts/preview').length;
    await evaluate(client, `document.querySelector('#starterPrimaryButton').focus()`);
    await pressKey(client, 'Enter', 'Enter', 13);
    starterActivations.push('focus one declared component field');
    assert.equal(await evaluate(client, `document.activeElement?.closest('.component-editor')?.dataset.componentId`), 'iftar-registration', 'catalog defaults route the starter to the first selected declared field instead of signing');
    assert.equal(requests.filter((request) => request.method === 'POST' && request.path === '/api/artifacts/preview').length, previewRequestsBeforeCustomization, 'catalog-default documents cannot advance through the signed starter action');
    await evaluate(client, `(() => { const input = document.activeElement; input.value = 'Customized signed registration'; input.dispatchEvent(new Event('input', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#starterPathStatus').textContent.includes('3 of 5') && document.querySelector('#starterPrimaryButton').textContent === 'Complete app details'`), 'canonical component document differs from its validated catalog default');
    await evaluate(client, `(() => { for (const [id, value] of [['appName', 'Signed starter app'], ['organizationName', 'QA Community'], ['city', 'Salt Lake City']]) { const input = document.getElementById(id); input.value = value; input.dispatchEvent(new Event('input', { bubbles: true })); } })()`);
    assert.equal(await evaluate(client, `document.querySelector('.sample-title').textContent`), 'Signed starter app', 'vanilla app-name input keeps the visible sample title in sync without loading the public runtime');
    await waitFor(() => evaluate(client, `document.querySelector('#starterPrimaryButton').textContent === 'Create signed preview'`), 'starter signed-preview action');
    await evaluate(client, `document.querySelector('#starterPrimaryButton').focus()`);
    await pressKey(client, 'Enter', 'Enter', 13);
    starterActivations.push('create signed preview');
    await waitFor(() => evaluate(client, `document.querySelector('#previewStatus').textContent === 'Verified staging ready'
      && document.querySelector('#buildHistoryState').textContent.includes('Exact verified preview track track_browser is confirmed')
      && document.querySelector('#signedStarterPath').hidden`), 'verified preview and exact history confirmation remain separate and close the ephemeral starter');
    assert.match(await evaluate(client, `document.querySelector('#starterRecoveryState').textContent`), /Exact workspace-bound signed preview verified; durable history confirmed; this preview is not published.*Track track_browser is a selector, not authority.*session-only acceptance token/isu, 'post-success handoff remains outside the hidden starter with exact trust, publication, selector, and Architect invitation boundaries');
    assert.deepEqual(await evaluate(client, `({ hidden: document.querySelector('#recoveryInviteButton').hidden, label: document.querySelector('#recoveryInviteButton').textContent, title: document.querySelector('#recoveryInviteButton').title })`), { hidden: false, label: 'Invite a Viewer', title: 'Creates a Viewer invitation whose acceptance token is shown only in this browser session.' }, 'Architect post-success Viewer invitation remains role-valid and session-only');
    assert.ok((await evaluate(client, `[document.querySelector('#recoveryInviteButton'), document.querySelector('#openNewestBuildButton')].filter((button) => !button.hidden).map((button) => button.getBoundingClientRect().height)`)).every((height) => height >= 44), 'visible post-success handoff actions are at least 44px');
    const firstTrustedStarterEvidence = await evaluate(client, `({ srcdoc: document.querySelector('#previewFrame').srcdoc, raw: document.querySelector('#rawManifest').textContent })`);
    assert.deepEqual(starterActivations, [
      'focus explicit template choice',
      'choose Community Iftar template',
      'choose Iftar registration',
      'choose announcements',
      'choose donation campaign',
      'focus one declared component field',
      'create signed preview',
    ], 'the promoted first-success path records its exact control activations after workspace selection; text entry and network waits are excluded');
    assert.ok(starterActivations.length <= 8, `signed first-success path must take no more than eight controls; observed ${starterActivations.length}`);
    const firstPreviewRequest = requests.find((request) => request.method === 'POST' && request.path === '/api/artifacts/preview');
    assert.equal(firstPreviewRequest.body.components.find((component) => component.id === 'iftar-registration').data.title, 'Customized signed registration', 'the meaningful non-default component document is the exact signed request');
    assert.equal(JSON.parse(await evaluate(client, `document.querySelector('#rawManifest').textContent`)).components.find((component) => component.id === 'iftar-registration').data.title, 'Customized signed registration', 'the visible verified manifest contains the same meaningful component document');

    const historyRecoveryActivations = [];
    historyFailNext = true;
    await evaluate(client, `document.querySelector('#retryBuildHistory').click()`);
    historyRecoveryActivations.push('test exact-history failure');
    await waitFor(() => evaluate(client, `document.querySelector('#buildHistoryState').textContent.includes('Exact history confirmation is pending; the verified preview and receipt are retained') && document.querySelector('#signedStarterPath').hidden`), 'history failure retains verified evidence and never revives inferred starter progress');
    assert.deepEqual(await evaluate(client, `({ srcdoc: document.querySelector('#previewFrame').srcdoc, raw: document.querySelector('#rawManifest').textContent })`), firstTrustedStarterEvidence, 'history failure cannot replace trusted preview evidence');
    await evaluate(client, `document.querySelector('#retryBuildHistory').click()`);
    historyRecoveryActivations.push('retry exact-history confirmation');
    await waitFor(() => evaluate(client, `document.querySelector('#buildHistoryState').textContent.includes('Exact verified preview track track_browser is confirmed')`), 'history confirmation retry');
    assert.deepEqual(historyRecoveryActivations, ['test exact-history failure', 'retry exact-history confirmation'], 'history recovery is measured separately from the first-success activation budget');

    const selectionTrack = structuredClone(activeTrackResponse.track);
    selectionTrack.id = 'track_selection';
    selectionTrack.updatedAt = '2026-08-18T03:00:00Z';
    extraTracks.set(selectionTrack.id, selectionTrack);
    delayTrackResponse = true;
    const delayedRecoveryRequestStart = requests.length;
    await within(client.send('Page.navigate', { url: `${origin}/account` }), 'delayed recovery navigation');
    await waitFor(() => evaluate(client, `document.readyState === 'complete' && !document.querySelector('#loginForm').hidden`), 'delayed recovery login');
    await evaluate(client, `(() => {
      for (const [id, value] of [['loginEmail', 'qa@example.test'], ['loginPassword', 'correct horse battery staple']]) {
        const input = document.getElementById(id); input.value = value; input.dispatchEvent(new Event('input', { bubbles: true }));
      }
      document.getElementById('loginForm').requestSubmit();
    })()`);
    await waitFor(() => requests.slice(delayedRecoveryRequestStart).some((request) => request.path === '/api/conductor/tracks/track_browser' && request.search === '?includeVerifiedPreview=true'), 'delayed newest verification request');
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceRole').textContent === 'Architect' && document.querySelectorAll('#buildHistoryList li').length === 2`), 'same-workspace alternate history selection');
    await evaluate(client, `(() => { const item = [...document.querySelectorAll('#buildHistoryList li')].find((candidate) => candidate.querySelector('strong')?.textContent === 'track_selection'); item.querySelector('button').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#buildRecordTrackID').value === 'track_selection'`), 'newer same-workspace selection');
    await wait(240);
    assert.deepEqual(await evaluate(client, `({ selected: document.querySelector('#buildRecordTrackID').value, srcdoc: document.querySelector('#previewFrame').srcdoc, verified: document.querySelector('#previewStatus').textContent === 'Verified staging ready' })`), { selected: 'track_selection', srcdoc: '', verified: false }, 'an older automatic newest-track verification cannot overwrite a newer same-workspace history selection');
    extraTracks.delete(selectionTrack.id);

    includeInapplicableNewest = true;
    const reloadRequestStart = requests.length;
    await within(client.send('Page.navigate', { url: `${origin}/account` }), 'starter reload navigation');
    await waitFor(() => evaluate(client, `document.readyState === 'complete' && !document.querySelector('#loginForm').hidden`), 'starter reload login');
    await evaluate(client, `(() => {
      for (const [id, value] of [['loginEmail', 'qa@example.test'], ['loginPassword', 'correct horse battery staple']]) {
        const input = document.getElementById(id); input.value = value; input.dispatchEvent(new Event('input', { bubbles: true }));
      }
      document.getElementById('loginForm').requestSubmit();
    })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceRole').textContent === 'Architect'
      && document.querySelector('#previewStatus').textContent === 'Verified staging ready'
      && document.querySelector('#starterRecoveryState').textContent.includes('Exact workspace-bound signed preview verified; durable history confirmed')
      && document.querySelector('#starterRecoveryState').textContent.includes('exact component documents differ from the current validated catalog defaults')
      && document.querySelector('#signedStarterPath').hidden
      && !document.querySelector('#recoveryInviteButton').hidden`), 'fresh login derives customized signed progress from one exact verified track');
    const reloadRequests = requests.slice(reloadRequestStart);
    const reloadTrackReads = reloadRequests.filter((request) => request.method === 'GET' && request.path === '/api/conductor/tracks/track_browser');
    assert.equal(reloadTrackReads.length, 1, 'fresh login verifies only the selected-or-newest history track, never every summary row');
    assert.equal(reloadTrackReads[0].search, '?includeVerifiedPreview=true', 'fresh login uses the explicit verified-preview trust gate rather than a plain track read');
    assert.equal(reloadRequests.some((request) => request.path === '/api/conductor/tracks/track_newest_draft'), false, 'newest draft summary without preview/artifact presence is skipped in favor of the older applicable signed preview');
    assert.equal(reloadRequests.filter((request) => request.method === 'GET' && request.path === '/api/conductor/tracks/track_browser/events').length, 0, 'starter reload performs no event or N+1 inspection reads');
    assert.equal(JSON.parse(await evaluate(client, `document.querySelector('#rawManifest').textContent`)).components.find((component) => component.id === 'iftar-registration').data.title, 'Customized signed registration', 'reload milestone compares the exact verified component documents with catalog defaults');
    includeInapplicableNewest = false;

    const evidenceBeforeLoaderFailures = await evaluate(client, `({ srcdoc: document.querySelector('#previewFrame').srcdoc, raw: document.querySelector('#rawManifest').textContent })`);
    delayPeopleResponse = true;
    failPeopleNext = true;
    await evaluate(client, `document.querySelector('#retryPeople').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceRole').textContent === 'Checking access'
      && document.querySelector('#recoveryInviteButton').hidden
      && document.querySelector('#recoveryInviteButton').disabled`), 'People reload immediately withdraws post-success invitation authority while membership is loading');
    await waitFor(() => evaluate(client, `document.querySelector('#peopleState').textContent.includes('Could not load workspace people')
      && document.querySelector('#buildHistoryList').textContent.includes('track_browser')
      && document.querySelector('#domainClaimSelect').value === 'claim_browser'
      && document.querySelector('#templateSelect').disabled
      && document.querySelector('#recoveryInviteButton').hidden
      && document.querySelector('#recoveryInviteButton').disabled`), 'People failure leaves independent history and domain evidence visible while authority-sensitive controls stay disabled');
    assert.deepEqual(await evaluate(client, `({ srcdoc: document.querySelector('#previewFrame').srcdoc, raw: document.querySelector('#rawManifest').textContent })`), evidenceBeforeLoaderFailures, 'People failure retains the trusted signed pair');
    await evaluate(client, `document.querySelector('#retryPeople').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceRole').textContent === 'Architect'
      && !document.querySelector('#templateSelect').disabled
      && !document.querySelector('#recoveryInviteButton').hidden
      && !document.querySelector('#recoveryInviteButton').disabled`), 'People retry restores role-sensitive builder and post-success invitation access');

    domainFailNext = true;
    await evaluate(client, `document.querySelector('#retryDomainClaims').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainClaimsState').textContent.includes('Domain evidence is unavailable')
      && document.querySelector('#workspaceRole').textContent === 'Architect'
      && document.querySelector('#buildHistoryList').textContent.includes('track_browser')
      && !document.querySelector('#templateSelect').disabled`), 'domain failure leaves People authority and Build history independently usable');
    assert.deepEqual(await evaluate(client, `({ srcdoc: document.querySelector('#previewFrame').srcdoc, raw: document.querySelector('#rawManifest').textContent })`), evidenceBeforeLoaderFailures, 'domain failure retains the trusted signed pair');
    await evaluate(client, `document.querySelector('#retryDomainClaims').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainClaimSelect').value === 'claim_browser'`), 'domain evidence retry');

    const trackReadsBeforeRecovery = requests.filter((request) => request.method === 'GET' && request.path === '/api/conductor/tracks/track_browser').length;
    await evaluate(client, `document.querySelector('#openNewestBuildButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#buildRecordAlert').textContent.includes('Verified preview reopened')`), 'selected-or-newest recovery reopens one exact verified preview');
    assert.equal(requests.filter((request) => request.method === 'GET' && request.path === '/api/conductor/tracks/track_browser').length - trackReadsBeforeRecovery, 2, 'selected-or-newest recovery performs one plain authorized read and one verified-preview trust-gate read, never an N+1 scan');
    previewBuilds = 0;
    assert.match(await evaluate(client, `[...document.querySelector('#domainClaimSelect').options].map((option) => option.textContent).join(' ')`), /pending.*revoked.*expired/isu, 'real pending, revoked, and expired claim states render honestly');
    await evaluate(client, `(() => { const select = document.querySelector('#domainClaimSelect'); select.value = 'claim_pending'; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainClaimsState').textContent.includes('Verify an already-published proof') && document.querySelector('#domainProof').hidden && !document.querySelector('#verifyDomainButton').disabled`), 'pending claim next action never synthesizes a TXT proof');
    delayVerifyResponse = true;
    await evaluate(client, `(() => { document.querySelector('#verifyDomainButton').click(); const select = document.querySelector('#domainClaimSelect'); select.value = 'claim_browser'; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await wait(240);
    assert.equal(await evaluate(client, `document.querySelector('#domainClaimSelect').value`), 'claim_browser', 'delayed verify response cannot overwrite a newer same-workspace claim selection');
    delayClaimResponse = true;
    await evaluate(client, `(() => {
      const input = document.querySelector('#domainOrigin');
      input.value = 'https://first-delayed.community.example'; input.dispatchEvent(new Event('input', { bubbles: true }));
      document.querySelector('#claimDomainButton').click();
      input.value = 'https://newer-edit.community.example'; input.dispatchEvent(new Event('input', { bubbles: true }));
    })()`);
    await wait(240);
    assert.deepEqual(await evaluate(client, `({ origin: document.querySelector('#domainOrigin').value, selected: document.querySelector('#domainClaimSelect').value, proofHidden: document.querySelector('#domainProof').hidden })`), { origin: 'https://newer-edit.community.example', selected: '', proofHidden: true }, 'delayed claim response cannot overwrite a newer same-workspace origin edit');
    await evaluate(client, `document.querySelector('#retryDomainClaims').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainClaimSelect').value === 'claim_browser'`), 'domain claim recovery after stale operations');

    await evaluate(client, `(() => {
      const select = document.querySelector('#templateSelect');
      select.value = 'community-iftar';
      select.dispatchEvent(new Event('change', { bubbles: true }));
      while (document.querySelector('#moduleList input[name="selectedModule"]:not(:checked)')) {
        document.querySelector('#moduleList input[name="selectedModule"]:not(:checked)').click();
      }
    })()`);
    await waitFor(() => evaluate(client, `document.querySelectorAll('#moduleList input[name="selectedModule"]:checked').length === 3 && !document.querySelector('#previewButton').disabled`), 'explicit component selection');

    await evaluate(client, `document.querySelector('[data-component-id="announcements"] .component-tools button').click()`);
    await waitFor(() => evaluate(client, `Boolean(document.querySelector('[data-component-id="announcements"] .custom-field'))`), 'custom field add');
    assert.equal(await evaluate(client, `document.activeElement?.value`), 'custom_1', 'Add custom field restores focus to the announced new field name');
    await evaluate(client, `(() => { const input = document.querySelector('[data-component-id="announcements"] .custom-field input[aria-label$="field name"]'); input.value = 'audience'; input.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('[data-component-id="announcements"] .custom-field input[aria-label$="field name"]')?.value === 'audience'`), 'custom field rename');
    await evaluate(client, `(() => { const select = document.querySelector('[data-component-id="announcements"] .custom-field select'); select.value = 'array'; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await evaluate(client, `(() => { const value = document.querySelector('[data-component-id="announcements"] .custom-field textarea'); value.value = '["families","students"]'; value.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('[data-component-id="announcements"] .advanced-document textarea').value.includes('students')`), 'custom array edit');

    await evaluate(client, `document.querySelector('[data-component-id="announcements"] .component-tools button').click()`);
    await waitFor(() => evaluate(client, `document.querySelectorAll('[data-component-id="announcements"] .custom-field').length === 2`), 'custom object field add');
    await evaluate(client, `(() => {
      const row = [...document.querySelectorAll('[data-component-id="announcements"] .custom-field')].find((candidate) => candidate.querySelector('input[aria-label$="field name"]').value === 'custom_1');
      const key = row.querySelector('input[aria-label$="field name"]');
      key.value = 'details';
      key.dispatchEvent(new Event('change', { bubbles: true }));
    })()`);
    await waitFor(() => evaluate(client, `[...document.querySelectorAll('[data-component-id="announcements"] .custom-field')].some((row) => row.querySelector('input[aria-label$="field name"]').value === 'details')`), 'custom object field rename');
    await evaluate(client, `(() => {
      const row = [...document.querySelectorAll('[data-component-id="announcements"] .custom-field')].find((candidate) => candidate.querySelector('input[aria-label$="field name"]').value === 'details');
      const type = row.querySelector('select');
      type.value = 'object';
      type.dispatchEvent(new Event('change', { bubbles: true }));
    })()`);
    await waitFor(() => evaluate(client, `Boolean([...document.querySelectorAll('[data-component-id="announcements"] .custom-field')].find((row) => row.querySelector('input[aria-label$="field name"]').value === 'details')?.querySelector('textarea'))`), 'custom object editor');

    await evaluate(client, `(() => {
      const row = [...document.querySelectorAll('[data-component-id="announcements"] .custom-field')].find((candidate) => candidate.querySelector('input[aria-label$="field name"]').value === 'details');
      const value = row.querySelector('textarea');
      value.value = '{"nested":{"note":1,"note":2}}';
      value.dispatchEvent(new Event('change', { bubbles: true }));
    })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#component-error-announcements').textContent.includes('duplicate key nested.note')`), 'custom object nested duplicate rejection');
    await evaluate(client, `(() => {
      const row = [...document.querySelectorAll('[data-component-id="announcements"] .custom-field')].find((candidate) => candidate.querySelector('input[aria-label$="field name"]').value === 'details');
      const value = row.querySelector('textarea');
      value.value = JSON.stringify({ nested: String.fromCharCode(0xDC00) });
      value.dispatchEvent(new Event('change', { bubbles: true }));
    })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#component-error-announcements').textContent.includes('invalid Unicode scalar')`), 'custom object nested lone low surrogate rejection');
    await evaluate(client, `(() => {
      const row = [...document.querySelectorAll('[data-component-id="announcements"] .custom-field')].find((candidate) => candidate.querySelector('input[aria-label$="field name"]').value === 'details');
      const value = row.querySelector('textarea');
      value.value = JSON.stringify({ symbol: String.fromCodePoint(0x1F319) });
      value.dispatchEvent(new Event('change', { bubbles: true }));
    })()`);
    await waitFor(() => evaluate(client, `document.querySelector('[data-component-id="announcements"] .advanced-document textarea').value.includes('🌙')`), 'valid custom object Unicode scalar pair');

    for (const [source, expected] of [
      ['[{"note":1,"note":2}]', 'duplicate key note'],
      ['[1e0]', 'canonical number format'],
      ['[-0]', 'canonical number format'],
      ['[1.20]', 'canonical number format'],
      ['["families"] true', 'Unexpected non-whitespace character'],
    ]) {
      await evaluate(client, `(() => {
        const row = [...document.querySelectorAll('[data-component-id="announcements"] .custom-field')].find((candidate) => candidate.querySelector('input[aria-label$="field name"]').value === 'audience');
        const value = row.querySelector('textarea');
        value.value = ${JSON.stringify(source)};
        value.dispatchEvent(new Event('change', { bubbles: true }));
      })()`);
      assert.match(await evaluate(client, `document.querySelector('#component-error-announcements').textContent`), new RegExp(expected, 'iu'), `custom array raw JSON must reject ${source}`);
    }

    await evaluate(client, `(() => {
      const input = document.querySelector('[data-component-id="announcements"] .advanced-document textarea');
      input.value = JSON.stringify({ title: 'Unicode component', summary: 'Evening ' + String.fromCodePoint(0x1F319), audience: ['families', 'students'], details: { symbol: String.fromCodePoint(0x1F319) } });
      input.parentElement.querySelector('button').click();
    })()`);
    await waitFor(() => evaluate(client, `document.querySelector('[data-component-id="announcements"] .advanced-document textarea').value.includes('Evening 🌙')`), 'valid surrogate pair and emoji draft');
    assert.deepEqual(await evaluate(client, `({ active: document.activeElement?.textContent, open: document.querySelector('[data-component-id="announcements"] .advanced-document').open })`), { active: 'Apply valid JSON', open: true }, 'valid Advanced Apply reopens the surviving details and restores focus to Apply');

    await evaluate(client, `document.querySelector('[data-component-id="donation-campaign"] .component-tools button').click()`);
    await waitFor(() => evaluate(client, `Boolean(document.querySelector('[data-component-id="donation-campaign"] .custom-field'))`), 'removable custom field add');
    await evaluate(client, `document.querySelector('[data-component-id="donation-campaign"] .custom-field button').click()`);
    await waitFor(() => evaluate(client, `!document.querySelector('[data-component-id="donation-campaign"] .custom-field')`), 'custom field remove');
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceAnnouncer').textContent === 'Removed custom field custom_1. Focus returned to the surviving donation-campaign component heading.'`), 'custom field removal live-region announcement');
    assert.deepEqual(await evaluate(client, `({ heading: document.activeElement?.dataset.componentHeading, announcement: document.querySelector('#workspaceAnnouncer').textContent })`), { heading: 'true', announcement: 'Removed custom field custom_1. Focus returned to the surviving donation-campaign component heading.' }, 'Remove focuses and announces the surviving component heading');

    const lastValidAnnouncement = await evaluate(client, `document.querySelector('[data-component-id="announcements"] .advanced-document textarea').value`);
    await evaluate(client, `(() => { const input = document.querySelector('[data-component-id="announcements"] .advanced-document textarea'); input.value = JSON.stringify({title:'Unicode',summary:String.fromCharCode(0xD800)}); input.parentElement.querySelector('button').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#component-error-announcements').textContent.includes('invalid Unicode scalar')`), 'root declared lone high surrogate rejection');
    await evaluate(client, `(() => { const input = document.querySelector('[data-component-id="announcements"] .advanced-document textarea'); input.value = JSON.stringify({title:'Unicode',summary:'C1 ' + String.fromCharCode(0x85)}); input.parentElement.querySelector('button').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#component-error-announcements').textContent.includes('invalid or oversized text')`), 'C1 control text rejection');
    await evaluate(client, `(() => { const input = document.querySelector('[data-component-id="announcements"] .advanced-document textarea'); input.value = '{"title":"Unsafe","summary":"No","origin":"https://evil.example"}'; input.parentElement.querySelector('button').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#component-error-announcements').textContent.includes('reserved security key')`), 'reserved component key rejection');
    await evaluate(client, `(() => { const input = document.querySelector('[data-component-id="announcements"] .advanced-document textarea'); input.value = '{"title":"Number","summary":"No","amount":1e0}'; input.parentElement.querySelector('button').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#component-error-announcements').textContent.includes('canonical number format')`), 'non-canonical number rejection');
    await evaluate(client, `(() => { const input = document.querySelector('[data-component-id="announcements"] .advanced-document textarea'); input.value = '{"title":"Duplicate","summary":"No","meta":{"note":1,"note":2}}'; input.parentElement.querySelector('button').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#component-error-announcements').textContent.includes('duplicate key meta.note')`), 'duplicate JSON key rejection');
    await evaluate(client, `(() => { const input = document.querySelector('[data-component-id="announcements"] .advanced-document textarea'); input.value = '{"title":"Precision","summary":"No","amount":9007199254740993}'; input.parentElement.querySelector('button').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#component-error-announcements').textContent.includes('canonical number format')`), 'precision-losing number rejection');
    await evaluate(client, `(() => { const input = document.querySelector('[data-component-id="announcements"] .advanced-document textarea'); input.value = '{"title":"Missing","summary":""}'; input.parentElement.querySelector('button').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#component-error-announcements').textContent.includes('required declared field')`), 'declared required field rejection');
    await evaluate(client, `(() => { const input = document.querySelector('[data-component-id="announcements"] .advanced-document textarea'); input.value = '{"title":7,"summary":"Wrong type"}'; input.parentElement.querySelector('button').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#component-error-announcements').textContent.includes('expected string')`), 'declared field type rejection');
    await evaluate(client, `(() => { const input = document.querySelector('[data-component-id="announcements"] .advanced-document textarea'); input.value = JSON.stringify({title:'x'.repeat(121),summary:'Too long'}); input.parentElement.querySelector('button').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#component-error-announcements').textContent.includes('exceeds 120')`), 'declared max length rejection');
    await evaluate(client, `(() => { const input = document.querySelector('[data-component-id="announcements"] .advanced-document textarea'); input.value = '{"title":"Nested","summary":"No","meta":{"prototype":{"value":"unsafe"}}}'; input.parentElement.querySelector('button').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#component-error-announcements').textContent.includes('reserved security key')`), 'recursive reserved key rejection');
    await evaluate(client, `(() => { const input = document.querySelector('[data-component-id="announcements"] .advanced-document textarea'); input.value = '{"title":"Depth","summary":"No","meta":{"a":{"b":{"c":{"d":{"e":{"f":"too deep"}}}}}}}'; input.parentElement.querySelector('button').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#component-error-announcements').textContent.includes('maximum depth exceeded')`), 'component depth rejection');
    await evaluate(client, `(() => { const input = document.querySelector('[data-component-id="announcements"] .custom-field input[aria-label$="field name"]'); input.value = '__proto__'; input.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#component-error-announcements').textContent.includes('invalid key') || document.querySelector('#component-error-announcements').textContent.includes('reserved security key')`), 'prototype-like custom rename rejection');
    await evaluate(client, `(() => { const input = document.querySelector('[data-component-id="announcements"] .advanced-document textarea'); input.value = JSON.stringify({title:'Large',summary:'x'.repeat(9000)}); input.parentElement.querySelector('button').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#component-error-announcements').textContent.includes('oversized') || document.querySelector('#component-error-announcements').textContent.includes('exceeds')`), 'oversized component rejection');
    await evaluate(client, `(() => { const select = document.querySelector('#templateSelect'); select.value = 'bazaar-cooperative'; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#templateReconcile').hidden && document.querySelector('#templateReconcileMessage').textContent.includes('iftar-registration')`), 'incompatible template reconciliation');
    await evaluate(client, `document.querySelector('#cancelTemplateSwitch').click()`);
    assert.equal(await evaluate(client, `document.querySelector('#templateSelect').value`), 'community-iftar', 'cancel keeps the current template');
    assert.equal(await evaluate(client, `document.activeElement?.id`), 'templateSelect', 'template switch Cancel returns focus to the template selector');
    assert.match(await evaluate(client, `document.querySelector('#component-error-announcements').textContent`), /last valid document is retained/iu, 'invalid input explains last-valid recovery');
    await evaluate(client, `(() => { const select = document.querySelector('#templateSelect'); select.value = 'bazaar-cooperative'; select.dispatchEvent(new Event('change', { bubbles: true })); document.querySelector('#applyTemplateSwitch').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#templateSelect').value === 'bazaar-cooperative' && document.querySelectorAll('#moduleList input:checked').length === 2`), 'template reconciliation apply');
    assert.equal(await evaluate(client, `document.activeElement?.id`), 'templateSelect', 'template switch Apply returns focus to the template selector');
    await evaluate(client, `(() => { const select = document.querySelector('#templateSelect'); select.value = 'community-iftar'; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#templateSelect').value === 'community-iftar' && document.querySelectorAll('#moduleList input:checked').length === 3 && document.querySelector('[data-component-id="announcements"] .advanced-document textarea').value.includes('students')`), 'template draft restoration');
    assert.equal(await evaluate(client, `document.querySelector('[data-component-id="announcements"] .advanced-document textarea').value`), lastValidAnnouncement, 'template return restores the exact last-valid document, not invalid input');

    incompleteCatalogNext = true;
    await evaluate(client, `document.querySelector('#retryTemplate').click()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#retryTemplate').hidden && document.querySelector('#templateBadge').textContent.includes('Refresh failed') && document.querySelector('#previewButton').disabled`), 'incomplete catalog fail-closed state');
    assert.equal(await evaluate(client, `document.querySelector('[data-component-id="announcements"] .advanced-document textarea').value`), lastValidAnnouncement, 'an incomplete catalog refresh preserves the exact last-valid draft');
    await evaluate(client, `(() => { const select = document.querySelector('#workspaceSelect'); select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#templateBadge').textContent.includes('Refresh failed') && document.querySelector('#templateSelect').disabled && document.querySelector('#previewButton').disabled`), 'catalog failure survives workspace access rerender');
    assert.equal(await evaluate(client, `document.querySelector('[data-component-id="announcements"] .advanced-document textarea').value`), lastValidAnnouncement, 'fail-closed access rerender retains the exact draft');
    await evaluate(client, `document.querySelector('#retryTemplate').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#templateBadge').textContent.includes('Ready') && document.querySelector('#templateSelect').value === 'community-iftar' && !document.querySelector('#previewButton').disabled`), 'catalog retry recovery');
    assert.equal(await evaluate(client, `document.querySelector('[data-component-id="announcements"] .advanced-document textarea').value`), lastValidAnnouncement, 'a successful catalog retry restores the exact scoped draft');

    await evaluate(client, `(() => {
      const set = (id, value) => {
        const input = document.getElementById(id);
        input.value = value;
        input.dispatchEvent(new Event('input', { bubbles: true }));
      };
      set('organizationName', 'QA Community');
      set('city', 'Salt Lake City');
      document.getElementById('builderForm').requestSubmit();
      return true;
    })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#previewStatus').textContent === 'Verified staging ready'
      && Boolean(document.querySelector('#previewFrame').getAttribute('srcdoc'))`), 'signed staging preview');

    const cockpitState = await evaluate(client, `(() => {
      const frame = document.querySelector('#previewFrame');
      return {
        sandbox: frame.getAttribute('sandbox'),
        hidden: frame.hidden,
        srcdoc: frame.getAttribute('srcdoc'),
      };
    })()`);
    assert.equal(cockpitState.sandbox, 'allow-scripts allow-forms', 'production preview sandbox must remain exact');
    assert.equal(cockpitState.hidden, false);

    const receipt = await evaluate(client, `Object.fromEntries([...document.querySelectorAll('#manifestList .manifest-row')].map((row) => [row.querySelector('dt').textContent, row.querySelector('dd').textContent]))`);
    assert.equal(receipt['Signature verification'], 'Verified by Taawun build service · authorization active');
    assert.equal(receipt['Signing key'], 'Ed25519 · railway-artifact-v1');
    assert.equal(receipt['Workspace binding'], 'Workspace 41');
    assert.equal(receipt['Lifecycle / expiry'], 'preview · expires 2099-08-18T04:23:28.000Z');
    assert.equal(receipt['Exact allowed origins'], `Surface: ${origin} · Surface: https://app.community.example · Embedder: https://community.example · Connection: https://relay.example · Resource: https://assets.example`);
    assert.equal(receipt['Review references'], 'ref-iftar-1 (pending-qualified-review) · Reference-only; not scholar approval');
    assert.match(receipt['Component documents'], /announcements · [0-9a-f]{64}/u);
    await waitFor(() => evaluate(client, `document.querySelectorAll('#buildHistoryList li').length === 1 && document.querySelector('#buildHistoryList').textContent.includes('track_browser') && document.querySelector('#buildRecordTrackID').value === 'track_browser'`), 'successful preview refreshes real build history and current record');
    historyFailNext = true;
    await evaluate(client, `document.querySelector('#retryBuildHistory').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#buildHistoryState').textContent.includes('Synthetic history interruption') && document.querySelectorAll('#buildHistoryList li').length === 1`), 'history error retains last real records');
    extraTracks.set('track_partial', {
      id: 'track_partial', workspaceId: 41, request: structuredClone(activeTrackResponse.track.request), status: 'STAGED', version: 1, createdBy: 7,
      createdAt: '2026-08-18T04:30:00Z', updatedAt: '2026-08-18T04:30:00Z',
    });
    trackEvents.set('track_partial', [{ type: 'TRACK_STAGED', toStatus: 'STAGED', trackVersion: 1, createdAt: '2026-08-18T04:30:00Z', detail: { credential: 'never-render-this-credential' } }]);
    await evaluate(client, `document.querySelector('#retryBuildHistory').click()`);
    await waitFor(() => evaluate(client, `document.querySelectorAll('#buildHistoryList li').length === 2 && document.querySelector('#buildHistoryList').textContent.includes('track_partial')`), 'history retry loads real records');
    await evaluate(client, `[...document.querySelectorAll('#buildHistoryList li')].find((item) => item.textContent.includes('track_partial')).querySelector('button').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#buildRecordTrackID').value === 'track_partial' && !document.querySelector('#resumeBuildButton').disabled`), 'creator resumable track selection');
    await evaluate(client, `document.querySelector('#resumeBuildButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#buildRecordMeta').textContent.includes('Version 2') && document.querySelector('#buildRecordAlert').textContent.includes('changed before resume')`), 'resume conflict refreshes authoritative version');
    await evaluate(client, `document.querySelector('#resumeBuildButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#buildRecordTitle').textContent.includes('VALIDATED') && document.querySelector('#buildRecordMeta').textContent.includes('Version 3')`), 'resumable track advances without replacing trusted preview');

    const trustedUnicodeBoundary = await evaluate(client, `({ srcdoc: document.querySelector('#previewFrame').srcdoc, raw: document.querySelector('#rawManifest').textContent })`);
    await evaluate(client, `(() => {
      const input = document.querySelector('[data-component-id="announcements"] .advanced-document textarea');
      input.value = JSON.stringify({ title: 'Unicode', summary: String.fromCharCode(0xD800) });
      input.parentElement.querySelector('button').click();
    })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#component-error-announcements').textContent.includes('invalid Unicode scalar')`), 'verified-preview lone surrogate rejection');
    assert.deepEqual(await evaluate(client, `({ srcdoc: document.querySelector('#previewFrame').srcdoc, raw: document.querySelector('#rawManifest').textContent, status: document.querySelector('#previewStatus').textContent, stale: document.querySelector('#previewFrame').dataset.stale })`), {
      ...trustedUnicodeBoundary,
      status: 'Verified staging ready',
      stale: 'false',
    }, 'invalid Unicode input retains the exact last valid draft, receipt, and verified preview');

    await evaluate(client, `(() => {
      const input = document.querySelector('#trackLookup');
      input.value = 'not-a-track';
      input.dispatchEvent(new Event('input', { bubbles: true }));
      document.querySelector('#loadTrackButton').click();
    })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#builderAlert').textContent.includes('Enter a valid track ID')`), 'invalid track recovery input');
    assert.deepEqual(await evaluate(client, `({ srcdoc: document.querySelector('#previewFrame').srcdoc, raw: document.querySelector('#rawManifest').textContent, status: document.querySelector('#previewStatus').textContent, stale: document.querySelector('#previewFrame').dataset.stale })`), {
      ...trustedUnicodeBoundary,
      status: 'Verified staging ready',
      stale: 'false',
    }, 'recovery-only track input cannot stale an otherwise verified draft and receipt');

    const runtimeStartTag = '<script type="module">';
    const runtimeEndTag = '</script>';
    const runtimeStart = cockpitState.srcdoc.indexOf(runtimeStartTag);
    const runtimeEnd = cockpitState.srcdoc.indexOf(runtimeEndTag, runtimeStart);
    assert.ok(runtimeStart >= 0 && runtimeEnd > runtimeStart, 'srcdoc must contain the approved inline runtime');
    assert.equal(cockpitState.srcdoc.indexOf(runtimeStartTag, runtimeStart + runtimeStartTag.length), -1, 'srcdoc must contain one inline module');
    assert.doesNotMatch(cockpitState.srcdoc, /<script type="module" src=/u, 'external runtime tag must be replaced');
    const sourceRuntime = cockpitState.srcdoc.slice(runtimeStart + runtimeStartTag.length, runtimeEnd);
    assert.equal(sourceRuntime, signedRuntime, 'srcdoc must contain the intact canonical-LF signed runtime');
    assert.match(sourceRuntime, /\$&/u, 'the intact inline runtime must retain literal $&');

    const frameTarget = await waitFor(async () => {
      const { targetInfos } = await client.send('Target.getTargets');
      const pageTarget = targetInfos.find((target) => target.type === 'page' && target.url === `${origin}/account`);
      return targetInfos.find((target) => target.type === 'iframe'
        && target.parentId === pageTarget?.targetId
        && target.url === 'about:srcdoc');
    }, 'opaque srcdoc target');
    const { sessionId: frameSession } = await client.send('Target.attachToTarget', { targetId: frameTarget.targetId, flatten: true });
    await client.send('Runtime.enable', {}, frameSession);
    await waitFor(() => evaluate(client, `document.querySelector('#notice')?.textContent === 'Signed community card ready'`, undefined, frameSession), 'Datastar card initialization');
    const frameState = await evaluate(client, `(() => {
      const scripts = [...document.querySelectorAll('script[type="module"]')];
      const visible = [...document.body.children].find((element) => getComputedStyle(element).display !== 'none' && element.innerText.trim());
      return {
        bodyText: document.body.innerText.trim(),
        firstVisibleId: visible?.id || '',
        scripts: scripts.map((script) => ({ src: script.getAttribute('src'), text: script.textContent })),
      };
    })()`, undefined, frameSession);
    assert.equal(frameState.firstVisibleId, 'signed-card', 'the signed card must be the first visible element');
    assert.match(frameState.bodyText, /^Signed community card/u, 'no runtime text may precede the signed card');
    assert.doesNotMatch(frameState.bodyText, /datastar-patch-elements|UndefinedAction/u, 'Datastar source must not be visible');
    assert.equal(frameState.scripts.length, 1, 'parsed sandbox must contain one module script');
    assert.equal(frameState.scripts[0].src, null, 'approved runtime must remain inline in the sandbox');
    assert.equal(frameState.scripts[0].text, signedRuntime, 'parsed runtime must remain the exact canonical-LF signed bytes after UTF-8 decode');
    const signedRuntimeContract = JSON.parse(await evaluate(client, `document.querySelector('#rawManifest').textContent`));
    const signedRuntimeDescriptor = signedRuntimeContract.files.find((file) => file.path === runtimeBundlePath);
    assert.deepEqual({ path: signedRuntimeContract.runtime.path, sha256: signedRuntimeContract.runtime.sha256, bundled: signedRuntimeContract.runtime.bundled }, { path: '/assets/datastar-v1.0.2.js', sha256: runtimeDigest, bundled: true }, 'visible signed runtime requirement must bind the canonical runtime digest');
    assert.deepEqual(signedRuntimeDescriptor, { path: runtimeBundlePath, sha256: runtimeDigest, bytes: Buffer.byteLength(signedRuntime, 'utf8') }, 'visible manifest must list the exact signed runtime bytes executed by the iframe');

    const interaction = await evaluate(client, `new Promise((resolve) => {
      document.querySelector('#card-control').click();
      setTimeout(() => resolve(document.querySelector('#notice').textContent), 100);
    })`, undefined, frameSession);
    assert.equal(interaction, 'Card control activated', 'signed card controls must remain interactive');

    const requiredAuthenticatedPaths = ['/api/profile', '/api/workspaces', '/api/workspaces/41/people', '/api/templates', '/api/modules', '/api/artifacts/preview'];
    for (const requiredPath of requiredAuthenticatedPaths) {
      const request = requests.find((item) => item.path === requiredPath);
      assert.ok(request, `${requiredPath} must be reached through the cockpit`);
      assert.equal(request.authorization, `Bearer ${token}`, `${requiredPath} must use the login token`);
    }
    const previewRequests = requests.filter((item) => previewFiles.has(item.path));
    assert.ok(previewRequests.length >= previewFiles.size, 'the cockpit must fetch every signed preview file');
    for (const request of previewRequests) assert.equal(request.authorization, `Bearer ${token}`, `${request.path} must use the login token on every verified reopen`);
    for (const request of previewRequests) assert.match(request.cacheControl, /(?:^|,)\s*no-cache\s*(?:,|$)/iu, `${request.path} must bypass shared/browser cache on every signed-file read`);
    assert.deepEqual([...new Set(previewRequests.map((item) => item.path))].sort(), [...previewFiles.keys()].sort(), 'authenticated preview fetches must cover the exact signed file set even when verified reopens repeat them');
    const artifactRequest = requests.filter((item) => item.method === 'POST' && item.path === '/api/artifacts/preview').at(-1);
    assert.ok(artifactRequest, 'the advanced component matrix must submit a signed preview request');
    assert.equal(artifactRequest.body.workspaceId, 41);
    assert.equal(Object.hasOwn(artifactRequest.body, 'ttlHours'), false, 'ordinary signed preview builds retain the server default 24-hour authorization');
    assert.equal(artifactRequest.body.templateId, 'community-iftar');
    assert.deepEqual(artifactRequest.body.modules, ['iftar-registration', 'announcements', 'donation-campaign']);
    assert.deepEqual(artifactRequest.body.components.map(({ id, type }) => ({ id, type })), [
      { id: 'iftar-registration', type: 'iftar-registration' },
      { id: 'announcements', type: 'announcements' },
      { id: 'donation-campaign', type: 'donation-campaign' },
    ]);
    assert.deepEqual(artifactRequest.body.components[1].data.audience, ['families', 'students'], 'exact custom JSON arrays must reach the composition request');
    assert.equal(artifactRequest.body.components[1].data.summary, 'Evening 🌙', 'valid Unicode scalar pairs must reach the composition request exactly');
    assert.deepEqual(artifactRequest.body.components[1].data.details, { symbol: '🌙' }, 'nested valid Unicode must reach the composition request exactly');
    assert.equal(JSON.stringify(activeTrackResponse.manifest.components[1].data), JSON.stringify(artifactRequest.body.components[1].data), 'the signed manifest must preserve the exact submitted Unicode component document bytes');

    await evaluate(client, `(() => { const input = document.querySelector('[data-component-id="announcements"] input'); const heading = [...document.querySelectorAll('[data-component-id="announcements"] .field input')][0]; heading.value = 'Unsaved replacement'; heading.dispatchEvent(new Event('input', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#previewFrame').dataset.stale === 'true' && document.querySelector('#deployButton').disabled`), 'dirty verified preview state');
    await evaluate(client, `(() => { document.querySelector('#trackLookup').value = 'track_browser'; document.querySelector('#loadTrackButton').click(); })()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#buildRecord').hidden && document.querySelector('#buildRecordTrackID').value === 'track_browser' && document.querySelectorAll('#buildEventList li').length === 2`), 'plain build inspection and safe event timeline');
    assert.doesNotMatch(await evaluate(client, `document.querySelector('#buildTimeline').innerText`), /never-render-this/u, 'event detail must never render');
    assert.match(await evaluate(client, `document.querySelector('[data-component-id="announcements"] .advanced-document textarea').value`), /Unsaved replacement/u, 'plain inspection does not replace the current draft');
    await evaluate(client, `document.querySelector('#restoreBuildDraftButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('[data-component-id="announcements"] .advanced-document textarea').value.includes('students')`), 'exact request components start a new local draft');
    await evaluate(client, `document.querySelector('#reopenBuildPreviewButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#previewFrame').dataset.stale === 'false'`), 'history verified-preview reopen');
    const trackRestore = await evaluate(client, `({ status: document.querySelector('#previewStatus').textContent, stale: document.querySelector('#previewFrame').dataset.stale, alert: document.querySelector('#builderAlert').textContent, document: document.querySelector('[data-component-id="announcements"] .advanced-document textarea')?.value || '' })`);
    assert.equal(trackRestore.status, 'Verified staging ready', `track restore status: ${JSON.stringify(trackRestore)}`);
    assert.equal(trackRestore.stale, 'false');
    assert.match(trackRestore.document, /students/u, 'verified track restores exact component document');
    const trustedPairBeforeTrackSubstitution = await evaluate(client, `({ receipt: document.querySelector('#manifestList').textContent, raw: document.querySelector('#rawManifest').textContent, srcdoc: document.querySelector('#previewFrame').srcdoc })`);
    activeTrackResponse.track.id = 'track_substituted';
    await evaluate(client, `document.querySelector('#reopenBuildPreviewButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#buildRecordAlert').textContent.includes('track does not exactly bind') && document.querySelector('#previewFrame').dataset.stale === 'true' && document.querySelector('#deployButton').disabled`), 'selected track ID substitution fails before trust commit');
    assert.deepEqual(await evaluate(client, `({ receipt: document.querySelector('#manifestList').textContent, raw: document.querySelector('#rawManifest').textContent, srcdoc: document.querySelector('#previewFrame').srcdoc })`), trustedPairBeforeTrackSubstitution, 'a substituted selected track ID must retain the prior trusted receipt and iframe bytes');
    activeTrackResponse.track.id = 'track_browser';
    await evaluate(client, `document.querySelector('#reopenBuildPreviewButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#previewFrame').dataset.stale === 'false' && document.querySelector('#previewStatus').textContent === 'Verified staging ready'`), 'exact selected track ID recovery');
    const trackRequest = requests.find((item) => item.path === '/api/conductor/tracks/track_browser');
    assert.equal(trackRequest.authorization, `Bearer ${token}`, 'verified track reload must remain authenticated');
    const verifiedSourceBeforeFailure = await evaluate(client, `document.querySelector('#previewFrame').srcdoc`);
    failNextPreview = true;
    await evaluate(client, `document.querySelector('#builderForm').requestSubmit()`);
    await waitFor(() => evaluate(client, `document.querySelector('#builderAlert').textContent.includes('last valid draft and last verified preview were not replaced') && document.querySelector('#previewFrame').dataset.stale === 'true' && document.querySelector('#deployButton').disabled`), 'network failure recovery');
    assert.equal(await evaluate(client, `document.querySelector('#previewFrame').srcdoc`), verifiedSourceBeforeFailure, 'network failure must retain the last verified iframe bytes');
    await evaluate(client, `document.querySelector('#loadTrackButton').click()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#buildRecord').hidden && document.querySelector('#buildRecordTrackID').value === 'track_browser'`), 'build inspect after network failure');
    await evaluate(client, `document.querySelector('#reopenBuildPreviewButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#previewFrame').dataset.stale === 'false'`), 'verified track recovery after network failure');
    delayTrackResponse = true;
    delayHistoryResponse = true;
    await evaluate(client, `(() => { document.querySelector('#loadTrackButton').click(); document.querySelector('#retryBuildHistory').click(); const select = document.querySelector('#workspaceSelect'); select.value = '42'; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceSelect').value === '42' && document.querySelector('#workspaceRole').textContent === 'Architect'`), 'component track workspace switch');
    await wait(240);
    const staleTrackBoundary = await evaluate(client, `({ srcdoc: document.querySelector('#previewFrame').getAttribute('srcdoc'), manifestHidden: document.querySelector('#manifestList').hidden, template: document.querySelector('#templateSelect').value, selected: document.querySelectorAll('#moduleList input:checked').length, history: document.querySelectorAll('#buildHistoryList li').length, recordHidden: document.querySelector('#buildRecord').hidden })`);
    assert.deepEqual(staleTrackBoundary, { srcdoc: null, manifestHidden: true, template: '', selected: 0, history: 0, recordHidden: true }, 'delayed prior-workspace track/history must not restore records, preview, or documents into another workspace');
    const workspaceReturnRecoveryRequestStart = requests.length;
    await evaluate(client, `(() => { const select = document.querySelector('#workspaceSelect'); select.value = '41'; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceSelect').value === '41' && document.querySelector('#templateSelect').value === 'community-iftar' && document.querySelectorAll('#moduleList input:checked').length === 3`), 'component draft workspace recovery');
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceRole').textContent === 'Architect' && !document.querySelector('#loadTrackButton').disabled`), 'workspace access recovery before build inspection');
    await waitFor(() => requests.slice(workspaceReturnRecoveryRequestStart).some((request) => request.method === 'GET'
      && request.path === '/api/conductor/tracks/track_browser' && request.search === '?includeVerifiedPreview=true'), 'workspace return exact verified-preview recovery request');
    try {
      await waitFor(() => evaluate(client, `document.querySelector('#previewStatus').textContent === 'Verified staging ready'
        && document.querySelector('#previewFrame').dataset.stale === 'false'
        && document.querySelector('#starterRecoveryState').textContent.includes('Exact workspace-bound signed preview verified; durable history confirmed')
        && document.querySelector('#buildHistoryState').textContent.includes('Exact verified preview track track_browser is confirmed')
        && document.querySelector('#domainClaimSelect').value === 'claim_browser'
        && !document.querySelector('#deployButton').disabled`), 'automatic selected or newest verified recovery after workspace return', 16_000);
    } catch (error) {
      const recoveryDiagnostic = await evaluate(client, `({
        workspace: document.querySelector('#workspaceSelect').value,
        role: document.querySelector('#workspaceRole').textContent,
        previewStatus: document.querySelector('#previewStatus').textContent,
        previewStale: document.querySelector('#previewFrame').dataset.stale || '',
        previewLoading: !document.querySelector('#previewLoading').hidden,
        starterRecovery: document.querySelector('#starterRecoveryState').textContent,
        historyState: document.querySelector('#buildHistoryState').textContent,
        historyRecords: document.querySelectorAll('#buildHistoryList li').length,
        claim: document.querySelector('#domainClaimSelect').value,
        deployDisabled: document.querySelector('#deployButton').disabled,
        builderAlert: document.querySelector('#builderAlert').textContent,
        buildRecordAlert: document.querySelector('#buildRecordAlert').textContent,
      })`);
      throw new Error(`${error.message} · workspace return recovery state ${JSON.stringify(recoveryDiagnostic)}`);
    }
    const automaticRecoveryPreviewReads = requests.filter((item) => item.path === '/api/conductor/tracks/track_browser' && item.search === '?includeVerifiedPreview=true').length;
    assert.deepEqual(await evaluate(client, `({ appName: document.querySelector('#appName').value, organization: document.querySelector('#organizationName').value, city: document.querySelector('#city').value })`), { appName: 'Community app', organization: 'QA Community', city: '' }, 'workspace return restores only scoped component documents, not prior app identity or city');
    assert.equal(await evaluate(client, `document.querySelector('#trackLookup').value`), '', 'workspace return must not restore an opaque track reference');
    await evaluate(client, `(() => { document.querySelector('#trackLookup').value = 'track_browser'; document.querySelector('#loadTrackButton').click(); })()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#buildRecord').hidden && document.querySelector('#buildRecordTrackID').value === 'track_browser'`), 'build inspect after workspace return');
    assert.equal(requests.filter((item) => item.path === '/api/conductor/tracks/track_browser' && item.search === '?includeVerifiedPreview=true').length, automaticRecoveryPreviewReads, 'plain build inspection must not issue a redundant verified-preview read after automatic recovery');
    await waitFor(() => evaluate(client, `document.querySelector('#domainClaimSelect').value === 'claim_browser' && !document.querySelector('#deployButton').disabled`), 'reloaded domain claim is ready for the exact signed origin');

    activationFailNext = false;
    delayPublicationRequest = true;
    const delayedExactPublicationStart = requests.length;
    await evaluate(client, `document.querySelector('#deployButton').click()`);
    await waitFor(() => requests.slice(delayedExactPublicationStart).some((request) => request.method === 'POST' && request.path === '/api/conductor/tracks/track_browser/publication'), 'delayed exact-claim publication request in flight');
    assert.deepEqual(await evaluate(client, `({ claim: document.querySelector('#domainClaimSelect').value, selectDisabled: document.querySelector('#domainClaimSelect').disabled, publishBusy: document.querySelector('#deployButton').getAttribute('aria-busy') })`), { claim: 'claim_browser', selectDisabled: true, publishBusy: 'true' }, 'publication natively locks exact claim selection while its request is in flight');
    const exactClaimStateBeforeBlockedSelection = await evaluate(client, `document.querySelector('#domainClaimsState').textContent`);
    const domainRequestsBeforeBlockedSelection = requests.filter((request) => request.path.startsWith('/api/workspaces/41/domains')).length;
    await evaluate(client, `(() => { const select = document.querySelector('#domainClaimSelect'); select.value = 'claim_pending'; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    assert.deepEqual(await evaluate(client, `({ claim: document.querySelector('#domainClaimSelect').value, selectedState: document.querySelector('#domainClaimsState').textContent, selectDisabled: document.querySelector('#domainClaimSelect').disabled, publishBusy: document.querySelector('#deployButton').getAttribute('aria-busy') })`), {
      claim: 'claim_browser', selectedState: exactClaimStateBeforeBlockedSelection, selectDisabled: true, publishBusy: 'true',
    }, 'synthetic change on the disabled select is rejected without changing exact claim state');
    assert.equal(requests.filter((request) => request.path.startsWith('/api/workspaces/41/domains')).length, domainRequestsBeforeBlockedSelection, 'blocked same-workspace claim change emits no domain load or mutation');
    await waitFor(() => evaluate(client, `document.querySelector('#deployStateText').textContent === 'Serving'
      && document.querySelector('#domainPublicationRecordTitle').textContent.includes('publication_browser')
      && document.querySelector('#deployButton').getAttribute('aria-busy') === 'false'
      && !document.querySelector('#domainClaimSelect').disabled
      && !document.querySelector('#retryDomainClaims').disabled`), 'delayed publication completes against the original exact claim and releases controls');
    const delayedExactPublicationRequests = requests.slice(delayedExactPublicationStart);
    assert.ok(delayedExactPublicationRequests.some((request) => request.method === 'POST' && request.path === '/api/conductor/tracks/track_browser/publication' && request.body?.claimId === 'claim_browser'), 'publication mutation stays bound to the original exact claim');
    assert.ok(delayedExactPublicationRequests.some((request) => request.method === 'GET' && request.path.endsWith('/domains/claim_browser/publications') && request.search === '?limit=20'), 'publication completion reloads authoritative history for the original exact claim');
    assert.ok(delayedExactPublicationRequests.some((request) => request.method === 'GET' && request.path.endsWith('/domains/claim_browser/publications') && request.search === '?publicationId=publication_browser'), 'publication completion exact-selects its authoritative activation record');
    assert.equal(delayedExactPublicationRequests.some((request) => request.path.includes('claim_pending') || request.path === '/api/workspaces/41/domains'), false, 'blocked selection cannot redirect publication or issue a domain-claims reload');
    assert.equal(delayedExactPublicationRequests.some((request) => request.path.startsWith('/api/workspaces/41/domains') && request.method !== 'GET'), false, 'blocked selection emits no domain mutation while authoritative publication reads complete');
    assert.deepEqual(await evaluate(client, `({ claim: document.querySelector('#domainClaimSelect').value, revokeDisabled: document.querySelector('#revokeDomainButton').disabled, track: document.querySelector('#buildRecordTitle').textContent, published: document.querySelector('#deployStateText').textContent, exact: document.querySelector('#domainPublicationRecordTitle').textContent.includes('publication_browser'), publishBusy: document.querySelector('#deployButton').getAttribute('aria-busy') })`), { claim: 'claim_browser', revokeDisabled: false, track: 'community-iftar · PUBLISHED', published: 'Serving', exact: true, publishBusy: 'false' }, 'authoritative completion retains original claim truth without stranded controls or busy state');
    activationFailNext = true;
    domainPublications = [];
    Object.assign(activeTrackResponse.track, { status: 'PREVIEW_READY', version: 6, claimId: '', publication: undefined, updatedAt: '2026-08-18T04:23:28Z' });
    await evaluate(client, `(() => { const select = document.querySelector('#domainClaimSelect'); select.value = 'claim_browser'; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainPublicationRecord').hidden && document.querySelector('#domainPublicationState').textContent.includes('No publication activation records')`), 'publication context reset after isolated exact-claim journey');
    await evaluate(client, `(() => { document.querySelector('#trackLookup').value = 'track_browser'; document.querySelector('#loadTrackButton').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#buildRecordTitle').textContent === 'community-iftar · PREVIEW_READY'`), 'exact-claim publication fixture reset');
    await evaluate(client, `document.querySelector('#reopenBuildPreviewButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#previewFrame').dataset.stale === 'false' && !document.querySelector('#deployButton').disabled`), 'publication claim and verified preview reset');

    delayPublicationRequest = true;
    const trustedPairBeforePublicationRace = await evaluate(client, `({
      srcdoc: document.querySelector('#previewFrame').srcdoc,
      raw: document.querySelector('#rawManifest').textContent,
    })`);
    await evaluate(client, `(() => { document.querySelector('#deployButton').click(); for (const [id, value] of [['appName', 'Newer same-workspace preview'], ['organizationName', 'QA Community'], ['city', 'Salt Lake City']]) { const input = document.querySelector('#' + id); input.value = value; input.dispatchEvent(new Event('input', { bubbles: true })); } document.querySelector('#builderForm').requestSubmit(); })()`);
    assert.equal(await evaluate(client, `document.querySelector('.sample-title').textContent`), 'Newer same-workspace preview', 'vanilla app-name sync remains live after a previously verified preview becomes a newer draft');
    await waitFor(() => evaluate(client, `document.querySelector('#previewLoading').hidden
      && document.querySelector('#previewStatus').textContent === 'Verification unavailable'
      && document.querySelector('#builderAlert').textContent.includes('returned evidence is not authorized for use')
      && document.querySelector('#previewFrame').dataset.stale === 'true'
      && document.querySelector('#deployButton').disabled`), 'newer same-workspace preview generation');
    assert.deepEqual(await evaluate(client, `({
      srcdoc: document.querySelector('#previewFrame').srcdoc,
      raw: document.querySelector('#rawManifest').textContent,
    })`), trustedPairBeforePublicationRace, 'unattested race response retains but does not trust the prior signed iframe and receipt');
    await wait(240);
    assert.notEqual(await evaluate(client, `document.querySelector('#buildRecordTitle').textContent`), 'community-iftar · PUBLICATION_REQUESTED', 'delayed publication request cannot overwrite a newer preview generation');
    previewBuilds = 1;
    Object.assign(activeTrackResponse.track, { status: 'PREVIEW_READY', version: 6, claimId: '', publication: undefined, updatedAt: '2026-08-18T04:23:28Z' });
    await evaluate(client, `(() => { document.querySelector('#trackLookup').value = 'track_browser'; document.querySelector('#loadTrackButton').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#buildRecordTrackID').value === 'track_browser'`), 'publication race build recovery');
    await evaluate(client, `document.querySelector('#reopenBuildPreviewButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#previewFrame').dataset.stale === 'false' && !document.querySelector('#deployButton').disabled`), 'publication race verified preview recovery');

    const publicationRequestsBeforeActivation = requests.filter((request) => request.path === '/api/conductor/tracks/track_browser/publication').length;
    await evaluate(client, `document.querySelector('#deployButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainStatus').textContent.includes('durably recorded') && document.querySelector('#deployButton').textContent === 'Retry activation' && document.querySelector('#buildRecordTitle').textContent.includes('PUBLICATION_REQUESTED')`), 'activation failure reconciles durable publication request');
    assert.doesNotMatch(await evaluate(client, `document.querySelector('#buildTimeline').innerText`), /never-render-this/u, 'publication event detail must remain hidden');
    await evaluate(client, `document.querySelector('#deployButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#deployStateText').textContent === 'Serving' && document.querySelector('#domainPublicationRecordTitle').textContent.includes('Serving — activation record retained') && document.querySelector('#domainPublicationList').textContent.includes('publication_browser')`), 'activation retry confirms exact server serving truth without a second publication request');
    assert.equal(requests.filter((request) => request.path === '/api/conductor/tracks/track_browser/publication').length, publicationRequestsBeforeActivation + 1, 'activation retry must not repeat the durable publication request');
    assert.ok(requests.some((request) => request.method === 'GET' && request.path.endsWith('/domains/claim_browser/publications') && request.search === '?limit=20'), 'ordinary publication history uses the bounded default page');
    assert.ok(requests.some((request) => request.method === 'GET' && request.path.endsWith('/domains/claim_browser/publications') && request.search === '?publicationId=publication_browser'), 'active serving proof is read through an exact mutually exclusive publication selection');
    assert.doesNotMatch(await evaluate(client, `document.querySelector('#domainPublicationRecord').innerText`), /\b(?:Active|Verified|healthy|live)\b/iu, 'activation fact is not presented as a health or trust label');
    const trustedReceiptBeforeNegatives = await evaluate(client, `Object.fromEntries([...document.querySelectorAll('#manifestList .manifest-row')].map((row) => [row.querySelector('dt').textContent, row.querySelector('dd').textContent]))`);
    const trustedRawBeforeNegatives = await evaluate(client, `document.querySelector('#rawManifest').textContent`);
    const trustedSourceBeforeNegatives = await evaluate(client, `document.querySelector('#previewFrame').srcdoc`);
    assert.equal(requests.filter((item) => item.path === '/assets/datastar-v1.0.2.js').length, 0, 'the mismatched public runtime path must never be fetched or executed');
    const signedRuntimeRequests = requests.filter((item) => item.path === signedRuntimePath);
    assert.ok(signedRuntimeRequests.length > 0, 'the track-scoped manifest-listed runtime must be fetched');
    assert.ok(signedRuntimeRequests.every((item) => item.authorization === `Bearer ${token}`), 'every signed runtime fetch must use the current track reader Bearer token');

    for (const failure of [
      { name: 'missing runtime descriptor', variant: 'missing-runtime', preflight: true, message: /runtime and theme descriptors are missing|file descriptor is missing/iu },
      { name: 'runtime descriptor mismatch', variant: 'runtime-descriptor-mismatch', preflight: true, message: /runtime and theme descriptors are missing|inconsistent/iu },
      { name: 'duplicate signed descriptor', variant: 'duplicate-descriptor', preflight: true, message: /descriptors are incomplete or ambiguous/iu },
      { name: 'malformed signed descriptor path', variant: 'malformed-descriptor-path', preflight: true, message: /descriptors are incomplete or ambiguous/iu },
      { name: 'theme query substitution', variant: 'theme-url-query', preflight: true, message: /theme URL is not bound/iu },
      { name: 'document wrong-track substitution', variant: 'document-wrong-track', preflight: true, message: /document URL is not bound/iu },
      { name: 'missing signed runtime response', missing: signedRuntimePath, message: /signed preview runtime could not be loaded/iu },
      { name: 'document same-length SHA corruption', corrupt: `${previewPrefix}index.html`, message: /document digest does not match/iu },
      { name: 'theme same-length SHA corruption', corrupt: `${previewPrefix}theme.css`, message: /theme digest does not match/iu },
      { name: 'styles same-length SHA corruption', corrupt: `${previewPrefix}app.css`, message: /styles digest does not match/iu },
      { name: 'runtime same-length SHA corruption', corrupt: signedRuntimePath, message: /runtime digest does not match/iu },
      { name: 'runtime byte-length mismatch', lengthMismatch: signedRuntimePath, message: /runtime digest does not match/iu },
    ]) {
      const signedFileFetchesBefore = requests.filter((item) => previewFiles.has(item.path)).length;
      if (failure.variant) signedFileVariantNext = failure.variant;
      if (failure.corrupt) corruptSignedFileNext = failure.corrupt;
      if (failure.lengthMismatch) lengthMismatchSignedFileNext = failure.lengthMismatch;
      if (failure.missing) missingSignedFileNext = failure.missing;
      await evaluate(client, `document.querySelector('#reopenBuildPreviewButton').click()`);
      await waitFor(() => evaluate(client, `document.querySelector('#previewStatus').textContent === 'Last verified preview retained' && document.querySelector('#previewFrame').dataset.stale === 'true' && document.querySelector('#deployButton').disabled`), `${failure.name} fail-closed preview`);
      const failureState = await evaluate(client, `({ receipt: Object.fromEntries([...document.querySelectorAll('#manifestList .manifest-row')].map((row) => [row.querySelector('dt').textContent, row.querySelector('dd').textContent])), raw: document.querySelector('#rawManifest').textContent, srcdoc: document.querySelector('#previewFrame').srcdoc, alert: document.querySelector('#buildRecordAlert').textContent })`);
      assert.deepEqual(failureState.receipt, trustedReceiptBeforeNegatives, `${failure.name} must retain the trusted receipt`);
      assert.equal(failureState.raw, trustedRawBeforeNegatives, `${failure.name} must retain the trusted raw manifest`);
      assert.equal(failureState.srcdoc, trustedSourceBeforeNegatives, `${failure.name} must retain the trusted iframe bytes`);
      assert.match(failureState.alert, failure.message, `${failure.name} must expose the bounded signed-file reason`);
      if (failure.preflight) assert.equal(requests.filter((item) => previewFiles.has(item.path)).length, signedFileFetchesBefore, `${failure.name} must reject before any signed-file fetch`);
      await evaluate(client, `document.querySelector('#reopenBuildPreviewButton').click()`);
      await waitFor(() => evaluate(client, `document.querySelector('#buildRecordAlert').textContent.includes('Verified preview reopened') && document.querySelector('#previewStatus').textContent === 'Verified staging ready'`), `${failure.name} exact signed-file recovery`);
    }

    const declaredHeadingBeforeDelayedRuntime = await evaluate(client, `document.querySelector('[data-component-id="announcements"] .component-fields .field input').value`);
    const signedRuntimeRequestsBeforeDraftRace = requests.filter((item) => item.path === signedRuntimePath).length;
    delaySignedRuntimeResponse = true;
    await evaluate(client, `document.querySelector('#reopenBuildPreviewButton').click()`);
    await waitFor(() => requests.filter((item) => item.path === signedRuntimePath).length > signedRuntimeRequestsBeforeDraftRace, 'delayed signed runtime draft-race request');
    await evaluate(client, `(() => { const input = document.querySelector('[data-component-id="announcements"] .component-fields .field input'); input.value = 'Newer local draft during signed runtime read'; input.dispatchEvent(new Event('input', { bubbles: true })); })()`);
    await wait(240);
    assert.deepEqual(await evaluate(client, `({ raw: document.querySelector('#rawManifest').textContent, srcdoc: document.querySelector('#previewFrame').srcdoc, stale: document.querySelector('#previewFrame').dataset.stale, publishDisabled: document.querySelector('#deployButton').disabled })`), { raw: trustedRawBeforeNegatives, srcdoc: trustedSourceBeforeNegatives, stale: 'true', publishDisabled: true }, 'a delayed signed runtime cannot replace trusted evidence after the same-workspace draft changes');
    await evaluate(client, `(() => { const input = document.querySelector('[data-component-id="announcements"] .component-fields .field input'); input.value = ${JSON.stringify(declaredHeadingBeforeDelayedRuntime)}; input.dispatchEvent(new Event('input', { bubbles: true })); document.querySelector('#reopenBuildPreviewButton').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#buildRecordAlert').textContent.includes('Verified preview reopened') && document.querySelector('#previewStatus').textContent === 'Verified staging ready'`), 'exact signed files recover after delayed draft race');

    for (let index = 1; index < receiptVariants.length; index += 1) {
      const variant = receiptVariants[index];
      const previewFileRequestsBefore = requests.filter((item) => previewFiles.has(item.path)).length;
      await evaluate(client, `document.querySelector('#builderForm').requestSubmit()`);
      await waitFor(async () => previewBuilds === index + 1
        && await evaluate(client, `document.querySelector('#previewLoading').hidden && ['Verification unavailable', 'Authorization expired · preview unavailable', 'Last verified preview retained'].includes(document.querySelector('#previewStatus').textContent)`), `${variant} receipt response`);
      const retainedEvidence = await evaluate(client, `({
        receipt: Object.fromEntries([...document.querySelectorAll('#manifestList .manifest-row')].map((row) => [row.querySelector('dt').textContent, row.querySelector('dd').textContent])),
        raw: document.querySelector('#rawManifest').textContent,
        srcdoc: document.querySelector('#previewFrame').srcdoc,
        stale: document.querySelector('#previewFrame').dataset.stale,
        publishDisabled: document.querySelector('#deployButton').disabled,
        alert: document.querySelector('#builderAlert').textContent,
      })`);
      if (variant === 'elapsed-expiry') {
        assert.deepEqual(retainedEvidence.receipt, {}, 'expired evidence removes prior healthy manifest labels');
        assert.equal(retainedEvidence.raw, '', 'expired evidence removes the raw receipt from the current trust surface');
        assert.equal(retainedEvidence.srcdoc, '', 'expired evidence removes staging iframe bytes');
      } else {
        assert.deepEqual(retainedEvidence.receipt, trustedReceiptBeforeNegatives, `${variant} must not replace the last trusted receipt with untrusted fields`);
        assert.equal(retainedEvidence.raw, trustedRawBeforeNegatives, `${variant} must retain the trusted raw manifest`);
        assert.equal(retainedEvidence.srcdoc, trustedSourceBeforeNegatives, `${variant} must retain the last verified iframe bytes`);
      }
      assert.equal(retainedEvidence.stale, 'true');
      assert.equal(retainedEvidence.publishDisabled, true);
      assert.match(retainedEvidence.alert, /returned evidence is not authorized for use|last verified preview (?:was retained|were not replaced)/iu);
      if (variant.startsWith('track-')) {
        assert.equal(requests.filter((item) => previewFiles.has(item.path)).length, previewFileRequestsBefore, `${variant} must fail before fetching or committing preview files`);
        assert.match(retainedEvidence.alert, /track does not exactly bind/iu, `${variant} must explain the track binding failure without trusting it`);
      }
      if (variant === 'elapsed-expiry') {
        await evaluate(client, `document.querySelector('#reopenBuildPreviewButton').click()`);
        await waitFor(() => evaluate(client, `document.querySelector('#previewStatus').textContent === 'Verified staging ready' && document.querySelector('#previewFrame').dataset.stale === 'false'`), 'post-expiry exact signed preview recovery for subsequent negative cases');
      }
    }

    const originalPublicationExpiry = domainPublications[0].authorizationExpiresAt;
    publicationServerTime = '2026-08-18T07:00:00.000Z';
    Object.assign(domainPublications[0], { servingState: 'serving', authorizationExpiresAt: '2026-08-18T07:00:01.500Z' });
    automaticPublicationExpiry = true;
    automaticPublicationExactReads = 0;
    Object.assign(activeTrackResponse.verification, { authorizationState: 'expired', serverTime: '2100-01-01T00:00:00Z' });
    const exactReadsBeforeAutomaticExpiry = requests.filter((request) => request.method === 'GET' && request.path.endsWith('/domains/claim_browser/publications') && request.search === '?publicationId=publication_browser').length;
    await evaluate(client, `(() => {
      const select = document.querySelector('#domainClaimSelect');
      select.value = '';
      select.dispatchEvent(new Event('change', { bubbles: true }));
      select.value = 'claim_browser';
      select.dispatchEvent(new Event('change', { bubbles: true }));
    })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainPublicationList').textContent.includes('Serving — activation record retained')
      && document.querySelector('#domainPublicationRecord').hidden`), 'ordinary history load arms the active publication without exact selection');
    assert.equal(automaticPublicationExactReads, 0, 'ordinary active history does not need an eager exact read to own its deadline');
    delayPublicationContextResponse = true;
    const resumeSnapshot = await evaluate(client, `(() => {
      window.dispatchEvent(new Event('focus'));
      return {
        list: document.querySelector('#domainPublicationList').textContent,
        record: document.querySelector('#domainPublicationRecordTitle').textContent,
        replaceHidden: document.querySelector('#replaceDomainPublication').hidden,
        rollbackHidden: document.querySelector('#rollbackDomainPublication').hidden,
      };
    })()`);
    assert.doesNotMatch(`${resumeSnapshot.list} ${resumeSnapshot.record}`, /Serving —/u, 'focus after possible sleep removes serving truth before the exact response arrives');
    assert.match(resumeSnapshot.record, /Authorization expired · not serving — activation record retained/u);
    assert.deepEqual({ replaceHidden: resumeSnapshot.replaceHidden, rollbackHidden: resumeSnapshot.rollbackHidden }, { replaceHidden: true, rollbackHidden: true }, 'conservative resume recheck grants no publication action');
    await waitFor(() => evaluate(client, `document.querySelector('#domainPublicationRecordTitle').textContent.startsWith('Serving — activation record retained')`), 'authoritative exact resume response may restore serving truth');
    await waitFor(() => evaluate(client, `document.querySelector('#domainPublicationRecordTitle').textContent.startsWith('Authorization expired · not serving — activation record retained')
      && document.querySelector('#deployStateText').textContent === 'Activated · not serving'
      && !document.querySelector('#replaceDomainPublication').hidden
      && document.querySelector('#previewEmptyTitle').textContent === 'Signed preview unavailable'
      && document.querySelector('#previewEmptyCopy').textContent.includes('Authorization expired · not serving')
      && document.querySelector('#manifestEmpty').textContent.includes('Authorization expired · not serving')
      && document.querySelector('#previewStatus').textContent === 'Authorization expired · preview unavailable'
      && document.querySelector('#manifestList').hidden
      && document.querySelector('#rawManifest').textContent === ''
      && document.querySelector('#buildRecordAlert').textContent.includes('Authorization expired · not serving')
      && !document.querySelector('#previewFrame').getAttribute('srcdoc')`), 'automatic exact server-clock expiry presentation');
    automaticPublicationExpiry = false;
    assert.ok(requests.filter((request) => request.method === 'GET' && request.path.endsWith('/domains/claim_browser/publications') && request.search === '?publicationId=publication_browser').length >= exactReadsBeforeAutomaticExpiry + 2, 'monotonic deadline triggers its own exact authoritative re-read without a manual Recheck action');
    const expiredPresentation = await evaluate(client, `[
      document.querySelector('#previewTitle').innerText,
      document.querySelector('#domainPublicationRecord').innerText,
      document.querySelector('#deployTitle').innerText,
      document.querySelector('#deployStateHelp').innerText,
      document.querySelector('#previewStatus').innerText,
      document.querySelector('#previewEmpty').innerText,
      document.querySelector('#manifestEmpty').innerText,
      document.querySelector('#buildStateText').innerText,
      document.querySelector('#buildRecordAlert').innerText,
      document.querySelector('#builderAlert').innerText,
      document.querySelector('#starterRecoveryState').innerText,
      document.querySelector('#buildHistoryState').innerText,
    ].join(' ')`);
    assert.match(expiredPresentation, /Authorization expired · not serving — activation record retained/u);
    assert.doesNotMatch(expiredPresentation, /\b(?:Active|Verified|healthy|live)\b/iu, 'expired context exposes no active, verified, healthy, or live affordance');
    const expiredAccessibleLabels = await evaluate(client, `[
      document.querySelector('#previewFrame').title,
      ...[...document.querySelectorAll('#domainPublicationRecord button, #previewStatus, #previewEmpty, #manifestEmpty, #buildRecordAlert, #builderAlert, #deployStateText, #deployStateHelp')]
        .filter((node) => !node.hidden && !node.closest('[hidden]'))
        .map((node) => node.getAttribute('aria-label') || node.textContent),
    ].join(' ')`);
    assert.doesNotMatch(expiredAccessibleLabels, /\b(?:Active|Verified|healthy|live)\b/iu, 'expired accessible names expose no healthy or current-serving label');

    const publicationActionHeights = await evaluate(client, `[...document.querySelectorAll('#domainPublicationRecord button, #loadMoreDomainPublications')]
      .filter((button) => !button.hidden && !button.closest('[hidden]'))
      .map((button) => button.getBoundingClientRect().height)`);
    assert.ok(publicationActionHeights.every((height) => height >= 44), `publication action targets must be at least 44px: ${publicationActionHeights.join(', ')}`);
    const replacementPreviewCountBeforeCancel = requests.filter((request) => request.method === 'POST' && request.path === '/api/artifacts/preview' && request.body?.ttlHours === 2160).length;
    await client.send('Emulation.setFocusEmulationEnabled', { enabled: true });
    await client.send('Page.bringToFront');
    await waitFor(() => evaluate(client, `document.hasFocus()`), 'publication action page foreground');
    const replacementActionPreconditions = await evaluate(client, `(() => {
      const replacement = document.querySelector('#replaceDomainPublication');
      const confirmation = document.querySelector('#domainPublicationConfirm');
      const disclosure = replacement.closest('details');
      if (disclosure) disclosure.open = true;
      const replacementRect = replacement.getBoundingClientRect();
      window.__taawunReplacementKeyCapture = null;
      replacement.addEventListener('keydown', (event) => {
        window.__taawunReplacementKeyCapture = {
          key: event.key,
          code: event.code,
          isTrusted: event.isTrusted,
          activeElement: document.activeElement?.id || document.activeElement?.tagName || '',
        };
      }, { once: true });
      return {
        replacementHidden: replacement.hidden || Boolean(replacement.closest('[hidden]')),
        replacementDisabled: replacement.disabled,
        disclosureOpen: Boolean(disclosure && disclosure.open),
        replacementRects: replacement.getClientRects().length,
        replacementWidth: replacementRect.width,
        replacementHeight: replacementRect.height,
        confirmationHidden: confirmation.hidden,
        activeElementBeforeDOMFocus: document.activeElement?.id || document.activeElement?.tagName || '',
        documentHasFocus: document.hasFocus(),
      };
    })()`);
    assert.deepEqual({
      replacementHidden: replacementActionPreconditions.replacementHidden,
      replacementDisabled: replacementActionPreconditions.replacementDisabled,
      disclosureOpen: replacementActionPreconditions.disclosureOpen,
      confirmationHidden: replacementActionPreconditions.confirmationHidden,
      documentHasFocus: replacementActionPreconditions.documentHasFocus,
    }, {
      replacementHidden: false,
      replacementDisabled: false,
      disclosureOpen: true,
      confirmationHidden: true,
      documentHasFocus: true,
    }, `replacement keyboard preconditions must be actionable: ${JSON.stringify(replacementActionPreconditions)}`);
    assert.ok(replacementActionPreconditions.replacementRects > 0
      && replacementActionPreconditions.replacementWidth > 0 && replacementActionPreconditions.replacementHeight >= 44,
    `replacement control must have focusable rendered geometry: ${JSON.stringify(replacementActionPreconditions)}`);
    await client.send('DOM.enable');
    const publicationDocument = await client.send('DOM.getDocument', { depth: 0 });
    const replacementNode = await client.send('DOM.querySelector', { nodeId: publicationDocument.root.nodeId, selector: '#replaceDomainPublication' });
    assert.ok(replacementNode.nodeId > 0, `replacement control must be addressable through Chromium DOM focus: ${JSON.stringify(replacementActionPreconditions)}`);
    await client.send('DOM.focus', { nodeId: replacementNode.nodeId });
    const replacementEnter = { key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13, nativeVirtualKeyCode: 13 };
    await client.send('Input.dispatchKeyEvent', { type: 'keyDown', text: '\r', unmodifiedText: '\r', ...replacementEnter });
    await client.send('Input.dispatchKeyEvent', { type: 'keyUp', ...replacementEnter });
    let replacementConfirmationFocus;
    try {
      replacementConfirmationFocus = await waitFor(() => evaluate(client, `(() => {
        const captured = window.__taawunReplacementKeyCapture;
        const confirmation = document.querySelector('#domainPublicationConfirm');
        const confirm = document.querySelector('#confirmDomainPublicationAction');
        if (!captured || confirmation.hidden || confirm.disabled || document.activeElement?.id !== 'confirmDomainPublicationAction') return null;
        delete window.__taawunReplacementKeyCapture;
        return {
          captured,
          confirmationHidden: confirmation.hidden,
          confirmDisabled: confirm.disabled,
          confirmConnected: confirm.isConnected,
          confirmRects: confirm.getClientRects().length,
          activeElement: document.activeElement?.id || document.activeElement?.tagName || '',
          confirmationCopy: document.querySelector('#domainPublicationConfirmCopy').textContent,
        };
      })()`), `replacement keyboard activation and confirmation focus: ${JSON.stringify(replacementActionPreconditions)}`);
    } catch (error) {
      const diagnostic = await evaluate(client, `(() => {
        const replacement = document.querySelector('#replaceDomainPublication');
        const confirmation = document.querySelector('#domainPublicationConfirm');
        const confirm = document.querySelector('#confirmDomainPublicationAction');
        const confirmRect = confirm.getBoundingClientRect();
        return {
          captured: window.__taawunReplacementKeyCapture,
          confirmationHidden: confirmation.hidden,
          confirmDisabled: confirm.disabled,
          confirmConnected: confirm.isConnected,
          confirmRects: confirm.getClientRects().length,
          confirmWidth: confirmRect.width,
          confirmHeight: confirmRect.height,
          activeElement: document.activeElement?.id || document.activeElement?.tagName || '',
          replacementHidden: replacement.hidden || Boolean(replacement.closest('[hidden]')),
          replacementDisabled: replacement.disabled,
          replacementDisclosureOpen: Boolean(replacement.closest('details')?.open),
          recordTitle: document.querySelector('#domainPublicationRecordTitle').textContent,
          confirmationCopy: document.querySelector('#domainPublicationConfirmCopy').textContent,
          publicationAlert: document.querySelector('#domainPublicationAlert').textContent,
          workspaceAnnouncer: document.querySelector('#workspaceAnnouncer').textContent,
        };
      })()`);
      error.message = `${error.message}; replacement confirmation diagnostic: ${JSON.stringify(diagnostic)}`;
      throw error;
    }
    assert.deepEqual(replacementConfirmationFocus.captured, {
      key: 'Enter', code: 'Enter', isTrusted: true, activeElement: 'replaceDomainPublication',
    }, `replacement must be activated by a trusted Enter event on its focused initiator: ${JSON.stringify(replacementConfirmationFocus)}`);
    assert.deepEqual({
      confirmationHidden: replacementConfirmationFocus.confirmationHidden,
      confirmDisabled: replacementConfirmationFocus.confirmDisabled,
      confirmConnected: replacementConfirmationFocus.confirmConnected,
      activeElement: replacementConfirmationFocus.activeElement,
    }, { confirmationHidden: false, confirmDisabled: false, confirmConnected: true, activeElement: 'confirmDomainPublicationAction' }, `replacement confirmation must open with keyboard focus: ${JSON.stringify(replacementConfirmationFocus)}`);
    assert.ok(replacementConfirmationFocus.confirmRects > 0, `replacement confirmation must be rendered: ${JSON.stringify(replacementConfirmationFocus)}`);
    assert.match(replacementConfirmationFocus.confirmationCopy, /new immutable 90-day signed replacement/u);
    await evaluate(client, `document.querySelector('#cancelDomainPublicationAction').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainPublicationConfirm').hidden && document.activeElement?.id === 'replaceDomainPublication' && document.querySelector('#workspaceAnnouncer').textContent.includes('cancelled')`), 'replacement cancellation focus and live feedback');
    assert.equal(requests.filter((request) => request.method === 'POST' && request.path === '/api/artifacts/preview' && request.body?.ttlHours === 2160).length, replacementPreviewCountBeforeCancel, 'cancelling replacement sends no mutation');

    await evaluate(client, `document.querySelector('#replaceDomainPublication').click()`);
    Object.assign(domainPublications[0], { servingState: 'inactive' });
    await evaluate(client, `document.querySelector('#confirmDomainPublicationAction').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainPublicationAlert').textContent.includes('Inactive publication proof is inconsistent') && document.activeElement?.id === 'domainPublicationRecord'`), 'stale publication confirmation fails closed and restores record focus');
    assert.equal(requests.filter((request) => request.method === 'POST' && request.path === '/api/artifacts/preview' && request.body?.ttlHours === 2160).length, replacementPreviewCountBeforeCancel, 'changed exact publication context blocks mutation before successor creation');

    publicationServerTime = '2026-08-18T06:02:00Z';
    Object.assign(domainPublications[0], { servingState: 'serving', authorizationExpiresAt: originalPublicationExpiry });
    Object.assign(activeTrackResponse.verification, { authorizationState: 'active', serverTime: '2026-08-18T04:23:28Z' });
    await evaluate(client, `document.querySelector('#refreshDomainPublication').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainPublicationRecordTitle').textContent.startsWith('Serving — activation record retained') && !document.querySelector('#replaceDomainPublication').hidden`), 'proactive replacement source exact recheck');

    publicationContextFailNext = true;
    await evaluate(client, `document.querySelector('#refreshDomainPublication').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainPublicationAlert').textContent.includes('Synthetic publication context interruption')`), 'serving publication exact reread failure');
    const unavailablePublicationPresentation = await evaluate(client, `({
      copy: [
        document.querySelector('#domainPublicationRecord').innerText,
        document.querySelector('#deployStateText').innerText,
        document.querySelector('#deployStateHelp').innerText,
        document.querySelector('#deployButton').innerText,
      ].join(' '),
      recordTitle: document.querySelector('#domainPublicationRecordTitle').textContent,
      deployState: document.querySelector('#deployStateText').textContent,
      replaceHidden: document.querySelector('#replaceDomainPublication').hidden,
      rollbackHidden: document.querySelector('#rollbackDomainPublication').hidden,
      confirmHidden: document.querySelector('#domainPublicationConfirm').hidden,
    })`);
    assert.match(unavailablePublicationPresentation.recordTitle, /Public delivery truth unavailable · not serving — activation record retained/u);
    assert.equal(unavailablePublicationPresentation.deployState, 'Activated · not serving');
    assert.doesNotMatch(unavailablePublicationPresentation.copy, /\b(?:Active|Verified|healthy|live)\b/iu, 'failed exact reread neutralizes both record and deployment affordances');
    assert.deepEqual({
      replaceHidden: unavailablePublicationPresentation.replaceHidden,
      rollbackHidden: unavailablePublicationPresentation.rollbackHidden,
      confirmHidden: unavailablePublicationPresentation.confirmHidden,
    }, { replaceHidden: true, rollbackHidden: true, confirmHidden: true }, 'failed exact reread exposes no replacement, rollback, or confirmation action');
    await evaluate(client, `document.querySelector('#refreshDomainPublication').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainPublicationRecordTitle').textContent.startsWith('Serving — activation record retained') && !document.querySelector('#replaceDomainPublication').hidden`), 'publication truth recovers only after a successful exact reread');

    const successorNegativeVariants = ['wrong-created-by', 'wrong-current-subject', 'wrong-origin-policy'];
    for (const successorVariant of successorNegativeVariants) {
      replacementSuccessorVariant = successorVariant;
      const publicationMutationsBefore = requests.filter((request) => request.method === 'POST' && request.path === '/api/conductor/tracks/track_replacement/publication').length;
      const activationMutationsBefore = requests.filter((request) => request.method === 'POST' && request.path === '/api/conductor/tracks/track_replacement/activate').length;
      await evaluate(client, `document.querySelector('#replaceDomainPublication').click(); document.querySelector('#confirmDomainPublicationAction').click()`);
      try {
        await waitFor(() => evaluate(client, `(() => {
          const record = document.querySelector('#domainPublicationRecord');
          const replacement = document.querySelector('#replaceDomainPublication');
          const replacementRect = replacement.getBoundingClientRect();
          return document.querySelector('#domainPublicationAlert').textContent === 'The replacement preview did not retain the current principal, claim, and exact requested origin authority. The same idempotency key is retained for a bounded retry; serving truth was not changed in the cockpit.'
            && document.activeElement === record && !record.hidden && record.tabIndex === -1 && record.getClientRects().length > 0
            && document.querySelector('#domainPublicationConfirm').hidden
            && !replacement.hidden && !replacement.disabled && replacement.getClientRects().length > 0
            && replacementRect.width > 0 && replacementRect.height >= 44 && Boolean(replacement.closest('details')?.open);
        })()`), `replacement successor ${successorVariant} rejection, cleanup, and record focus restoration`);
      } catch (error) {
        const cockpitDiagnostic = await evaluate(client, `(() => {
          const replacement = document.querySelector('#replaceDomainPublication');
          const replacementRect = replacement.getBoundingClientRect();
          const record = document.querySelector('#domainPublicationRecord');
          const confirmation = document.querySelector('#domainPublicationConfirm');
          const confirm = document.querySelector('#confirmDomainPublicationAction');
          return {
            alert: document.querySelector('#domainPublicationAlert').textContent,
            activeElement: document.activeElement?.id || document.activeElement?.tagName || '',
            replacementHidden: replacement.hidden || Boolean(replacement.closest('[hidden]')),
            replacementDisabled: replacement.disabled,
            replacementRects: replacement.getClientRects().length,
            replacementWidth: replacementRect.width,
            replacementHeight: replacementRect.height,
            replacementDisclosureOpen: Boolean(replacement.closest('details')?.open),
            recordHidden: record.hidden,
            recordTabIndex: record.tabIndex,
            recordRects: record.getClientRects().length,
            confirmationHidden: confirmation.hidden,
            confirmDisabled: confirm.disabled,
            confirmConnected: confirm.isConnected,
            confirmRects: confirm.getClientRects().length,
            confirmationCopy: document.querySelector('#domainPublicationConfirmCopy').textContent,
          };
        })()`);
        const relevantRequests = requests.filter((request) => request.method === 'POST' && (
          request.path === '/api/artifacts/preview'
          || request.path === '/api/conductor/tracks/track_replacement/publication'
          || request.path === '/api/conductor/tracks/track_replacement/activate'
        ));
        const lastRelevantRequest = relevantRequests.at(-1);
        const requestDiagnostic = {
          variant: successorVariant,
          publicationMutationsBefore,
          publicationMutationsAfter: requests.filter((request) => request.method === 'POST' && request.path === '/api/conductor/tracks/track_replacement/publication').length,
          activationMutationsBefore,
          activationMutationsAfter: requests.filter((request) => request.method === 'POST' && request.path === '/api/conductor/tracks/track_replacement/activate').length,
          lastRelevantRequest: lastRelevantRequest ? {
            method: lastRelevantRequest.method,
            path: lastRelevantRequest.path,
            search: lastRelevantRequest.search,
            body: lastRelevantRequest.body,
          } : null,
        };
        error.message = `${error.message}; cockpit diagnostic: ${JSON.stringify(cockpitDiagnostic)}; request diagnostic: ${JSON.stringify(requestDiagnostic)}`;
        throw error;
      }
      assert.equal(requests.filter((request) => request.method === 'POST' && request.path === '/api/conductor/tracks/track_replacement/publication').length, publicationMutationsBefore, `${successorVariant} sends no publication mutation`);
      assert.equal(requests.filter((request) => request.method === 'POST' && request.path === '/api/conductor/tracks/track_replacement/activate').length, activationMutationsBefore, `${successorVariant} sends no activation mutation`);
    }
    replacementSuccessorVariant = '';

    replacementPreviewFailAfterCommit = true;
    await evaluate(client, `document.querySelector('#replaceDomainPublication').click(); document.querySelector('#confirmDomainPublicationAction').click()`);
    await waitFor(() => evaluate(client, `(() => {
      const record = document.querySelector('#domainPublicationRecord');
      const replacement = document.querySelector('#replaceDomainPublication');
      const replacementRect = replacement.getBoundingClientRect();
      return document.querySelector('#domainPublicationAlert').textContent === 'Synthetic committed replacement response interruption. The same idempotency key is retained for a bounded retry; serving truth was not changed in the cockpit.'
        && document.activeElement === record && !record.hidden && record.tabIndex === -1 && record.getClientRects().length > 0
        && document.querySelector('#domainPublicationConfirm').hidden
        && !replacement.hidden && !replacement.disabled && replacement.getClientRects().length > 0
        && replacementRect.width > 0 && replacementRect.height >= 44 && Boolean(replacement.closest('details')?.open);
    })()`), 'uncertain replacement response retains bounded retry key, completes cleanup, and restores record focus');
    const firstReplacementAttempt = requests.filter((request) => request.method === 'POST' && request.path === '/api/artifacts/preview' && request.body?.ttlHours === 2160).at(-1);
    assert.ok(firstReplacementAttempt?.body?.idempotencyKey, 'replacement attempt carries an idempotency key');
    const replacementAttemptsBeforeRevokeOverlap = requests.filter((request) => request.method === 'POST' && request.path === '/api/artifacts/preview' && request.body?.ttlHours === 2160).length;
    await evaluate(client, `document.querySelector('#revokeDomainButton').click()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#domainRevokeConfirm').hidden && document.querySelector('#replaceDomainPublication').hidden && document.querySelector('#replaceDomainPublication').disabled`), 'open domain revoke excludes replacement controls');
    await evaluate(client, `(() => { document.querySelector('#replaceDomainPublication').click(); document.querySelector('#confirmDomainPublicationAction').click(); })()`);
    assert.equal(requests.filter((request) => request.method === 'POST' && request.path === '/api/artifacts/preview' && request.body?.ttlHours === 2160).length, replacementAttemptsBeforeRevokeOverlap, 'open domain revoke prevents replacement preview requests');
    await evaluate(client, `document.querySelector('#cancelDomainRevoke').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainRevokeConfirm').hidden && document.activeElement?.id === 'revokeDomainButton' && !document.querySelector('#replaceDomainPublication').hidden && !document.querySelector('#replaceDomainPublication').disabled`), 'domain revoke cancellation restores focus and exact replacement eligibility');
    delayReplacementSuccessResponse = true;
    const domainDeletesBeforeReplacement = requests.filter((request) => request.method === 'DELETE' && request.path === '/api/workspaces/41/domains/claim_browser').length;
    await evaluate(client, `document.querySelector('#replaceDomainPublication').click(); document.querySelector('#confirmDomainPublicationAction').click()`);
    await waitFor(() => requests.filter((request) => request.method === 'POST' && request.path === '/api/artifacts/preview' && request.body?.ttlHours === 2160).length > replacementAttemptsBeforeRevokeOverlap, 'replacement preview success request in flight');
    assert.deepEqual(await evaluate(client, `({ publishing: document.querySelector('#confirmDomainPublicationAction').getAttribute('aria-busy'), reload: document.querySelector('#retryDomainClaims').disabled, select: document.querySelector('#domainClaimSelect').disabled, revoke: document.querySelector('#revokeDomainButton').disabled })`), { publishing: 'true', reload: true, select: true, revoke: true }, 'replacement keeps exact domain controls disabled while the success response is delayed');
    await evaluate(client, `(() => { document.querySelector('#revokeDomainButton').click(); document.querySelector('#confirmDomainRevoke').click(); })()`);
    assert.equal(requests.filter((request) => request.method === 'DELETE' && request.path === '/api/workspaces/41/domains/claim_browser').length, domainDeletesBeforeReplacement, 'replacement publication in flight prevents domain revoke DELETE');
    await waitFor(() => evaluate(client, `document.querySelector('#domainPublicationRecordTitle').textContent.includes('publication_replacement')
      && document.querySelector('#deployStateText').textContent === 'Serving'
      && !document.querySelector('#loadMoreDomainPublications').hidden`), 'immutable replacement activation and bounded history');
    assert.deepEqual(await evaluate(client, `({ busy: document.querySelector('#confirmDomainPublicationAction').getAttribute('aria-busy'), claim: document.querySelector('#domainClaimSelect').value, reloadDisabled: document.querySelector('#retryDomainClaims').disabled, revokeDisabled: document.querySelector('#revokeDomainButton').disabled, exactDigest: document.querySelector('#domainPublicationRecordDigest').textContent.includes('Exact stored manifest SHA-256') })`), { busy: 'false', claim: 'claim_browser', reloadDisabled: false, revokeDisabled: false, exactDigest: true }, 'delayed replacement success retains authoritative exact context without stranded busy state');
    assert.deepEqual(await evaluate(client, `({ hidden: document.querySelector('#replaceDomainPublication').hidden, disabled: document.querySelector('#replaceDomainPublication').disabled })`), { hidden: false, disabled: false }, 'replacement is genuinely eligible before claims reload begins');
    const replacementRequestsBeforeDomainReload = requests.filter((request) => request.method === 'POST' && request.path === '/api/artifacts/preview' && request.body?.ttlHours === 2160).length;
    const rollbackRequestsBeforeDomainReload = requests.filter((request) => request.method === 'POST' && request.path.includes('/domains/claim_browser/publications/') && request.path.endsWith('/activate')).length;
    const delayedDomainReloadStart = requests.length;
    delayPublicationExclusionDomainResponse = true;
    await evaluate(client, `document.querySelector('#retryDomainClaims').click()`);
    await waitFor(() => requests.slice(delayedDomainReloadStart).some((request) => request.method === 'GET' && request.path === '/api/workspaces/41/domains'), 'distinct delayed domain-claims reload in flight');
    assert.deepEqual(await evaluate(client, `({ reload: document.querySelector('#retryDomainClaims').disabled, select: document.querySelector('#domainClaimSelect').disabled, origin: document.querySelector('#domainOrigin').disabled, claim: document.querySelector('#claimDomainButton').disabled, verify: document.querySelector('#verifyDomainButton').disabled, revoke: document.querySelector('#revokeDomainButton').disabled, publish: document.querySelector('#deployButton').disabled, inspect: [...document.querySelectorAll('[data-publication-id]')].length > 0 && [...document.querySelectorAll('[data-publication-id]')].every((control) => control.disabled), replaceHidden: document.querySelector('#replaceDomainPublication').hidden, replaceDisabled: document.querySelector('#replaceDomainPublication').disabled, rollbackHidden: document.querySelector('#rollbackDomainPublication').hidden, rollbackDisabled: document.querySelector('#rollbackDomainPublication').disabled })`), { reload: true, select: true, origin: true, claim: true, verify: true, revoke: true, publish: true, inspect: true, replaceHidden: true, replaceDisabled: true, rollbackHidden: true, rollbackDisabled: true }, 'domain-claims loading synchronously disables every exact-domain and publication control');
    await evaluate(client, `(() => { document.querySelector('#replaceDomainPublication').click(); document.querySelector('#rollbackDomainPublication').click(); document.querySelector('#confirmDomainPublicationAction').click(); })()`);
    assert.equal(requests.filter((request) => request.method === 'POST' && request.path === '/api/artifacts/preview' && request.body?.ttlHours === 2160).length, replacementRequestsBeforeDomainReload, 'domain-claims loading prevents replacement requests');
    assert.equal(requests.filter((request) => request.method === 'POST' && request.path.includes('/domains/claim_browser/publications/') && request.path.endsWith('/activate')).length, rollbackRequestsBeforeDomainReload, 'domain-claims loading prevents rollback requests');
    await waitFor(() => evaluate(client, `document.querySelector('#domainClaimSelect').value === 'claim_browser' && !document.querySelector('#retryDomainClaims').disabled && document.querySelector('#domainPublicationList').textContent.includes('publication_replacement')`), 'delayed domain-claims readback restores the exact claim and publication list');
    await evaluate(client, `document.querySelector('[data-publication-id="publication_replacement"]').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainPublicationRecordTitle').textContent.includes('publication_replacement') && !document.querySelector('#replaceDomainPublication').hidden && !document.querySelector('#replaceDomainPublication').disabled`), 'exact publication readback restores replacement eligibility after domain reload');
    assert.deepEqual(await evaluate(client, `({ confirmBusy: document.querySelector('#confirmDomainPublicationAction').getAttribute('aria-busy'), claimBusy: document.querySelector('#claimDomainButton').getAttribute('aria-busy'), verifyBusy: document.querySelector('#verifyDomainButton').getAttribute('aria-busy') })`), { confirmBusy: 'false', claimBusy: 'false', verifyBusy: 'false' }, 'domain reload completion leaves no publication or claim busy state stranded');
    const replacementAttempts = requests.filter((request) => request.method === 'POST' && request.path === '/api/artifacts/preview' && request.body?.ttlHours === 2160);
    assert.equal(replacementAttempts.length, replacementPreviewCountBeforeCancel + successorNegativeVariants.length + 2, 'invalid successors are rejected before mutation and one uncertain valid response is retried once');
    assert.equal(replacementAttempts.at(-1).body.idempotencyKey, firstReplacementAttempt.body.idempotencyKey, 'uncertain replacement retry retains the exact idempotency key');
    assert.match(firstReplacementAttempt.body.idempotencyKey, /^replacement-/u);
    assert.equal(firstReplacementAttempt.body.ttlHours, 2160, 'manual replacement requests the immutable 90-day authorization cap');
    assert.deepEqual(firstReplacementAttempt.body.modules, activeTrackResponse.track.request.modules, 'replacement copies the exact curated module request');
    for (const forbidden of ['trackId', 'artifactId', 'contentHash', 'publicationId', 'createdBy', 'signerKeyId', 'lifecycle']) {
      assert.equal(Object.hasOwn(firstReplacementAttempt.body, forbidden), false, `replacement request must not copy ${forbidden}`);
    }
    assert.ok(requests.some((request) => request.method === 'POST' && request.path === '/api/conductor/tracks/track_replacement/publication' && Number.isInteger(request.body?.expectedVersion)), 'replacement publication uses the exact optimistic track version');
    assert.ok(requests.some((request) => request.method === 'POST' && request.path === '/api/conductor/tracks/track_replacement/activate' && Number.isInteger(request.body?.expectedVersion)), 'replacement activation uses the exact optimistic track version');
    assert.ok(requests.some((request) => request.method === 'GET' && request.path.endsWith('/domains/claim_browser/publications') && request.search === '?publicationId=publication_replacement'), 'replacement success requires exact bounded publication readback');
    assert.equal(domainPublications[0].sourcePublicationId, 'publication_browser', 'replacement activation records immutable predecessor lineage');

    await evaluate(client, `document.querySelector('#loadMoreDomainPublications').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainPublicationList').textContent.includes('publication_browser') && document.querySelectorAll('#domainPublicationList li').length >= 2`), 'older lightweight publication page');
    assert.match(requests.find((request) => request.method === 'GET' && request.path.endsWith('/domains/claim_browser/publications') && request.search.includes('cursor=publication-page-2'))?.search || '', /^\?limit=20&cursor=publication-page-2$/u, 'opaque cursor is used only with the bounded page size');
    await evaluate(client, `document.querySelector('[data-publication-id="publication_browser"]').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainPublicationRecordTitle').textContent.includes('publication_browser') && !document.querySelector('#rollbackDomainPublication').hidden && document.querySelector('#domainPublicationRecordDigest').textContent.includes('Exact stored manifest SHA-256')`), 'exact historical proof enables rollback');
    assert.deepEqual(await evaluate(client, `({ hidden: document.querySelector('#rollbackDomainPublication').hidden, disabled: document.querySelector('#rollbackDomainPublication').disabled, exact: document.querySelector('#domainPublicationRecordDigest').textContent.includes('Exact stored manifest SHA-256'), activeCurrent: document.querySelector('#domainPublicationList').textContent.includes('Serving — activation record retained · publication_replacement') })`), { hidden: false, disabled: false, exact: true, activeCurrent: true }, 'inactive prior publication is genuinely rollback-eligible beside the active replacement before claims reload begins');
    const rollbackRequestsBeforeClaimsReload = requests.filter((request) => request.method === 'POST' && request.path.endsWith('/domains/claim_browser/publications/publication_browser/activate')).length;
    const delayedRollbackClaimsReloadStart = requests.length;
    delayPublicationExclusionDomainResponse = true;
    await evaluate(client, `document.querySelector('#retryDomainClaims').click()`);
    await waitFor(() => requests.slice(delayedRollbackClaimsReloadStart).some((request) => request.method === 'GET' && request.path === '/api/workspaces/41/domains'), 'distinct delayed claims reload begins from an eligible rollback');
    assert.deepEqual(await evaluate(client, `({ reload: document.querySelector('#retryDomainClaims').disabled, select: document.querySelector('#domainClaimSelect').disabled, origin: document.querySelector('#domainOrigin').disabled, revoke: document.querySelector('#revokeDomainButton').disabled, inspect: [...document.querySelectorAll('[data-publication-id]')].length > 0 && [...document.querySelectorAll('[data-publication-id]')].every((control) => control.disabled), rollbackHidden: document.querySelector('#rollbackDomainPublication').hidden, rollbackDisabled: document.querySelector('#rollbackDomainPublication').disabled })`), { reload: true, select: true, origin: true, revoke: true, inspect: true, rollbackHidden: true, rollbackDisabled: true }, 'claims reload natively removes a previously eligible rollback and disables the remaining exact-domain controls');
    await evaluate(client, `(() => { document.querySelector('#rollbackDomainPublication').click(); document.querySelector('#confirmDomainPublicationAction').click(); })()`);
    assert.equal(requests.filter((request) => request.method === 'POST' && request.path.endsWith('/domains/claim_browser/publications/publication_browser/activate')).length, rollbackRequestsBeforeClaimsReload, 'claims reload blocks rollback activation from a previously eligible exact context');
    await waitFor(() => evaluate(client, `document.querySelector('#domainClaimSelect').value === 'claim_browser' && !document.querySelector('#retryDomainClaims').disabled && document.querySelector('#domainPublicationList').textContent.includes('publication_replacement')`), 'rollback claims reload restores authoritative claim and current publication history');
    await evaluate(client, `document.querySelector('#loadMoreDomainPublications').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('[data-publication-id="publication_browser"]') !== null`), 'rollback source returns through authoritative bounded history after claims reload');
    await evaluate(client, `document.querySelector('[data-publication-id="publication_browser"]').click()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#rollbackDomainPublication').hidden && !document.querySelector('#rollbackDomainPublication').disabled`), 'authoritative exact readback restores rollback after claims reload');
    assert.deepEqual(await evaluate(client, `({ reload: document.querySelector('#retryDomainClaims').disabled, select: document.querySelector('#domainClaimSelect').disabled, revoke: document.querySelector('#revokeDomainButton').disabled, rollback: document.querySelector('#rollbackDomainPublication').disabled, confirmBusy: document.querySelector('#confirmDomainPublicationAction').getAttribute('aria-busy'), claimBusy: document.querySelector('#claimDomainButton').getAttribute('aria-busy'), verifyBusy: document.querySelector('#verifyDomainButton').getAttribute('aria-busy') })`), { reload: false, select: false, revoke: false, rollback: false, confirmBusy: 'false', claimBusy: 'false', verifyBusy: 'false' }, 'rollback claims reload completes with exact controls restored and no busy state stranded');
    const rollbackMutationsBeforeStaleConfirm = requests.filter((request) => request.method === 'POST' && request.path.endsWith('/domains/claim_browser/publications/publication_browser/activate')).length;
    await evaluate(client, `document.querySelector('#rollbackDomainPublication').click()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#domainPublicationConfirm').hidden && document.activeElement?.id === 'confirmDomainPublicationAction'`), 'rollback inline confirmation focus');
    const rollbackSource = domainPublications.find((publication) => publication.id === 'publication_browser');
    const rollbackSourceProof = { authorizationExpiresAt: rollbackSource.authorizationExpiresAt, manifestDigest: rollbackSource.manifestDigest };
    Object.assign(rollbackSource, { servingState: 'artifact_invalid', authorizationExpiresAt: null, manifestDigest: null });
    await evaluate(client, `document.querySelector('#confirmDomainPublicationAction').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainPublicationAlert').textContent.includes('Publication context changed after confirmation opened') && document.activeElement?.id === 'domainPublicationRecord'`), 'stale rollback target fails closed and restores exact record focus');
    assert.equal(requests.filter((request) => request.method === 'POST' && request.path.endsWith('/domains/claim_browser/publications/publication_browser/activate')).length, rollbackMutationsBeforeStaleConfirm, 'changed rollback proof blocks activation before mutation');
    Object.assign(rollbackSource, { servingState: 'inactive', ...rollbackSourceProof });
    await evaluate(client, `document.querySelector('#loadMoreDomainPublications').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('[data-publication-id="publication_browser"]') !== null`), 'rollback source history recovery');
    await evaluate(client, `document.querySelector('[data-publication-id="publication_browser"]').click()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#rollbackDomainPublication').hidden`), 'rollback source exact proof recovery');
    const rollbackMutationsBeforeRevokeOverlap = requests.filter((request) => request.method === 'POST' && request.path.endsWith('/domains/claim_browser/publications/publication_browser/activate')).length;
    await evaluate(client, `document.querySelector('#revokeDomainButton').click()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#domainRevokeConfirm').hidden && document.querySelector('#rollbackDomainPublication').hidden && document.querySelector('#rollbackDomainPublication').disabled`), 'open domain revoke excludes publication controls');
    await evaluate(client, `(() => { document.querySelector('#rollbackDomainPublication').click(); document.querySelector('#confirmDomainPublicationAction').click(); })()`);
    assert.equal(requests.filter((request) => request.method === 'POST' && request.path.endsWith('/domains/claim_browser/publications/publication_browser/activate')).length, rollbackMutationsBeforeRevokeOverlap, 'open domain revoke prevents publication mutation requests');
    await evaluate(client, `document.querySelector('#cancelDomainRevoke').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainRevokeConfirm').hidden && document.activeElement?.id === 'revokeDomainButton' && !document.querySelector('#rollbackDomainPublication').hidden && !document.querySelector('#rollbackDomainPublication').disabled`), 'domain revoke cancellation restores focus and exact rollback eligibility');
    delayRollbackActivationResponse = true;
    const domainDeletesBeforeRollback = requests.filter((request) => request.method === 'DELETE' && request.path === '/api/workspaces/41/domains/claim_browser').length;
    const domainClaimReadsBeforeRollback = requests.filter((request) => request.method === 'GET' && request.path === '/api/workspaces/41/domains').length;
    await evaluate(client, `document.querySelector('#rollbackDomainPublication').click()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#domainPublicationConfirm').hidden && document.activeElement?.id === 'confirmDomainPublicationAction'`), 'rollback reconfirmation after exact recovery');
    await evaluate(client, `document.querySelector('#confirmDomainPublicationAction').click()`);
    await waitFor(() => requests.filter((request) => request.method === 'POST' && request.path.endsWith('/domains/claim_browser/publications/publication_browser/activate')).length > rollbackMutationsBeforeRevokeOverlap, 'rollback publication request in flight');
    assert.deepEqual(await evaluate(client, `({ reload: document.querySelector('#retryDomainClaims').disabled, select: document.querySelector('#domainClaimSelect').disabled, claim: document.querySelector('#claimDomainButton').disabled, verify: document.querySelector('#verifyDomainButton').disabled, revoke: document.querySelector('#revokeDomainButton').disabled })`), { reload: true, select: true, claim: true, verify: true, revoke: true }, 'domain reload, selection, and mutation controls remain natively disabled during rollback publication');
    await evaluate(client, `(() => { document.querySelector('#retryDomainClaims').click(); document.querySelector('#verifyDomainButton').click(); document.querySelector('#revokeDomainButton').click(); document.querySelector('#confirmDomainRevoke').click(); })()`);
    assert.equal(requests.filter((request) => request.method === 'GET' && request.path === '/api/workspaces/41/domains').length, domainClaimReadsBeforeRollback, 'rollback publication in flight prevents domain claim reload requests');
    assert.equal(requests.filter((request) => request.method === 'DELETE' && request.path === '/api/workspaces/41/domains/claim_browser').length, domainDeletesBeforeRollback, 'rollback publication in flight prevents domain revoke DELETE');
    await waitFor(() => evaluate(client, `document.querySelector('#domainPublicationRecordTitle').textContent.includes('publication_rollback') && document.querySelector('#deployStateText').textContent === 'Serving'`), 'rollback creates and confirms a source-linked immutable activation');
    await waitFor(() => evaluate(client, `document.querySelector('#domainClaimSelect').value === 'claim_browser' && !document.querySelector('#retryDomainClaims').disabled && !document.querySelector('#domainClaimSelect').disabled && !document.querySelector('#revokeDomainButton').disabled && document.querySelector('#verifyDomainButton').disabled`), 'authoritative rollback completion restores exact verified-claim controls without enabling verify');
    assert.equal(domainPublications[0].sourcePublicationId, 'publication_browser');
    assert.ok(requests.some((request) => request.method === 'POST' && request.path.endsWith('/domains/claim_browser/publications/publication_browser/activate')), 'rollback uses the exact prior-publication activation route');
    assert.ok(requests.some((request) => request.method === 'GET' && request.path.endsWith('/domains/claim_browser/publications') && request.search === '?publicationId=publication_rollback'), 'rollback success requires exact bounded publication readback');

    publicationServerTime = '2027-01-01T00:00:00Z';
    Object.assign(domainPublications.find((publication) => publication.id === 'publication_replacement'), { servingState: 'expired', trackBinding: null });
    await evaluate(client, `document.querySelector('#loadMoreDomainPublications').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('[data-publication-id="publication_replacement"]') !== null`), 'replacement retained after rollback');
    await evaluate(client, `document.querySelector('[data-publication-id="publication_replacement"]').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainPublicationRecordTitle').textContent.startsWith('Authorization expired · not serving — activation record retained')
      && document.querySelector('#domainPublicationRecordBinding').textContent.includes('No unique stored build binding')`), 'expired null-binding history is honest and non-oracular');
    assert.equal(await evaluate(client, `document.querySelector('#rollbackDomainPublication').hidden`), true, 'expired historical artifact is never offered as a usable rollback');
    assert.doesNotMatch(await evaluate(client, `document.querySelector('#domainPublicationRecord').innerText`), /\b(?:Active|Verified|healthy|live)\b/iu);

    delayPublicationContextResponse = true;
    await evaluate(client, `(() => {
      document.querySelector('[data-publication-id="publication_rollback"]').click();
      const select = document.querySelector('#domainClaimSelect');
      select.value = 'claim_pending';
      select.dispatchEvent(new Event('change', { bubbles: true }));
    })()`);
    await wait(380);
    assert.deepEqual(await evaluate(client, `({ claim: document.querySelector('#domainClaimSelect').value, recordHidden: document.querySelector('#domainPublicationRecord').hidden, list: document.querySelector('#domainPublicationList').textContent, replaceHidden: document.querySelector('#replaceDomainPublication').hidden, rollbackHidden: document.querySelector('#rollbackDomainPublication').hidden })`), { claim: 'claim_pending', recordHidden: true, list: '', replaceHidden: true, rollbackHidden: true }, 'late exact publication response cannot cross the claim boundary or restore actions');
    await evaluate(client, `(() => { const select = document.querySelector('#domainClaimSelect'); select.value = 'claim_browser'; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainPublicationList').textContent.includes('publication_rollback')`), 'publication context recovers after stale claim response');
    await evaluate(client, `document.querySelector('[data-publication-id="publication_rollback"]').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainPublicationRecordTitle').textContent.includes('publication_rollback')`), 'active rollback exact context before reflow');
    for (const publicationWidth of [160, 200, 320, 400]) {
      await client.send('Emulation.setDeviceMetricsOverride', { width: publicationWidth, height: 1_000, deviceScaleFactor: 1, mobile: false });
      await waitFor(() => evaluate(client, `window.innerWidth === ${publicationWidth}`), `${publicationWidth}px publication context`);
      const layout = await evaluate(client, `(() => {
        const root = document.documentElement;
        const viewport = root.clientWidth;
        const offenders = [root, document.body, ...document.querySelectorAll('body *')].flatMap((node) => {
          const style = getComputedStyle(node);
          const rect = node.getBoundingClientRect();
          if (style.display === 'none' || style.visibility === 'hidden' || rect.width <= 0 || rect.height <= 0) return [];
          const severity = Math.max(0, -rect.left, rect.right - viewport, node.scrollWidth - viewport);
          if (severity <= 0.5) return [];
          return [{
            tag: node.tagName,
            id: node.id || '',
            classes: typeof node.className === 'string' ? node.className : (node.getAttribute('class') || ''),
            rect: { left: rect.left, right: rect.right, top: rect.top, bottom: rect.bottom, width: rect.width, height: rect.height },
            scrollWidth: node.scrollWidth,
            clientWidth: node.clientWidth,
            overflowX: style.overflowX,
            severity,
          }];
        }).sort((left, right) => right.severity - left.severity).slice(0, 16);
        return {
          overflow: root.scrollWidth > root.clientWidth,
          scrollWidth: root.scrollWidth,
          clientWidth: root.clientWidth,
          record: document.querySelector('#domainPublicationRecord').getBoundingClientRect().width,
          viewport,
          offenders,
        };
      })()`);
      assert.equal(layout.overflow, false, `${publicationWidth}px publication context must not create horizontal overflow: ${JSON.stringify(layout)}`);
      assert.ok(layout.record <= layout.viewport, `${publicationWidth}px exact publication record stays within the viewport`);
    }
    await client.send('Emulation.setDeviceMetricsOverride', { width: 1_280, height: 1_000, deviceScaleFactor: 1, mobile: false });
    await waitFor(() => evaluate(client, `window.innerWidth === 1280`), 'publication context desktop reset');

    await evaluate(client, `(() => { document.querySelector('#trackLookup').value = 'track_browser'; document.querySelector('#loadTrackButton').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#buildRecordTrackID').value === 'track_browser'`), 'original signed build selected after publication lifetime journey');
    await evaluate(client, `document.querySelector('#reopenBuildPreviewButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#previewStatus').textContent === 'Verified staging ready' && document.querySelector('#previewFrame').dataset.stale === 'false'`), 'original trusted preview restored after publication lifetime journey');

    const delayedBuildRequests = requests.filter((item) => item.path === '/api/artifacts/preview').length;
    delayPreviewResponse = true;
    await evaluate(client, `document.querySelector('#builderForm').requestSubmit()`);
    await waitFor(() => requests.filter((item) => item.path === '/api/artifacts/preview').length === delayedBuildRequests + 1, 'delayed component build start');
    await evaluate(client, `(() => {
      const heading = document.querySelector('[data-component-id="announcements"] .field input');
      heading.value = 'Changed while build was pending';
      heading.dispatchEvent(new Event('input', { bubbles: true }));
      const select = document.querySelector('#templateSelect');
      select.value = 'bazaar-cooperative';
      select.dispatchEvent(new Event('change', { bubbles: true }));
      document.querySelector('#applyTemplateSwitch').click();
    })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#previewLoading').hidden && document.querySelector('#templateSelect').value === 'bazaar-cooperative'`), 'stale delayed build completion');
    const delayedBuildBoundary = await evaluate(client, `({
      srcdoc: document.querySelector('#previewFrame').srcdoc,
      stale: document.querySelector('#previewFrame').dataset.stale,
      publishDisabled: document.querySelector('#deployButton').disabled,
      selected: document.querySelectorAll('#moduleList input:checked').length,
    })`);
    assert.equal(delayedBuildBoundary.srcdoc, trustedSourceBeforeNegatives, 'a delayed build cannot replace the trusted iframe after its draft changes');
    assert.equal(delayedBuildBoundary.stale, 'true');
    assert.equal(delayedBuildBoundary.publishDisabled, true);
    assert.equal(delayedBuildBoundary.selected, 2, 'the explicitly reconciled next-template draft remains selected');

    const delayedFileRequests = requests.filter((item) => item.path === signedRuntimePath).length;
    delaySignedRuntimeResponse = true;
    await evaluate(client, `document.querySelector('#builderForm').requestSubmit()`);
    await waitFor(() => requests.filter((item) => item.path === signedRuntimePath).length > delayedFileRequests, 'delayed signed runtime workspace request');
    await evaluate(client, `(() => { const select = document.querySelector('#workspaceSelect'); select.value = '42'; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceSelect').value === '42' && document.querySelector('#workspaceRole').textContent === 'Architect'`), 'workspace switch during preview file load');
    await wait(240);
    assert.deepEqual(await evaluate(client, `({ srcdoc: document.querySelector('#previewFrame').getAttribute('srcdoc'), manifestHidden: document.querySelector('#manifestList').hidden, template: document.querySelector('#templateSelect').value })`), { srcdoc: null, manifestHidden: true, template: '' }, 'a delayed preview-file response cannot repopulate a cleared workspace scope');

    await evaluate(client, `(() => { const select = document.querySelector('#workspaceSelect'); select.value = '41'; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceSelect').value === '41' && document.querySelector('#templateSelect').value === 'bazaar-cooperative' && document.querySelectorAll('#moduleList input:checked').length === 2 && document.querySelector('#domainClaimSelect').value === 'claim_browser'`), 'second template scoped draft and domain recovery');
    assert.deepEqual(await evaluate(client, `({ appName: document.querySelector('#appName').value, organization: document.querySelector('#organizationName').value, city: document.querySelector('#city').value })`), { appName: 'Community app', organization: 'QA Community', city: '' }, 'second template return does not restore broader app identity or city');
    await evaluate(client, `(() => { for (const [id, value] of [['appName', 'Bazaar cooperative app'], ['organizationName', 'QA Community'], ['city', 'Salt Lake City']]) { const input = document.querySelector('#' + id); input.value = value; input.dispatchEvent(new Event('input', { bubbles: true })); } })()`);
    const secondTemplateRequestIndex = requests.filter((item) => item.method === 'POST' && item.path === '/api/artifacts/preview').length;
    await evaluate(client, `document.querySelector('#builderForm').requestSubmit()`);
    await waitFor(() => requests.filter((item) => item.method === 'POST' && item.path === '/api/artifacts/preview').length === secondTemplateRequestIndex + 1, 'second real template preview request');
    await waitFor(() => evaluate(client, `document.querySelector('#previewLoading').hidden && document.querySelector('#previewStatus').textContent === 'Verified staging ready' && document.querySelector('#previewFrame').dataset.stale === 'false' && document.querySelector('#builderAlert').textContent.includes('Exact workspace-bound signed preview verified')`), 'second real template signed build');
    const secondTemplateRequest = requests.filter((item) => item.method === 'POST' && item.path === '/api/artifacts/preview')[secondTemplateRequestIndex];
    assert.equal(secondTemplateRequest.body.templateId, 'bazaar-cooperative');
    assert.deepEqual(secondTemplateRequest.body.modules, ['announcements', 'donation-campaign']);
    assert.equal(await evaluate(client, `[...document.querySelectorAll('#manifestList .manifest-row')].find((row) => row.querySelector('dt').textContent === 'Template').querySelector('dd').textContent`), 'bazaar-cooperative · v1.0.0', 'the receipt must bind the second real template');

    const runtimeRequestsBeforeScopeSwitch = requests.filter((item) => item.path === signedRuntimePath).length;
    delaySignedRuntimeResponse = true;
    delaySecondPrincipalHistory = true;
    const secondPrincipalRequestStart = requests.length;
    await evaluate(client, `(() => { document.querySelector('#trackLookup').value = 'track_browser'; document.querySelector('#loadTrackButton').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#buildRecordTrackID').value === 'track_browser'`), 'principal-switch build inspection');
    await evaluate(client, `document.querySelector('#reopenBuildPreviewButton').click()`);
    await waitFor(() => requests.filter((item) => item.path === signedRuntimePath).length > runtimeRequestsBeforeScopeSwitch, 'delayed signed runtime reload request');
    await evaluate(client, `document.querySelector('#logoutButton').click()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#authView').hidden`), 'principal switch sign-out');
    await evaluate(client, `(() => {
      const set = (id, value) => { const input = document.getElementById(id); input.value = value; input.dispatchEvent(new Event('input', { bubbles: true })); };
      set('loginEmail', 'second@example.test');
      set('loginPassword', 'correct horse battery staple');
      document.getElementById('loginForm').requestSubmit();
    })()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#appView').hidden && document.querySelector('#profileName').textContent === 'Second Architect' && document.querySelector('#workspaceSelect').value === '41'`), 'second principal workspace');
    await wait(240);
    assert.deepEqual(await evaluate(client, `({ srcdoc: document.querySelector('#previewFrame').getAttribute('srcdoc'), manifestHidden: document.querySelector('#manifestList').hidden, template: document.querySelector('#templateSelect').value, selected: document.querySelectorAll('#moduleList input:checked').length })`), { srcdoc: null, manifestHidden: true, template: '', selected: 0 }, 'a delayed runtime response cannot restore another principal\'s preview, receipt, or component draft');
    await waitFor(() => evaluate(client, `document.querySelector('#previewStatus').textContent === 'Verified staging ready'
      && document.querySelector('#starterRecoveryState').textContent.includes('Exact workspace-bound signed preview verified; durable history confirmed')`), 'second principal authorized signed-history recovery');
    const secondPrincipalRequests = requests.slice(secondPrincipalRequestStart);
    assert.equal(secondPrincipalRequests.filter((item) => item.method === 'GET' && item.path === '/api/conductor/tracks/track_browser' && item.search === '?includeVerifiedPreview=true' && item.authorization === `Bearer ${secondToken}`).length, 1, 'the new principal performs one independently authorized verified-track reopen');
    assert.deepEqual(await evaluate(client, `({ manifestHidden: document.querySelector('#manifestList').hidden, template: document.querySelector('#templateSelect').value, selected: document.querySelectorAll('#moduleList input:checked').length, starterSummary: document.querySelector('#starterPathStatus').textContent, announcement: document.querySelector('#workspaceAnnouncer').textContent })`), { manifestHidden: false, template: '', selected: 0, starterSummary: '', announcement: '' }, 'authorized recovery restores trusted evidence without copying another principal\'s local draft, starter milestone, or announcement');

    Object.assign(activeTrackResponse.track, { status: 'PREVIEW_READY', version: 6, claimId: '', publication: undefined, updatedAt: '2026-08-18T07:00:00Z' });
    await evaluate(client, `document.querySelector('#openNewestBuildButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#previewStatus').textContent === 'Verified staging ready' && !document.querySelector('#deployButton').disabled`), 'scope-race verified preview precondition');
    delayTrackResponse = true;
    delayScopeClaimResponse = true;
    delayInvitationResponse = true;
    delayAcceptanceResponse = true;
    const delayedCreateBoundaryStart = requests.length;
    await evaluate(client, `(() => {
      document.querySelector('#openNewestBuildButton').click();
      const origin = document.querySelector('#domainOrigin');
      origin.value = 'https://scope-race.community.example';
      origin.dispatchEvent(new Event('input', { bubbles: true }));
      document.querySelector('#claimDomainButton').click();
      document.querySelector('#invitee').value = 'scope-race@example.test';
      document.querySelector('#createInviteButton').click();
      document.querySelector('#acceptInviteToken').value = 'pending-scope-acceptance-token';
      document.querySelector('#acceptInviteButton').click();
    })()`);
    await waitFor(() => requests.slice(delayedCreateBoundaryStart).some((item) => item.path === '/api/conductor/tracks/track_browser')
      && requests.slice(delayedCreateBoundaryStart).some((item) => item.method === 'POST' && item.path === '/api/workspaces/41/domains')
      && requests.slice(delayedCreateBoundaryStart).some((item) => item.method === 'POST' && item.path === '/api/shura/v1/invitations')
      && requests.slice(delayedCreateBoundaryStart).some((item) => item.method === 'POST' && item.path === '/api/shura/v1/invitations/accept'), 'delayed track, domain, invitation, and acceptance operations before workspace creation');
    assert.equal(requests.slice(delayedCreateBoundaryStart).some((item) => item.method === 'POST' && item.path === '/api/conductor/tracks/track_browser/publication'), false, 'workspace-create boundary does not overlap mutually exclusive publication and domain mutation lanes');
    delayPeopleResponse = true;
    delayHistoryResponse = true;
    delayDomainResponse = true;
    await evaluate(client, `(() => {
      document.querySelector('#retryPeople').click();
      document.querySelector('#retryBuildHistory').click();
      document.querySelector('#retryDomainClaims').click();
    })()`);
    await waitFor(() => requests.slice(delayedCreateBoundaryStart).some((item) => item.method === 'GET' && item.path === '/api/workspaces/41/people')
      && requests.slice(delayedCreateBoundaryStart).some((item) => item.method === 'GET' && item.path === '/api/conductor/tracks' && item.search.includes('workspaceId=41'))
      && requests.slice(delayedCreateBoundaryStart).some((item) => item.method === 'GET' && item.path === '/api/workspaces/41/domains'), 'all three delayed workspace A evidence loaders were launched before creation');
    await evaluate(client, `(() => {
      document.querySelector('#inviteTokenOutput').value = 'old-workspace-session-token';
      document.querySelector('#inviteGrant').hidden = false;
      document.querySelector('#acceptInviteToken').value = 'old-workspace-acceptance-token';
      document.querySelector('#acceptInviteResult').textContent = 'Old workspace acceptance pending retry.';
      document.querySelector('#acceptInviteResult').hidden = false;
      document.querySelector('#inviteResult').textContent = 'Old workspace invitation pending retry.';
      document.querySelector('#inviteResult').hidden = false;
      document.querySelector('#builderAlert').textContent = 'Old workspace build retry pending.';
      document.querySelector('#builderAlert').hidden = false;
      document.querySelector('#workspaceAnnouncer').textContent = 'Old workspace announcement.';
      document.querySelector('#workspaceName').value = 'Fresh Scope B';
      document.querySelector('#workspaceDescription').value = 'Created through the cockpit.';
      document.querySelector('#createWorkspaceButton').click();
    })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceSelect').value === '43'
      && document.querySelector('#workspaceRole').textContent === 'Architect'
      && document.querySelector('#buildHistoryState').textContent.includes('No workspace build records yet')`), 'create-workspace selects its exact new authorized scope');
    await wait(380);
    assert.deepEqual(await evaluate(client, `({
      workspace: document.querySelector('#workspaceSelect').value,
      template: document.querySelector('#templateSelect').value,
      selected: document.querySelectorAll('#moduleList input:checked').length,
      componentEditors: document.querySelectorAll('#moduleList .component-editor').length,
      starter: document.querySelector('#starterPathStatus').textContent,
      appName: document.querySelector('#appName').value,
      organization: document.querySelector('#organizationName').value,
      city: document.querySelector('#city').value,
      madhhab: document.querySelector('#madhhab').value,
      accent: document.querySelector('#accentColor').value.toUpperCase(),
      accentHex: document.querySelector('#accentHex').value,
      brief: document.querySelector('#buildBrief').value,
      srcdoc: document.querySelector('#previewFrame').getAttribute('srcdoc'),
      manifestHidden: document.querySelector('#manifestList').hidden,
      trackLookup: document.querySelector('#trackLookup').value,
      recordHidden: document.querySelector('#buildRecord').hidden,
      historyRecords: document.querySelectorAll('#buildHistoryList li').length,
      domainSelection: document.querySelector('#domainClaimSelect').value,
      domainOrigin: document.querySelector('#domainOrigin').value,
      inviteToken: document.querySelector('#inviteTokenOutput').value,
      inviteHidden: document.querySelector('#inviteGrant').hidden,
      acceptToken: document.querySelector('#acceptInviteToken').value,
      acceptResult: document.querySelector('#acceptInviteResult').textContent,
      inviteRecords: document.querySelectorAll('#invitationList li').length,
      builderAlert: document.querySelector('#builderAlert').textContent,
      announcement: document.querySelector('#workspaceAnnouncer').textContent,
      previewLabel: document.querySelector('#previewButton').textContent,
      previewStatus: document.querySelector('#previewStatus').textContent,
      claimLabel: document.querySelector('#claimDomainButton').textContent,
      claimBusy: document.querySelector('#claimDomainButton').getAttribute('aria-busy'),
      verifyLabel: document.querySelector('#verifyDomainButton').textContent,
      verifyBusy: document.querySelector('#verifyDomainButton').getAttribute('aria-busy'),
      inviteLabel: document.querySelector('#createInviteButton').textContent,
      inviteBusy: document.querySelector('#createInviteButton').getAttribute('aria-busy'),
      acceptLabel: document.querySelector('#acceptInviteButton').textContent,
      acceptBusy: document.querySelector('#acceptInviteButton').getAttribute('aria-busy'),
      deployLabel: document.querySelector('#deployButton').textContent,
      deployBusy: document.querySelector('#deployButton').getAttribute('aria-busy'),
      deployDisabled: document.querySelector('#deployButton').disabled,
    })`), {
      workspace: '43', template: '', selected: 0, componentEditors: 0,
      starter: '0 of 5 signed starter checks complete. Choose a template.',
      appName: 'Community app', organization: 'Fresh Scope B', city: '', madhhab: 'hanafi', accent: '#57A68E', accentHex: '#57A68E', brief: '',
      srcdoc: null, manifestHidden: true, trackLookup: '', recordHidden: true, historyRecords: 0, domainSelection: '', domainOrigin: '',
      inviteToken: '', inviteHidden: true, acceptToken: '', acceptResult: '', inviteRecords: 0, builderAlert: '', announcement: '',
      previewLabel: 'Create staging preview', previewStatus: 'Waiting for build',
      claimLabel: 'Issue DNS proof', claimBusy: 'false', verifyLabel: 'Verify DNS proof', verifyBusy: 'false', inviteLabel: 'Create invitation', inviteBusy: 'false',
      acceptLabel: 'Accept and join', acceptBusy: 'false', deployLabel: 'Publish verified domain', deployBusy: 'false', deployDisabled: true,
    }, 'created workspace starts with exact defaults and no prior draft, starter, trusted evidence, track, domain, invitation, retry, or live-region state');
    assert.equal(await evaluate(client, `[...Array(sessionStorage.length)].map((_, index) => sessionStorage.key(index)).some((key) => key.includes(':8:43'))`), false, 'the created workspace has no fabricated local component-draft key');
    assert.ok(requests.slice(delayedCreateBoundaryStart).some((item) => item.method === 'GET' && item.path === '/api/workspaces/41/people'), 'the prior workspace People request was in flight');
    assert.ok(requests.slice(delayedCreateBoundaryStart).some((item) => item.method === 'GET' && item.path === '/api/conductor/tracks' && item.search.includes('workspaceId=41')), 'the prior workspace history request was in flight');
    assert.ok(requests.slice(delayedCreateBoundaryStart).some((item) => item.method === 'GET' && item.path === '/api/workspaces/41/domains'), 'the prior workspace domain request was in flight');
    await evaluate(client, `(() => {
      const track = document.querySelector('#openNewestBuildButton'); track.textContent = 'Workspace B track operation'; track.setAttribute('aria-busy', 'true'); track.disabled = true;
      const claim = document.querySelector('#claimDomainButton'); claim.textContent = 'Workspace B claim operation'; claim.setAttribute('aria-busy', 'true'); claim.disabled = true;
      const invite = document.querySelector('#createInviteButton'); invite.textContent = 'Workspace B invite operation'; invite.setAttribute('aria-busy', 'true'); invite.disabled = true;
      const accept = document.querySelector('#acceptInviteButton'); accept.textContent = 'Workspace B acceptance operation'; accept.setAttribute('aria-busy', 'true'); accept.disabled = true;
    })()`);
    await wait(500);
    assert.deepEqual(await evaluate(client, `({ workspace: document.querySelector('#workspaceSelect').value, track: document.querySelector('#openNewestBuildButton').textContent, trackBusy: document.querySelector('#openNewestBuildButton').getAttribute('aria-busy'), claim: document.querySelector('#claimDomainButton').textContent, claimBusy: document.querySelector('#claimDomainButton').getAttribute('aria-busy'), invite: document.querySelector('#createInviteButton').textContent, inviteBusy: document.querySelector('#createInviteButton').getAttribute('aria-busy'), accept: document.querySelector('#acceptInviteButton').textContent, acceptBusy: document.querySelector('#acceptInviteButton').getAttribute('aria-busy'), acceptToken: document.querySelector('#acceptInviteToken').value, acceptResult: document.querySelector('#acceptInviteResult').textContent, inviteRecords: document.querySelectorAll('#invitationList li').length })`), { workspace: '43', track: 'Workspace B track operation', trackBusy: 'true', claim: 'Workspace B claim operation', claimBusy: 'true', invite: 'Workspace B invite operation', inviteBusy: 'true', accept: 'Workspace B acceptance operation', acceptBusy: 'true', acceptToken: '', acceptResult: '', inviteRecords: 0 }, 'late workspace A track, claim, invitation, and acceptance finalizers cannot select another workspace, append invitations, or relabel/reset workspace B controls');

    await evaluate(client, `(() => { const select = document.querySelector('#workspaceSelect'); select.value = '41'; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceSelect').value === '41'
      && document.querySelector('#previewStatus').textContent === 'Verified staging ready'
      && document.querySelector('#starterRecoveryState').textContent.includes('durable history confirmed')
      && document.querySelector('#domainClaimSelect').value === 'claim_browser'
      && !document.querySelector('#deployButton').disabled`), 'return to signed workspace restores one genuinely eligible exact-claim publication context');

    const exactPublicationFixtureTrack = structuredClone(activeTrackResponse.track);
    const exactPublicationFixtureEvents = structuredClone(trackEvents.get('track_browser') || []);
    delayScopePublicationResponse = true;
    const delayedPublicationScopeBoundaryStart = requests.length;
    await evaluate(client, `document.querySelector('#deployButton').click()`);
    await waitFor(() => requests.slice(delayedPublicationScopeBoundaryStart).some((item) => item.method === 'POST'
      && item.path === '/api/conductor/tracks/track_browser/publication' && item.body?.claimId === 'claim_browser'), 'delayed exact workspace-41 publication request in flight');
    await waitFor(() => typeof releaseScopePublicationResponse === 'function', 'workspace-41 publication response held before release');
    assert.deepEqual(await evaluate(client, `({ claim: document.querySelector('#domainClaimSelect').value, selectDisabled: document.querySelector('#domainClaimSelect').disabled, reloadDisabled: document.querySelector('#retryDomainClaims').disabled, publishBusy: document.querySelector('#deployButton').getAttribute('aria-busy') })`), { claim: 'claim_browser', selectDisabled: true, reloadDisabled: true, publishBusy: 'true' }, 'scope-race publication begins from the exact verified claim with native domain exclusion');

    await evaluate(client, `(() => { const select = document.querySelector('#workspaceSelect'); select.value = '42'; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceSelect').value === '42'
      && document.querySelector('#workspaceRole').textContent === 'Architect'
      && document.querySelector('#buildHistoryState').textContent.includes('No workspace build records yet')
      && document.querySelector('#domainClaimsState').textContent.includes('No domain claims yet')
      && !document.querySelector('#retryDomainClaims').disabled`), 'workspace 42 resolves while workspace-41 publication remains delayed');
    await evaluate(client, `(() => {
      const deploy = document.querySelector('#deployButton'); deploy.textContent = 'Workspace 42 publication operation'; deploy.setAttribute('aria-busy', 'true'); deploy.disabled = true;
      document.querySelector('#deployStateText').textContent = 'Workspace 42 publication state';
      const status = document.querySelector('#domainStatus'); status.textContent = 'Workspace 42 publication identity'; status.hidden = false;
    })()`);
    releaseScopePublicationResponse();
    await waitFor(() => releaseScopePublicationResponse === null, 'held workspace-41 publication response released after workspace-42 marker');
    await wait(240);
    const delayedPublicationScopeRequests = requests.slice(delayedPublicationScopeBoundaryStart);
    assert.equal(delayedPublicationScopeRequests.some((item) => item.method === 'POST' && item.path === '/api/conductor/tracks/track_browser/activate'), false, 'stale workspace-41 publication response cannot advance activation in workspace 42');
    assert.equal(delayedPublicationScopeRequests.some((item) => item.method === 'GET' && item.path.includes('/domains/claim_browser/publications')), false, 'stale workspace-41 publication response cannot launch authoritative readback into workspace 42');
    assert.deepEqual(await evaluate(client, `({ workspace: document.querySelector('#workspaceSelect').value, role: document.querySelector('#workspaceRole').textContent, domainSelection: document.querySelector('#domainClaimSelect').value, deploy: document.querySelector('#deployButton').textContent, deployBusy: document.querySelector('#deployButton').getAttribute('aria-busy'), deployDisabled: document.querySelector('#deployButton').disabled, deployState: document.querySelector('#deployStateText').textContent, domainStatus: document.querySelector('#domainStatus').textContent })`), { workspace: '42', role: 'Architect', domainSelection: '', deploy: 'Workspace 42 publication operation', deployBusy: 'true', deployDisabled: true, deployState: 'Workspace 42 publication state', domainStatus: 'Workspace 42 publication identity' }, 'late workspace-41 publication finalizer cannot overwrite scope or clear workspace-42 publication control identity');
    Object.assign(activeTrackResponse.track, exactPublicationFixtureTrack);
    trackEvents.set('track_browser', exactPublicationFixtureEvents);

    await evaluate(client, `(() => {
      const template = document.querySelector('#templateSelect');
      template.value = 'community-iftar';
      template.dispatchEvent(new Event('change', { bubbles: true }));
      document.querySelector('#moduleList input[value="announcements"]').click();
      const title = document.querySelector('[data-component-id="announcements"] .component-fields .field input');
      title.value = 'Workspace 42 exact local draft';
      title.dispatchEvent(new Event('input', { bubbles: true }));
      for (const [id, value] of [['appName', 'Workspace 42 app'], ['organizationName', 'Workspace 42 organization'], ['city', 'Ogden']]) {
        const input = document.getElementById(id); input.value = value; input.dispatchEvent(new Event('input', { bubbles: true }));
      }
    })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#starterPathStatus').textContent.includes('3 of 5')
      && document.querySelector('[data-component-id="announcements"] .component-fields .field input').value === 'Workspace 42 exact local draft'`), 'validated workspace-scoped local draft before cockpit creation');
    delayPeopleResponse = true;
    delayHistoryResponse = true;
    delayDomainResponse = true;
    await evaluate(client, `(() => {
      document.querySelector('#retryPeople').click();
      document.querySelector('#inviteTokenOutput').value = 'workspace-42-session-token';
      document.querySelector('#inviteGrant').hidden = false;
      document.querySelector('#workspaceAnnouncer').textContent = 'Workspace 42 stale announcement.';
      document.querySelector('#workspaceName').value = 'Fresh Scope C';
      document.querySelector('#createWorkspaceButton').click();
    })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceSelect').value === '44'
      && document.querySelector('#workspaceRole').textContent === 'Architect'
      && document.querySelector('#starterPathStatus').textContent.startsWith('0 of 5')`), 'second create-workspace boundary starts blank');
    await wait(380);
    assert.deepEqual(await evaluate(client, `({ template: document.querySelector('#templateSelect').value, selected: document.querySelectorAll('#moduleList input:checked').length, appName: document.querySelector('#appName').value, organization: document.querySelector('#organizationName').value, city: document.querySelector('#city').value, domainOrigin: document.querySelector('#domainOrigin').value, inviteToken: document.querySelector('#inviteTokenOutput').value, announcement: document.querySelector('#workspaceAnnouncer').textContent })`), { template: '', selected: 0, appName: 'Community app', organization: 'Fresh Scope C', city: '', domainOrigin: '', inviteToken: '', announcement: '' }, 'created workspace cannot inherit broader builder, domain, invitation, or announcement state');

    await evaluate(client, `(() => { const select = document.querySelector('#workspaceSelect'); select.value = '42'; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceSelect').value === '42'
      && document.querySelector('#workspaceRole').textContent === 'Architect'
      && document.querySelector('#templateSelect').value === 'community-iftar'
      && document.querySelector('[data-component-id="announcements"] .component-fields .field input').value === 'Workspace 42 exact local draft'`), 'only the exact workspace-scoped validated local component draft restores');
    assert.deepEqual(await evaluate(client, `({
      selected: [...document.querySelectorAll('#moduleList input:checked')].map((input) => input.value),
      starter: document.querySelector('#starterPathStatus').textContent,
      completed: [...document.querySelectorAll('#signedStarterPath [data-complete]')].map((step) => step.dataset.complete),
      appName: document.querySelector('#appName').value,
      organization: document.querySelector('#organizationName').value,
      city: document.querySelector('#city').value,
      srcdoc: document.querySelector('#previewFrame').getAttribute('srcdoc'),
      manifestHidden: document.querySelector('#manifestList').hidden,
      inviteHidden: document.querySelector('#inviteGrant').hidden,
    })`), {
      selected: ['announcements'], starter: '0 of 5 signed starter checks complete. Confirm restored template.',
      completed: ['false', 'false', 'false', 'false', 'false'], appName: 'Community app', organization: 'QA Other Workspace', city: '',
      srcdoc: null, manifestHidden: true, inviteHidden: true,
    }, 'restored local draft remains editable but cannot restore starter milestones, identity, receipt, or invitation state without authorized server history');
    const restoredDocumentBeforeConfirmation = await evaluate(client, `document.querySelector('[data-component-id="announcements"] .advanced-document textarea').value`);
    for (const [expectedCount, expectedAction] of [[1, 'Confirm restored components'], [2, 'Confirm restored customization'], [3, 'Complete app details']]) {
      await evaluate(client, `document.querySelector('#starterPrimaryButton').click()`);
      await waitFor(() => evaluate(client, `document.querySelector('#starterPathStatus').textContent.startsWith('${expectedCount} of 5') && document.querySelector('#starterPrimaryButton').textContent === ${JSON.stringify(expectedAction)}`), `explicit restored-draft confirmation ${expectedCount}`);
      assert.equal(await evaluate(client, `document.querySelector('[data-component-id="announcements"] .advanced-document textarea').value`), restoredDocumentBeforeConfirmation, `restored-draft confirmation ${expectedCount} must not switch templates or rewrite exact documents`);
    }
    assert.deepEqual(await evaluate(client, `({ template: document.querySelector('#templateSelect').value, selected: [...document.querySelectorAll('#moduleList input:checked')].map((input) => input.value), title: document.querySelector('[data-component-id="announcements"] .component-fields .field input').value })`), { template: 'community-iftar', selected: ['announcements'], title: 'Workspace 42 exact local draft' }, 'three intentional confirmation controls advance only current-scope starter guidance without discarding the validated local draft');

    await evaluate(client, `(() => { const select = document.querySelector('#workspaceSelect'); select.value = '44'; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceSelect').value === '44' && document.querySelector('#workspaceRole').textContent === 'Architect'`), 'created workspace selected before deletion fallback');
    await evaluate(client, `(() => { document.querySelector('#builderAlert').textContent = 'Deleted workspace pending retry.'; document.querySelector('#builderAlert').hidden = false; document.querySelector('#workspaceAnnouncer').textContent = 'Deleted workspace announcement.'; })()`);
    workspaces = workspaces.filter((workspace) => workspace.id !== 44);
    await evaluate(client, `document.querySelector('#retryWorkspaces').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceSelect').value === '41'
      && document.querySelector('#workspaceRole').textContent === 'Architect'
      && document.querySelector('#previewStatus').textContent === 'Verified staging ready'`), 'deleted selection falls back through the same fail-closed scope boundary');
    await waitFor(() => evaluate(client, `document.querySelector('#starterPathStatus').textContent === ''`), 'authorized nonempty history recovery settles the fallback starter summary');
    assert.deepEqual(await evaluate(client, `({ builderAlert: document.querySelector('#builderAlert').textContent, announcement: document.querySelector('#workspaceAnnouncer').textContent, starterSummary: document.querySelector('#starterPathStatus').textContent, inviteToken: document.querySelector('#inviteTokenOutput').value })`), { builderAlert: '', announcement: '', starterSummary: '', inviteToken: '' }, 'fallback selection clears deleted-workspace retry, announcement, starter, and invitation state before authorized history recovery');

    delayWorkspaceCreateResponse = true;
    const samePrincipalCreateStart = requests.length;
    await evaluate(client, `(() => { document.querySelector('#workspaceName').value = 'Stale same-principal workspace'; document.querySelector('#createWorkspaceButton').click(); })()`);
    await waitFor(() => requests.slice(samePrincipalCreateStart).some((item) => item.method === 'POST' && item.path === '/api/workspaces'), 'delayed create before same-principal scope switch');
    await evaluate(client, `(() => { const select = document.querySelector('#workspaceSelect'); select.value = '43'; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceSelect').value === '43' && document.querySelector('#workspaceRole').textContent === 'Architect'`), 'newer same-principal workspace selection');
    await wait(380);
    assert.deepEqual(await evaluate(client, `({ workspace: document.querySelector('#workspaceSelect').value, createAlert: document.querySelector('#createWorkspaceAlert').textContent, createLabel: document.querySelector('#createWorkspaceButton').textContent, createBusy: document.querySelector('#createWorkspaceButton').getAttribute('aria-busy') })`), { workspace: '43', createAlert: '', createLabel: 'Create workspace', createBusy: 'false' }, 'a delayed create response cannot select its created workspace or reset controls after a newer same-principal scope selection');
    await evaluate(client, `(() => { const select = document.querySelector('#workspaceSelect'); select.value = '41'; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceSelect').value === '41' && document.querySelector('#workspaceRole').textContent === 'Architect'`), 'signed workspace restored after same-principal create race');

    delayWorkspaceCreateResponse = true;
    const delayedWorkspaceCreateStart = requests.length;
    await evaluate(client, `(() => { document.querySelector('#workspaceName').value = 'Stale principal workspace'; document.querySelector('#createWorkspaceButton').click(); })()`);
    await waitFor(() => requests.slice(delayedWorkspaceCreateStart).some((item) => item.method === 'POST' && item.path === '/api/workspaces'), 'delayed create-workspace request before principal switch');
    await evaluate(client, `document.querySelector('#logoutButton').click()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#authView').hidden && !document.querySelector('#createWorkspaceButton').disabled`), 'logout resets the pending create-workspace control');
    const postLogoutRequestStart = requests.length;
    await evaluate(client, `(() => {
      for (const [id, value] of [['loginEmail', 'qa@example.test'], ['loginPassword', 'correct horse battery staple']]) {
        const input = document.getElementById(id); input.value = value; input.dispatchEvent(new Event('input', { bubbles: true }));
      }
      document.querySelector('#loginForm').requestSubmit();
    })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#profileName').textContent === 'QA Architect'
      && document.querySelector('#workspaceSelect').value === '41'
      && document.querySelector('#workspaceRole').textContent === 'Architect'`), 'new principal establishes its own workspace scope while old creation is pending');
    await wait(380);
    assert.deepEqual(await evaluate(client, `({ workspace: document.querySelector('#workspaceSelect').value, profile: document.querySelector('#profileName').textContent, workspaceName: document.querySelector('#workspaceName').value, createAlert: document.querySelector('#createWorkspaceAlert').textContent, announcement: document.querySelector('#workspaceAnnouncer').textContent })`), { workspace: '41', profile: 'QA Architect', workspaceName: '', createAlert: '', announcement: '' }, 'a delayed create response cannot select a workspace or write UI state into a new principal session');
    assert.equal(requests.slice(postLogoutRequestStart).filter((item) => item.method === 'GET' && item.path === '/api/workspaces').length, 1, 'the stale create response cannot trigger a second preferred-workspace reload in the new session');

    await evaluate(client, `document.querySelector('#peopleTab').click()`);
    await waitFor(() => evaluate(client, `document.querySelectorAll('#peopleList li').length === 4 && document.querySelectorAll('[data-remove-member]').length === 2`), 'Architect-only removable member controls');
    assert.deepEqual(await evaluate(client, `[...document.querySelectorAll('[data-remove-member]')].map((button) => button.dataset.removeMember)`), ['9', '10'], 'owner and self rows never expose removal controls');
    assert.deepEqual(await evaluate(client, `[...document.querySelectorAll('#peopleList li')].filter((row) => row.querySelector('strong').textContent === 'Removable Viewer').map((row) => row.querySelector('span').textContent)`), [
      'User ID 8 · Architect · workspace role owner · QA Community (workspace ID 41)',
      'User ID 9 · Viewer · workspace role viewer · QA Community (workspace ID 41)',
    ], 'duplicate usernames remain distinguishable by visible stable user and workspace identity');
    assert.equal(await evaluate(client, `fetch('/api/workspaces/41/people', { headers: { Authorization: 'Bearer ${removedMemberToken}' } }).then((response) => response.status)`), 200, 'member token is authorized for People before removal');
    assert.equal(await evaluate(client, `fetch('/api/conductor/tracks?workspaceId=41', { headers: { Authorization: 'Bearer ${removedMemberToken}' } }).then((response) => response.status)`), 200, 'member token is authorized for workspace track scope before removal');
    const memberDeleteStart = requests.length;
    await evaluate(client, `document.querySelector('[data-remove-member="9"]').click()`);
    assert.equal(await evaluate(client, `document.activeElement?.id`), 'confirmMemberRemoval', 'member confirmation receives focus');
    assert.deepEqual(await evaluate(client, `(() => { const dialog = document.querySelector('#confirmMemberRemoval').closest('[role="alertdialog"]'); const title = document.getElementById(dialog.getAttribute('aria-labelledby')); const copy = document.getElementById(dialog.getAttribute('aria-describedby')); return { title: title?.textContent, copy: copy?.textContent }; })()`), {
      title: 'Remove Removable Viewer (user ID 9)?',
      copy: 'Remove this exact member from QA Community (workspace ID 41)? Their existing sign-in token will no longer authorize that workspace. This does not delete their account or retained audit history.',
    }, 'member alertdialog binds unique visible target and consequence copy');
    await evaluate(client, `document.querySelector('#cancelMemberRemoval').click()`);
    assert.equal(requests.slice(memberDeleteStart).filter((item) => item.method === 'DELETE' && item.path === '/api/workspaces/41/users/9').length, 0, 'member Cancel sends no request');
    assert.equal(await evaluate(client, `document.activeElement?.dataset.removeMember`), '9', 'member Cancel returns focus to the exact trigger');
    await evaluate(client, `new Promise((resolve) => requestAnimationFrame(() => { document.querySelector('#workspaceAnnouncer').textContent = ''; requestAnimationFrame(resolve); }))`);
    delayMemberDeleteResponse = true;
    await evaluate(client, `(() => { document.querySelector('[data-remove-member="9"]').click(); const confirm = document.querySelector('#confirmMemberRemoval'); confirm.click(); confirm.click(); })()`);
    await waitFor(() => requests.slice(memberDeleteStart).some((item) => item.method === 'DELETE' && item.path === '/api/workspaces/41/users/9'), 'member DELETE in flight');
    assert.equal(await evaluate(client, `document.querySelector('#cancelMemberRemoval').disabled`), true, 'member Cancel disables after request starts');
    await evaluate(client, `document.querySelector('#cancelMemberRemoval').click()`);
    assert.equal(await evaluate(client, `document.querySelector('#workspaceAnnouncer').textContent`), '', 'in-flight member Cancel cannot announce that no request was sent');
    await waitFor(() => evaluate(client, `document.querySelector('#peopleState').textContent.includes('authoritative People readback') && !document.querySelector('[data-remove-member="9"]') && document.querySelector('#peopleList').textContent.includes('User ID 8')`), 'member removal authoritative People readback');
    assert.equal(requests.slice(memberDeleteStart).filter((item) => item.method === 'DELETE' && item.path === '/api/workspaces/41/users/9').length, 1, 'member confirmation accepts one mutation despite double activation');
    assert.equal(await evaluate(client, `document.activeElement?.id`), 'membersTitle', 'confirmed member removal focuses the surviving People heading');
    assert.equal(await evaluate(client, `fetch('/api/workspaces/41/people', { headers: { Authorization: 'Bearer ${removedMemberToken}' } }).then((response) => response.status)`), 403, 'the removed member token immediately loses workspace scope');
    assert.equal(await evaluate(client, `fetch('/api/conductor/tracks?workspaceId=41', { headers: { Authorization: 'Bearer ${removedMemberToken}' } }).then((response) => response.status)`), 403, 'the removed member token also loses workspace-scoped track authority');

    memberDeleteAmbiguousNext = true;
    const ambiguousMemberStart = requests.length;
    await evaluate(client, `(() => { document.querySelector('[data-remove-member="10"]').click(); document.querySelector('#confirmMemberRemoval').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#peopleState').textContent.includes('outcome is unknown') && document.activeElement?.id === 'retryPeople'`), 'ambiguous member response fails closed on authoritative recovery');
    assert.equal(requests.slice(ambiguousMemberStart).filter((item) => item.method === 'DELETE' && item.path === '/api/workspaces/41/users/10').length, 1, 'ambiguous member response still records one issued mutation');
    await evaluate(client, `document.querySelector('#retryPeople').click()`);
    await waitFor(() => evaluate(client, `document.querySelectorAll('#peopleList li').length === 2 && document.querySelector('#peopleState').textContent.includes('2 real workspace members')`), 'People reload resolves the ambiguous committed removal authoritatively');

    await evaluate(client, `(() => { document.querySelector('#invitee').value = 'revoke@example.test'; document.querySelector('#inviteRole').value = 'Viewer'; document.querySelector('#createInviteButton').click(); })()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#inviteGrant').hidden && Boolean(document.querySelector('[data-revoke-invitation]'))`), 'current-session pending invitation revoke control');
    const invitationRevokeStart = requests.length;
    await evaluate(client, `document.querySelector('[data-revoke-invitation]').click()`);
    assert.equal(await evaluate(client, `document.activeElement?.id`), 'confirmInvitationRevoke', 'invitation confirmation receives focus');
    const invitationDialog = await evaluate(client, `(() => { const dialog = document.querySelector('#confirmInvitationRevoke').closest('[role="alertdialog"]'); const labelledby = dialog.getAttribute('aria-labelledby'); const describedby = dialog.getAttribute('aria-describedby'); return { labelledby, describedby, title: document.getElementById(labelledby)?.textContent, copy: document.getElementById(describedby)?.textContent }; })()`);
    assert.match(invitationDialog.labelledby, /^invitation-revoke-title-/u);
    assert.match(invitationDialog.describedby, /^invitation-revoke-copy-/u);
    assert.match(`${invitationDialog.title} ${invitationDialog.copy}`, /invitation_scope_race_.*workspace ID 41.*version 1.*secret will be cleared/isu, 'invitation alertdialog binds unique visible target and consequence copy');
    await evaluate(client, `document.querySelector('#cancelInvitationRevoke').click()`);
    assert.equal(requests.slice(invitationRevokeStart).filter((item) => /\/revoke$/u.test(item.path)).length, 0, 'invitation Cancel sends no request');
    assert.equal(await evaluate(client, `document.activeElement?.dataset.revokeInvitation?.startsWith('invitation_scope_race_')`), true, 'invitation Cancel returns focus to its exact current-session trigger');
    await evaluate(client, `new Promise((resolve) => requestAnimationFrame(() => { document.querySelector('#workspaceAnnouncer').textContent = ''; requestAnimationFrame(resolve); }))`);
    delayInvitationRevokeResponse = true;
    await evaluate(client, `(() => { document.querySelector('[data-revoke-invitation]').click(); const confirm = document.querySelector('#confirmInvitationRevoke'); confirm.click(); confirm.click(); })()`);
    await waitFor(() => requests.slice(invitationRevokeStart).some((item) => /\/revoke$/u.test(item.path)), 'invitation revoke in flight');
    assert.equal(await evaluate(client, `document.querySelector('#cancelInvitationRevoke').disabled`), true, 'invitation Cancel disables after request starts');
    await evaluate(client, `(() => { document.querySelector('#cancelInvitationRevoke').click(); document.querySelector('#invitee').value = 'blocked-create@example.test'; document.querySelector('#createInviteButton').click(); })()`);
    assert.equal(await evaluate(client, `document.querySelector('#workspaceAnnouncer').textContent`), '', 'in-flight invitation Cancel cannot announce that no request was sent');
    assert.equal(requests.slice(invitationRevokeStart).filter((item) => item.method === 'POST' && item.path === '/api/shura/v1/invitations').length, 0, 'invitation create is suppressed while revoke is in flight');
    await waitFor(() => evaluate(client, `document.querySelector('#inviteResult').textContent.includes('expected version') && document.querySelector('#inviteGrant').hidden && document.querySelector('#inviteTokenOutput').value === ''`), 'invitation REVOKED validation and secret clearing');
    assert.equal(requests.slice(invitationRevokeStart).filter((item) => /\/revoke$/u.test(item.path)).length, 1, 'invitation revoke double activation emits one versioned request');
    assert.deepEqual(requests.slice(invitationRevokeStart).find((item) => /\/revoke$/u.test(item.path)).body, { expected_version: 1 }, 'invitation revoke sends the exact current-session expected_version');

    await evaluate(client, `(() => { document.querySelector('#invitee').value = 'race@example.test'; document.querySelector('#createInviteButton').click(); })()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#inviteGrant').hidden && [...document.querySelectorAll('[data-revoke-invitation]')].length === 1`), 'second pending invitation for accept-revoke race');
    invitationAcceptRaceNext = true;
    await evaluate(client, `(() => { document.querySelector('[data-revoke-invitation]').click(); document.querySelector('#confirmInvitationRevoke').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#inviteResult').textContent.includes('cannot claim its current status') && document.querySelector('#inviteGrant').hidden && document.querySelector('#invitationList').textContent.includes('STATUS UNKNOWN')`), 'invite accept-revoke conflict remains honest without durable readback');
    assert.equal(await evaluate(client, `document.activeElement?.id`), 'inviteTitle', 'invitation conflict restores focus to the surviving invitation heading');

    await evaluate(client, `(() => { document.querySelector('#invitee').value = 'retry@example.test'; document.querySelector('#createInviteButton').click(); })()`);
    await waitFor(() => evaluate(client, `[...document.querySelectorAll('[data-revoke-invitation]')].length === 1`), 'pending invitation for non-conflict failure');
    const failedInvitationID = await evaluate(client, `document.querySelector('[data-revoke-invitation]').dataset.revokeInvitation`);
    invitationRevokeFailNext = true;
    await evaluate(client, `(() => { document.querySelector('[data-revoke-invitation]').click(); document.querySelector('#confirmInvitationRevoke').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#inviteResult').textContent.includes('not confirmed revoked')`), 'non-conflict invitation revoke failure');
    assert.equal(await evaluate(client, `document.activeElement?.dataset.revokeInvitation`), failedInvitationID, 'non-conflict invitation failure restores focus to the surviving revoke control');

    delayInvitationResponse = true;
    const delayedInviteCreateStart = requests.length;
    await evaluate(client, `(() => { document.querySelector('#invitee').value = 'delayed-create@example.test'; document.querySelector('#createInviteButton').click(); })()`);
    await waitFor(() => requests.slice(delayedInviteCreateStart).some((item) => item.method === 'POST' && item.path === '/api/shura/v1/invitations'), 'invitation create in flight');
    await evaluate(client, `document.querySelector('[data-revoke-invitation]').click()`);
    assert.equal(requests.slice(delayedInviteCreateStart).filter((item) => /\/revoke$/u.test(item.path)).length, 0, 'invitation revoke is suppressed while create is in flight');
    await waitFor(() => evaluate(client, `document.querySelector('#inviteResult').textContent.includes('Invitation created') && !document.querySelector('#createInviteButton').disabled`), 'delayed invitation create response remains authoritative');

    const rejectedMutationStatuses = await evaluate(client, `Promise.all([
      fetch('/api/shura/v1/invitations', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ workspace_id: 41, invitee: 'blocked@example.test', role: 'Viewer' }) }).then((response) => response.status),
      fetch('/api/shura/v1/invitations/${failedInvitationID}/revoke', { method: 'POST', headers: { Authorization: 'Bearer viewer-token', 'Content-Type': 'application/json' }, body: JSON.stringify({ expected_version: 1 }) }).then((response) => response.status),
      fetch('/api/workspaces/41/domains/claim_browser', { headers: { Authorization: 'Bearer maintainer-token' } }).then((response) => response.status),
      fetch('/api/workspaces/41/domains/claim_browser', { method: 'DELETE', headers: { Authorization: 'Bearer wrong-principal-token' } }).then((response) => response.status),
      fetch('/api/users/7', { method: 'PUT', headers: { Authorization: 'Bearer viewer-token', 'Content-Type': 'application/json' }, body: JSON.stringify({ password: 'blocked-password-2026' }) }).then((response) => response.status),
      fetch('/api/users/7', { method: 'DELETE' }).then((response) => response.status),
    ])`);
    assert.ok(rejectedMutationStatuses.every((status) => status === 401 || status === 403), `absent, Viewer, Maintainer, and wrong-principal mutation bearers must be rejected: ${rejectedMutationStatuses}`);

    await evaluate(client, `document.querySelector('#buildTab').click()`);
    await evaluate(client, `(() => { const select = document.querySelector('#domainClaimSelect'); select.value = 'claim_pending'; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainClaimSelect').value === 'claim_pending' && !document.querySelector('#verifyDomainButton').disabled`), 'pending claim selected for domain overlap guard');
    delayVerifyResponse = true;
    const verifyOverlapStart = requests.length;
    await evaluate(client, `(() => { document.querySelector('#verifyDomainButton').click(); document.querySelector('#revokeDomainButton').click(); })()`);
    await waitFor(() => requests.slice(verifyOverlapStart).some((item) => item.method === 'POST' && item.path.endsWith('/claim_pending/verify')), 'domain verify in flight');
    assert.equal(requests.slice(verifyOverlapStart).filter((item) => item.method === 'DELETE' && item.path.includes('claim_pending')).length, 0, 'domain revoke cannot open or issue while verify is in flight');
    await waitFor(() => evaluate(client, `document.querySelector('#domainClaimsState').textContent.includes('Verified until')`), 'delayed verify remains authoritative');

    domainRevokeFailNext = true;
    delayDomainRevokeResponse = true;
    const ambiguousDomainStart = requests.length;
    await evaluate(client, `(() => { document.querySelector('#revokeDomainButton').click(); document.querySelector('#confirmDomainRevoke').click(); })()`);
    await waitFor(() => requests.slice(ambiguousDomainStart).some((item) => item.method === 'DELETE' && item.path.includes('claim_pending')), 'domain revoke in flight before ambiguous response');
    await evaluate(client, `(() => { document.querySelector('#claimDomainButton').click(); document.querySelector('#verifyDomainButton').click(); document.querySelector('#cancelDomainRevoke').click(); })()`);
    assert.equal(requests.slice(ambiguousDomainStart).filter((item) => item.method === 'POST' && (item.path === '/api/workspaces/41/domains' || item.path.endsWith('/verify'))).length, 0, 'claim and verify cannot overlap an in-flight revoke');
    await waitFor(() => evaluate(client, `document.querySelector('#domainStatus').textContent.includes('outcome is unknown') && document.querySelector('#domainProof').hidden && document.querySelector('#domainClaimSelect').value === '' && document.querySelector('#domainClaimSelect').disabled && document.querySelector('#verifyDomainButton').disabled && document.querySelector('#deployButton').disabled && document.querySelector('#replaceDomainPublication').hidden && document.querySelector('#rollbackDomainPublication').hidden`), 'ambiguous domain DELETE fails closed across claim and publication truth');
    await evaluate(client, `document.querySelector('#retryDomainClaims').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainClaimSelect').value === 'claim_browser' && document.querySelector('#domainClaimsState').textContent.includes('Verified until')`), 'exact domain and history reload recovers from ambiguous revoke');

    await evaluate(client, `(() => { const select = document.querySelector('#domainClaimSelect'); select.value = 'claim_pending_revoke'; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainClaimSelect').value === 'claim_pending_revoke' && !document.querySelector('#revokeDomainButton').disabled && document.querySelector('#domainPublicationState').textContent.includes('No publication activation records')`), 'pending domain selected for revocation');
    assert.ok(await evaluate(client, `document.querySelector('#revokeDomainButton').getBoundingClientRect().height >= 44`), 'pending-domain Revoke domain target is at least 44 CSS px');
    const pendingDomainDeleteStart = requests.length;
    await evaluate(client, `document.querySelector('#revokeDomainButton').click()`);
    assert.match(await evaluate(client, `document.querySelector('#domainRevokeConfirmCopy').textContent`), /pending claim.*not presented as currently serving/iu, 'pending confirmation does not imply public serving');
    await evaluate(client, `document.querySelector('#cancelDomainRevoke').click()`);
    assert.equal(requests.slice(pendingDomainDeleteStart).filter((item) => item.method === 'DELETE' && item.path.includes('claim_pending_revoke')).length, 0, 'pending-domain Cancel sends no request');
    assert.equal(await evaluate(client, `document.activeElement?.id`), 'revokeDomainButton', 'domain Cancel returns focus to Revoke domain');
    await evaluate(client, `(() => { document.querySelector('#revokeDomainButton').click(); const confirm = document.querySelector('#confirmDomainRevoke'); confirm.click(); confirm.click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainStatus').textContent.includes('exact claim and publication-history readback') && document.querySelector('#domainClaimSelect').value === 'claim_pending_revoke' && document.querySelector('#domainClaimsState').textContent.includes('not presented as publicly serving')`), 'pending domain exact revoke readback without serving overclaim');
    assert.deepEqual(requests.slice(pendingDomainDeleteStart).filter((item) => item.path.includes('claim_pending_revoke')).map((item) => item.method), ['DELETE', 'GET', 'GET'], 'domain confirmation issues exactly DELETE then claim GET and history GET');

    const retainedRollbackPublication = domainPublications.find((publication) => publication.id === 'publication_rollback');
    assert.ok(retainedRollbackPublication, 'verified revoke journey starts from the retained rollback activation record');
    publicationServerTime = '2026-08-18T10:00:00Z';
    domainPublications = domainPublications.map((publication) => publication.id === retainedRollbackPublication.id
      ? { ...publication, active: true, servingState: 'serving', deactivatedAt: null, authorizationExpiresAt: '2099-08-18T10:00:00Z', manifestDigest: publication.manifestDigest || 'd'.repeat(64) }
      : (publication.active ? { ...publication, active: false, servingState: 'inactive', deactivatedAt: '2026-08-18T10:00:00Z' } : publication));
    const verifiedClaimFixture = domainClaims.find((claim) => claim.id === 'claim_browser');
    assert.ok(verifiedClaimFixture, 'verified revoke journey retains the exact claim_browser fixture');
    Object.assign(verifiedClaimFixture, { status: 'verified', verifiedAt: '2026-08-18T00:00:00Z', verificationExpiresAt: '2099-08-18T10:00:00Z', updatedAt: '2026-08-18T10:00:00Z' });
    delete verifiedClaimFixture.revocationReason;
    delete verifiedClaimFixture.revokedAt;
    assert.deepEqual(domainPublications.filter((publication) => publication.active).map((publication) => ({ id: publication.id, claimId: publication.claimId, servingState: publication.servingState })), [
      { id: 'publication_rollback', claimId: 'claim_browser', servingState: 'serving' },
    ], 'verified revoke boundary has one exact active serving claim_browser publication');

    await evaluate(client, `(() => { const select = document.querySelector('#domainClaimSelect'); select.value = 'claim_browser'; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainClaimSelect').value === 'claim_browser'
      && !document.querySelector('#retryDomainClaims').disabled
      && document.querySelector('#domainPublicationList').textContent.includes('Serving — activation record retained · publication_rollback')`), 'verified serving fixture selected before authoritative claims reload');
    const verifiedRevokeReloadStart = requests.length;
    await evaluate(client, `document.querySelector('#retryDomainClaims').click()`);
    await waitFor(() => requests.slice(verifiedRevokeReloadStart).some((item) => item.method === 'GET' && item.path === '/api/workspaces/41/domains')
      && requests.slice(verifiedRevokeReloadStart).some((item) => item.method === 'GET' && item.path.endsWith('/domains/claim_browser/publications') && item.search === '?limit=20'), 'verified revoke precondition performs authoritative claim and history readback');
    await waitFor(() => evaluate(client, `document.querySelector('#domainClaimSelect').value === 'claim_browser'
      && !document.querySelector('#retryDomainClaims').disabled
      && !document.querySelector('#revokeDomainButton').disabled
      && document.querySelector('#domainPublicationList').textContent.includes('Serving — activation record retained · publication_rollback')`), 'authoritative reload restores exact serving claim and history before revoke');
    await evaluate(client, `document.querySelector('[data-publication-id="publication_rollback"]').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainPublicationRecordTitle').textContent.includes('Serving — activation record retained · publication_rollback')
      && document.querySelector('#domainPublicationRecordDigest').textContent.includes('Exact stored manifest SHA-256')`), 'exact active serving publication is authoritatively inspected before Host probe');
    const publicHostBeforeRevoke = await requestWithLiteralHost(`${origin}/public-host`, 'app.community.example');
    assert.equal(publicHostBeforeRevoke.status, 200, 'mocked verified public Host serves before claim revocation');
    const trustedPreviewBeforeDomainRevoke = await evaluate(client, `document.querySelector('#previewFrame').getAttribute('srcdoc')`);
    const verifiedDomainDeleteStart = requests.length;
    await evaluate(client, `document.querySelector('#revokeDomainButton').click()`);
    assert.match(await evaluate(client, `document.querySelector('#domainRevokeConfirmCopy').textContent`), /verified claim.*Public serving and publication authority.*ends immediately/iu, 'verified confirmation explains public Host authority ends while history remains');
    await evaluate(client, `(() => { const confirm = document.querySelector('#confirmDomainRevoke'); confirm.click(); confirm.click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#domainStatus').textContent.includes('exact claim and publication-history readback') && document.querySelector('#domainClaimsState').textContent.includes('Public serving and publication authority ended')`), 'verified domain authoritative revoke readback');
    assert.deepEqual(requests.slice(verifiedDomainDeleteStart).filter((item) => item.path.includes('claim_browser')).map((item) => item.method), ['DELETE', 'GET', 'GET'], 'verified domain uses one DELETE followed by exact claim/history readback');
    const revokedDomainState = await evaluate(client, `({ preview: document.querySelector('#previewFrame').getAttribute('srcdoc'), previewStatus: document.querySelector('#previewStatus').textContent, proofHidden: document.querySelector('#domainProof').hidden, verifyDisabled: document.querySelector('#verifyDomainButton').disabled, publishDisabled: document.querySelector('#deployButton').disabled, replaceHidden: document.querySelector('#replaceDomainPublication').hidden, rollbackHidden: document.querySelector('#rollbackDomainPublication').hidden, historyRows: document.querySelectorAll('#domainPublicationList li').length, historyState: document.querySelector('#domainPublicationState').textContent })`);
    assert.deepEqual({ ...revokedDomainState, historyRows: undefined, historyState: undefined }, { preview: trustedPreviewBeforeDomainRevoke, previewStatus: 'Verified staging ready', proofHidden: true, verifyDisabled: true, publishDisabled: true, replaceHidden: true, rollbackHidden: true, historyRows: undefined, historyState: undefined }, 'domain revoke retains the trusted staging iframe while disabling proof, verify, publish, replacement, and rollback');
    assert.ok(revokedDomainState.historyRows >= 1, 'immutable activation history remains visible after verified claim revoke');
    assert.match(revokedDomainState.historyState, /immutable activation record.*Serving truth comes from the server/iu, 'retained history is re-read from authoritative publication context');
    const publicHostAfterRevoke = await requestWithLiteralHost(`${origin}/public-host`, 'app.community.example');
    assert.equal(publicHostAfterRevoke.status, 404, 'mocked public Host shuts off after verified claim revocation');

    await evaluate(client, `document.querySelector('#accountTab').click()`);
    const passwordChangeStart = requests.length;
    passwordUpdateAmbiguousNext = true;
    delayPasswordUpdateResponse = true;
    await evaluate(client, `(() => { document.querySelector('#newPassword').value = 'replacement-password-2026'; document.querySelector('#confirmNewPassword').value = 'replacement-password-2026'; const form = document.querySelector('#passwordForm'); form.requestSubmit(); form.requestSubmit(); })()`);
    await waitFor(() => requests.slice(passwordChangeStart).some((item) => item.method === 'PUT' && item.path === '/api/users/7'), 'password rotation in flight');
    assert.equal(await evaluate(client, `document.querySelector('#openDeleteAccount').disabled`), true, 'account deletion trigger disables during password rotation');
    await evaluate(client, `document.querySelector('#openDeleteAccount').click()`);
    assert.equal(requests.slice(passwordChangeStart).filter((item) => item.method === 'DELETE' && item.path === '/api/users/7').length, 0, 'account deletion cannot overlap password rotation');
    await waitFor(() => evaluate(client, `!document.querySelector('#authView').hidden && !document.querySelector('#authAlert').hidden && document.querySelector('#authAlert').textContent.includes('Password change outcome is unknown')`), 'ambiguous password rotation signs out without stale truth');
    assert.deepEqual(await evaluate(client, `(() => { const alert = document.querySelector('#authAlert'); return { visible: !alert.hidden && alert.getClientRects().length > 0, role: alert.getAttribute('role'), live: alert.getAttribute('aria-live'), message: alert.textContent.includes('Password change outcome is unknown'), loginSelected: document.querySelector('#loginTab').getAttribute('aria-selected'), focus: document.activeElement?.id }; })()`), {
      visible: true, role: 'alert', live: 'polite', message: true, loginSelected: 'true', focus: 'loginPassword',
    }, 'password ambiguity remains visible in the live authentication alert after selecting sign-in');
    assert.equal(requests.slice(passwordChangeStart).filter((item) => item.method === 'PUT' && item.path === '/api/users/7').length, 1, 'password rotation double activation emits one self-only PUT');
    assert.deepEqual(Object.keys(requests.slice(passwordChangeStart).find((item) => item.method === 'PUT' && item.path === '/api/users/7').body), ['password'], 'Account exposes no admin or unrelated user mutation');
    assert.equal(await evaluate(client, `fetch('/api/profile', { headers: { Authorization: 'Bearer ${token}' } }).then((response) => response.status)`), 401, 'old session loses authority after password rotation');
    await evaluate(client, `(() => { document.querySelector('#loginEmail').value = 'qa@example.test'; document.querySelector('#loginPassword').value = 'replacement-password-2026'; document.querySelector('#loginForm').requestSubmit(); })()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#appView').hidden
      && document.querySelector('#profileName').textContent === 'QA Architect'
      && document.querySelector('#workspaceLoading').hidden
      && document.querySelector('#workspaceSelect').value === '41'
      && document.querySelector('#workspaceRole').textContent === 'Architect'
      && !document.querySelector('#openDeleteAccount').disabled`), 'ambiguous committed password rotation re-authenticates only with the new password and restores authoritative Account readiness');

    await evaluate(client, `document.querySelector('#accountTab').click()`);
    const accountDeleteStart = requests.length;
    await evaluate(client, `document.querySelector('#openDeleteAccount').click()`);
    assert.equal(await evaluate(client, `document.activeElement?.id`), 'confirmDeleteAccount', 'account deletion confirmation receives focus');
    await evaluate(client, `document.querySelector('#cancelDeleteAccount').click()`);
    assert.equal(requests.slice(accountDeleteStart).filter((item) => item.method === 'DELETE' && item.path === '/api/users/7').length, 0, 'account Cancel sends no request');
    assert.equal(await evaluate(client, `document.activeElement?.id`), 'openDeleteAccount', 'account Cancel returns focus to its trigger');
    delayAccountDeleteResponse = true;
    await evaluate(client, `(() => { document.querySelector('#openDeleteAccount').click(); const confirm = document.querySelector('#confirmDeleteAccount'); confirm.click(); confirm.click(); })()`);
    await waitFor(() => requests.slice(accountDeleteStart).some((item) => item.method === 'DELETE' && item.path === '/api/users/7'), 'account delete in flight');
    await evaluate(client, `(() => { document.querySelector('#newPassword').value = 'blocked-overlap-password'; document.querySelector('#confirmNewPassword').value = 'blocked-overlap-password'; document.querySelector('#passwordForm').requestSubmit(); document.querySelector('#cancelDeleteAccount').click(); })()`);
    assert.equal(requests.slice(accountDeleteStart).filter((item) => item.method === 'PUT' && item.path === '/api/users/7').length, 0, 'password rotation cannot overlap account deletion');
    await waitFor(() => evaluate(client, `document.querySelector('#deleteAccountResult').textContent.includes('Nothing was deleted') && !document.querySelector('#appView').hidden`), 'owned-workspace self-delete 409 is non-destructive');
    assert.equal(requests.slice(accountDeleteStart).filter((item) => item.method === 'DELETE' && item.path === '/api/users/7').length, 1, 'owned-workspace conflict emits one guarded request despite double activation');
    assert.equal(accountDeleted, false, 'owned-workspace conflict retains the account');

    const user7OwnedTargets = workspaces.filter((workspace) => workspaceOwners.get(workspace.id)?.has(7)).map((workspace) => workspace.id);
    assert.ok(user7OwnedTargets.length > 1 && user7OwnedTargets[0] === 41, 'fixture exposes multiple exact owned targets for supported cleanup');
    const workspaceCleanupStart = requests.length;
    delayWorkspaceDeleteResponse = true;
    workspaceDeleteAmbiguousNext = true;
    await evaluate(client, `(() => { document.querySelector('#openDeleteOwnedWorkspace').click(); document.querySelector('#confirmDeleteOwnedWorkspace').click(); })()`);
    await waitFor(() => requests.slice(workspaceCleanupStart).some((item) => item.method === 'DELETE' && item.path === '/api/workspaces/41'), 'supported owned-workspace cleanup request');
    await evaluate(client, `(() => {
      const select = document.querySelector('#workspaceSelect');
      select.value = ${JSON.stringify(String(user7OwnedTargets[1]))};
      select.dispatchEvent(new Event('change', { bubbles: true }));
      document.querySelector('#createWorkspaceButton').click();
      document.querySelector('#retryWorkspaces').click();
    })()`);
    assert.deepEqual(await evaluate(client, `({ selected: document.querySelector('#workspaceSelect').value, selectDisabled: document.querySelector('#workspaceSelect').disabled, createDisabled: document.querySelector('#createWorkspaceButton').disabled, retryDisabled: document.querySelector('#retryWorkspaces').disabled, accountDisabled: document.querySelector('#openDeleteAccount').disabled, confirmBusy: document.querySelector('#confirmDeleteOwnedWorkspace').getAttribute('aria-busy'), cancelDisabled: document.querySelector('#cancelDeleteOwnedWorkspace').disabled })`), {
      selected: '41', selectDisabled: true, createDisabled: true, retryDisabled: true, accountDisabled: true, confirmBusy: 'true', cancelDisabled: true,
    }, 'workspace cleanup locks scope and related activators while the exact DELETE is in flight');
    await waitFor(() => evaluate(client, `document.querySelector('#deleteOwnedWorkspaceResult').textContent.includes('outcome is unknown')
      && !document.querySelector('#buildSurface').hidden
      && !document.querySelector('#workspaceError').hidden
      && document.querySelector('#workspaceError').textContent.includes('outcome is unknown')
      && !document.querySelector('#retryWorkspaces').hidden
      && !document.querySelector('#retryWorkspaces').disabled
      && document.activeElement?.id === 'retryWorkspaces'`), 'ambiguous cleanup clears busy truth and focuses visible authoritative recovery');
    assert.equal(await evaluate(client, `document.querySelector('#buildTab').getAttribute('aria-selected')`), 'true', 'ambiguous cleanup opens the Build surface containing its visible reload recovery');
    assert.equal(requests.slice(workspaceCleanupStart).filter((item) => item.method === 'DELETE' && item.path === '/api/workspaces/41').length, 1, 'attempted scope switch and retry cannot duplicate the exact cleanup request');
    assert.deepEqual(await evaluate(client, `({ accountDisabled: document.querySelector('#openDeleteAccount').disabled, cleanupDisabled: document.querySelector('#openDeleteOwnedWorkspace').disabled, selectDisabled: document.querySelector('#workspaceSelect').disabled, retryDisabled: document.querySelector('#retryWorkspaces').disabled, confirmBusy: document.querySelector('#confirmDeleteOwnedWorkspace').getAttribute('aria-busy'), cancelDisabled: document.querySelector('#cancelDeleteOwnedWorkspace').disabled })`), {
      accountDisabled: true, cleanupDisabled: true, selectDisabled: false, retryDisabled: false, confirmBusy: 'false', cancelDisabled: false,
    }, 'ambiguous cleanup releases the action guard but blocks cleanup and account deletion until authoritative reload');
    await evaluate(client, `(() => { document.querySelector('#openDeleteOwnedWorkspace').click(); document.querySelector('#confirmDeleteOwnedWorkspace').click(); })()`);
    assert.equal(requests.slice(workspaceCleanupStart).filter((item) => item.method === 'DELETE' && item.path.startsWith('/api/workspaces/')).length, 1, 'disabled cleanup cannot emit a second DELETE before authoritative reload');
    await evaluate(client, `document.querySelector('#retryWorkspaces').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceSelect').value !== '41' && document.querySelector('#workspaceLoading').hidden && document.querySelector('#workspaceRole').textContent === 'Architect' && !document.querySelector('#openDeleteOwnedWorkspace').disabled && !document.querySelector('#openDeleteAccount').disabled`), 'authoritative owner workspace reload re-enables cleanup and account actions');
    await evaluate(client, `document.querySelector('#accountTab').click()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#accountSurface').hidden && document.querySelector('#accountTab').getAttribute('aria-selected') === 'true'`), 'Account surface is restored before continuing exact owned-workspace cleanup');

    for (const workspaceID of user7OwnedTargets.slice(1)) {
      await evaluate(client, `(() => { const select = document.querySelector('#workspaceSelect'); select.value = ${JSON.stringify(String(workspaceID))}; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
      await waitFor(() => evaluate(client, `document.querySelector('#workspaceSelect').value === ${JSON.stringify(String(workspaceID))} && document.querySelector('#workspaceRole').textContent === 'Architect' && !document.querySelector('#openDeleteOwnedWorkspace').disabled`), `owned workspace ${workspaceID} selected for exact cleanup`);
      const targetDeleteStart = requests.length;
      await evaluate(client, `document.querySelector('#openDeleteOwnedWorkspace').click()`);
      assert.deepEqual(await evaluate(client, `(() => { const dialog = document.querySelector('#deleteOwnedWorkspaceConfirm'); return { visible: !dialog.hidden && dialog.getClientRects().length > 0, role: dialog.getAttribute('role'), confirmDisabled: document.querySelector('#confirmDeleteOwnedWorkspace').disabled, focus: document.activeElement?.id, exactTarget: document.querySelector('#deleteOwnedWorkspaceCopy').textContent.includes(${JSON.stringify(`workspace ID ${workspaceID}`)}) }; })()`), {
        visible: true, role: 'alertdialog', confirmDisabled: false, focus: 'confirmDeleteOwnedWorkspace', exactTarget: true,
      }, `workspace ${workspaceID} cleanup renders an exact-target confirmation and focuses its enabled action`);
      await evaluate(client, `document.querySelector('#confirmDeleteOwnedWorkspace').click()`);
      await waitFor(() => requests.slice(targetDeleteStart).some((item) => item.method === 'DELETE' && item.path === `/api/workspaces/${workspaceID}`), `workspace ${workspaceID} exact DELETE`);
      await waitFor(() => evaluate(client, `![...document.querySelector('#workspaceSelect').options].some((option) => option.value === ${JSON.stringify(String(workspaceID))}) && document.querySelector('#confirmDeleteOwnedWorkspace').getAttribute('aria-busy') === 'false'`), `workspace ${workspaceID} authoritative removal readback`);
      assert.equal(requests.slice(targetDeleteStart).filter((item) => item.method === 'DELETE' && item.path === `/api/workspaces/${workspaceID}`).length, 1, `workspace ${workspaceID} emits one exact cleanup request`);
    }
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceSelect').options.length === 0 && !document.querySelector('#workspaceEmpty').hidden`), 'every user7-owned workspace removed through exact UI-supported cleanup');
    assert.equal(accountHasOwnedWorkspaces(7), false, 'workspace DELETE readbacks, not a fixture flip, clear user7 ownership honestly');
    assert.ok(workspaces.some((workspace) => workspace.id === 81) && workspaces.some((workspace) => workspace.id === 91), 'exact user7 cleanup preserves other principals\' workspaces');

    accountDeleteCommitAmbiguousNext = true;
    delayAccountDeleteResponse = true;
    await evaluate(client, `(() => { document.querySelector('#openDeleteAccount').click(); document.querySelector('#confirmDeleteAccount').click(); })()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#authView').hidden && !document.querySelector('#authAlert').hidden && document.querySelector('#authAlert').textContent.includes('Account deletion outcome is unknown')`), 'post-commit account response ambiguity signs out without false existence claims');
    assert.deepEqual(await evaluate(client, `(() => { const alert = document.querySelector('#authAlert'); return { visible: !alert.hidden && alert.getClientRects().length > 0, role: alert.getAttribute('role'), live: alert.getAttribute('aria-live'), message: alert.textContent.includes('Account deletion outcome is unknown'), loginSelected: document.querySelector('#loginTab').getAttribute('aria-selected'), focus: document.activeElement?.id }; })()`), {
      visible: true, role: 'alert', live: 'polite', message: true, loginSelected: 'true', focus: 'loginEmail',
    }, 'ambiguous account deletion keeps its recovery message visible and live after local sign-out');
    assert.equal(accountDeleted, true, 'mock commits account deletion and token invalidation before returning 503');
    assert.equal(await evaluate(client, `fetch('/api/profile', { headers: { Authorization: 'Bearer ${token}' } }).then((response) => response.status)`), 401, 'committed ambiguous deletion invalidates the old bearer');
    const failedReauthStart = requests.length;
    await evaluate(client, `(() => { document.querySelector('#loginEmail').value = 'qa@example.test'; document.querySelector('#loginPassword').value = 'replacement-password-2026'; document.querySelector('#loginForm').requestSubmit(); })()`);
    await waitFor(() => requests.slice(failedReauthStart).some((item) => item.method === 'POST' && item.path === '/api/login'), 'deleted principal re-authentication attempt');
    await waitFor(() => evaluate(client, `document.querySelector('#appView').hidden && document.querySelector('#authAlert').textContent.includes('Authentication failed')`), 'committed deletion is resolved by failed re-authentication');

    await evaluate(client, `(() => { document.querySelector('#loginEmail').value = 'second@example.test'; document.querySelector('#loginPassword').value = 'correct horse battery staple'; document.querySelector('#loginForm').requestSubmit(); })()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#appView').hidden && document.querySelector('#profileName').textContent === 'Second Architect' && document.querySelector('#workspaceSelect').value === '81' && document.querySelector('#workspaceRole').textContent === 'Architect' && !document.querySelector('#openDeleteOwnedWorkspace').disabled`), 'clearly new principal session begins the independent zero-owned lifecycle');
    await evaluate(client, `document.querySelector('#accountTab').click()`);
    const secondOwnedTargets = workspaces.filter((workspace) => workspaceOwners.get(workspace.id)?.has(8)).map((workspace) => workspace.id);
    assert.ok(secondOwnedTargets.length >= 2 && [81, 43].every((workspaceID) => secondOwnedTargets.includes(workspaceID)), 'new principal fixture exposes initial and retained UI-created exact owned targets');
    assert.ok(!secondOwnedTargets.includes(44) && !workspaces.some((workspace) => workspace.id === 44), 'workspace 44 remains absent after the earlier deleted-selection fallback');
    assert.ok(workspaces.some((workspace) => workspace.id === 91) && !secondOwnedTargets.includes(91), 'unrelated workspace 91 is not included in the new principal cleanup targets');
    for (const workspaceID of secondOwnedTargets) {
      await evaluate(client, `(() => { const select = document.querySelector('#workspaceSelect'); select.value = ${JSON.stringify(String(workspaceID))}; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
      await waitFor(() => evaluate(client, `document.querySelector('#workspaceSelect').value === ${JSON.stringify(String(workspaceID))} && document.querySelector('#workspaceRole').textContent === 'Architect' && !document.querySelector('#openDeleteOwnedWorkspace').disabled`), `new principal owned workspace ${workspaceID} selected for exact cleanup`);
      const targetDeleteStart = requests.length;
      await evaluate(client, `document.querySelector('#openDeleteOwnedWorkspace').click()`);
      assert.deepEqual(await evaluate(client, `(() => { const dialog = document.querySelector('#deleteOwnedWorkspaceConfirm'); return { visible: !dialog.hidden && dialog.getClientRects().length > 0, role: dialog.getAttribute('role'), confirmDisabled: document.querySelector('#confirmDeleteOwnedWorkspace').disabled, focus: document.activeElement?.id, exactTarget: document.querySelector('#deleteOwnedWorkspaceCopy').textContent.includes(${JSON.stringify(`workspace ID ${workspaceID}`)}) }; })()`), {
        visible: true, role: 'alertdialog', confirmDisabled: false, focus: 'confirmDeleteOwnedWorkspace', exactTarget: true,
      }, `new principal workspace ${workspaceID} cleanup renders an exact-target confirmation and focuses its enabled action`);
      await evaluate(client, `document.querySelector('#confirmDeleteOwnedWorkspace').click()`);
      await waitFor(() => requests.slice(targetDeleteStart).some((item) => item.method === 'DELETE' && item.path === `/api/workspaces/${workspaceID}`), `new principal workspace ${workspaceID} exact DELETE`);
      await waitFor(() => evaluate(client, `![...document.querySelector('#workspaceSelect').options].some((option) => option.value === ${JSON.stringify(String(workspaceID))}) && document.querySelector('#confirmDeleteOwnedWorkspace').getAttribute('aria-busy') === 'false'`), `new principal workspace ${workspaceID} authoritative removal readback`);
      assert.equal(requests.slice(targetDeleteStart).filter((item) => item.method === 'DELETE' && item.path === `/api/workspaces/${workspaceID}`).length, 1, `new principal workspace ${workspaceID} emits one exact cleanup request`);
    }
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceSelect').options.length === 0 && !document.querySelector('#workspaceEmpty').hidden && !document.querySelector('#openDeleteAccount').disabled`), 'new principal authoritative zero-owned readback enables account deletion');
    assert.equal(accountHasOwnedWorkspaces(8), false, 'new principal ownership precondition derives from exact retained workspace state');
    assert.ok(workspaces.some((workspace) => workspace.id === 91), 'unrelated non-owned workspace survives both exact cleanup sequences');
    await evaluate(client, `(() => { document.querySelector('#openDeleteAccount').click(); document.querySelector('#confirmDeleteAccount').click(); })()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#authView').hidden && !document.querySelector('#authAlert').hidden && document.querySelector('#authAlert').textContent.includes('Required audit and control records remain retained')`), 'new zero-owned principal account delete 204 signs out explicitly');
    assert.deepEqual(await evaluate(client, `(() => { const alert = document.querySelector('#authAlert'); return { visible: !alert.hidden && alert.getClientRects().length > 0, role: alert.getAttribute('role'), live: alert.getAttribute('aria-live'), message: alert.textContent.includes('Required audit and control records remain retained'), success: alert.classList.contains('success'), loginSelected: document.querySelector('#loginTab').getAttribute('aria-selected'), focus: document.activeElement?.id }; })()`), {
      visible: true, role: 'alert', live: 'polite', message: true, success: true, loginSelected: 'true', focus: 'loginEmail',
    }, 'successful account deletion keeps retained-record truth visible and live after selecting sign-in');
    assert.equal(secondAccountDeleted, true, 'zero-owned self-delete reaches the supported 204 lifecycle for the new principal');
  } finally {
    await closeChromium(chromium, 'component-journey');
    server.closeAllConnections?.();
    await new Promise((resolve) => server.close(resolve));
    await rm(tempDirectory, { recursive: true, force: true, maxRetries: 10, retryDelay: 100 });
  }
});

test('workspace tools guide organizer, invited Viewer, and Maintainer through real role-aware data', { timeout: 90_000 }, async () => {
  const browser = await installedChromium();
  assert.ok(browser, 'Chromium is required; workspace discoverability cannot be accepted without a browser');

  const cockpitHTML = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const requests = [];
  let viewerAccepted = false;
  let roleHistoryEmpty = true;
  let bazaarAttempts = 0;
  let delayedWorkspace41Responses = 0;
  let delayedProposalResponses = 0;
  let delayedRaceVoteResponses = 0;
  let delayedDoubleVoteResponses = 0;
  let decisionRelationshipOverride = null;
  let proposal = { id: 'proposal_browser_role', workspace_id: 41, title: 'Open the community pantry', body: 'Approve a bounded pantry pilot.', policy: { quorum: 1, approval_threshold: 1 }, status: 'OPEN', version: 1 };
  let raceProposal = { id: 'proposal_race', workspace_id: 41, title: 'Race-safe proposal', body: 'Keep response ordering isolated.', policy: { quorum: 1, approval_threshold: 1 }, status: 'OPEN', version: 1 };
  let doubleProposal = { id: 'proposal_double', workspace_id: 41, title: 'Double-activation proposal', body: 'Accept one deliberate mutation.', policy: { quorum: 1, approval_threshold: 1 }, status: 'OPEN', version: 1 };
  const alternateProposal = { id: 'proposal_alternate', workspace_id: 41, title: 'Alternate proposal', body: 'Selected while another mutation is pending.', policy: { quorum: 1, approval_threshold: 1 }, status: 'OPEN', version: 1 };
  let votes = [];
  let decision = null;
  let quest = null;
  let purchase = null;
  const users = {
    'architect@example.test': { token: 'architect-token', user: { id: 7, username: 'QA Architect', email: 'architect@example.test' } },
    'viewer@example.test': { token: 'viewer-token', user: { id: 8, username: 'QA Viewer', email: 'viewer@example.test' } },
    'maintainer@example.test': { token: 'maintainer-token', user: { id: 9, username: 'QA Maintainer', email: 'maintainer@example.test' } },
  };
  const roleForToken = (token) => ({ 'architect-token': 'owner', 'viewer-token': 'viewer', 'maintainer-token': 'member' })[token] || '';
  const actorForToken = (token) => ({ 'architect-token': users['architect@example.test'].user, 'viewer-token': users['viewer@example.test'].user, 'maintainer-token': users['maintainer@example.test'].user })[token];
  const templates = [
    { id: 'bazaar-cooperative', title: 'Bazaar cooperative', version: '1.0.0', description: 'Published marketplace workflow.', allowedModules: ['announcements', 'shura-governance', 'compliance-source-review', 'bazaar-template-lifecycle', 'qard-hasan', 'volunteer-stipend', 'sandbox-escrow', 'revenue-split'] },
    { id: 'community-iftar', title: 'Community Iftar', version: '1.0.0', description: 'Registration and community coordination.', allowedModules: ['iftar-registration', 'announcements', 'donation-campaign', 'volunteer-stipend'] },
    { id: 'community-workspace', title: 'Community workspace', version: '1.0.0', description: 'Complete cooperative workspace.', allowedModules: ['iftar-registration', 'announcements', 'donation-campaign', 'shura-governance', 'compliance-source-review', 'bazaar-template-lifecycle', 'zakat', 'qard-hasan', 'volunteer-stipend', 'sandbox-escrow', 'revenue-split'] },
  ];
  const moduleIDs = ['announcements', 'bazaar-template-lifecycle', 'compliance-source-review', 'donation-campaign', 'iftar-registration', 'qard-hasan', 'revenue-split', 'sandbox-escrow', 'shura-governance', 'volunteer-stipend', 'zakat'];
  const roleDocumentFields = [
    { key: 'title', label: 'Card heading', valueType: 'string', description: 'Curated heading.', required: true, maxLength: 120 },
    { key: 'summary', label: 'Summary', valueType: 'string', description: 'Curated summary.', required: true, maxLength: 600 },
  ];
  const roleComponentPolicy = { contractVersion: 'taawun.artifact/v2', rootType: 'object', stableIdRule: 'one instance per allowed module; id equals type', keyGrammar: 'ASCII letter first', numberFormat: 'canonical base-10 JSON', allowedValueTypes: ['null', 'boolean', 'number', 'string', 'array', 'object'], reservedKeyParts: ['proto', 'script', 'html', 'origin', 'workspace', 'auth', 'finance', 'token', 'password'], maxComponentBytes: 8192, maxTotalBytes: 32768, maxDepth: 6, maxKeyBytes: 64, maxObjectFields: 32, maxArrayItems: 32, maxTotalKeys: 128, maxStringRunes: 2048, maxNumberBytes: 64 };
  const modules = moduleIDs.map((id) => ({ id, title: id.replaceAll('-', ' '), dataClassifications: [`${id}-records`], documentFields: roleDocumentFields, defaultDocument: { title: id.replaceAll('-', ' '), summary: `Curated ${id} summary.` } }));
  const flows = ['donation', 'marketplace-escrow', 'multi-party-approval', 'qard-hasan', 'revenue-split', 'volunteer-stipend', 'zakat'].map((id, index) => ({ id, title: id.replaceAll('-', ' '), defaultApprovals: index ? 2 : 1, minimumParties: index ? 2 : 1 }));
  const publishedListing = { id: 'listing_browser', state: 'published', version: 3, revision: { title: 'Synthetic cooperative template', summary: 'A clearly labelled browser-test listing.', currency: 'USD', priceMinor: 1200, license: 'private-beta-sandbox' } };
  const roleTrack = {
    id: 'track_role_handoff', workspaceId: 41, status: 'PREVIEW_READY', version: 6, createdBy: 7,
    createdAt: '2026-08-18T03:00:00Z', updatedAt: '2026-08-18T03:10:00Z',
    request: {
      workspaceId: 41, appName: 'Cooperative handoff', organizationName: 'QA Community', city: 'Denver', madhhab: 'hanafi', templateId: 'bazaar-cooperative',
      theme: { accentColor: '#57A68E' }, modules: ['announcements', 'shura-governance'],
      components: [
        { id: 'announcements', type: 'announcements', data: { title: 'Handoff announcements', summary: 'Exact Architect-authored request.' } },
        { id: 'shura-governance', type: 'shura-governance', data: { title: 'Handoff governance', summary: 'Review before creating a new draft.' } },
      ],
      requestedOrigins: { surfaces: [], embedders: [], connections: [], resources: [] }, ttlHours: 24,
    },
    artifact: { artifactId: 'artifact_role_handoff', contentHash: 'c'.repeat(64) },
    preview: {
      artifactId: 'artifact_role_handoff', contentHash: 'c'.repeat(64), workspaceId: 41,
      subject: { id: 'user:7', userId: 7 },
      authorizationExpiresAt: '2099-08-18T03:10:00Z', allowedOrigins: { surfaces: [], embedders: [], connections: [], resources: [] }, authenticationRequired: true,
    },
  };
  const roleClaim = { id: 'claim_role', workspaceId: 41, origin: 'https://role.community.example', host: 'role.community.example', status: 'verified', challengeExpiresAt: '2099-08-18T00:00:00Z', verifiedAt: '2026-08-18T00:00:00Z', verificationExpiresAt: '2099-08-18T00:00:00Z', createdAt: '2026-08-18T00:00:00Z', updatedAt: '2026-08-18T00:00:00Z' };
  const rolePublication = { id: 'publication_role', workspaceId: 41, claimId: roleClaim.id, origin: roleClaim.origin, contentHash: 'c'.repeat(64), artifactId: 'artifact_role_handoff', activatedAt: '2026-08-18T03:20:00Z', deactivatedAt: null, active: true, authorizationExpiresAt: '2099-08-18T03:10:00Z', manifestDigest: 'd'.repeat(64), servingState: 'serving', trackBinding: { trackId: roleTrack.id, status: 'PUBLISHED', version: 7 } };

  const server = createServer(async (request, response) => {
    const chunks = [];
    for await (const chunk of request) chunks.push(chunk);
    const rawBody = Buffer.concat(chunks).toString('utf8');
    const requestURL = new URL(request.url, 'http://localhost');
    const pathName = requestURL.pathname;
    const bearer = String(request.headers.authorization || '').replace(/^Bearer\s+/iu, '');
    const body = rawBody ? JSON.parse(rawBody) : null;
    requests.push({ method: request.method, path: pathName, bearer, body });
    const send = (status, contentType, payload) => {
      response.writeHead(status, { 'Content-Type': contentType, 'Cache-Control': 'no-store' });
      response.end(payload);
    };
    const json = (status, payload) => send(status, 'application/json; charset=utf-8', JSON.stringify(payload));
    if (request.method === 'GET' && pathName === '/account') return send(200, 'text/html; charset=utf-8', cockpitHTML);
    if (request.method === 'POST' && pathName === '/api/login') {
      const login = users[body.email];
      return login ? json(200, login) : json(401, { error: 'Unauthorized' });
    }
    if (request.method === 'GET' && pathName === '/api/bazaar/listings') {
      bazaarAttempts += 1;
      if (bazaarAttempts === 1) return json(503, { error: { code: 'unavailable', message: 'Catalog temporarily unavailable.' } });
      return json(200, { listings: bazaarAttempts === 2 ? [] : [publishedListing] });
    }
    if (request.method === 'GET' && pathName === '/api/bazaar/listings/listing_browser/test-drive') return send(200, 'text/html; charset=utf-8', '<!doctype html><title>Synthetic test drive</title>');
    const actor = actorForToken(bearer);
    if (request.method === 'GET' && pathName === '/api/profile' && actor) return json(200, actor);
    if (request.method === 'GET' && pathName === '/api/workspaces' && actor) {
      if (bearer === 'viewer-token' && !viewerAccepted) return json(200, { workspaces: [] });
      const workspaces = [{ id: 41, name: 'QA Community', owner_id: 7, status: 'active' }];
      if (bearer === 'architect-token') workspaces.push({ id: 42, name: 'QA Second Workspace', owner_id: 7, status: 'active' });
      return json(200, { workspaces });
    }
    if (request.method === 'GET' && pathName === '/api/workspaces/99/people') return json(403, { error: 'Workspace access forbidden' });
    if (request.method === 'GET' && pathName === '/api/workspaces/41/people' && actor) {
      if (bearer === 'viewer-token' && !viewerAccepted) return json(403, { error: 'Workspace access forbidden' });
      if (delayedWorkspace41Responses > 0) {
        delayedWorkspace41Responses -= 1;
        await wait(180);
      }
      const members = [{ user_id: 7, username: 'QA Architect', role: 'owner', joined_at: '2026-08-18T00:00:00Z' }];
      if (viewerAccepted) members.push({ user_id: 8, username: 'QA Viewer', role: 'viewer', joined_at: '2026-08-18T01:00:00Z' });
      members.push({ user_id: 9, username: 'QA Maintainer', role: 'member', joined_at: '2026-08-18T02:00:00Z' });
      return json(200, { members });
    }
    if (request.method === 'GET' && pathName === '/api/workspaces/42/people') {
      return bearer === 'architect-token'
        ? json(200, { members: [{ user_id: 7, username: 'QA Architect · workspace B', role: 'owner', joined_at: '2026-08-18T00:00:00Z' }] })
        : json(403, { error: 'Workspace access forbidden' });
    }
    if (request.method === 'GET' && pathName === '/api/templates' && actor) return json(200, { templates });
    if (request.method === 'GET' && pathName === '/api/modules' && actor) return json(200, { modules, componentDocumentPolicy: roleComponentPolicy });
    if (request.method === 'GET' && pathName === '/api/conductor/tracks' && actor) {
      if (bearer === 'viewer-token' && !viewerAccepted) return json(403, { error: { code: 'workspace_forbidden', message: 'Workspace access forbidden.' } });
      return json(200, { tracks: roleHistoryEmpty ? [] : [{ id: roleTrack.id, templateId: roleTrack.request.templateId, status: roleTrack.status, version: roleTrack.version, updatedAt: roleTrack.updatedAt, previewPresent: true, artifactPresent: true, publicationPresent: false, authorizationExpiresAt: roleTrack.preview.authorizationExpiresAt }] });
    }
    if (request.method === 'GET' && pathName === `/api/conductor/tracks/${roleTrack.id}` && actor) {
      if (bearer === 'viewer-token' && !viewerAccepted) return json(403, { error: { code: 'workspace_forbidden', message: 'Workspace access forbidden.' } });
      return json(200, roleTrack);
    }
    if (request.method === 'GET' && pathName === `/api/conductor/tracks/${roleTrack.id}/events` && actor) return json(200, { events: [{ type: 'PREVIEW_READY', toStatus: 'PREVIEW_READY', trackVersion: 6, createdAt: roleTrack.updatedAt, detail: { email: 'never-render-role-detail@example.test' } }] });
    if (request.method === 'GET' && pathName === '/api/workspaces/41/domains' && actor) {
      return bearer === 'architect-token'
        ? json(200, { claims: [roleClaim] })
        : json(403, { error: { code: 'domain_forbidden', message: 'Domain evidence is unavailable for this membership.' } });
    }
    if (request.method === 'GET' && pathName === '/api/workspaces/41/domains/claim_role/publications' && bearer === 'architect-token') {
      const exact = requestURL.searchParams.get('publicationId');
      return json(200, { publications: exact && exact !== rolePublication.id ? [] : [rolePublication], serverTime: '2026-08-18T03:20:00Z' });
    }
    if (request.method === 'POST' && pathName === '/api/artifacts/preview' && bearer === 'maintainer-token') return json(422, { error: { code: 'invalid_composition', message: 'Synthetic stop after new-track request capture.' } });
    if (request.method === 'GET' && pathName === '/api/financial/flows' && actor) return json(200, { flows });
    if (request.method === 'POST' && pathName === '/api/shura/v1/invitations' && bearer === 'architect-token') {
      return json(201, { invitation: { id: 'invite_browser', workspace_id: 41, invitee: body.invitee, role: body.role, status: 'PENDING', version: 1 }, token: 'accept_browser_viewer' });
    }
    if (request.method === 'POST' && pathName === '/api/shura/v1/invitations/accept' && bearer === 'viewer-token' && body.token === 'accept_browser_viewer') {
      viewerAccepted = true;
      return json(200, { id: 'invite_browser', workspace_id: 41, invitee: 'viewer@example.test', role: 'Viewer', status: 'ACCEPTED', version: 2 });
    }
    if (request.method === 'POST' && pathName === '/api/shura/v1/capabilities' && actor) {
      const expectedRole = ({ owner: 'Architect', viewer: 'Viewer', member: 'Maintainer' })[roleForToken(bearer)];
      if (body.role !== expectedRole) return json(403, { message: 'role mismatch' });
      return json(201, { token: `shura-${expectedRole}`, claims: { role: expectedRole, workspace_id: 41, scopes: body.scopes, exp: Math.floor(Date.now() / 1000) + 900 } });
    }
    if (request.method === 'POST' && pathName === '/api/shura/v1/workspaces/41/proposals' && ['shura-Architect', 'shura-Maintainer'].includes(bearer)) {
      proposal = { ...proposal, id: bearer === 'shura-Maintainer' ? 'proposal_maintainer' : 'proposal_browser_role', title: body.title, body: body.body, policy: body.policy, version: 1, status: 'OPEN' };
      votes = []; decision = null;
      return json(201, proposal);
    }
    if (request.method === 'GET' && pathName === '/api/shura/v1/proposals/proposal_race' && bearer.startsWith('shura-')) return json(200, { proposal: raceProposal, deliberation: [], votes: [], decision: null });
    if (request.method === 'GET' && pathName === '/api/shura/v1/proposals/proposal_double' && bearer.startsWith('shura-')) return json(200, { proposal: doubleProposal, deliberation: [], votes: doubleProposal.version > 1 ? [{ id: 'vote-double', choice: 'APPROVE' }] : [], decision: null });
    if (request.method === 'GET' && pathName === '/api/shura/v1/proposals/proposal_alternate' && bearer.startsWith('shura-')) return json(200, { proposal: alternateProposal, deliberation: [], votes: [], decision: null });
    if (request.method === 'POST' && pathName === '/api/shura/v1/proposals/proposal_race/votes' && ['shura-Architect', 'shura-Maintainer'].includes(bearer)) {
      if (delayedRaceVoteResponses > 0) {
        delayedRaceVoteResponses -= 1;
        await wait(180);
      }
      raceProposal = { ...raceProposal, version: raceProposal.version + 1 };
      return json(201, { vote: { id: 'vote-race', choice: body.choice }, proposal: raceProposal });
    }
    if (request.method === 'POST' && pathName === '/api/shura/v1/proposals/proposal_double/votes' && ['shura-Architect', 'shura-Maintainer'].includes(bearer)) {
      if (delayedDoubleVoteResponses > 0) {
        delayedDoubleVoteResponses -= 1;
        await wait(180);
      }
      doubleProposal = { ...doubleProposal, version: doubleProposal.version + 1 };
      return json(201, { vote: { id: 'vote-double', choice: body.choice }, proposal: doubleProposal });
    }
    if (request.method === 'GET' && pathName === `/api/shura/v1/proposals/${proposal.id}` && bearer.startsWith('shura-')) {
      const snapshot = structuredClone({ proposal, deliberation: [], votes, decision: decisionRelationshipOverride && decision ? { ...decision, ...decisionRelationshipOverride } : decision });
      if (delayedProposalResponses > 0) {
        delayedProposalResponses -= 1;
        await wait(180);
      }
      return json(200, snapshot);
    }
    if (request.method === 'POST' && pathName === `/api/shura/v1/proposals/${proposal.id}/votes` && ['shura-Architect', 'shura-Maintainer'].includes(bearer)) {
      proposal = { ...proposal, version: proposal.version + 1 };
      votes.push({ id: `vote-${votes.length + 1}`, choice: body.choice });
      return json(201, { vote: votes.at(-1), proposal });
    }
    if (request.method === 'POST' && pathName === `/api/shura/v1/proposals/${proposal.id}/decision` && bearer === 'shura-Architect') {
      proposal = { ...proposal, version: proposal.version + 1, status: 'DECIDED' };
      decision = { id: 'decision_browser', proposal_id: proposal.id, proposal_version: proposal.version, outcome: body.outcome };
      return json(201, { decision, proposal });
    }
    if (request.method === 'POST' && pathName === '/api/financial/quests' && bearer === 'architect-token') {
      if (!body.shuraDecisionRef) return json(422, { error: { code: 'decision_required', message: 'An approved Shura decision is required.' } });
      if (body.shuraDecisionRef !== decision?.id || decision.outcome !== 'APPROVED') return json(422, { error: { code: 'decision_invalid', message: 'The Shura decision is not approved for this workspace.' } });
      quest = { id: 'quest_browser', workspaceId: 41, flowId: body.flowId, status: 'PENDING', version: 1, approvalsReceived: 0, approvalsRequired: body.approvalsRequired, providerReference: '', reconciliationReference: '', intent: { parties: body.parties } };
      return json(201, { quest, created: true });
    }
    if (request.method === 'GET' && pathName === '/api/financial/quests/quest_browser' && actor) return quest ? json(200, quest) : json(404, { error: 'not found' });
    if (request.method === 'POST' && pathName === '/api/financial/quests/quest_browser/approve' && bearer === 'architect-token') {
      quest = { ...quest, status: 'APPROVED', version: quest.version + 1, approvalsReceived: 1 };
      return json(200, quest);
    }
    if (request.method === 'POST' && pathName === '/api/financial/quests/quest_browser/execute' && bearer === 'architect-token') {
      quest = { ...quest, status: 'EXECUTING', version: quest.version + 1, providerReference: 'sandbox-provider-browser' };
      return json(200, quest);
    }
    if (request.method === 'POST' && pathName === '/api/financial/quests/quest_browser/reconcile' && bearer === 'architect-token') {
      quest = { ...quest, status: 'SETTLED', version: quest.version + 1, reconciliationReference: 'sandbox-reconciliation-browser' };
      return json(200, quest);
    }
    if (request.method === 'POST' && pathName === '/api/bazaar/listings/listing_browser/purchases' && bearer === 'architect-token') {
      purchase = { id: 'purchase_browser', listingId: publishedListing.id, targetWorkspaceId: body.targetWorkspaceId, questStatus: 'PENDING' };
      return json(201, purchase);
    }
    if (request.method === 'POST' && pathName === '/api/bazaar/purchases/purchase_browser/refresh' && bearer === 'architect-token') {
      purchase = { ...purchase, questStatus: 'SETTLED', entitlementId: 'entitlement_browser' };
      return json(200, purchase);
    }
    if (request.method === 'POST' && pathName === '/api/bazaar/entitlements/entitlement_browser/install' && bearer === 'architect-token') {
      return json(201, { id: 'installation_browser', revision: 1, version: 1 });
    }
    return json(actor ? 404 : 401, { error: { code: actor ? 'not_found' : 'unauthorized', message: actor ? 'Not found.' : 'Authentication required.' } });
  });
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const origin = `http://127.0.0.1:${server.address().port}`;
  const tempDirectory = await mkdtemp(path.join(tmpdir(), 'taawun-roles-browser-'));
  let chromium;
  const login = async (client, email) => {
    await evaluate(client, `(() => {
      const set = (id, value) => { const input = document.getElementById(id); input.value = value; input.dispatchEvent(new Event('input', { bubbles: true })); };
      set('loginEmail', ${JSON.stringify(email)}); set('loginPassword', 'correct horse battery staple'); document.getElementById('loginForm').requestSubmit();
    })()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#appView').hidden && document.querySelector('#profileEmail').textContent === ${JSON.stringify(email)}`), `${email} login`);
  };
  try {
    chromium = await launchChromium(browser, path.join(tempDirectory, 'profile'));
    const { client } = chromium;
    await client.send('Page.enable');
    await client.send('Runtime.enable');
    await client.send('Page.navigate', { url: `${origin}/account#login` });
    await waitFor(() => evaluate(client, `document.readyState === 'complete' && !document.querySelector('#loginForm').hidden`), 'role journey login');

    await login(client, 'architect@example.test');
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceRole').textContent === 'Architect' && document.querySelector('#templateCatalogCount').textContent.includes('3 real templates · 11 real modules')`), 'architect workspace and catalog');
    await waitFor(() => evaluate(client, `document.querySelector('#domainPublicationList').textContent.includes('publication_role') && document.querySelector('#domainClaimSelect').value === 'claim_role'`), 'Architect-only bounded publication context');
    await waitFor(() => evaluate(client, `!document.querySelector('#signedStarterPath').hidden && !document.querySelector('#starterInviteButton').disabled && document.querySelector('#starterInviteButton').textContent === 'Invite a Viewer'`), 'Architect exact-empty starter invitation action');
    await evaluate(client, `document.querySelector('#starterInviteButton').click()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#peopleSurface').hidden && document.activeElement?.id === 'invitee' && document.querySelector('#inviteRole').value === 'Viewer' && document.querySelector('#workspaceAnnouncer').textContent.includes('session-only')`), 'starter Viewer invitation focus and live announcement');
    await evaluate(client, `(() => { document.querySelector('#invitee').value = 'viewer@example.test'; document.querySelector('#createInviteButton').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#inviteTokenOutput').value === 'accept_browser_viewer' && document.querySelectorAll('#invitationList li').length === 1`), 'starter creates a real session-only Viewer invitation');
    assert.match(await evaluate(client, `document.querySelector('#peopleSurface').innerText`), /only records created or accepted in this browser session/iu);
    await evaluate(client, `document.querySelector('#buildTab').click()`);
    const catalog = await evaluate(client, `(() => {
      const select = document.querySelector('#templateSelect'); select.value = 'community-workspace'; select.dispatchEvent(new Event('change', { bubbles: true }));
      return { templates: select.options.length, modules: document.querySelectorAll('#moduleList input[name="selectedModule"]').length, checked: document.querySelectorAll('#moduleList input[name="selectedModule"]:checked').length };
    })()`);
    assert.deepEqual(catalog, { templates: 4, modules: 11, checked: 0 }, 'real catalog requires explicit template and component choices; no catalog data is invented or implicitly selected');
    await evaluate(client, `(() => { for (let index = 0; index < 3; index += 1) document.querySelector('#moduleList input[name="selectedModule"]:not(:checked)').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelectorAll('#moduleList input[name="selectedModule"]:checked').length === 3`), 'organizer explicit component choices');
    roleHistoryEmpty = false;
    await evaluate(client, `document.querySelector('#retryBuildHistory').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#signedStarterPath').hidden && !document.querySelector('#openNewestBuildButton').hidden && document.querySelector('#buildHistoryList').textContent.includes('track_role_handoff')`), 'real nonempty history replaces the ephemeral starter with selected-or-newest recovery');

    delayedWorkspace41Responses = 1;
    await evaluate(client, `(() => {
      const select = document.querySelector('#workspaceSelect');
      select.value = '41'; select.dispatchEvent(new Event('change', { bubbles: true }));
      select.value = '42'; select.dispatchEvent(new Event('change', { bubbles: true }));
    })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceSelect').value === '42' && document.querySelector('#peopleList').innerText.includes('workspace B')`), 'workspace B wins rapid selection');
    await wait(240);
    const workspaceRace = await evaluate(client, `({ selected: document.querySelector('#workspaceSelect').value, people: document.querySelector('#peopleList').innerText, role: document.querySelector('#workspaceRole').textContent })`);
    assert.equal(workspaceRace.selected, '42');
    assert.match(workspaceRace.people, /workspace B/u);
    assert.doesNotMatch(workspaceRace.people, /QA Maintainer/u, 'delayed workspace A people must never overwrite workspace B');
    assert.equal(workspaceRace.role, 'Architect');
    await evaluate(client, `(() => { const select = document.querySelector('#workspaceSelect'); select.value = '41'; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceSelect').value === '41' && document.querySelectorAll('#peopleList li').length === 2`), 'return to workspace A');

    await evaluate(client, `document.querySelector('#peopleTab').click()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#peopleSurface').hidden && document.querySelectorAll('#peopleList li').length === 2 && document.querySelectorAll('#invitationList li').length === 1`), 'architect People surface retains only this session’s invitation');

    await evaluate(client, `document.querySelector('#shuraTab').click()`);
    assert.equal(await evaluate(client, `document.querySelectorAll('#proposalList li').length`), 0, 'Shura must not synthesize a proposal feed');
    assert.match(await evaluate(client, `document.querySelector('#shuraSurface').innerText`), /There is no fabricated workspace proposal feed/u);
    await evaluate(client, `document.querySelector('#proposalLookup').value = 'proposal_double'; document.querySelector('#loadProposalButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#proposalRecordTitle').textContent === 'Double-activation proposal' && !document.querySelector('#approveVoteButton').disabled`), 'double-activation proposal');
    delayedDoubleVoteResponses = 1;
    await evaluate(client, `document.querySelector('#approveVoteButton').click(); document.querySelector('#approveVoteButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#proposalRecordMeta').textContent === 'OPEN · version 2 · 1 vote(s)'`), 'single accepted double-activation vote');
    assert.equal(requests.filter((request) => request.method === 'POST' && request.path === '/api/shura/v1/proposals/proposal_double/votes').length, 1, 'an in-flight proposal mutation must admit exactly one POST');
    await evaluate(client, `document.querySelector('#proposalLookup').value = 'proposal_race'; document.querySelector('#loadProposalButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#proposalLookup').value === 'proposal_race' && document.querySelector('#proposalRecordTitle').textContent === 'Race-safe proposal'`), 'same-workspace mutation race source');
    delayedRaceVoteResponses = 1;
    await evaluate(client, `document.querySelector('#approveVoteButton').click()`);
    await waitFor(() => requests.some((request) => request.method === 'POST' && request.path === '/api/shura/v1/proposals/proposal_race/votes'), 'delayed race vote dispatched');
    await evaluate(client, `document.querySelector('#proposalLookup').value = 'proposal_alternate'; document.querySelector('#loadProposalButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#proposalLookup').value === 'proposal_alternate' && document.querySelector('#proposalRecordTitle').textContent === 'Alternate proposal'`), 'new same-workspace proposal selection');
    await wait(240);
    const mutationRace = await evaluate(client, `({ lookup: document.querySelector('#proposalLookup').value, title: document.querySelector('#proposalRecordTitle').textContent, meta: document.querySelector('#proposalRecordMeta').textContent })`);
    assert.deepEqual(mutationRace, { lookup: 'proposal_alternate', title: 'Alternate proposal', meta: 'OPEN · version 1 · 0 vote(s)' }, 'a delayed mutation for proposal A must not overwrite or merge into selected proposal B');
    await evaluate(client, `(() => { document.querySelector('#proposalTitle').value = 'Open the community pantry'; document.querySelector('#proposalBody').value = 'Approve a bounded pantry pilot.'; document.querySelector('#createProposalButton').click(); })()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#proposalRecord').hidden && document.querySelector('#proposalLookup').value === 'proposal_browser_role'`), 'architect proposal');
    assert.equal(await evaluate(client, `document.querySelector('#approveDecisionButton').disabled`), false, 'Architect may explicitly decide');
    delayedProposalResponses = 1;
    await evaluate(client, `document.querySelector('#loadProposalButton').click(); document.querySelector('#approveVoteButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#proposalRecordMeta').textContent.includes('1 vote(s)') && document.querySelector('#proposalRecordMeta').textContent.includes('version 2')`), 'architect vote');
    await wait(240);
    assert.match(await evaluate(client, `document.querySelector('#proposalList').innerText`), /OPEN · version 2/u, 'a delayed version 1 read must not roll back the voted proposal');
    await evaluate(client, `document.querySelector('#approveDecisionButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#proposalRecordMeta').textContent.includes('DECIDED') && document.querySelector('#proposalRecordMeta').textContent.includes('decision APPROVED')`), 'architect decision');
    const decidedProposal = await evaluate(client, `({
      decision: document.querySelector('#proposalDecisionID').value,
      decisionVisible: !document.querySelector('#proposalDecisionGrant').hidden,
      sessionRecord: [...document.querySelectorAll('#proposalList li')].find((item) => item.querySelector('strong')?.textContent === 'Open the community pantry')?.querySelector('span')?.textContent || '',
      financeDecision: document.querySelector('#financeDecision').value,
      selectedDecision: document.querySelector('#approvedDecisionSelect').value,
    })`);
    assert.equal(decidedProposal.decision, 'decision_browser', 'the durable approved decision ID must be visible and copyable');
    assert.equal(decidedProposal.decisionVisible, true);
    assert.equal(decidedProposal.sessionRecord, 'DECIDED · version 3', 'the authoritative session record must replace OPEN version 1 for this proposal');
    assert.equal(decidedProposal.financeDecision, 'decision_browser', 'the selected-workspace approved decision must flow into Finance');
    assert.equal(decidedProposal.selectedDecision, 'decision_browser');

    await within(client.send('Page.navigate', { url: `${origin}/account` }), 'session refresh navigation');
    await waitFor(() => evaluate(client, `document.readyState === 'complete' && !document.querySelector('#loginForm').hidden`), 'session refresh login');
    await login(client, 'architect@example.test');
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceRole').textContent === 'Architect'`), 'architect after refresh');
    assert.deepEqual(await evaluate(client, `({ grantHidden: document.querySelector('#inviteGrant').hidden, token: document.querySelector('#inviteTokenOutput').value, sessionInvites: document.querySelectorAll('#invitationList li').length })`), { grantHidden: true, token: '', sessionInvites: 0 }, 'page reload clears the session-only invitation token, grant, and in-memory invitation list before further work');
    await waitFor(() => evaluate(client, `document.querySelector('#signedStarterPath').hidden && !document.querySelector('#openNewestBuildButton').hidden && document.querySelector('#buildHistoryList').textContent.includes('track_role_handoff')`), 'reload derives completed starter progress only from real server history');
    await waitFor(() => evaluate(client, `document.querySelector('#templateSelect').value === 'community-workspace' && document.querySelectorAll('#moduleList input:checked').length === 3`), 'principal workspace component draft after page refresh');
    await evaluate(client, `document.querySelector('#shuraTab').click(); document.querySelector('#proposalLookup').value = 'proposal_browser_role'; document.querySelector('#loadProposalButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#proposalDecisionID').value === 'decision_browser' && document.querySelector('#financeDecision').value === 'decision_browser'`), 'decision recovery after page refresh');

    decisionRelationshipOverride = { proposal_id: 'proposal_other' };
    await evaluate(client, `document.querySelector('#loadProposalButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#proposalDecisionGrant').hidden && document.querySelector('#financeDecision').value === '' && document.querySelector('#approvedDecisionSelect').options.length === 1`), 'mismatched decision proposal rejection');
    decisionRelationshipOverride = { proposal_id: 'proposal_browser_role', proposal_version: 2 };
    await evaluate(client, `document.querySelector('#loadProposalButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#proposalDecisionGrant').hidden && document.querySelector('#financeDecision').value === ''`), 'mismatched decision version rejection');
    decisionRelationshipOverride = null;
    await evaluate(client, `document.querySelector('#loadProposalButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#proposalDecisionID').value === 'decision_browser' && document.querySelector('#financeDecision').value === 'decision_browser'`), 'decision relationship recovery');

    delayedProposalResponses = 1;
    await evaluate(client, `(() => {
      document.querySelector('#loadProposalButton').click();
      const select = document.querySelector('#workspaceSelect'); select.value = '42'; select.dispatchEvent(new Event('change', { bubbles: true }));
    })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceSelect').value === '42' && document.querySelector('#peopleList').innerText.includes('workspace B')`), 'proposal workspace switch');
    await wait(240);
    const isolatedDecision = await evaluate(client, `({ recordHidden: document.querySelector('#proposalRecord').hidden, grantHidden: document.querySelector('#proposalDecisionGrant').hidden, financeDecision: document.querySelector('#financeDecision').value, choices: document.querySelector('#approvedDecisionSelect').options.length })`);
    assert.deepEqual(isolatedDecision, { recordHidden: true, grantHidden: true, financeDecision: '', choices: 1 }, 'a delayed workspace-A proposal must not carry its decision into workspace B');
    await evaluate(client, `(() => { const select = document.querySelector('#workspaceSelect'); select.value = '41'; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceRole').textContent === 'Architect' && document.querySelector('#workspaceSelect').value === '41'`), 'return to decision workspace');
    await evaluate(client, `document.querySelector('#shuraTab').click(); document.querySelector('#proposalLookup').value = 'proposal_browser_role'; document.querySelector('#loadProposalButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#proposalDecisionID').value === 'decision_browser' && document.querySelector('#financeDecision').value === 'decision_browser'`), 'decision recovery after workspace return');

    await evaluate(client, `document.querySelector('#financeTab').click()`);
    await waitFor(() => evaluate(client, `document.querySelectorAll('#flowList li').length === 7`), 'real finance flow catalog');
    const financeCopy = await evaluate(client, `document.querySelector('#financeSurface').innerText`);
    assert.match(financeCopy, /Sandbox only · no custody/u);
    assert.match(financeCopy, /not balances or transactions/u);
    assert.match(financeCopy, /server revalidates the final decision and exact workspace/u);
    assert.equal(await evaluate(client, `document.querySelector('#questRecord').hidden`), true, 'finance must not invent a quest');
    const questRequestsBeforeValidation = requests.filter((request) => request.method === 'POST' && request.path === '/api/financial/quests').length;
    await evaluate(client, `(() => { document.querySelector('#financeFlow').value = 'donation'; document.querySelector('#financeDecision').value = ''; document.querySelector('#createQuestButton').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#questAlert').textContent.includes('enter a final approved Shura decision ID')`), 'missing decision client rejection');
    assert.equal(requests.filter((request) => request.method === 'POST' && request.path === '/api/financial/quests').length, questRequestsBeforeValidation, 'missing decision must not reach the quest API');
    await evaluate(client, `(() => { document.querySelector('#financeDecision').value = 'decision_wrong_workspace'; document.querySelector('#createQuestButton').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#questAlert').textContent.includes('not approved for this workspace')`), 'wrong decision server rejection');
    assert.equal(await evaluate(client, `document.querySelector('#questRecord').hidden`), true, 'a rejected decision must not create a local quest');
    await evaluate(client, `(() => { const select = document.querySelector('#approvedDecisionSelect'); select.value = 'decision_browser'; select.dispatchEvent(new Event('change', { bubbles: true })); document.querySelector('#createQuestButton').click(); })()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#questRecord').hidden && document.querySelector('#questRecordMeta').textContent.includes('version 1')`), 'sandbox quest create');
    let questActions = await evaluate(client, `({ approve: document.querySelector('#questApproveButton').disabled, execute: document.querySelector('#questExecuteButton').disabled, reconcile: document.querySelector('#questReconcileButton').disabled, cancel: document.querySelector('#questCancelButton').disabled })`);
    assert.deepEqual(questActions, { approve: false, execute: true, reconcile: true, cancel: false }, 'PENDING quest actions must match the durable state machine');
    await evaluate(client, `document.querySelector('#loadQuestButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#questAlert').textContent.includes('Durable sandbox quest loaded')`), 'sandbox quest ID recovery');
    await evaluate(client, `document.querySelector('#questApproveButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#questRecordTitle').textContent.includes('APPROVED')`), 'sandbox quest approval');
    questActions = await evaluate(client, `({ approve: document.querySelector('#questApproveButton').disabled, execute: document.querySelector('#questExecuteButton').disabled, reconcile: document.querySelector('#questReconcileButton').disabled, cancel: document.querySelector('#questCancelButton').disabled })`);
    assert.deepEqual(questActions, { approve: true, execute: false, reconcile: true, cancel: false }, 'APPROVED quest actions must match the durable state machine');
    await evaluate(client, `document.querySelector('#questExecuteButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#questRecordTitle').textContent.includes('EXECUTING')`), 'sandbox quest execution');
    questActions = await evaluate(client, `({ approve: document.querySelector('#questApproveButton').disabled, execute: document.querySelector('#questExecuteButton').disabled, reconcile: document.querySelector('#questReconcileButton').disabled, cancel: document.querySelector('#questCancelButton').disabled })`);
    assert.deepEqual(questActions, { approve: true, execute: true, reconcile: false, cancel: true }, 'confirmed EXECUTING quest exposes reconciliation only');
    await evaluate(client, `document.querySelector('#questReconcileButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#questRecordTitle').textContent.includes('SETTLED')`), 'sandbox quest reconciliation');
    questActions = await evaluate(client, `({ approve: document.querySelector('#questApproveButton').disabled, execute: document.querySelector('#questExecuteButton').disabled, reconcile: document.querySelector('#questReconcileButton').disabled, cancel: document.querySelector('#questCancelButton').disabled })`);
    assert.deepEqual(questActions, { approve: true, execute: true, reconcile: true, cancel: true }, 'terminal quest exposes no invalid lifecycle action');
    const createQuestRequest = requests.find((request) => request.method === 'POST' && request.path === '/api/financial/quests');
    assert.ok(createQuestRequest, 'finance creation must call the durable quest API');
    assert.equal(Object.hasOwn(createQuestRequest.body, 'actorId'), false, 'client must not select the financial actor');
    assert.ok(createQuestRequest.body.parties.includes('taawun:user:7'), 'the authenticated actor party is represented while the server remains authoritative');

    bazaarAttempts = 0;
    await evaluate(client, `document.querySelector('#bazaarTab').click(); document.querySelector('#retryBazaar').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#bazaarState').textContent.includes('unavailable')`), 'Bazaar error state');
    await evaluate(client, `document.querySelector('#retryBazaar').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#bazaarState').textContent.includes('No published Bazaar listings yet')`), 'Bazaar honest empty retry');
    assert.equal(await evaluate(client, `document.querySelectorAll('#bazaarList li').length`), 0);
    await evaluate(client, `document.querySelector('#retryBazaar').click()`);
    await waitFor(() => evaluate(client, `document.querySelectorAll('#bazaarList li').length === 1 && document.querySelector('#bazaarState').textContent.includes('1 real published listing')`), 'real published Bazaar listing');
    const testDriveLink = await evaluate(client, `(() => { const link = document.querySelector('#bazaarList a'); return { href: link.getAttribute('href'), target: link.target, rel: link.rel }; })()`);
    assert.deepEqual(testDriveLink, { href: '/api/bazaar/listings/listing_browser/test-drive', target: '_blank', rel: 'noopener noreferrer' });
    await evaluate(client, `fetch(document.querySelector('#bazaarList a').href).then((response) => response.text())`);
    await waitFor(() => requests.some((request) => request.path === '/api/bazaar/listings/listing_browser/test-drive'), 'published listing test drive');
    await evaluate(client, `document.querySelector('#bazaarList button').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#bazaarAlert').textContent.includes('purchase_browser')`), 'sandbox Bazaar purchase');
    const purchaseRequest = requests.find((request) => request.method === 'POST' && request.path === '/api/bazaar/listings/listing_browser/purchases');
    assert.equal(purchaseRequest.body.targetWorkspaceId, 41);
    assert.match(purchaseRequest.body.idempotencyKey, /^bazaar-/u);
    await evaluate(client, `[...document.querySelectorAll('#bazaarAlert button')].find((button) => button.textContent.includes('Refresh'))?.click()`);
    await waitFor(() => evaluate(client, `[...document.querySelectorAll('#bazaarAlert button')].some((button) => button.textContent.includes('Install'))`), 'reconciled Bazaar entitlement');
    await evaluate(client, `[...document.querySelectorAll('#bazaarAlert button')].find((button) => button.textContent.includes('Install'))?.click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#bazaarAlert').textContent.includes('installation_browser')`), 'Bazaar installation');

    delayedWorkspace41Responses = 1;
    delayedProposalResponses = 1;
    await evaluate(client, `document.querySelector('#shuraTab').click(); document.querySelector('#proposalLookup').value = 'proposal_browser_role'; document.querySelector('#loadProposalButton').click(); document.querySelector('#workspaceSelect').dispatchEvent(new Event('change', { bubbles: true })); document.querySelector('#logoutButton').click()`);
    await login(client, 'viewer@example.test');
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceSelect').options.length === 0 && !document.querySelector('#workspaceEmpty').hidden && document.querySelector('#workspaceLoading').hidden`), 'pre-invite Viewer isolation');
    await wait(240);
    const principalBoundary = await evaluate(client, `({ grantHidden: document.querySelector('#inviteGrant').hidden, grantValue: document.querySelector('#inviteTokenOutput').value, decisionHidden: document.querySelector('#proposalDecisionGrant').hidden, financeDecision: document.querySelector('#financeDecision').value, sessionInvites: document.querySelectorAll('#invitationList li').length, people: document.querySelector('#peopleList').innerText, role: document.querySelector('#workspaceRole').textContent })`);
    assert.deepEqual(principalBoundary, { grantHidden: true, grantValue: '', decisionHidden: true, financeDecision: '', sessionInvites: 0, people: '', role: 'Select a workspace' }, 'sign-out must reject delayed prior-principal responses and clear invitation and decision handoffs');
    const isolated = await fetch(`${origin}/api/workspaces/41/people`, { headers: { Authorization: 'Bearer viewer-token' } });
    assert.equal(isolated.status, 403, 'Viewer must not see a workspace before accepting its invitation');
    roleHistoryEmpty = true;
    await evaluate(client, `document.querySelector('#peopleTab').click()`);
    await evaluate(client, `(() => { document.querySelector('#acceptInviteToken').value = 'accept_browser_viewer'; document.querySelector('#acceptInviteButton').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceRole').textContent === 'Viewer' && document.querySelector('#workspaceSelect').value === '41'`), 'Viewer invitation acceptance');
    await evaluate(client, `document.querySelector('#peopleTab').click()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#peopleSurface').hidden && document.querySelectorAll('#peopleList li').length === 3`), 'Viewer People readback');
    assert.deepEqual(await evaluate(client, `({ removals: document.querySelectorAll('[data-remove-member]').length, inviteDisabled: document.querySelector('#createInviteButton').disabled, domainDisabled: document.querySelector('#revokeDomainButton').disabled, cleanupDisabled: document.querySelector('#openDeleteOwnedWorkspace').disabled })`), { removals: 0, inviteDisabled: true, domainDisabled: true, cleanupDisabled: true }, 'Viewer receives no enabled offboarding, invitation, domain, or owner-cleanup authority');
    await evaluate(client, `document.querySelector('#buildTab').click()`);
    assert.equal(await evaluate(client, `document.querySelector('#templateSelect').value`), '', 'component drafts never cross principal scope');
    await waitFor(() => evaluate(client, `!document.querySelector('#signedStarterPath').hidden && document.querySelector('#starterPrimaryButton').disabled && document.querySelector('#starterPrimaryButton').textContent === 'Viewer inspect-only' && document.querySelector('#starterInviteButton').disabled`), 'Viewer exact-empty starter is inspect-only');
    roleHistoryEmpty = false;
    await evaluate(client, `document.querySelector('#retryBuildHistory').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#buildHistoryList').textContent.includes('track_role_handoff')`), 'Viewer authorized build-history read');
    await evaluate(client, `document.querySelector('#buildHistoryList button').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#buildRecordTrackID').value === 'track_role_handoff'`), 'Viewer build inspection');
    const viewerBuildControls = await evaluate(client, `({ draft: document.querySelector('#restoreBuildDraftButton').disabled, resumeHidden: document.querySelector('#resumeBuildButton').hidden, domain: document.querySelector('#domainOrigin').disabled, issue: document.querySelector('#claimDomainButton').disabled })`);
    assert.deepEqual(viewerBuildControls, { draft: true, resumeHidden: true, domain: true, issue: true }, 'Viewer history is inspect-only and domain mutation remains unavailable');
    assert.match(await evaluate(client, `document.querySelector('#domainClaimsState').textContent`), /Domain evidence is unavailable.*no mutation controls are enabled/isu, 'Viewer domain authorization failure is honest while history remains independently inspectable');
    assert.deepEqual(await evaluate(client, `({ refresh: document.querySelector('#refreshDomainPublication').disabled, replace: document.querySelector('#replaceDomainPublication').hidden, rollback: document.querySelector('#rollbackDomainPublication').hidden, records: document.querySelectorAll('#domainPublicationList li').length })`), { refresh: true, replace: true, rollback: true, records: 0 }, 'Viewer receives no publication-history action or retained Architect context');
    assert.doesNotMatch(await evaluate(client, `document.querySelector('#buildTimeline').innerText`), /never-render-role-detail/u, 'Viewer timeline excludes event detail');
    await evaluate(client, `(() => { const select = document.querySelector('#templateSelect'); select.value = 'community-workspace'; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(() => evaluate(client, `document.querySelectorAll('#moduleList input[name="selectedModule"]').length === 11`), 'Viewer component catalog');
    assert.equal(await evaluate(client, `document.querySelector('#previewButton').disabled`), true, 'Viewer cannot build');
    assert.equal(await evaluate(client, `[...document.querySelectorAll('#moduleList input, #moduleList textarea, #moduleList select, #moduleList button')].every((control) => control.disabled)`), true, 'Viewer may inspect component documents but every mutation control is disabled');
    assert.match(await evaluate(client, `document.querySelector('#workspaceHelp').textContent`), /read-only/u);
    await evaluate(client, `document.querySelector('#shuraTab').click(); document.querySelector('#proposalLookup').value = 'proposal_browser_role'; document.querySelector('#loadProposalButton').click()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#proposalRecord').hidden`), 'Viewer proposal read');
    const viewerControls = await evaluate(client, `({ vote: document.querySelector('#approveVoteButton').disabled, decide: document.querySelector('#approveDecisionButton').disabled, create: document.querySelector('#createProposalButton').disabled, decision: document.querySelector('#proposalDecisionID').value, decisionVisible: !document.querySelector('#proposalDecisionGrant').hidden, financeDisabled: document.querySelector('#financeDecision').disabled, choiceDisabled: document.querySelector('#approvedDecisionSelect').disabled, questDisabled: document.querySelector('#createQuestButton').disabled })`);
    assert.deepEqual(viewerControls, { vote: true, decide: true, create: true, decision: 'decision_browser', decisionVisible: true, financeDisabled: true, choiceDisabled: true, questDisabled: true }, 'Viewer may inspect the durable decision ID but receives no mutable finance or quest authority');
    const crossWorkspace = await fetch(`${origin}/api/workspaces/99/people`, { headers: { Authorization: 'Bearer viewer-token' } });
    assert.equal(crossWorkspace.status, 403, 'workspace people reads remain isolated');

    await evaluate(client, `document.querySelector('#logoutButton').click()`);
    roleHistoryEmpty = true;
    await login(client, 'maintainer@example.test');
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceRole').textContent === 'Maintainer'`), 'Maintainer role');
    await evaluate(client, `document.querySelector('#peopleTab').click()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#peopleSurface').hidden && document.querySelectorAll('#peopleList li').length === 3`), 'Maintainer People readback');
    assert.deepEqual(await evaluate(client, `({ removals: document.querySelectorAll('[data-remove-member]').length, inviteDisabled: document.querySelector('#createInviteButton').disabled, domainDisabled: document.querySelector('#revokeDomainButton').disabled, cleanupDisabled: document.querySelector('#openDeleteOwnedWorkspace').disabled })`), { removals: 0, inviteDisabled: true, domainDisabled: true, cleanupDisabled: true }, 'Maintainer receives no enabled offboarding, invitation, domain, or owner-cleanup authority');
    await evaluate(client, `document.querySelector('#buildTab').click()`);
    assert.equal(await evaluate(client, `document.querySelector('#templateSelect').value`), '', 'Maintainer begins with an independent principal-scoped draft');
    await waitFor(() => evaluate(client, `!document.querySelector('#signedStarterPath').hidden && !document.querySelector('#starterPrimaryButton').disabled && document.querySelector('#starterInviteButton').disabled && document.querySelector('#starterInviteButton').textContent.includes('Ask an Architect')`), 'Maintainer starter can build but delegates Viewer invitation authority');
    assert.match(await evaluate(client, `document.querySelector('#starterInviteButton').title`), /existing member/iu);
    assert.match(await evaluate(client, `document.querySelector('#domainClaimsState').textContent`), /Domain evidence is unavailable.*no mutation controls are enabled/isu, 'Maintainer domain authorization failure does not block the independently authorized starter');
    assert.deepEqual(await evaluate(client, `({ refresh: document.querySelector('#refreshDomainPublication').disabled, replace: document.querySelector('#replaceDomainPublication').hidden, rollback: document.querySelector('#rollbackDomainPublication').hidden, records: document.querySelectorAll('#domainPublicationList li').length })`), { refresh: true, replace: true, rollback: true, records: 0 }, 'Maintainer receives no publication-history action or retained Architect context');
    roleHistoryEmpty = false;
    await evaluate(client, `document.querySelector('#retryBuildHistory').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#buildHistoryList').textContent.includes('track_role_handoff')`), 'Maintainer authorized build-history read');
    await evaluate(client, `document.querySelector('#buildHistoryList button').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#buildRecordTrackID').value === 'track_role_handoff' && !document.querySelector('#restoreBuildDraftButton').disabled`), 'Maintainer build inspection and draft permission');
    await evaluate(client, `document.querySelector('#restoreBuildDraftButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#templateSelect').value === 'bazaar-cooperative' && document.querySelectorAll('#moduleList input:checked').length === 2 && !document.querySelector('[data-component-id="announcements"] .field input').disabled`), 'Maintainer exact request becomes an editable new local draft');
    await evaluate(client, `(() => { const input = document.querySelector('[data-component-id="announcements"] .field input'); input.value = 'Maintainer-owned revision'; input.dispatchEvent(new Event('input', { bubbles: true })); document.querySelector('#builderForm').requestSubmit(); })()`);
    await waitFor(() => requests.some((request) => request.method === 'POST' && request.path === '/api/artifacts/preview' && request.bearer === 'maintainer-token'), 'Maintainer submits a new composition request');
    await waitFor(() => evaluate(client, `document.querySelector('#builderAlert').textContent.includes('Synthetic stop after new-track request capture') && document.querySelector('#previewLoading').hidden`), 'Maintainer new-track request returns without adopting the source track');
    const maintainerBuild = requests.find((request) => request.method === 'POST' && request.path === '/api/artifacts/preview' && request.bearer === 'maintainer-token');
    assert.equal(maintainerBuild.body.components.find((component) => component.id === 'announcements').data.title, 'Maintainer-owned revision');
    assert.equal(Object.hasOwn(maintainerBuild.body, 'trackId'), false, 'history handoff never transfers the source track identity');
    assert.equal(Object.hasOwn(maintainerBuild.body, 'createdBy'), false, 'history handoff never transfers creator authority');
    const domainReadsByRole = new Set(requests.filter((request) => request.method === 'GET' && request.path === '/api/workspaces/41/domains').map((request) => request.bearer));
    assert.deepEqual([...domainReadsByRole].sort(), ['architect-token', 'maintainer-token', 'viewer-token'], 'People, history, and domain evidence load independently while the server remains authoritative for each role');
    assert.deepEqual([...new Set(requests.filter((request) => request.method === 'GET' && request.path === '/api/workspaces/41/domains/claim_role/publications').map((request) => request.bearer))], ['architect-token'], 'only an Architect with resolved People authority requests publication context');
    assert.equal(requests.some((request) => request.method !== 'GET' && request.path.startsWith('/api/workspaces/41/domains') && request.bearer !== 'architect-token'), false, 'Viewer and Maintainer never attempt domain mutation');
    await evaluate(client, `document.querySelector('#shuraTab').click(); (() => { document.querySelector('#proposalTitle').value = 'Schedule the pantry rota'; document.querySelector('#proposalBody').value = 'Approve the volunteer rota for one month.'; document.querySelector('#createProposalButton').click(); })()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#proposalRecord').hidden && document.querySelector('#proposalLookup').value === 'proposal_maintainer'`), 'Maintainer proposal create');
    const maintainerControls = await evaluate(client, `({ vote: document.querySelector('#approveVoteButton').disabled, decide: document.querySelector('#approveDecisionButton').disabled, create: document.querySelector('#createProposalButton').disabled })`);
    assert.deepEqual(maintainerControls, { vote: false, decide: true, create: false }, 'Maintainer may propose/vote but not decide');
    await evaluate(client, `document.querySelector('#approveVoteButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#proposalRecordMeta').textContent.includes('1 vote(s)')`), 'Maintainer vote');
    assert.equal(await evaluate(client, `document.querySelector('#approveDecisionButton').disabled`), true, 'Maintainer can never record the final decision');

    await evaluate(client, `document.querySelector('#buildTab').click()`);
    for (const layoutWidth of [160, 200, 320, 400]) {
      await client.send('Emulation.setDeviceMetricsOverride', { width: layoutWidth, height: 1_000, deviceScaleFactor: 1, mobile: false });
      await waitFor(() => evaluate(client, `window.innerWidth === ${layoutWidth}`), `${layoutWidth}px component editor`);
      const editorLayout = await evaluate(client, `({ overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth, visible: !document.querySelector('#buildSurface').hidden, cardWidth: document.querySelector('.component-editor')?.getBoundingClientRect().width || 0, viewport: document.documentElement.clientWidth, targetHeights: [...document.querySelectorAll('.button.tertiary, details summary, .publication-action, [id^="retry"]')].filter((control) => control.getClientRects().length > 0 && getComputedStyle(control).visibility !== 'hidden').map((control) => Math.round(control.getBoundingClientRect().height)) })`);
      assert.equal(editorLayout.overflow, false, `${layoutWidth}px component editor must reflow without horizontal overflow`);
      assert.equal(editorLayout.visible, true);
      assert.ok(editorLayout.cardWidth <= editorLayout.viewport, `${layoutWidth}px component card stays inside the viewport`);
      assert.ok(editorLayout.targetHeights.length > 0 && editorLayout.targetHeights.every((height) => height >= 44), `${layoutWidth}px tertiary, retry, details, and action targets must be at least 44 CSS px: ${editorLayout.targetHeights}`);
    }
    await client.send('Emulation.setDeviceMetricsOverride', { width: 1_280, height: 1_000, deviceScaleFactor: 1, mobile: false });
    await waitFor(() => evaluate(client, `window.innerWidth === 1280`), 'desktop component editor reset');

    await evaluate(client, `document.querySelector('#buildTab').focus()`);
    await pressKey(client, 'ArrowRight', 'ArrowRight', 39);
    assert.equal(await evaluate(client, `document.activeElement?.id`), 'peopleTab', 'workspace tabs use roving keyboard focus');
    await pressKey(client, 'ArrowDown', 'ArrowDown', 40);
    await waitFor(() => evaluate(client, `document.activeElement?.id === 'peopleSurface' && document.querySelector('#workspaceAnnouncer').textContent.includes('People workspace tool opened')`), 'tabpanel focus and announcement');
    const tabHeights = await evaluate(client, `[...document.querySelectorAll('.workspace-tab')].map((tab) => tab.getBoundingClientRect().height)`);
    assert.ok(tabHeights.every((height) => height >= 44), `workspace tab targets must be at least 44px: ${tabHeights.join(', ')}`);

    for (const [width, scale] of [[320, 1], [400, 1], [320, 2], [400, 2]]) {
      const layoutWidth = Math.round(width / scale);
      await client.send('Emulation.setDeviceMetricsOverride', { width: layoutWidth, height: 1_000, deviceScaleFactor: 1, mobile: false });
      await waitFor(() => evaluate(client, `window.innerWidth === ${layoutWidth}`), `${layoutWidth}px role surface`);
      const layout = await evaluate(client, `(() => ({ width: window.innerWidth, overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth, visible: !document.querySelector('#peopleSurface').hidden, copy: document.querySelector('#peopleSurface').innerText }))()`);
      assert.equal(layout.overflow, false, `${width}px at ${scale * 100}% must reflow without horizontal overflow`);
      assert.equal(layout.visible, true);
      assert.match(layout.copy, /Workspace members/u);
    }

    const capabilityRequests = requests.filter((request) => request.path === '/api/shura/v1/capabilities');
    const byRole = Object.fromEntries(capabilityRequests.map((request) => [request.body.role, request.body.scopes]));
    assert.deepEqual(byRole.Viewer, ['shura:read']);
    assert.deepEqual(byRole.Maintainer, ['shura:read', 'shura:deliberate', 'shura:vote', 'shura:propose']);
    assert.ok(byRole.Architect.includes('shura:decide') && byRole.Architect.includes('shura:invite'));
  } finally {
    await closeChromium(chromium, 'role-journey');
    server.closeAllConnections?.();
    await new Promise((resolve) => server.close(resolve));
    await rm(tempDirectory, { recursive: true, force: true, maxRetries: 10, retryDelay: 100 });
  }
});
