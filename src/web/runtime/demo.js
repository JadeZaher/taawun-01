import {
  RUNTIME_CONTRACT,
  RUNTIME_EVENTS,
  TaawunRuntime,
  bytesToBase64URL,
  generateWorkspaceKeyMaterial,
} from './index.js';

const byId = (id) => document.getElementById(id);
let runtime = null;
let announcements = null;

byId('peerId').value = `browser-${crypto.randomUUID().slice(0, 8)}`;

function setStatus(id, message) {
  byId(id).textContent = message;
}

function renderSnapshot() {
  byId('snapshot').textContent = JSON.stringify(runtime?.snapshot() || {}, null, 2);
}

function setEditorEnabled(enabled) {
  byId('announcement').disabled = !enabled;
  byId('saveAnnouncement').disabled = !enabled;
  byId('deleteAnnouncement').disabled = !enabled;
}

function buildConfig() {
  const role = byId('role').value;
  const actorId = byId('actorId').value.trim();
  const writeCollections = role === 'Viewer' ? [] : ['convergent:announcements'];
  const peerPolicies = {};
  for (const peerId of byId('trustedPeers').value.split(',').map((value) => value.trim()).filter(Boolean)) {
    peerPolicies[peerId] = { actorId, role, writeCollections };
  }
  return {
    contractVersion: RUNTIME_CONTRACT,
    artifactId: byId('artifactId').value.trim(),
    workspaceId: Number(byId('workspaceId').value),
    peerId: byId('peerId').value.trim(),
    actor: {
      id: actorId,
      role,
      writeCollections,
    },
    session: {
      id: `demo-${crypto.randomUUID()}`,
      expiresAt: Date.now() + 60 * 60 * 1000,
    },
    relay: { enabled: false },
    collections: {
      'convergent:announcements': { keys: ['notice', 'schedule'] },
    },
    peerPolicies,
  };
}

byId('generateKey').addEventListener('click', () => {
  byId('workspaceKey').value = bytesToBase64URL(generateWorkspaceKeyMaterial());
  setStatus('sessionStatus', 'Ephemeral 256-bit workspace key generated. It has not been persisted or sent anywhere.');
});

byId('startRuntime').addEventListener('click', async () => {
  runtime?.close();
  setEditorEnabled(false);
  try {
    runtime = await TaawunRuntime.create(buildConfig(), { workspaceKey: byId('workspaceKey').value.trim() });
    runtime.addEventListener(RUNTIME_EVENTS.CHANGED, renderSnapshot);
    runtime.addEventListener(RUNTIME_EVENTS.ERROR, (event) => setStatus('writeStatus', event.detail.message));
    await runtime.start();
    announcements = runtime.createModuleAdapter('announcements', { collections: ['convergent:announcements'] });
    setEditorEnabled(true);
    renderSnapshot();
    setStatus('sessionStatus', `Runtime active for ${runtime.artifactId}. Reload to test IndexedDB replay.`);
    setStatus('writeStatus', 'Ready for an offline write.');
  } catch (error) {
    runtime = null;
    announcements = null;
    setStatus('sessionStatus', error.message);
  }
});

byId('saveAnnouncement').addEventListener('click', async () => {
  try {
    await announcements.set('convergent:announcements', 'notice', { text: byId('announcement').value });
    setStatus('writeStatus', 'Append-only operation stored and broadcast to same-origin peers.');
  } catch (error) {
    setStatus('writeStatus', error.message);
  }
});

byId('deleteAnnouncement').addEventListener('click', async () => {
  try {
    await announcements.delete('convergent:announcements', 'notice');
    setStatus('writeStatus', 'Tombstone appended. Older values cannot reappear during replay.');
  } catch (error) {
    setStatus('writeStatus', error.message);
  }
});

window.addEventListener('pagehide', () => runtime?.close(), { once: true });
