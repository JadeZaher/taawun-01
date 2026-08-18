import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import vm from 'node:vm';

test('cockpit inline modules parse before they are embedded in the server binary', async () => {
  const html = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const modules = [...html.matchAll(/<script type="module">([\s\S]*?)<\/script>/g)];

  assert.equal(modules.length, 1, 'the cockpit must contain one executable module');
  assert.doesNotThrow(() => new vm.Script(modules[0][1], { filename: 'index.html:inline-module' }));
});

test('registration enforces the server password minimum', async () => {
  const html = await readFile(new URL('./index.html', import.meta.url), 'utf8');

  assert.match(html, /id="registerPassword"[^>]*minlength="12"/u);
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
