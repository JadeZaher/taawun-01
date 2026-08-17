import {
  ROLES,
  RUNTIME_CONTRACT,
  RUNTIME_EVENTS,
  TaawunRuntime,
  generateWorkspaceKeyMaterial,
} from './index.js';

const result = document.getElementById('result');
const artifactId = `browser-check-${crypto.randomUUID()}`;
const workspaceId = 991;
const actorId = 'browser-check-architect';
const workspaceKey = generateWorkspaceKeyMaterial();
const databaseName = `taawun-runtime-${workspaceId}-${artifactId}`;

function config(peerId, trustedPeerIds, role = ROLES.ARCHITECT, selectedArtifactId = artifactId) {
  const writeCollections = role === ROLES.VIEWER ? [] : ['convergent:announcements'];
  const peerPolicies = {};
  for (const trustedPeerId of trustedPeerIds) peerPolicies[trustedPeerId] = { actorId, role: ROLES.ARCHITECT, writeCollections: ['convergent:announcements'] };
  return {
    contractVersion: RUNTIME_CONTRACT,
    artifactId: selectedArtifactId,
    workspaceId,
    peerId,
    actor: { id: actorId, role, writeCollections },
    session: { id: `session-${peerId}`, expiresAt: Date.now() + 60_000 },
    relay: { enabled: false },
    collections: { 'convergent:announcements': { keys: ['notice'] } },
    peerPolicies,
  };
}

function nextChange(runtime) {
  return new Promise((resolve, reject) => {
    const timeout = setTimeout(() => reject(new Error('BroadcastChannel convergence timed out')), 3000);
    runtime.addEventListener(RUNTIME_EVENTS.CHANGED, (event) => {
      clearTimeout(timeout);
      resolve(event.detail);
    }, { once: true });
  });
}

function deleteDatabase(name) {
  return new Promise((resolve) => {
    const request = indexedDB.deleteDatabase(name);
    request.onsuccess = request.onerror = request.onblocked = () => resolve();
  });
}

const checks = [];
let runtimeA;
let runtimeB;
let replayRuntime;
let viewerRuntime;

try {
  runtimeA = await TaawunRuntime.create(config('browser-a', ['browser-b']), { workspaceKey });
  runtimeB = await TaawunRuntime.create(config('browser-b', ['browser-a']), { workspaceKey });
  await runtimeA.start();
  await runtimeB.start();
  const adapterA = runtimeA.createModuleAdapter('announcements', { collections: ['convergent:announcements'] });
  const converged = nextChange(runtimeB);
  await adapterA.set('convergent:announcements', 'notice', { text: 'Browser-owned content' });
  await converged;
  if (runtimeB.get('convergent:announcements', 'notice')?.text !== 'Browser-owned content') throw new Error('BroadcastChannel projection did not converge');
  checks.push('PASS BroadcastChannel convergence');

  runtimeA.close();
  runtimeB.close();
  runtimeA = null;
  runtimeB = null;

  replayRuntime = await TaawunRuntime.create(config('browser-c', ['browser-a']), { workspaceKey });
  await replayRuntime.start();
  if (replayRuntime.get('convergent:announcements', 'notice')?.text !== 'Browser-owned content') throw new Error('IndexedDB replay did not restore the projection');
  checks.push('PASS IndexedDB replay');

  const viewerArtifact = `${artifactId}-viewer`;
  viewerRuntime = await TaawunRuntime.create(config('browser-viewer', [], ROLES.VIEWER, viewerArtifact), { workspaceKey });
  await viewerRuntime.start();
  await viewerRuntime.set('convergent:announcements', 'notice', { text: 'denied' }).then(
    () => { throw new Error('Viewer write was unexpectedly allowed'); },
    () => checks.push('PASS Viewer write denied'),
  );

  if (localStorage.length || sessionStorage.length) throw new Error('Runtime wrote browser storage outside IndexedDB');
  checks.push('PASS no local/session storage key persistence');
  document.body.dataset.status = 'passed';
  result.textContent = checks.join('\n');
} catch (error) {
  document.body.dataset.status = 'failed';
  result.textContent = `FAIL ${error.message}\n${checks.join('\n')}`;
} finally {
  runtimeA?.close();
  runtimeB?.close();
  replayRuntime?.close();
  viewerRuntime?.close();
  await deleteDatabase(databaseName);
  await deleteDatabase(`taawun-runtime-${workspaceId}-${artifactId}-viewer`);
}
