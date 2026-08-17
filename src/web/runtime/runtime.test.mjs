import test from 'node:test';
import assert from 'node:assert/strict';
import { webcrypto } from 'node:crypto';

if (!globalThis.crypto) Object.defineProperty(globalThis, 'crypto', { value: webcrypto });

const {
  OPERATION_CONTRACT,
  RUNTIME_CONTRACT,
  ROLES,
  authorizeOperationPolicy,
  chunkOperations,
  createOperation,
  decryptOperations,
  defaultWritePolicy,
  encryptOperations,
  generateWorkspaceKeyMaterial,
  materializeOperations,
  mergeOperations,
  validateOperation,
  validateRuntimeConfig,
} = await import('./core.js');

function config(overrides = {}) {
  return {
    contractVersion: RUNTIME_CONTRACT,
    artifactId: 'iftar-demo-v1',
    workspaceId: 42,
    peerId: 'browser-a',
    actor: { id: 'architect-a', role: ROLES.ARCHITECT, writeCollections: ['*'] },
    session: { id: 'session-a', expiresAt: Date.now() + 60_000 },
    relay: { enabled: false },
    collections: {
      'convergent:announcements': { keys: ['notice', 'schedule'] },
      'convergent:registrations': { keyPrefixes: ['registration:'] },
    },
    peerPolicies: {
      'browser-b': { actorId: 'maintainer-b', role: ROLES.MAINTAINER, writeCollections: ['convergent:announcements'] },
      'browser-c': { actorId: 'viewer-c', role: ROLES.VIEWER, writeCollections: [] },
    },
    ...overrides,
  };
}

function operation(overrides = {}) {
  return {
    contractVersion: OPERATION_CONTRACT,
    artifactId: 'iftar-demo-v1',
    workspaceId: 42,
    opId: '00000000-0000-4000-8000-000000000001',
    actorId: 'architect-a',
    peerId: 'browser-a',
    collection: 'convergent:announcements',
    key: 'notice',
    lamport: 1,
    deleted: false,
    value: { text: 'Iftar begins at sunset.' },
    ...overrides,
  };
}

test('merge is deterministic across replay order and preserves tombstones', () => {
  const older = operation({ opId: 'op-1', lamport: 2, value: { text: 'Older' } });
  const actorTie = operation({ opId: 'op-2', actorId: 'maintainer-b', lamport: 3, value: { text: 'Maintainer' } });
  const actorWinner = operation({ opId: 'op-3', actorId: 'z-architect', lamport: 3, value: { text: 'Actor tie winner' } });
  const tombstone = operation({ opId: 'op-4', actorId: 'architect-a', lamport: 4, deleted: true, value: null });
  const schedule = operation({ opId: 'op-5', key: 'schedule', lamport: 1, value: ['18:30'] });

  const forward = [older, actorTie, actorWinner, tombstone, schedule];
  const reverse = [...forward].reverse();
  assert.deepEqual(mergeOperations(forward), mergeOperations(reverse));
  assert.deepEqual(materializeOperations(forward), {
    'convergent:announcements': { schedule: ['18:30'] },
  });
});

test('operation validation is scoped, size-bounded, and rejects unsafe values', () => {
  const validated = validateRuntimeConfig(config());
  assert.doesNotThrow(() => validateOperation(operation(), validated));
  assert.throws(() => validateOperation(operation({ key: 'unapproved' }), validated), /unapproved/u);
  assert.throws(() => validateOperation(operation({ workspaceId: 7 }), validated), /scope/u);
  assert.throws(() => validateOperation(operation({ value: { constructor: 'unsafe' } }), validated), /forbidden/u);

  const created = createOperation(validated, {
    collection: 'convergent:registrations',
    key: 'registration:household-7',
    value: { guests: 4 },
    lamport: 9,
    opId: 'op-created-9',
  });
  assert.equal(created.lamport, 9);
  assert.equal(created.value.guests, 4);
});

test('runtime config validates sessions, relay security, and role policies', () => {
  const validated = validateRuntimeConfig(config());
  assert.equal(validated.relay.enabled, false);
  assert.equal(defaultWritePolicy(validated.actor, 'convergent:announcements'), true);
  assert.equal(defaultWritePolicy(validated.peerPolicies['browser-b'], 'convergent:announcements'), true);
  assert.equal(defaultWritePolicy(validated.peerPolicies['browser-b'], 'convergent:registrations'), false);
  assert.equal(defaultWritePolicy(validated.peerPolicies['browser-c'], 'convergent:announcements'), false);

  assert.throws(() => validateRuntimeConfig(config({ session: { id: 'expired', expiresAt: Date.now() - 1 } })), /future/u);
  assert.throws(() => validateRuntimeConfig(config({
    relay: { enabled: true, url: 'ws://relay.example/api/p2p/stream', token: '0123456789abcdef', iceServers: [] },
  })), /wss/u);
  assert.doesNotThrow(() => validateRuntimeConfig(config({
    relay: { enabled: true, url: 'ws://localhost:8080/api/p2p/stream', token: '0123456789abcdef', iceServers: [{ urls: 'stun:stun.example.test:3478' }] },
  })));
});

test('remote operation policy binds relay peer, actor, role, and collection', () => {
  const validated = validateRuntimeConfig(config());
  const maintainerWrite = operation({ actorId: 'maintainer-b', peerId: 'browser-b' });
  assert.equal(authorizeOperationPolicy(validated, maintainerWrite, { source: 'relay', peerId: 'browser-b' }).role, ROLES.MAINTAINER);
  assert.throws(
    () => authorizeOperationPolicy(validated, maintainerWrite, { source: 'relay', peerId: 'browser-c' }),
    /transport peer/u,
  );
  assert.throws(
    () => authorizeOperationPolicy(validated, operation({ actorId: 'viewer-c', peerId: 'browser-c' }), { source: 'webrtc', peerId: 'browser-c' }),
    /Viewer cannot write/u,
  );
  assert.throws(
    () => authorizeOperationPolicy(validated, operation({ actorId: 'architect-a', peerId: 'browser-b' }), { source: 'relay', peerId: 'browser-b' }),
    /matching injected capability/u,
  );
});

test('AES-GCM sync envelopes round-trip and bind artifact and sender', async () => {
  const workspaceKey = generateWorkspaceKeyMaterial();
  const operations = [operation({ value: { text: 'Private community message' } })];
  const context = { artifactId: 'iftar-demo-v1', senderPeerId: 'browser-a' };
  const envelope = await encryptOperations(workspaceKey, context, operations);
  assert.equal(envelope.algorithm, 'A256GCM');
  assert.equal(JSON.stringify(envelope).includes('Private community message'), false);
  assert.deepEqual(await decryptOperations(workspaceKey, envelope, context), operations);

  await assert.rejects(
    () => decryptOperations(generateWorkspaceKeyMaterial(), envelope, context),
    /authentication/u,
  );
  await assert.rejects(
    () => decryptOperations(workspaceKey, envelope, { ...context, senderPeerId: 'browser-b' }),
    /scope mismatch/u,
  );
});

test('sync batching stays inside the configured plaintext bound', () => {
  const operations = Array.from({ length: 12 }, (_, index) => operation({
    opId: `op-${index}`,
    key: index % 2 ? 'notice' : 'schedule',
    lamport: index + 1,
    value: { text: 'x'.repeat(350) },
  }));
  const chunks = chunkOperations(operations, 2048);
  assert.ok(chunks.length > 1);
  assert.equal(chunks.flat().length, operations.length);
});
