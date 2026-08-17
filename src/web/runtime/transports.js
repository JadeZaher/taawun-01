import {
  RUNTIME_CONTRACT,
  assertRelayId,
  assertSessionActive,
  chunkOperations,
  decryptOperations,
  encryptOperations,
} from './core.js';

const RELAY_MESSAGE_TYPES = new Set(['join', 'offer', 'answer', 'ice-candidate', 'crdt-sync', 'leave']);
const DATA_CHANNEL_LABEL = 'taawun-crdt-v1';
export const RELAY_WEBSOCKET_SUBPROTOCOL = 'taawun-relay-v1';

export class BroadcastSync {
  #artifactId;
  #peerId;
  #channel = null;
  #getOperations;
  #onOperations;
  #onError;

  constructor({ artifactId, peerId, getOperations, onOperations, onError = () => {} }) {
    this.#artifactId = assertRelayId(artifactId, 'artifactId');
    this.#peerId = assertRelayId(peerId, 'peerId');
    this.#getOperations = getOperations;
    this.#onOperations = onOperations;
    this.#onError = onError;
  }

  start() {
    if (this.#channel || typeof BroadcastChannel !== 'function') return false;
    this.#channel = new BroadcastChannel(`taawun-runtime-${this.#artifactId}`);
    this.#channel.onmessage = (event) => this.#handle(event.data).catch(this.#onError);
    this.#post('hello');
    return true;
  }

  async #handle(message) {
    if (!message || message.contractVersion !== RUNTIME_CONTRACT || message.artifactId !== this.#artifactId || message.peerId === this.#peerId) return;
    assertRelayId(message.peerId, 'broadcast peerId');
    if (message.targetPeerId && message.targetPeerId !== this.#peerId) return;
    if (message.type === 'hello') {
      const operations = await this.#getOperations();
      if (operations.length) this.#post('operations', operations, message.peerId);
      return;
    }
    if (message.type === 'operations' && Array.isArray(message.operations)) {
      await this.#onOperations(message.operations, { source: 'broadcast', peerId: message.peerId });
    }
  }

  publish(operations, targetPeerId = '') {
    if (!this.#channel || !operations.length) return;
    this.#post('operations', operations, targetPeerId);
  }

  #post(type, operations, targetPeerId = '') {
    this.#channel?.postMessage({
      contractVersion: RUNTIME_CONTRACT,
      type,
      artifactId: this.#artifactId,
      peerId: this.#peerId,
      targetPeerId,
      ...(operations ? { operations } : {}),
    });
  }

  close() {
    this.#channel?.close();
    this.#channel = null;
  }
}

export class RelayPeerNetwork {
  #config;
  #workspaceKey;
  #getOperations;
  #onOperations;
  #onState;
  #onError;
  #socket = null;
  #peers = new Map();
  #knownPeers = new Set();
  #closedByUser = false;
  #ticketConsumed = false;
  #reconnectAttempts = 0;
  #reconnectTimer = 0;

  constructor({ config, workspaceKey, getOperations, onOperations, onState = () => {}, onError = () => {} }) {
    this.#config = config;
    this.#workspaceKey = workspaceKey;
    this.#getOperations = getOperations;
    this.#onOperations = onOperations;
    this.#onState = onState;
    this.#onError = onError;
  }

  async connect() {
    if (!this.#config.relay.enabled) return false;
    assertSessionActive(this.#config);
    if (this.#socket && [WebSocket.CONNECTING, WebSocket.OPEN].includes(this.#socket.readyState)) return true;
    if (typeof WebSocket !== 'function') throw new Error('WebSocket is unavailable');
    this.#closedByUser = false;
    await this.#openSocket();
    return true;
  }

  async #openSocket() {
    if (this.#ticketConsumed) throw new Error('relay session ticket was consumed; request a fresh runtime config before reconnecting');
    const relayURL = new URL(this.#config.relay.url);
    relayURL.searchParams.set('artifactId', this.#config.artifactId);
    relayURL.searchParams.set('peerId', this.#config.peerId);
    this.#onState({ state: 'connecting', transport: 'relay' });

    await new Promise((resolve, reject) => {
      this.#ticketConsumed = true;
      const socket = new WebSocket(relayURL.href, [RELAY_WEBSOCKET_SUBPROTOCOL, this.#config.relay.token]);
      this.#socket = socket;
      let opened = false;
      socket.onopen = () => {
        opened = true;
        this.#reconnectAttempts = 0;
        this.#onState({ state: 'connected', transport: 'relay' });
        this.#sendRelay('join', { contractVersion: RUNTIME_CONTRACT });
        resolve();
      };
      socket.onmessage = (event) => this.#handleRelayMessage(event.data).catch(this.#onError);
      socket.onerror = () => {
        if (!opened) reject(new Error('relay WebSocket connection failed'));
        this.#onState({ state: 'error', transport: 'relay' });
      };
      socket.onclose = () => {
        this.#socket = null;
        this.#onState({ state: 'disconnected', transport: 'relay' });
        if (!opened) reject(new Error('relay WebSocket closed before connecting'));
        if (!this.#closedByUser && this.#config.session.expiresAt > Date.now()) this.#scheduleReconnect();
      };
    });
  }

  #scheduleReconnect() {
    if (this.#ticketConsumed) {
      this.#onError(new Error('relay session ticket was consumed; request a fresh runtime config before reconnecting'));
      return;
    }
    clearTimeout(this.#reconnectTimer);
    const delay = Math.min(30_000, 750 * (2 ** this.#reconnectAttempts));
    this.#reconnectAttempts += 1;
    this.#reconnectTimer = setTimeout(() => this.#openSocket().catch(this.#onError), delay);
  }

  async #handleRelayMessage(raw) {
    if (typeof raw !== 'string' || raw.length > 64 * 1024) return;
    const message = JSON.parse(raw);
    if (!message || !RELAY_MESSAGE_TYPES.has(message.type) || message.artifactId !== this.#config.artifactId) return;
    const senderPeerId = assertRelayId(message.peerId, 'relay sender peerId');
    if (senderPeerId === this.#config.peerId) return;
    if (message.targetId && message.targetId !== this.#config.peerId) return;
    const firstSeen = !this.#knownPeers.has(senderPeerId);
    this.#knownPeers.add(senderPeerId);

    switch (message.type) {
      case 'join':
        await this.#sendAllToPeer(senderPeerId);
        await this.#createOffer(senderPeerId);
        break;
      case 'offer':
        await this.#acceptOffer(senderPeerId, message.payload);
        break;
      case 'answer':
        await this.#acceptAnswer(senderPeerId, message.payload);
        break;
      case 'ice-candidate':
        await this.#acceptCandidate(senderPeerId, message.payload);
        break;
      case 'crdt-sync': {
        const operations = await decryptOperations(this.#workspaceKey, message.payload, { artifactId: this.#config.artifactId, senderPeerId });
        await this.#onOperations(operations, { source: 'relay', peerId: senderPeerId });
        if (firstSeen) await this.#sendAllToPeer(senderPeerId);
        break;
      }
      case 'leave':
        this.#closePeer(senderPeerId);
        this.#knownPeers.delete(senderPeerId);
        break;
      default:
        break;
    }
  }

  async publish(operations) {
    if (!operations.length || !this.#knownPeers.size) return;
    for (const peerId of this.#knownPeers) await this.#sendOperationsToPeer(operations, peerId);
  }

  async #sendAllToPeer(peerId) {
    const operations = await this.#getOperations();
    if (operations.length) await this.#sendOperationsToPeer(operations, peerId);
  }

  async #sendOperationsToPeer(operations, peerId) {
    for (const chunk of chunkOperations(operations)) {
      const envelope = await encryptOperations(this.#workspaceKey, { artifactId: this.#config.artifactId, senderPeerId: this.#config.peerId }, chunk);
      const channel = this.#peers.get(peerId)?.channel;
      if (channel?.readyState === 'open' && channel.bufferedAmount < 512 * 1024) channel.send(JSON.stringify(envelope));
      else this.#sendRelay('crdt-sync', envelope, peerId);
    }
  }

  #sendRelay(type, payload, targetId = '') {
    if (!this.#socket || this.#socket.readyState !== WebSocket.OPEN) return false;
    this.#socket.send(JSON.stringify({
      type,
      artifactId: this.#config.artifactId,
      peerId: this.#config.peerId,
      ...(targetId ? { targetId } : {}),
      ...(payload === undefined ? {} : { payload }),
    }));
    return true;
  }

  #ensurePeer(peerId) {
    let peer = this.#peers.get(peerId);
    if (peer) return peer;
    if (typeof RTCPeerConnection !== 'function') return null;
    const connection = new RTCPeerConnection({ iceServers: this.#config.relay.iceServers });
    peer = { connection, channel: null, pendingCandidates: [] };
    this.#peers.set(peerId, peer);
    connection.onicecandidate = (event) => {
      if (event.candidate) this.#sendRelay('ice-candidate', event.candidate.toJSON ? event.candidate.toJSON() : event.candidate, peerId);
    };
    connection.ondatachannel = (event) => this.#attachDataChannel(peerId, peer, event.channel);
    connection.onconnectionstatechange = () => {
      this.#onState({ state: connection.connectionState, transport: 'webrtc', peerId });
      if (['failed', 'closed'].includes(connection.connectionState)) this.#closePeer(peerId);
    };
    return peer;
  }

  async #createOffer(peerId) {
    const peer = this.#ensurePeer(peerId);
    if (!peer || peer.connection.signalingState !== 'stable') return;
    if (!peer.channel) this.#attachDataChannel(peerId, peer, peer.connection.createDataChannel(DATA_CHANNEL_LABEL, { ordered: true }));
    const offer = await peer.connection.createOffer();
    await peer.connection.setLocalDescription(offer);
    this.#sendRelay('offer', { type: offer.type, sdp: offer.sdp }, peerId);
  }

  async #acceptOffer(peerId, payload) {
    if (!validDescription(payload, 'offer')) return;
    const peer = this.#ensurePeer(peerId);
    if (!peer) return;
    await peer.connection.setRemoteDescription(payload);
    await this.#flushCandidates(peer);
    const answer = await peer.connection.createAnswer();
    await peer.connection.setLocalDescription(answer);
    this.#sendRelay('answer', { type: answer.type, sdp: answer.sdp }, peerId);
  }

  async #acceptAnswer(peerId, payload) {
    if (!validDescription(payload, 'answer')) return;
    const peer = this.#ensurePeer(peerId);
    if (!peer) return;
    await peer.connection.setRemoteDescription(payload);
    await this.#flushCandidates(peer);
  }

  async #acceptCandidate(peerId, payload) {
    if (!validCandidate(payload)) return;
    const peer = this.#ensurePeer(peerId);
    if (!peer) return;
    if (!peer.connection.remoteDescription) peer.pendingCandidates.push(payload);
    else await peer.connection.addIceCandidate(payload);
  }

  async #flushCandidates(peer) {
    for (const candidate of peer.pendingCandidates.splice(0)) await peer.connection.addIceCandidate(candidate);
  }

  #attachDataChannel(peerId, peer, channel) {
    if (channel.label !== DATA_CHANNEL_LABEL) {
      channel.close();
      return;
    }
    peer.channel = channel;
    channel.onopen = () => {
      this.#onState({ state: 'connected', transport: 'webrtc', peerId });
      this.#sendAllToPeer(peerId).catch(this.#onError);
    };
    channel.onmessage = (event) => {
      this.#handleDataChannelMessage(peerId, event.data).catch(this.#onError);
    };
    channel.onerror = () => this.#onState({ state: 'error', transport: 'webrtc', peerId });
    channel.onclose = () => this.#onState({ state: 'disconnected', transport: 'webrtc', peerId });
  }

  async #handleDataChannelMessage(peerId, raw) {
    if (typeof raw !== 'string' || raw.length > 64 * 1024) return;
    const envelope = JSON.parse(raw);
    const operations = await decryptOperations(this.#workspaceKey, envelope, { artifactId: this.#config.artifactId, senderPeerId: peerId });
    await this.#onOperations(operations, { source: 'webrtc', peerId });
  }

  #closePeer(peerId) {
    const peer = this.#peers.get(peerId);
    peer?.channel?.close();
    peer?.connection?.close();
    this.#peers.delete(peerId);
  }

  close() {
    this.#closedByUser = true;
    clearTimeout(this.#reconnectTimer);
    this.#sendRelay('leave', { contractVersion: RUNTIME_CONTRACT });
    this.#socket?.close(1000, 'runtime closed');
    this.#socket = null;
    for (const peerId of this.#peers.keys()) this.#closePeer(peerId);
    this.#knownPeers.clear();
    this.#workspaceKey = null;
  }
}

function validDescription(payload, expectedType) {
  return Boolean(payload && payload.type === expectedType && typeof payload.sdp === 'string' && payload.sdp.length > 0 && payload.sdp.length <= 56 * 1024);
}

function validCandidate(payload) {
  return Boolean(payload && typeof payload === 'object' && typeof payload.candidate === 'string' && payload.candidate.length <= 4096);
}
