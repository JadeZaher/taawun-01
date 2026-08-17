import { canonicalJSON, compareOperations, validateOperation } from './core.js';

const DATABASE_VERSION = 1;
const OPERATION_STORE = 'operations';

function requestResult(request) {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error || new Error('IndexedDB request failed'));
  });
}

export class IndexedDBOperationStore {
  #config;
  #factory;
  #database = null;

  constructor(config, indexedDBFactory = globalThis.indexedDB) {
    if (!indexedDBFactory?.open) throw new Error('IndexedDB is unavailable');
    this.#config = config;
    this.#factory = indexedDBFactory;
  }

  get databaseName() {
    return `taawun-runtime-${this.#config.workspaceId}-${this.#config.artifactId}`;
  }

  async open() {
    if (this.#database) return this;
    const request = this.#factory.open(this.databaseName, DATABASE_VERSION);
    request.onupgradeneeded = () => {
      const database = request.result;
      if (!database.objectStoreNames.contains(OPERATION_STORE)) {
        const store = database.createObjectStore(OPERATION_STORE, { keyPath: 'opId' });
        store.createIndex('lamport', 'lamport', { unique: false });
        store.createIndex('collection', 'collection', { unique: false });
      }
    };
    this.#database = await requestResult(request);
    this.#database.onversionchange = () => this.close();
    return this;
  }

  async append(operation) {
    validateOperation(operation, this.#config);
    await this.open();
    return new Promise((resolve, reject) => {
      const transaction = this.#database.transaction(OPERATION_STORE, 'readwrite');
      const store = transaction.objectStore(OPERATION_STORE);
      let inserted = false;
      let collision = null;
      const lookup = store.get(operation.opId);
      lookup.onsuccess = () => {
        const existing = lookup.result;
        if (existing) {
          if (canonicalJSON(existing) !== canonicalJSON(operation)) {
            collision = new Error('operation identifier collision');
            transaction.abort();
          }
          return;
        }
        store.add(operation);
        inserted = true;
      };
      lookup.onerror = () => {
        collision = lookup.error || new Error('operation lookup failed');
        transaction.abort();
      };
      transaction.oncomplete = () => resolve(inserted);
      transaction.onabort = () => reject(collision || transaction.error || new Error('operation append aborted'));
      transaction.onerror = () => reject(collision || transaction.error || new Error('operation append failed'));
    });
  }

  async appendMany(operations) {
    let inserted = 0;
    for (const operation of operations) {
      if (await this.append(operation)) inserted += 1;
    }
    return inserted;
  }

  async list() {
    await this.open();
    const transaction = this.#database.transaction(OPERATION_STORE, 'readonly');
    const operations = await requestResult(transaction.objectStore(OPERATION_STORE).getAll());
    return operations.sort(compareOperations);
  }

  close() {
    this.#database?.close();
    this.#database = null;
  }
}
