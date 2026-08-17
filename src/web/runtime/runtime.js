import {
  MODULE_ADAPTER_CONTRACT,
  authorizeOperationPolicy,
  assertModuleId,
  assertSessionActive,
  cloneJSONValue,
  createOperation,
  importWorkspaceKey,
  isApprovedKey,
  materializeOperations,
  maxLamport,
  validateOperation,
  validateRuntimeConfig,
} from './core.js';
import { IndexedDBOperationStore } from './store.js';
import { BroadcastSync, RelayPeerNetwork } from './transports.js';

export const RUNTIME_EVENTS = Object.freeze({
  READY: 'taawun.runtime.ready.v1',
  CHANGED: 'taawun.runtime.changed.v1',
  TRANSPORT: 'taawun.runtime.transport.v1',
  ERROR: 'taawun.runtime.error.v1',
});

function runtimeEvent(name, detail) {
  return new CustomEvent(name, { detail });
}

export class TaawunRuntime extends EventTarget {
  #config;
  #workspaceKey;
  #store;
  #operations = new Map();
  #lamport = 0;
  #started = false;
  #authorizeWrite;
  #broadcast = null;
  #network = null;

  static async create(configInput, { workspaceKey, authorizeWrite, indexedDBFactory } = {}) {
    const config = validateRuntimeConfig(configInput);
    const importedKey = await importWorkspaceKey(workspaceKey);
    const store = new IndexedDBOperationStore(config, indexedDBFactory);
    return new TaawunRuntime(config, importedKey, store, authorizeWrite);
  }

  constructor(config, workspaceKey, store, authorizeWrite) {
    super();
    this.#config = config;
    this.#workspaceKey = workspaceKey;
    this.#store = store;
    this.#authorizeWrite = typeof authorizeWrite === 'function' ? authorizeWrite : null;
  }

  get started() {
    return this.#started;
  }

  get artifactId() {
    return this.#config.artifactId;
  }

  async start({ connectRelay = false } = {}) {
    if (this.#started) return this;
    assertSessionActive(this.#config);
    await this.#store.open();
    const stored = await this.#store.list();
    for (const operation of stored) {
      validateOperation(operation, this.#config);
      await this.#assertAuthorized(operation, { source: 'replay', peerId: operation.peerId });
      this.#operations.set(operation.opId, operation);
    }
    this.#lamport = maxLamport(stored);

    const callbacks = {
      artifactId: this.#config.artifactId,
      peerId: this.#config.peerId,
      getOperations: () => this.operations(),
      onOperations: (operations, metadata) => this.#ingest(operations, metadata),
      onError: (error) => this.#emitError(error),
    };
    this.#broadcast = new BroadcastSync(callbacks);
    this.#broadcast.start();
    this.#network = new RelayPeerNetwork({
      ...callbacks,
      config: this.#config,
      workspaceKey: this.#workspaceKey,
      onState: (state) => this.dispatchEvent(runtimeEvent(RUNTIME_EVENTS.TRANSPORT, Object.freeze({ ...state }))),
    });
    this.#started = true;
    if (connectRelay) await this.connectRelay();
    this.dispatchEvent(runtimeEvent(RUNTIME_EVENTS.READY, Object.freeze({ artifactId: this.#config.artifactId, operationCount: this.#operations.size })));
    return this;
  }

  async connectRelay() {
    this.#assertStarted();
    return this.#network.connect();
  }

  async set(collection, key, value) {
    return this.#write(collection, key, value, false);
  }

  async delete(collection, key) {
    return this.#write(collection, key, null, true);
  }

  async #write(collection, key, value, deleted) {
    this.#assertStarted();
    assertSessionActive(this.#config);
    const operation = createOperation(this.#config, { collection, key, value, deleted, lamport: this.#lamport + 1 });
    await this.#assertAuthorized(operation, { source: 'local', peerId: this.#config.peerId });
    await this.#store.append(operation);
    this.#lamport = operation.lamport;
    this.#operations.set(operation.opId, operation);
    await this.#publish([operation], 'local');
    this.#emitChanged([operation], 'local');
    return operation;
  }

  async #ingest(operations, metadata) {
    this.#assertStarted();
    assertSessionActive(this.#config);
    if (!Array.isArray(operations) || operations.length > 512) throw new TypeError('incoming operation batch has invalid bounds');
    const accepted = [];
    for (const operation of operations) {
      validateOperation(operation, this.#config);
      await this.#assertAuthorized(operation, metadata);
    }
    for (const operation of operations) {
      const knownInMemory = this.#operations.has(operation.opId);
      const inserted = await this.#store.append(operation);
      this.#lamport = Math.max(this.#lamport, operation.lamport);
      if (inserted || !knownInMemory) {
        this.#operations.set(operation.opId, operation);
        accepted.push(operation);
      }
    }
    if (!accepted.length) return 0;
    if (metadata.source === 'broadcast') await this.#network.publish(accepted);
    else this.#broadcast.publish(accepted);
    this.#emitChanged(accepted, metadata.source);
    return accepted.length;
  }

  async #assertAuthorized(operation, metadata) {
    const actor = authorizeOperationPolicy(this.#config, operation, metadata);
    if (this.#authorizeWrite) {
      const allowed = await this.#authorizeWrite(Object.freeze({ actor, operation, source: metadata.source, transportPeerId: metadata.peerId, artifactId: this.#config.artifactId, workspaceId: this.#config.workspaceId }));
      if (allowed !== true) throw new Error('runtime write policy hook denied the operation');
    }
  }

  async #publish(operations, source) {
    if (source !== 'broadcast') this.#broadcast.publish(operations);
    if (!['relay', 'webrtc'].includes(source)) await this.#network.publish(operations);
  }

  operations() {
    return [...this.#operations.values()].sort((left, right) => {
      if (left.lamport !== right.lamport) return left.lamport - right.lamport;
      if (left.actorId !== right.actorId) return left.actorId < right.actorId ? -1 : 1;
      return left.opId < right.opId ? -1 : left.opId > right.opId ? 1 : 0;
    }).map((operation) => cloneJSONValue(operation));
  }

  snapshot() {
    return cloneJSONValue(materializeOperations(this.#operations.values()));
  }

  get(collection, key) {
    if (!isApprovedKey(this.#config, collection, key)) throw new TypeError('read targets an unapproved collection or key');
    const snapshot = materializeOperations(this.#operations.values());
    const value = snapshot[collection]?.[key];
    return value === undefined ? undefined : cloneJSONValue(value);
  }

  createModuleAdapter(moduleId, { collections } = {}) {
    assertModuleId(moduleId);
    if (!Array.isArray(collections) || !collections.length) throw new TypeError('module adapter must declare at least one collection');
    const approved = Object.freeze(collections.map((collection) => {
      if (!this.#config.collections[collection]) throw new TypeError(`module adapter collection ${collection} is not configured`);
      return collection;
    }));
    const runtime = this;
    return Object.freeze({
      contractVersion: MODULE_ADAPTER_CONTRACT,
      moduleId,
      collections: approved,
      get(collection, key) {
        assertModuleCollection(approved, collection);
        return runtime.get(collection, key);
      },
      set(collection, key, value) {
        assertModuleCollection(approved, collection);
        return runtime.set(collection, key, value);
      },
      delete(collection, key) {
        assertModuleCollection(approved, collection);
        return runtime.delete(collection, key);
      },
      snapshot() {
        const complete = runtime.snapshot();
        const scoped = {};
        for (const collection of approved) if (complete[collection]) scoped[collection] = complete[collection];
        return scoped;
      },
      subscribe(listener) {
        if (typeof listener !== 'function') throw new TypeError('module listener must be a function');
        const handler = (event) => {
          const relevant = event.detail.operations.filter((operation) => approved.includes(operation.collection));
          if (relevant.length) listener(Object.freeze({ ...event.detail, operations: relevant }));
        };
        runtime.addEventListener(RUNTIME_EVENTS.CHANGED, handler);
        return () => runtime.removeEventListener(RUNTIME_EVENTS.CHANGED, handler);
      },
    });
  }

  #emitChanged(operations, source) {
    this.dispatchEvent(runtimeEvent(RUNTIME_EVENTS.CHANGED, Object.freeze({
      contractVersion: RUNTIME_EVENTS.CHANGED,
      artifactId: this.#config.artifactId,
      source,
      operations: operations.map((operation) => cloneJSONValue(operation)),
      snapshot: this.snapshot(),
    })));
  }

  #emitError(error) {
    const normalized = error instanceof Error ? error : new Error(String(error));
    this.dispatchEvent(runtimeEvent(RUNTIME_EVENTS.ERROR, Object.freeze({ message: normalized.message })));
  }

  #assertStarted() {
    if (!this.#started) throw new Error('runtime has not started');
  }

  close() {
    this.#broadcast?.close();
    this.#network?.close();
    this.#store.close();
    this.#broadcast = null;
    this.#network = null;
    this.#workspaceKey = null;
    this.#operations.clear();
    this.#started = false;
  }
}

function assertModuleCollection(approved, collection) {
  if (!approved.includes(collection)) throw new Error(`module adapter cannot access ${collection}`);
}
