import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { access, mkdtemp, readFile, rm } from 'node:fs/promises';
import { createServer } from 'node:http';
import { tmpdir } from 'node:os';
import path from 'node:path';
import test from 'node:test';

// Slim smoke contract. The former exhaustive suite (cockpit trust flows, exact
// shader constants) was deliberately retired on 2026-08-23; it remains in git
// history at src/web/index.test.mjs before commit "test: cut suite to smoke tier".

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

async function waitFor(check, label, timeout = 10_000) {
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
    assert.equal(typeof WebSocket, 'function', 'Node with built-in WebSocket support is required for the Chromium smoke check');
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

async function evaluate(client, expression) {
  const response = await client.send('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true });
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

const mockGpuScript = `(() => {
  let loseDevice;
  const lost = new Promise((resolve) => { loseDevice = resolve; });
  window.__gpuQA = { adapterRequests: 0, submissions: 0, uniforms: [], lose: () => loseDevice({ reason: 'destroyed' }) };
  Object.defineProperty(navigator, 'hardwareConcurrency', { configurable: true, value: 8 });
  Object.defineProperty(navigator, 'deviceMemory', { configurable: true, value: 8 });
  const pass = { setPipeline() {}, setBindGroup() {}, draw() {}, end() {} };
  const device = {
    lost,
    queue: { writeBuffer(_buffer, _offset, data) { window.__gpuQA.uniforms.push(Array.from(data)); }, submit() { window.__gpuQA.submissions += 1; } },
    createShaderModule() { return { getCompilationInfo: async () => ({ messages: [] }) }; },
    createRenderPipeline() { return { getBindGroupLayout() { return {}; } }; },
    createBuffer() { return { destroy() {} }; },
    createBindGroup() { return {}; },
    createCommandEncoder() { return { beginRenderPass() { return pass; }, finish() { return {}; } }; },
    destroy() {},
  };
  Object.defineProperty(navigator, 'gpu', { configurable: true, value: {
    async requestAdapter() { window.__gpuQA.adapterRequests += 1; return { isFallbackAdapter: false, async requestDevice() { return device; } }; },
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
  window.__pageErrors = [];
  window.addEventListener('error', (event) => { window.__pageErrors.push(String(event.message)); });
  window.addEventListener('unhandledrejection', (event) => { window.__pageErrors.push(String(event.reason)); });
})();`;

async function serveLanding() {
  const [landingHTML, loader, renderer] = await Promise.all([
    readFile(new URL('./landing.html', import.meta.url), 'utf8'),
    readFile(new URL('./geometric-landing.js', import.meta.url), 'utf8'),
    readFile(new URL('./geometric-renderer.js', import.meta.url), 'utf8'),
  ]);
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
  return { server, origin: `http://127.0.0.1:${server.address().port}` };
}

async function closeServer(server) {
  server.closeAllConnections?.();
  await new Promise((resolve) => server.close(resolve));
}

test('landing design contract smoke: layers, core, overscan, accessibility gates', async () => {
  const html = await readFile(new URL('./landing.html', import.meta.url), 'utf8');
  const loader = await readFile(new URL('./geometric-landing.js', import.meta.url), 'utf8');
  const renderer = await readFile(new URL('./geometric-renderer.js', import.meta.url), 'utf8');
  const css = html.match(/<style>([\s\S]*?)<\/style>/u)?.[1] || '';

  // Decorative stage stays decorative.
  assert.match(html, /class="geometry-stage"[^>]*aria-hidden="true"/u);
  assert.match(css, /\.geometry-stage \{\s*position: fixed; inset: 0;[^}]*pointer-events: none/u);
  assert.doesNotMatch(css, /filter:\s*blur|backdrop-filter:[^;]*blur/iu, 'no blur manufactures the glass');
  // The exact 2.5 CSS px core exists in both the fallback and the shader.
  assert.match(css, /--geometry-center-half:\s*1\.25px/u);
  assert.match(renderer, /crispCenterStroke[\s\S]*?max\(u\.renderMetrics\.x, 1\.0\) \* 1\.25/u);
  // Bounded overscan with a visible-viewport feather.
  assert.match(css, /#geometryCanvas \{[^}]*inset: -40px;[^}]*width: calc\(100% \+ 80px\); height: calc\(100% \+ 80px\)/u);
  assert.match(renderer, /viewportFeather/u);
  // Accessibility and capability gates keep the still design available.
  assert.match(html, /prefers-reduced-motion: reduce/u);
  assert.match(html, /forced-colors: active/u);
  assert.match(css, /@media \(prefers-reduced-motion: reduce\)[\s\S]*?#geometryCanvas \{ display: none; \}/u);
  assert.match(loader, /prefers-reduced-motion: reduce/u);
  assert.match(loader, /prefers-contrast: more/u);
  assert.match(loader, /prefers-reduced-transparency: reduce/u);
  assert.match(loader, /connection && connection\.saveData/u);
  assert.match(html, /id="motionToggle"[^>]*aria-pressed="true"/u);
  assert.match(html, /<noscript>/u);
  assert.match(html, /Content-Security-Policy/u);
  // Input stays passive and interception-free.
  assert.match(loader, /\{ passive: true \}/u);
  assert.doesNotMatch(loader, /preventDefault\(\)/u);
  assert.doesNotMatch(renderer, /preventDefault\(\)|setInterval\(/u);
  for (const event of ['pointerup', 'pointercancel', 'pointerout', 'blur', 'pagehide', 'visibilitychange']) {
    assert.match(loader, new RegExp(`addEventListener\\('${event}'`, 'u'), `${event} participates in cleanup`);
  }
  // The chrome material and inertial panel are present as structures, not pinned constants.
  assert.match(renderer, /fn environmentColor\(/u);
  assert.match(renderer, /fn railDistance\(/u);
  assert.match(renderer, /const TIMELINE_STIFFNESS/u);
  assert.match(renderer, /while \(physicsAccumulator >= FIXED_STEP_SECONDS\)/u);
  assert.match(renderer, /device\.lost/u);
  assert.match(renderer, /if \(elapsed > 50 \|\| \(elapsed > 20 && lastFrameCostExceeded\)\)/u, 'slow frames still fail to the still design');
  assert.match(renderer, /adapter\.isFallbackAdapter \|\| adapter\.info\?\.isFallbackAdapter/u);
});

test('account document smoke: noindex, no decorative renderer, snippet controls', async () => {
  const html = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  assert.match(html, /<meta name="robots" content="noindex/u);
  assert.doesNotMatch(html, /geometry-stage|geometric-landing\.js|geometric-renderer\.js/u);
  assert.match(html, /data-nosnippet/u);
});

test('builder review link remains a locator and empty workspaces expose creation', async () => {
  const html = await readFile(new URL('./index.html', import.meta.url), 'utf8');

  assert.match(html, /id="copyPreviewLinkButton"[^>]*disabled/u);
  assert.match(html, /function sharedPreviewSelection\(\)[\s\S]*?new URLSearchParams\(location\.hash\.slice\(1\)\)[\s\S]*?\^track_\[A-Za-z0-9_-\]\+\$/u);
  assert.match(html, /loadWorkspaces\(sharedReview\.workspaceID\)/u);
  assert.match(html, /return `\$\{location\.origin\}\/account#preview=\$\{encodeURIComponent\(trackID\)\}&workspace=\$\{currentWorkspaceID\(\)\}`;/u);
  assert.match(html, /async function openSharedPreviewFromLocation\(\)[\s\S]*?inspectBuildTrack\(trackID, \{ quiet: true \}\)[\s\S]*?reopenSelectedBuildPreview\(\)/u);
  assert.match(html, /await loadWorkspaces\(invitation\.workspace_id\);[\s\S]*?await openSharedPreviewFromLocation\(\);/u);
  assert.match(html, /function markPreviewDirty\(\)[\s\S]*?state\.previewReady = false;[\s\S]*?syncPreviewShareControl\(\);/u);
  assert.match(html, /id="createWorkspaceDetails"/u);
  assert.match(html, /element\('createWorkspaceDetails'\)\.open = workspaces\.length === 0;/u);
});

test('custom-field typing commits without rebuilding the editor row', async () => {
  const html = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const rowStart = html.indexOf('function renderCustomFieldRow(');
  const rowEnd = html.indexOf('function renderComponentEditor(', rowStart);
  assert.ok(rowStart >= 0 && rowEnd > rowStart, 'custom-field editor source is present');
  const customFieldRow = html.slice(rowStart, rowEnd);
  const applyBody = customFieldRow.match(/const apply = \(\) => \{[\s\S]*?\r?\n      \};\r?\n      keyInput\.addEventListener/u)?.[0] || '';

  assert.match(customFieldRow, /keyInput\.addEventListener\('input', apply\)/u);
  assert.match(customFieldRow, /valueInput\.addEventListener\('input', apply\)/u);
  assert.match(applyBody, /row\.dataset\.fieldKey = key/u);
  assert.match(applyBody, /advancedInput\.value = JSON\.stringify\(state\.componentDocuments\[componentID\], null, 2\)/u);
  assert.doesNotMatch(applyBody, /renderComponentEditors\(\)/u, 'typing must preserve the current field controls and focus');
});

test('browser smoke: desktop enhancement, interaction, pause, reduced motion', { timeout: 90_000 }, async () => {
  const browser = await installedChromium();
  assert.ok(browser, 'Chromium is required for the landing smoke check');
  const { server, origin } = await serveLanding();
  const tempDirectory = await mkdtemp(path.join(tmpdir(), 'taawun-smoke-desktop-'));
  let chromium;
  try {
    chromium = await launchChromium(browser, path.join(tempDirectory, 'profile'));
    const { client } = chromium;
    await client.send('Page.enable');
    await client.send('Runtime.enable');
    await client.send('Emulation.setDeviceMetricsOverride', { width: 1_280, height: 900, deviceScaleFactor: 1, mobile: false });
    await client.send('Page.addScriptToEvaluateOnNewDocument', { source: mockGpuScript });
    await client.send('Page.navigate', { url: origin });
    await waitFor(() => evaluate(client, `document.querySelector('.geometry-stage')?.dataset.enhanced === 'true' && !document.querySelector('#motionToggle').hidden`), 'mocked WebGPU enhancement');
    const initial = await evaluate(client, `(() => { const canvas = document.querySelector('#geometryCanvas'); const rect = canvas.getBoundingClientRect(); return {
      errors: window.__pageErrors, renderer: canvas.dataset.renderer || '',
      overscan: rect.left <= -39 && rect.top <= -39 && rect.right >= innerWidth + 24 && rect.bottom >= innerHeight + 24,
      overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
      uniformLength: window.__gpuQA.uniforms.at(-1)?.length,
    }; })()`);
    assert.deepEqual(initial.errors, [], 'landing boots without page errors');
    assert.equal(initial.renderer, 'webgpu');
    assert.equal(initial.overscan, true, 'canvas renders past every visible edge');
    assert.equal(initial.overflow, false, 'no horizontal document overflow');
    assert.equal(initial.uniformLength, 16);

    // Pointer input eases in through the renderer and cleans up on blur.
    await client.send('Input.dispatchMouseEvent', { type: 'mouseMoved', x: 400, y: 300 });
    await waitFor(() => evaluate(client, `window.__gpuQA.uniforms.at(-1)?.[10] > 0.1`), 'pointer strength reaches the shader');
    await evaluate(client, `window.dispatchEvent(new Event('blur'))`);
    await waitFor(() => evaluate(client, `document.querySelector('.geometry-stage').dataset.interaction === 'idle' && window.__gpuQA.uniforms.at(-1)?.[10] === 0`), 'blur clears local input');
    const afterBlur = await evaluate(client, `window.__gpuQA.submissions`);
    await wait(200);
    assert.equal(await evaluate(client, `window.__gpuQA.submissions`), afterBlur, 'settled scene stops drawing');

    // The motion control reverses and restores.
    await evaluate(client, `document.getElementById('motionToggle').click()`);
    await waitFor(() => evaluate(client, `!document.querySelector('.geometry-stage').hasAttribute('data-enhanced') && document.querySelector('#motionToggle').getAttribute('aria-pressed') === 'false'`), 'reversible pause');
    await evaluate(client, `document.getElementById('motionToggle').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('.geometry-stage')?.dataset.enhanced === 'true'`), 'motion resume');

    // Native scrolling stays available through the decorative layer.
    const scrollStart = await evaluate(client, `window.scrollY`);
    await client.send('Input.dispatchMouseEvent', { type: 'mouseWheel', x: 600, y: 400, deltaX: 0, deltaY: 120 });
    await waitFor(() => evaluate(client, `window.scrollY > ${scrollStart}`), 'wheel scrolling through the decoration');

    // Reduced motion falls back to the still design.
    await client.send('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-reduced-motion', value: 'reduce' }] });
    await waitFor(() => evaluate(client, `!document.querySelector('.geometry-stage').hasAttribute('data-enhanced') && getComputedStyle(document.querySelector('#geometryCanvas')).display === 'none'`), 'reduced motion stays on the still design');
    assert.deepEqual(await evaluate(client, `window.__pageErrors`), [], 'no page errors across the journey');
  } finally {
    await closeChromium(chromium, 'desktop-smoke');
    await closeServer(server);
    await rm(tempDirectory, { recursive: true, force: true, maxRetries: 10, retryDelay: 100 });
  }
});

test('browser smoke: mobile metrics enhance with touch scroll preserved', { timeout: 90_000 }, async () => {
  const browser = await installedChromium();
  assert.ok(browser, 'Chromium is required for the landing smoke check');
  const { server, origin } = await serveLanding();
  const tempDirectory = await mkdtemp(path.join(tmpdir(), 'taawun-smoke-mobile-'));
  let chromium;
  try {
    chromium = await launchChromium(browser, path.join(tempDirectory, 'profile'));
    const { client } = chromium;
    await client.send('Page.enable');
    await client.send('Runtime.enable');
    await client.send('Emulation.setDeviceMetricsOverride', { width: 390, height: 844, deviceScaleFactor: 2, mobile: true });
    await client.send('Emulation.setTouchEmulationEnabled', { enabled: true, maxTouchPoints: 2 });
    await client.send('Page.addScriptToEvaluateOnNewDocument', { source: mockGpuScript });
    await client.send('Page.navigate', { url: origin });
    await waitFor(() => evaluate(client, `document.querySelector('.geometry-stage')?.dataset.enhanced === 'true'`), 'mobile mocked WebGPU enhancement');
    const state = await evaluate(client, `({ errors: window.__pageErrors, overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth, renderer: document.querySelector('#geometryCanvas').dataset.renderer || '' })`);
    assert.deepEqual(state.errors, [], 'mobile landing boots without page errors');
    assert.equal(state.overflow, false, 'no mobile horizontal overflow');
    assert.equal(state.renderer, 'webgpu');

    const scrollStart = await evaluate(client, `window.scrollY`);
    await client.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [{ x: 200, y: 600, radiusX: 1, radiusY: 1, force: 1, id: 0 }] });
    await wait(50);
    await client.send('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: [{ x: 200, y: 420, radiusX: 1, radiusY: 1, force: 1, id: 0 }] });
    await wait(50);
    await client.send('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: [{ x: 202, y: 240, radiusX: 1, radiusY: 1, force: 1, id: 0 }] });
    await client.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
    await waitFor(() => evaluate(client, `window.scrollY > ${scrollStart}`), 'native touch scrolling through the decoration');
  } finally {
    await closeChromium(chromium, 'mobile-smoke');
    await closeServer(server);
    await rm(tempDirectory, { recursive: true, force: true, maxRetries: 10, retryDelay: 100 });
  }
});
