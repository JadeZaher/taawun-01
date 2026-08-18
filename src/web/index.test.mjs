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
    const authenticated = ['/api/profile', '/api/workspaces', '/api/workspaces/41/people', '/api/templates', '/api/modules', '/api/artifacts/preview'];
    if (authenticated.includes(record.path) && record.authorization !== `Bearer ${token}`) {
      return json(401, { error: { code: 'unauthorized', message: 'Authentication required.' } });
    }
    if (record.method === 'GET' && record.path === '/api/profile') {
      return json(200, { id: 7, username: 'QA Architect', email: 'qa@example.test' });
    }
    if (record.method === 'GET' && record.path === '/api/workspaces') {
      return json(200, { workspaces: [{ id: 41, name: 'QA Community' }] });
    }
    if (record.method === 'GET' && record.path === '/api/workspaces/41/people') {
      return json(200, { members: [{ user_id: 7, username: 'QA Architect', role: 'owner', joined_at: '2026-08-18T00:00:00Z' }] });
    }
    if (record.method === 'GET' && record.path === '/api/templates') {
      return json(200, { templates: [{ id: 'community-iftar', title: 'Community Iftar', version: '1.0.0', description: 'Signed private-beta card.', allowedModules: ['iftar-registration', 'announcements', 'donation-campaign'] }] });
    }
    if (record.method === 'GET' && record.path === '/api/modules') {
      return json(200, { modules: [
        { id: 'iftar-registration', title: 'Iftar registration', dataClassifications: ['community-registrations'] },
        { id: 'announcements', title: 'Community announcements', dataClassifications: ['community-announcements'] },
        { id: 'donation-campaign', title: 'Donation campaign', dataClassifications: ['donation-intents'], allowedServerSignals: ['taawun_donation_status'] },
      ] });
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

    const requiredAuthenticatedPaths = ['/api/profile', '/api/workspaces', '/api/workspaces/41/people', '/api/templates', '/api/modules', '/api/artifacts/preview'];
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

test('workspace tools guide organizer, invited Viewer, and Maintainer through real role-aware data', { timeout: 90_000 }, async () => {
  const browser = await installedChromium();
  assert.ok(browser, 'Chromium is required; workspace discoverability cannot be accepted without a browser');

  const cockpitHTML = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const requests = [];
  let viewerAccepted = false;
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
  const modules = moduleIDs.map((id) => ({ id, title: id.replaceAll('-', ' '), dataClassifications: [`${id}-records`] }));
  const flows = ['donation', 'marketplace-escrow', 'multi-party-approval', 'qard-hasan', 'revenue-split', 'volunteer-stipend', 'zakat'].map((id, index) => ({ id, title: id.replaceAll('-', ' '), defaultApprovals: index ? 2 : 1, minimumParties: index ? 2 : 1 }));
  const publishedListing = { id: 'listing_browser', state: 'published', version: 3, revision: { title: 'Synthetic cooperative template', summary: 'A clearly labelled browser-test listing.', currency: 'USD', priceMinor: 1200, license: 'private-beta-sandbox' } };

  const server = createServer(async (request, response) => {
    const chunks = [];
    for await (const chunk of request) chunks.push(chunk);
    const rawBody = Buffer.concat(chunks).toString('utf8');
    const pathName = new URL(request.url, 'http://localhost').pathname;
    const bearer = String(request.headers.authorization || '').replace(/^Bearer\s+/iu, '');
    const body = rawBody ? JSON.parse(rawBody) : null;
    requests.push({ method: request.method, path: pathName, bearer, body });
    const send = (status, contentType, payload) => {
      response.writeHead(status, { 'Content-Type': contentType, 'Cache-Control': 'no-store' });
      response.end(payload);
    };
    const json = (status, payload) => send(status, 'application/json; charset=utf-8', JSON.stringify(payload));
    if (request.method === 'GET' && pathName === '/') return send(200, 'text/html; charset=utf-8', cockpitHTML);
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
    if (request.method === 'GET' && pathName === '/api/modules' && actor) return json(200, { modules });
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
    await client.send('Page.navigate', { url: origin });
    await waitFor(() => evaluate(client, `document.readyState === 'complete' && !document.querySelector('#loginForm').hidden`), 'role journey login');

    await login(client, 'architect@example.test');
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceRole').textContent === 'Architect' && document.querySelector('#templateCatalogCount').textContent.includes('3 real templates · 11 real modules')`), 'architect workspace and catalog');
    const catalog = await evaluate(client, `(() => {
      const select = document.querySelector('#templateSelect'); select.value = 'community-workspace'; select.dispatchEvent(new Event('change', { bubbles: true }));
      return { templates: select.options.length, modules: document.querySelectorAll('#moduleList input[name="selectedModule"]').length, checked: document.querySelectorAll('#moduleList input[name="selectedModule"]:checked').length };
    })()`);
    assert.deepEqual(catalog, { templates: 3, modules: 11, checked: 11 }, 'real catalog selection must feed the existing composer');

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
    await waitFor(() => evaluate(client, `!document.querySelector('#peopleSurface').hidden && document.querySelectorAll('#peopleList li').length === 2`), 'architect people surface');
    await evaluate(client, `(() => { document.querySelector('#invitee').value = 'viewer@example.test'; document.querySelector('#inviteRole').value = 'Viewer'; document.querySelector('#createInviteButton').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#inviteTokenOutput').value === 'accept_browser_viewer' && document.querySelectorAll('#invitationList li').length === 1`), 'real invitation grant');
    assert.match(await evaluate(client, `document.querySelector('#peopleSurface').innerText`), /only records created or accepted in this browser session/iu);

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

    await within(client.send('Page.navigate', { url: origin }), 'session refresh navigation');
    await waitFor(() => evaluate(client, `document.readyState === 'complete' && !document.querySelector('#loginForm').hidden`), 'session refresh login');
    await login(client, 'architect@example.test');
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceRole').textContent === 'Architect'`), 'architect after refresh');
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
    await evaluate(client, `document.querySelector('#peopleTab').click()`);
    await evaluate(client, `(() => { document.querySelector('#acceptInviteToken').value = 'accept_browser_viewer'; document.querySelector('#acceptInviteButton').click(); })()`);
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceRole').textContent === 'Viewer' && document.querySelector('#workspaceSelect').value === '41'`), 'Viewer invitation acceptance');
    assert.equal(await evaluate(client, `document.querySelector('#previewButton').disabled`), true, 'Viewer cannot build');
    assert.match(await evaluate(client, `document.querySelector('#workspaceHelp').textContent`), /read-only/u);
    await evaluate(client, `document.querySelector('#shuraTab').click(); document.querySelector('#proposalLookup').value = 'proposal_browser_role'; document.querySelector('#loadProposalButton').click()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#proposalRecord').hidden`), 'Viewer proposal read');
    const viewerControls = await evaluate(client, `({ vote: document.querySelector('#approveVoteButton').disabled, decide: document.querySelector('#approveDecisionButton').disabled, create: document.querySelector('#createProposalButton').disabled, decision: document.querySelector('#proposalDecisionID').value, decisionVisible: !document.querySelector('#proposalDecisionGrant').hidden, financeDisabled: document.querySelector('#financeDecision').disabled, choiceDisabled: document.querySelector('#approvedDecisionSelect').disabled, questDisabled: document.querySelector('#createQuestButton').disabled })`);
    assert.deepEqual(viewerControls, { vote: true, decide: true, create: true, decision: 'decision_browser', decisionVisible: true, financeDisabled: true, choiceDisabled: true, questDisabled: true }, 'Viewer may inspect the durable decision ID but receives no mutable finance or quest authority');
    const crossWorkspace = await fetch(`${origin}/api/workspaces/99/people`, { headers: { Authorization: 'Bearer viewer-token' } });
    assert.equal(crossWorkspace.status, 403, 'workspace people reads remain isolated');

    await evaluate(client, `document.querySelector('#logoutButton').click()`);
    await login(client, 'maintainer@example.test');
    await waitFor(() => evaluate(client, `document.querySelector('#workspaceRole').textContent === 'Maintainer'`), 'Maintainer role');
    await evaluate(client, `document.querySelector('#shuraTab').click(); (() => { document.querySelector('#proposalTitle').value = 'Schedule the pantry rota'; document.querySelector('#proposalBody').value = 'Approve the volunteer rota for one month.'; document.querySelector('#createProposalButton').click(); })()`);
    await waitFor(() => evaluate(client, `!document.querySelector('#proposalRecord').hidden && document.querySelector('#proposalLookup').value === 'proposal_maintainer'`), 'Maintainer proposal create');
    const maintainerControls = await evaluate(client, `({ vote: document.querySelector('#approveVoteButton').disabled, decide: document.querySelector('#approveDecisionButton').disabled, create: document.querySelector('#createProposalButton').disabled })`);
    assert.deepEqual(maintainerControls, { vote: false, decide: true, create: false }, 'Maintainer may propose/vote but not decide');
    await evaluate(client, `document.querySelector('#approveVoteButton').click()`);
    await waitFor(() => evaluate(client, `document.querySelector('#proposalRecordMeta').textContent.includes('1 vote(s)')`), 'Maintainer vote');
    assert.equal(await evaluate(client, `document.querySelector('#approveDecisionButton').disabled`), true, 'Maintainer can never record the final decision');

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
    if (chromium) {
      try { await within(chromium.client.send('Browser.close'), 'role-journey browser close', 1_000); } catch {}
      chromium.client.close();
      await Promise.race([new Promise((resolve) => chromium.processHandle.once('exit', resolve)), wait(1_000)]);
      if (chromium.processHandle.exitCode === null) chromium.processHandle.kill();
    }
    server.closeAllConnections?.();
    await new Promise((resolve) => server.close(resolve));
    await rm(tempDirectory, { recursive: true, force: true, maxRetries: 10, retryDelay: 100 });
  }
});
