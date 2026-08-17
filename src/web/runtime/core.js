export const RUNTIME_CONTRACT = 'taawun.runtime/v1';
export const OPERATION_CONTRACT = 'taawun.convergent-op/v1';
export const SYNC_CONTRACT = 'taawun.crdt-sync/v1';
export const MODULE_ADAPTER_CONTRACT = 'taawun.module-adapter/v1';

export const ROLES = Object.freeze({
  ARCHITECT: 'Architect',
  MAINTAINER: 'Maintainer',
  VIEWER: 'Viewer',
});

const RELAY_ID = /^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/;
const COLLECTION_ID = /^[A-Za-z0-9][A-Za-z0-9._:-]{0,95}$/;
const KEY_ID = /^[A-Za-z0-9][A-Za-z0-9._:-]{0,191}$/;
const MODULE_ID = /^[A-Za-z0-9][A-Za-z0-9._-]{0,95}$/;
const FORBIDDEN_OBJECT_KEYS = new Set(['__proto__', 'prototype', 'constructor']);
const DEFAULT_MAX_OPERATION_BYTES = 32 * 1024;
const MAX_SYNC_BYTES = 40 * 1024;

const textEncoder = new TextEncoder();
const textDecoder = new TextDecoder('utf-8', { fatal: true });

export function assertRelayId(value, label = 'identifier') {
  if (typeof value !== 'string' || !RELAY_ID.test(value)) {
    throw new TypeError(`${label} must contain only letters, digits, dot, underscore, or hyphen`);
  }
  return value;
}

function assertCollectionId(value, label = 'collection') {
  if (typeof value !== 'string' || !COLLECTION_ID.test(value) || FORBIDDEN_OBJECT_KEYS.has(value)) {
    throw new TypeError(`${label} is not a valid collection identifier`);
  }
  return value;
}

function assertKeyId(value, label = 'key') {
  if (typeof value !== 'string' || !KEY_ID.test(value) || FORBIDDEN_OBJECT_KEYS.has(value)) {
    throw new TypeError(`${label} is not a valid convergent key`);
  }
  return value;
}

function assertPlainObject(value, label) {
  const prototype = value && typeof value === 'object' ? Object.getPrototypeOf(value) : undefined;
  if (!value || typeof value !== 'object' || Array.isArray(value) || (prototype !== Object.prototype && prototype !== null)) {
    throw new TypeError(`${label} must be a plain object`);
  }
  return value;
}

function assertAllowedKeys(value, label, depth = 0) {
  if (depth > 16) throw new TypeError(`${label} exceeds the maximum nesting depth`);
  if (value === null || typeof value === 'string' || typeof value === 'boolean') return;
  if (typeof value === 'number') {
    if (!Number.isFinite(value)) throw new TypeError(`${label} contains a non-finite number`);
    return;
  }
  if (Array.isArray(value)) {
    value.forEach((item, index) => assertAllowedKeys(item, `${label}[${index}]`, depth + 1));
    return;
  }
  assertPlainObject(value, label);
  for (const [key, item] of Object.entries(value)) {
    if (FORBIDDEN_OBJECT_KEYS.has(key)) throw new TypeError(`${label} contains a forbidden object key`);
    assertAllowedKeys(item, `${label}.${key}`, depth + 1);
  }
}

export function cloneJSONValue(value, label = 'value') {
  assertAllowedKeys(value, label);
  return JSON.parse(JSON.stringify(value));
}

function sortedJSON(value) {
  if (Array.isArray(value)) return value.map(sortedJSON);
  if (value && typeof value === 'object') {
    const result = {};
    for (const key of Object.keys(value).sort()) result[key] = sortedJSON(value[key]);
    return result;
  }
  return value;
}

export function canonicalJSON(value) {
  return JSON.stringify(sortedJSON(value));
}

function normalizeCollectionPolicy(collection, policy) {
  assertCollectionId(collection);
  assertPlainObject(policy, `collections.${collection}`);
  const keys = Array.isArray(policy.keys) ? policy.keys.map((key) => assertKeyId(key, `${collection} key`)) : [];
  const keyPrefixes = Array.isArray(policy.keyPrefixes)
    ? policy.keyPrefixes.map((prefix) => {
        assertKeyId(`${prefix}x`, `${collection} key prefix`);
        if (!prefix.endsWith(':')) throw new TypeError(`${collection} key prefixes must end with a colon`);
        return prefix;
      })
    : [];
  if (!keys.length && !keyPrefixes.length) throw new TypeError(`${collection} must approve at least one key or key prefix`);
  return Object.freeze({ keys: Object.freeze([...new Set(keys)]), keyPrefixes: Object.freeze([...new Set(keyPrefixes)]) });
}

function normalizeActor(actor, label = 'actor') {
  assertPlainObject(actor, label);
  const id = assertRelayId(actor.id, `${label}.id`);
  if (!Object.values(ROLES).includes(actor.role)) throw new TypeError(`${label}.role is unsupported`);
  if (!Array.isArray(actor.writeCollections)) throw new TypeError(`${label}.writeCollections must be an array`);
  const writeCollections = actor.writeCollections.map((collection) => collection === '*' ? '*' : assertCollectionId(collection, `${label} write collection`));
  return Object.freeze({ id, role: actor.role, writeCollections: Object.freeze([...new Set(writeCollections)]) });
}

function localDevelopmentHost(hostname) {
  return hostname === 'localhost' || hostname === '127.0.0.1' || hostname === '[::1]' || hostname === '::1';
}

function normalizeRelay(relay) {
  assertPlainObject(relay, 'relay');
  if (typeof relay.enabled !== 'boolean') throw new TypeError('relay.enabled must be explicit');
  if (!relay.enabled) return Object.freeze({ enabled: false, url: '', token: '', iceServers: Object.freeze([]) });

  const url = new URL(String(relay.url || ''));
  if (!['wss:', 'ws:'].includes(url.protocol)) throw new TypeError('relay.url must use wss or local-development ws');
  if (url.protocol === 'ws:' && !localDevelopmentHost(url.hostname)) throw new TypeError('relay.url must use wss outside local development');
  if (url.username || url.password || url.search || url.hash) throw new TypeError('relay.url must not contain credentials, query parameters, or a fragment');
  if (typeof relay.token !== 'string' || relay.token.length < 16 || relay.token.length > 8192) throw new TypeError('relay.token must be an explicit short-lived relay session token');
  const iceServers = normalizeIceServers(relay.iceServers || []);
  return Object.freeze({ enabled: true, url: url.href, token: relay.token, iceServers });
}

function normalizeIceServers(servers) {
  if (!Array.isArray(servers) || servers.length > 12) throw new TypeError('relay.iceServers must be an array of at most 12 entries');
  return Object.freeze(servers.map((server, index) => {
    assertPlainObject(server, `relay.iceServers[${index}]`);
    const urls = Array.isArray(server.urls) ? server.urls : [server.urls];
    if (!urls.length || urls.some((url) => typeof url !== 'string' || !/^(stun|stuns|turn|turns):/i.test(url) || url.length > 2048)) {
      throw new TypeError(`relay.iceServers[${index}].urls contains an unsupported ICE URL`);
    }
    const normalized = { urls: Object.freeze([...urls]) };
    if (server.username !== undefined) normalized.username = String(server.username);
    if (server.credential !== undefined) normalized.credential = String(server.credential);
    return Object.freeze(normalized);
  }));
}

export function validateRuntimeConfig(input, now = Date.now()) {
  assertPlainObject(input, 'runtime config');
  if (input.contractVersion !== RUNTIME_CONTRACT) throw new TypeError(`contractVersion must be ${RUNTIME_CONTRACT}`);
  const artifactId = assertRelayId(input.artifactId, 'artifactId');
  if (!Number.isSafeInteger(input.workspaceId) || input.workspaceId <= 0) throw new TypeError('workspaceId must be a positive integer');
  const peerId = assertRelayId(input.peerId, 'peerId');
  const actor = normalizeActor(input.actor);

  assertPlainObject(input.session, 'session');
  const session = Object.freeze({
    id: assertRelayId(input.session.id, 'session.id'),
    expiresAt: input.session.expiresAt,
  });
  if (!Number.isSafeInteger(session.expiresAt) || session.expiresAt <= now) throw new TypeError('session.expiresAt must be a future Unix millisecond timestamp');

  assertPlainObject(input.collections, 'collections');
  const collections = {};
  for (const [collection, policy] of Object.entries(input.collections)) collections[collection] = normalizeCollectionPolicy(collection, policy);
  if (!Object.keys(collections).length) throw new TypeError('at least one convergent collection must be configured');

  const peerPolicies = {};
  const rawPeerPolicies = input.peerPolicies === undefined ? {} : assertPlainObject(input.peerPolicies, 'peerPolicies');
  for (const [remotePeerId, policy] of Object.entries(rawPeerPolicies)) {
    assertRelayId(remotePeerId, 'peerPolicies peer ID');
    assertPlainObject(policy, `peerPolicies.${remotePeerId}`);
    const normalized = normalizeActor({ ...policy, id: policy.actorId }, `peerPolicies.${remotePeerId}`);
    peerPolicies[remotePeerId] = Object.freeze({ ...normalized, peerId: remotePeerId });
  }

  const maxOperationBytes = input.maxOperationBytes === undefined ? DEFAULT_MAX_OPERATION_BYTES : input.maxOperationBytes;
  if (!Number.isSafeInteger(maxOperationBytes) || maxOperationBytes < 1024 || maxOperationBytes > MAX_SYNC_BYTES) {
    throw new TypeError(`maxOperationBytes must be between 1024 and ${MAX_SYNC_BYTES}`);
  }

  return Object.freeze({
    contractVersion: RUNTIME_CONTRACT,
    artifactId,
    workspaceId: input.workspaceId,
    peerId,
    actor,
    session,
    relay: normalizeRelay(input.relay),
    collections: Object.freeze(collections),
    peerPolicies: Object.freeze(peerPolicies),
    maxOperationBytes,
  });
}

export function assertSessionActive(config, now = Date.now()) {
  if (config.session.expiresAt <= now) throw new Error('runtime session expired');
}

export function isApprovedKey(config, collection, key) {
  const policy = config.collections[collection];
  if (!policy || typeof key !== 'string' || !KEY_ID.test(key)) return false;
  return policy.keys.includes(key) || policy.keyPrefixes.some((prefix) => key.startsWith(prefix));
}

export function defaultWritePolicy(actor, collection) {
  if (!actor || actor.role === ROLES.VIEWER) return false;
  if (actor.role === ROLES.ARCHITECT) return true;
  return actor.role === ROLES.MAINTAINER && (actor.writeCollections.includes('*') || actor.writeCollections.includes(collection));
}

export function authorizeOperationPolicy(config, operation, { source = 'replay', peerId = operation.peerId } = {}) {
  let actor;
  if (operation.peerId === config.peerId) {
    if (operation.actorId !== config.actor.id) throw new Error('local peer operation actor does not match the injected capability');
    actor = config.actor;
  } else {
    actor = config.peerPolicies[operation.peerId];
    if (!actor || actor.id !== operation.actorId) throw new Error(`operation peer ${operation.peerId} has no matching injected capability policy`);
  }
  if (['relay', 'webrtc'].includes(source) && peerId !== operation.peerId) {
    throw new Error('transport peer does not match the operation peer binding');
  }
  if (!defaultWritePolicy(actor, operation.collection)) throw new Error(`${actor.role} cannot write ${operation.collection}`);
  return actor;
}

export function validateOperation(operation, config) {
  assertPlainObject(operation, 'operation');
  if (operation.contractVersion !== OPERATION_CONTRACT) throw new TypeError('operation contract version is unsupported');
  assertRelayId(operation.artifactId, 'operation.artifactId');
  if (operation.artifactId !== config.artifactId || operation.workspaceId !== config.workspaceId) throw new TypeError('operation scope does not match this runtime');
  assertKeyId(operation.opId, 'operation.opId');
  assertRelayId(operation.actorId, 'operation.actorId');
  assertRelayId(operation.peerId, 'operation.peerId');
  assertCollectionId(operation.collection);
  assertKeyId(operation.key);
  if (!isApprovedKey(config, operation.collection, operation.key)) throw new TypeError('operation targets an unapproved collection or key');
  if (!Number.isSafeInteger(operation.lamport) || operation.lamport <= 0) throw new TypeError('operation.lamport must be a positive safe integer');
  if (typeof operation.deleted !== 'boolean') throw new TypeError('operation.deleted must be boolean');
  if (operation.deleted) {
    if (operation.value !== null) throw new TypeError('deleted operations must carry a null value');
  } else {
    assertAllowedKeys(operation.value, 'operation.value');
  }
  const bytes = textEncoder.encode(canonicalJSON(operation)).byteLength;
  if (bytes > config.maxOperationBytes) throw new TypeError('operation exceeds the configured byte limit');
  return operation;
}

export function createOperation(config, { collection, key, value = null, deleted = false, lamport, opId = globalThis.crypto.randomUUID() }) {
  const operation = {
    contractVersion: OPERATION_CONTRACT,
    artifactId: config.artifactId,
    workspaceId: config.workspaceId,
    opId,
    actorId: config.actor.id,
    peerId: config.peerId,
    collection,
    key,
    lamport,
    deleted: Boolean(deleted),
    value: deleted ? null : cloneJSONValue(value),
  };
  validateOperation(operation, config);
  return Object.freeze(operation);
}

function compareString(left, right) {
  return left < right ? -1 : left > right ? 1 : 0;
}

export function compareOperations(left, right) {
  if (left.lamport !== right.lamport) return left.lamport < right.lamport ? -1 : 1;
  const actorOrder = compareString(left.actorId, right.actorId);
  return actorOrder || compareString(left.opId, right.opId);
}

export function operationSlot(operation) {
  return `${operation.collection}\u0000${operation.key}`;
}

export function mergeOperations(operations) {
  const winners = new Map();
  for (const operation of operations) {
    const slot = operationSlot(operation);
    const current = winners.get(slot);
    if (!current || compareOperations(current, operation) < 0) winners.set(slot, operation);
  }
  return [...winners.values()].sort((left, right) => {
    const collectionOrder = compareString(left.collection, right.collection);
    return collectionOrder || compareString(left.key, right.key);
  });
}

export function materializeOperations(operations) {
  const snapshot = {};
  for (const operation of mergeOperations(operations)) {
    if (operation.deleted) continue;
    if (!snapshot[operation.collection]) snapshot[operation.collection] = {};
    snapshot[operation.collection][operation.key] = cloneJSONValue(operation.value);
  }
  return snapshot;
}

export function maxLamport(operations) {
  return operations.reduce((maximum, operation) => Math.max(maximum, operation.lamport), 0);
}

export function chunkOperations(operations, maxBytes = MAX_SYNC_BYTES) {
  if (!Number.isSafeInteger(maxBytes) || maxBytes < 1024 || maxBytes > 56 * 1024) throw new TypeError('invalid sync chunk byte limit');
  const chunks = [];
  let current = [];
  for (const operation of operations) {
    const candidate = [...current, operation];
    if (textEncoder.encode(JSON.stringify({ contractVersion: OPERATION_CONTRACT, operations: candidate })).byteLength > maxBytes) {
      if (!current.length) throw new TypeError('one operation exceeds the sync chunk limit');
      chunks.push(current);
      current = [operation];
    } else {
      current = candidate;
    }
  }
  if (current.length) chunks.push(current);
  return chunks;
}

function cryptoAPI() {
  if (!globalThis.crypto?.subtle || !globalThis.crypto?.getRandomValues) throw new Error('Web Crypto is unavailable');
  return globalThis.crypto;
}

export async function importWorkspaceKey(keyMaterial) {
  if (keyMaterial?.type === 'secret' && keyMaterial.algorithm?.name === 'AES-GCM') {
    if (keyMaterial.extractable || keyMaterial.algorithm.length !== 256 || !keyMaterial.usages.includes('encrypt') || !keyMaterial.usages.includes('decrypt')) {
      throw new TypeError('workspace CryptoKey must be non-extractable AES-256-GCM with encrypt/decrypt usage');
    }
    return keyMaterial;
  }
  let bytes;
  if (typeof keyMaterial === 'string') bytes = base64URLToBytes(keyMaterial);
  else if (keyMaterial instanceof Uint8Array) bytes = new Uint8Array(keyMaterial);
  else if (keyMaterial instanceof ArrayBuffer) bytes = new Uint8Array(keyMaterial.slice(0));
  else throw new TypeError('workspace key must be a 32-byte buffer, base64url string, or AES-GCM CryptoKey');
  if (bytes.byteLength !== 32) throw new TypeError('workspace key must contain exactly 32 bytes');
  return cryptoAPI().subtle.importKey('raw', bytes, { name: 'AES-GCM' }, false, ['encrypt', 'decrypt']);
}

export function generateWorkspaceKeyMaterial() {
  return cryptoAPI().getRandomValues(new Uint8Array(32));
}

function syncAAD(artifactId, senderPeerId) {
  return textEncoder.encode(canonicalJSON({ artifactId, contractVersion: SYNC_CONTRACT, senderPeerId }));
}

export async function encryptOperations(workspaceKey, { artifactId, senderPeerId }, operations) {
  assertRelayId(artifactId, 'artifactId');
  assertRelayId(senderPeerId, 'senderPeerId');
  if (!Array.isArray(operations) || !operations.length) throw new TypeError('at least one operation is required for encryption');
  const key = await importWorkspaceKey(workspaceKey);
  const iv = cryptoAPI().getRandomValues(new Uint8Array(12));
  const plaintext = textEncoder.encode(JSON.stringify({ contractVersion: OPERATION_CONTRACT, operations }));
  const ciphertext = await cryptoAPI().subtle.encrypt({ name: 'AES-GCM', iv, additionalData: syncAAD(artifactId, senderPeerId), tagLength: 128 }, key, plaintext);
  return Object.freeze({
    contractVersion: SYNC_CONTRACT,
    algorithm: 'A256GCM',
    artifactId,
    senderPeerId,
    iv: bytesToBase64URL(iv),
    ciphertext: bytesToBase64URL(new Uint8Array(ciphertext)),
  });
}

export async function decryptOperations(workspaceKey, envelope, { artifactId, senderPeerId }) {
  assertPlainObject(envelope, 'encrypted sync envelope');
  if (envelope.contractVersion !== SYNC_CONTRACT || envelope.algorithm !== 'A256GCM') throw new TypeError('encrypted sync envelope is unsupported');
  if (envelope.artifactId !== artifactId || envelope.senderPeerId !== senderPeerId) throw new TypeError('encrypted sync envelope scope mismatch');
  const iv = base64URLToBytes(envelope.iv);
  const ciphertext = base64URLToBytes(envelope.ciphertext);
  if (iv.byteLength !== 12 || ciphertext.byteLength < 17 || ciphertext.byteLength > 64 * 1024) throw new TypeError('encrypted sync envelope has invalid bounds');
  const key = await importWorkspaceKey(workspaceKey);
  let plaintext;
  try {
    plaintext = await cryptoAPI().subtle.decrypt({ name: 'AES-GCM', iv, additionalData: syncAAD(artifactId, senderPeerId), tagLength: 128 }, key, ciphertext);
  } catch {
    throw new Error('encrypted sync envelope failed authentication');
  }
  const decoded = JSON.parse(textDecoder.decode(plaintext));
  if (decoded.contractVersion !== OPERATION_CONTRACT || !Array.isArray(decoded.operations)) throw new TypeError('decrypted operation batch is unsupported');
  return decoded.operations;
}

export function bytesToBase64URL(bytes) {
  const value = bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes);
  let binary = '';
  for (let index = 0; index < value.length; index += 0x8000) binary += String.fromCharCode(...value.subarray(index, index + 0x8000));
  const encoded = typeof btoa === 'function' ? btoa(binary) : globalThis.Buffer.from(binary, 'binary').toString('base64');
  return encoded.replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/u, '');
}

export function base64URLToBytes(value) {
  if (typeof value !== 'string' || !/^[A-Za-z0-9_-]+$/u.test(value)) throw new TypeError('value must be unpadded base64url');
  const padded = value.replace(/-/g, '+').replace(/_/g, '/') + '='.repeat((4 - value.length % 4) % 4);
  const binary = typeof atob === 'function' ? atob(padded) : globalThis.Buffer.from(padded, 'base64').toString('binary');
  return Uint8Array.from(binary, (character) => character.charCodeAt(0));
}

export function assertModuleId(value) {
  if (typeof value !== 'string' || !MODULE_ID.test(value)) throw new TypeError('moduleId is invalid');
  return value;
}
