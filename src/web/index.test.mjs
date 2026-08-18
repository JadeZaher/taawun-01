import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { createHash } from 'node:crypto';
import { access, mkdtemp, readFile, rm } from 'node:fs/promises';
import { createServer } from 'node:http';
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

async function waitFor(check, label, timeout = 8_000) {
  const deadline = Date.now() + timeout;
  let lastError;
  while (Date.now() < deadline) {
    try {
      const value = await check();
      if (value) return value;
    } catch (error) {
      lastError = error;
    }
    await wait(40);
  }
  throw new Error(`${label} timed out${lastError ? `: ${lastError.message}` : ''}`);
}

class DevToolsClient {
  constructor(socket) {
    this.socket = socket;
    this.sequence = 0;
    this.pending = new Map();
    this.listeners = new Map();
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
  }

  static async connect(url) {
    assert.equal(typeof WebSocket, 'function', 'Node with built-in WebSocket support is required for the Chromium regression');
    const socket = new WebSocket(url);
    await new Promise((resolve, reject) => {
      socket.addEventListener('open', resolve, { once: true });
      socket.addEventListener('error', () => reject(new Error('Could not connect to Chromium DevTools')), { once: true });
    });
    return new DevToolsClient(socket);
  }

  on(method, listener) {
    if (!this.listeners.has(method)) this.listeners.set(method, new Set());
    this.listeners.get(method).add(listener);
  }

  send(method, params = {}, sessionId) {
    const id = ++this.sequence;
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      this.socket.send(JSON.stringify({ id, method, params, ...(sessionId ? { sessionId } : {}) }));
    });
  }

  close() {
    this.socket.close();
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
    '--disable-default-apps',
    '--disable-gpu',
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
  return { processHandle, client: await DevToolsClient.connect(target.webSocketDebuggerUrl) };
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

test('registration enforces the server password minimum', async () => {
  const html = await readFile(new URL('./index.html', import.meta.url), 'utf8');

  assert.match(html, /id="registerPassword"[^>]*minlength="12"/u);
  assert.match(html, /id="registerPassword"[^>]*aria-describedby="registerPasswordHelp"/u);
  assert.match(html, /Use at least 12 characters\./u);
  assert.doesNotMatch(html, /Use at least 8 characters\.|id="registerPassword"[^>]*minlength="8"/u);
});

test('successful registration signs in with ephemeral local credentials', async () => {
  const html = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const handler = html.match(/element\('registerForm'\)\.addEventListener\('submit',[\s\S]*?(?=\n\s*element\('logoutButton'\))/u)?.[0];

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
});

test('mobile auth shell preserves its value and custody explanation with accessible keyboard tabs', async () => {
  const browser = await installedChromium();
  assert.ok(browser, 'Chromium is required; the mobile accessibility regression cannot be skipped');

  const cockpitHTML = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const server = createServer((request, response) => {
    if (request.method === 'GET' && new URL(request.url, 'http://localhost').pathname === '/') {
      response.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8', 'Cache-Control': 'no-store' });
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
          const copy = document.querySelector('.auth-story p');
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
          };
        })()`);
        assert.match(authLayout.copy, /Workspace-signed artifacts/u);
        assert.match(authLayout.copy, /local-first records/u);
        assert.match(authLayout.copy, /sandbox actions/u);
        assert.match(authLayout.copy, /explicit transactional control records/u);
        assert.match(authLayout.copy, /Relay transit stays encrypted: an Amanah boundary with zero custody and no settlement/u);
        assert.equal(authLayout.visible, true, `${width}px at ${scale * 100}% must show the concrete product and custody explanation`);
        assert.equal(authLayout.overflow, false, `${width}px at ${scale * 100}% must reflow without horizontal document overflow: ${authLayout.overflowElements.join(', ')}`);
        assert.equal(authLayout.layoutWidth, layoutWidth, `${width}px at ${scale * 100}% must use the expected reflow width`);

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
    if (chromium) {
      try { await chromium.client.send('Browser.close'); } catch {}
      chromium.client.close();
      await Promise.race([
        new Promise((resolve) => chromium.processHandle.once('exit', resolve)),
        wait(1_000),
      ]);
      if (chromium.processHandle.exitCode === null) chromium.processHandle.kill();
    }
    server.closeAllConnections?.();
    await new Promise((resolve) => server.close(resolve));
    await rm(tempDirectory, { recursive: true, force: true, maxRetries: 10, retryDelay: 100 });
  }
});

test('customer cockpit renders an authenticated signed preview in the exact sandbox', async () => {
  const browser = await installedChromium();
  assert.ok(browser, 'Chromium is required; the cockpit browser regression cannot be skipped');

  const cockpitHTML = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const runtime = await readFile(new URL('./assets/datastar-v1.0.2.js', import.meta.url), 'utf8');
  assert.match(runtime, /\$&/u, 'the exact pinned runtime fixture must retain its literal $&');

  const token = 'browser-signed-preview-token';
  const previewPrefix = '/api/conductor/tracks/browser-track/preview/files/';
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
  ]);
  const requests = [];
  const receiptVariants = [
    'active',
    'missing-attestation',
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
    'missing-fields',
  ];
  let previewBuilds = 0;
  const server = createServer(async (request, response) => {
    const chunks = [];
    for await (const chunk of request) chunks.push(chunk);
    const rawBody = Buffer.concat(chunks).toString('utf8');
    const record = {
      method: request.method,
      path: new URL(request.url, 'http://localhost').pathname,
      authorization: request.headers.authorization || '',
      body: rawBody ? JSON.parse(rawBody) : null,
    };
    requests.push(record);
    const send = (status, contentType, body) => {
      response.writeHead(status, { 'Content-Type': contentType, 'Cache-Control': 'no-store' });
      response.end(body);
    };
    const json = (status, body) => send(status, 'application/json; charset=utf-8', JSON.stringify(body));

    if (record.method === 'GET' && record.path === '/') return send(200, 'text/html; charset=utf-8', cockpitHTML);
    if (record.method === 'GET' && record.path === '/assets/datastar-v1.0.2.js') return send(200, 'text/javascript; charset=utf-8', runtime);
    if (record.method === 'POST' && record.path === '/api/login') {
      return json(200, { token, user: { id: 7, username: 'QA Architect', email: 'qa@example.test' } });
    }
    if (previewFiles.has(record.path)) {
      if (record.authorization !== `Bearer ${token}`) return json(401, { error: { code: 'unauthorized', message: 'Authentication required.' } });
      const [contentType, body] = previewFiles.get(record.path);
      return send(200, contentType, body);
    }
    const authenticated = ['/api/profile', '/api/workspaces', '/api/templates', '/api/artifacts/preview'];
    if (authenticated.includes(record.path) && record.authorization !== `Bearer ${token}`) {
      return json(401, { error: { code: 'unauthorized', message: 'Authentication required.' } });
    }
    if (record.method === 'GET' && record.path === '/api/profile') {
      return json(200, { id: 7, username: 'QA Architect', email: 'qa@example.test' });
    }
    if (record.method === 'GET' && record.path === '/api/workspaces') {
      return json(200, { workspaces: [{ id: 41, name: 'QA Community' }] });
    }
    if (record.method === 'GET' && record.path === '/api/templates') {
      return json(200, { templates: [{ id: 'community-iftar', version: '1.0.0', description: 'Signed private-beta card.' }] });
    }
    if (record.method === 'POST' && record.path === '/api/artifacts/preview') {
      const variant = receiptVariants[previewBuilds] || 'active';
      const attestedManifest = {
        contractVersion: 'taawun.artifact-manifest/v1',
        artifactId: 'browser-signed-artifact',
        contentHash: 'a'.repeat(64),
        workspaceId: 41,
        template: { id: 'community-iftar', version: '1.0.0' },
        modules: ['iftar-registration', 'announcements', 'donation-campaign'],
        renderModes: ['standalone'],
        compliance: {
          status: 'reference-only-pending-qualified-review',
          references: [{ id: 'ref-iftar-1', title: 'Iftar reference', status: 'pending-qualified-review' }],
        },
        financial: { status: 'sandbox' },
        authorization: {
          subject: { id: 'user:7', userId: 7, workspaceId: 41 },
          allowedOrigins: {
            surfaces: [origin],
            embedders: ['https://community.example'],
            connections: ['https://relay.example'],
            resources: ['https://assets.example'],
          },
          expiresAt: '2099-08-18T04:23:28Z',
          signerKeyId: 'railway-artifact-v1',
          lifecycle: 'preview',
        },
        signature: {
          algorithm: 'Ed25519',
          keyId: 'railway-artifact-v1',
          value: 'browser-signature-value',
        },
      };
      if (variant === 'elapsed-expiry') attestedManifest.authorization.expiresAt = '2000-08-18T04:23:28Z';
      if (variant === 'subject-workspace') attestedManifest.authorization.subject.workspaceId = 42;
      if (variant === 'authorization-key') attestedManifest.authorization.signerKeyId = 'tampered-key';
      let responseManifest = structuredClone(attestedManifest);
      if (variant === 'lifecycle') responseManifest.authorization.lifecycle = 'published';
      if (variant === 'expiry') responseManifest.authorization.expiresAt = '2098-08-18T04:23:28Z';
      if (variant === 'surfaces') responseManifest.authorization.allowedOrigins.surfaces = ['https://tampered-surface.example'];
      if (variant === 'embedders') responseManifest.authorization.allowedOrigins.embedders = ['https://tampered-embedder.example'];
      if (variant === 'connections') responseManifest.authorization.allowedOrigins.connections = ['https://tampered-connection.example'];
      if (variant === 'resources') responseManifest.authorization.allowedOrigins.resources = ['https://tampered-resource.example'];
      if (variant === 'missing-fields') responseManifest = { artifactId: 'missing-optional-fields' };
      const manifestJson = JSON.stringify(attestedManifest);
      const verification = {
        status: 'verified',
        verified: true,
        artifactId: 'browser-signed-artifact',
        contentHash: 'a'.repeat(64),
        workspaceId: 41,
        signatureAlgorithm: 'Ed25519',
        signerKeyId: 'railway-artifact-v1',
        signatureValue: 'browser-signature-value',
        manifestDigest: createHash('sha256').update(manifestJson).digest('hex'),
        manifestJson,
      };
      if (variant === 'missing-digest') delete verification.manifestDigest;
      if (variant === 'digest-mismatch') verification.manifestDigest = '0'.repeat(64);
      if (variant === 'artifact') verification.artifactId = 'tampered-artifact';
      if (variant === 'content-hash') verification.contentHash = 'b'.repeat(64);
      if (variant === 'workspace') verification.workspaceId = 42;
      if (variant === 'signature') verification.signatureValue = 'tampered-signature';
      if (variant === 'key') verification.signerKeyId = 'tampered-key';
      if (variant === 'algorithm') verification.signatureAlgorithm = 'tampered-algorithm';
      previewBuilds += 1;
      return json(200, {
        manifest: responseManifest,
        verification: variant === 'missing-attestation' ? undefined : verification,
        previewUrl: `${previewPrefix}index.html`,
        preview: {
          documentUrl: `${previewPrefix}index.html`,
          themeUrl: `${previewPrefix}theme.css`,
          stylesUrl: `${previewPrefix}app.css`,
        },
        track: { id: 'browser-track', preview: { allowedOrigins: { surfaces: [] } } },
      });
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

  const tempDirectory = await mkdtemp(path.join(tmpdir(), 'taawun-cockpit-browser-'));
  let chromium;
  try {
    chromium = await launchChromium(browser, path.join(tempDirectory, 'profile'));
    const { client } = chromium;
    await client.send('Page.enable');
    await client.send('Runtime.enable');
    await client.send('Page.navigate', { url: origin });
    await waitFor(() => evaluate(client, `document.readyState === 'complete' && !document.querySelector('#loginForm').hidden`), 'cockpit login');

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
    await waitFor(() => evaluate(client, `!document.querySelector('#appView').hidden
      && document.querySelector('#workspaceSelect').value === '41'
      && document.querySelector('#templateBadge').textContent.startsWith('Ready')
      && !document.querySelector('#previewButton').disabled`), 'workspace and template load');

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
    await waitFor(() => evaluate(client, `document.querySelector('#previewStatus').textContent === 'Staging ready'
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
    assert.equal(receipt['Exact allowed origins'], `Surface: ${origin} · Embedder: https://community.example · Connection: https://relay.example · Resource: https://assets.example`);
    assert.equal(receipt['Review references'], 'ref-iftar-1 (pending-qualified-review) · Reference-only; not scholar approval');

    const runtimeStartTag = '<script type="module">';
    const runtimeEndTag = '</script>';
    const runtimeStart = cockpitState.srcdoc.indexOf(runtimeStartTag);
    const runtimeEnd = cockpitState.srcdoc.indexOf(runtimeEndTag, runtimeStart);
    assert.ok(runtimeStart >= 0 && runtimeEnd > runtimeStart, 'srcdoc must contain the approved inline runtime');
    assert.equal(cockpitState.srcdoc.indexOf(runtimeStartTag, runtimeStart + runtimeStartTag.length), -1, 'srcdoc must contain one inline module');
    assert.doesNotMatch(cockpitState.srcdoc, /<script type="module" src=/u, 'external runtime tag must be replaced');
    const sourceRuntime = cockpitState.srcdoc.slice(runtimeStart + runtimeStartTag.length, runtimeEnd);
    assert.equal(sourceRuntime, runtime, 'srcdoc must contain the intact approved runtime');
    assert.match(sourceRuntime, /\$&/u, 'the intact inline runtime must retain literal $&');

    const frameTarget = await waitFor(async () => {
      const { targetInfos } = await client.send('Target.getTargets');
      const pageTarget = targetInfos.find((target) => target.type === 'page' && target.url === `${origin}/`);
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
    assert.equal(frameState.scripts[0].text.replaceAll('\r', ''), runtime.replaceAll('\r', ''), 'parsed runtime must remain intact');

    const interaction = await evaluate(client, `new Promise((resolve) => {
      document.querySelector('#card-control').click();
      setTimeout(() => resolve(document.querySelector('#notice').textContent), 100);
    })`, undefined, frameSession);
    assert.equal(interaction, 'Card control activated', 'signed card controls must remain interactive');

    const requiredAuthenticatedPaths = ['/api/profile', '/api/workspaces', '/api/templates', '/api/artifacts/preview'];
    for (const requiredPath of requiredAuthenticatedPaths) {
      const request = requests.find((item) => item.path === requiredPath);
      assert.ok(request, `${requiredPath} must be reached through the cockpit`);
      assert.equal(request.authorization, `Bearer ${token}`, `${requiredPath} must use the login token`);
    }
    const previewRequests = requests.filter((item) => previewFiles.has(item.path) && item.authorization === `Bearer ${token}`);
    assert.deepEqual(previewRequests.map((item) => item.path).sort(), [...previewFiles.keys()].sort(), 'all signed preview files must be fetched with authentication');
    const artifactRequest = requests.find((item) => item.path === '/api/artifacts/preview');
    assert.equal(artifactRequest.body.workspaceId, 41);
    assert.equal(artifactRequest.body.templateId, 'community-iftar');
    assert.deepEqual(artifactRequest.body.modules, ['iftar-registration', 'announcements', 'donation-campaign']);
    for (const runtimeRequest of requests.filter((item) => item.path === '/assets/datastar-v1.0.2.js')) {
      assert.equal(runtimeRequest.authorization, '', 'the bearer token must not leak to the public pinned runtime');
    }

    for (let index = 1; index < receiptVariants.length; index += 1) {
      const variant = receiptVariants[index];
      await evaluate(client, `document.querySelector('#builderForm').requestSubmit()`);
      await waitFor(async () => previewBuilds === index + 1
        && await evaluate(client, `document.querySelector('#previewStatus').textContent === 'Staging ready'`), `${variant} receipt response`);
      const changedReceipt = await evaluate(client, `Object.fromEntries([...document.querySelectorAll('#manifestList .manifest-row')].map((row) => [row.querySelector('dt').textContent, row.querySelector('dd').textContent]))`);
      if (variant === 'elapsed-expiry') {
        assert.equal(changedReceipt['Signature verification'], 'Signature attested by Taawun build service · authorization expired');
        assert.match(changedReceipt['Lifecycle / expiry'], /expired$/u);
      } else {
        assert.equal(changedReceipt['Signature verification'], 'Verification unavailable — do not rely on this receipt', `${variant} must fail active verification`);
        assert.doesNotMatch(changedReceipt['Signature verification'], /^Verified\b/u, `${variant} must not retain the positive state`);
      }
      if (variant === 'missing-fields') {
        assert.equal(changedReceipt['Signing key'], 'Signing key not provided');
        assert.equal(changedReceipt['Workspace binding'], 'Workspace binding not provided');
        assert.equal(changedReceipt['Lifecycle / expiry'], 'Lifecycle not provided · expiry not provided');
        assert.equal(changedReceipt['Exact allowed origins'], 'No exact origins listed');
        assert.equal(changedReceipt['Review references'], 'No review references supplied · Reference-only; not scholar approval');
      }
    }
  } finally {
    if (chromium) {
      try { await chromium.client.send('Browser.close'); } catch {}
      chromium.client.close();
      await Promise.race([
        new Promise((resolve) => chromium.processHandle.once('exit', resolve)),
        wait(1_000),
      ]);
      if (chromium.processHandle.exitCode === null) chromium.processHandle.kill();
    }
    server.closeAllConnections?.();
    await new Promise((resolve) => server.close(resolve));
    await rm(tempDirectory, { recursive: true, force: true });
  }
});
