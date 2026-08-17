package financial

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	FederationProtocolVersion = "taawun-azoa-federation/v1"
	maxFederationPayloadBytes = 64 * 1024
)

var (
	ErrInvalidFederationEnvelope = errors.New("invalid federation envelope")
	ErrExpiredFederationEnvelope = errors.New("expired federation envelope")
	ErrFederationReplay          = errors.New("federation envelope was already processed")
)

// FederationMessageType separates a cross-node intent from a non-settling receipt.
type FederationMessageType string

const (
	FederationSettlementIntent  FederationMessageType = "SETTLEMENT_INTENT"
	FederationSettlementReceipt FederationMessageType = "SETTLEMENT_RECEIPT"
)

// FederationEnvelope is a signed transport envelope, not evidence of settled funds.
type FederationEnvelope struct {
	ProtocolVersion string                `json:"protocol_version"`
	MessageID       string                `json:"message_id"`
	Type            FederationMessageType `json:"type"`
	SourceNodeID    string                `json:"source_node_id"`
	TargetNodeID    string                `json:"target_node_id"`
	QuestID         string                `json:"quest_id"`
	SettlementID    string                `json:"settlement_id"`
	Payload         []byte                `json:"payload"`
	CreatedAt       int64                 `json:"created_at"`
	ExpiresAt       int64                 `json:"expires_at"`
	Signature       string                `json:"signature"`
}

// FederationSigner signs outbound envelopes for a single independently operated node.
type FederationSigner struct {
	nodeID     string
	privateKey ed25519.PrivateKey
}

// NewFederationSigner creates a signer from a node-owned Ed25519 private key.
func NewFederationSigner(nodeID string, privateKey ed25519.PrivateKey) (*FederationSigner, error) {
	if !validFederationID(nodeID) {
		return nil, fmt.Errorf("%w: invalid node ID", ErrInvalidFederationEnvelope)
	}
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("%w: invalid private key", ErrInvalidFederationEnvelope)
	}
	return &FederationSigner{nodeID: nodeID, privateKey: append(ed25519.PrivateKey(nil), privateKey...)}, nil
}

// GenerateFederationSigner creates a node keypair for local development or bootstrap.
// Production nodes must persist their own private key in a managed secret store.
func GenerateFederationSigner(nodeID string) (*FederationSigner, ed25519.PublicKey, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate federation key: %w", err)
	}
	signer, err := NewFederationSigner(nodeID, privateKey)
	if err != nil {
		return nil, nil, err
	}
	return signer, publicKey, nil
}

// SignSettlementIntent creates a short-lived cross-node settlement intent.
// Receiving a valid intent authorizes no fund movement by itself.
func (s *FederationSigner) SignSettlementIntent(targetNodeID, questID, settlementID string, payload []byte, expiresAt time.Time) (*FederationEnvelope, error) {
	return s.sign(FederationSettlementIntent, targetNodeID, questID, settlementID, payload, expiresAt)
}

// SignSettlementReceipt creates a signed acknowledgement for an already handled intent.
func (s *FederationSigner) SignSettlementReceipt(targetNodeID, questID, settlementID string, payload []byte, expiresAt time.Time) (*FederationEnvelope, error) {
	return s.sign(FederationSettlementReceipt, targetNodeID, questID, settlementID, payload, expiresAt)
}

func (s *FederationSigner) sign(messageType FederationMessageType, targetNodeID, questID, settlementID string, payload []byte, expiresAt time.Time) (*FederationEnvelope, error) {
	if s == nil || len(s.privateKey) != ed25519.PrivateKeySize || !validFederationID(s.nodeID) {
		return nil, fmt.Errorf("%w: signer is not configured", ErrInvalidFederationEnvelope)
	}
	if !validFederationID(targetNodeID) || !validFederationID(questID) || !validFederationID(settlementID) {
		return nil, fmt.Errorf("%w: target node, quest, and settlement IDs are required", ErrInvalidFederationEnvelope)
	}
	if !validFederationMessageType(messageType) {
		return nil, fmt.Errorf("%w: unsupported message type", ErrInvalidFederationEnvelope)
	}
	if len(payload) > maxFederationPayloadBytes {
		return nil, fmt.Errorf("%w: payload exceeds %d bytes", ErrInvalidFederationEnvelope, maxFederationPayloadBytes)
	}
	now := time.Now()
	if expiresAt.UnixMilli() <= now.UnixMilli() {
		return nil, fmt.Errorf("%w: expiry must be in the future", ErrExpiredFederationEnvelope)
	}
	messageID, err := federationMessageID()
	if err != nil {
		return nil, err
	}
	envelope := &FederationEnvelope{
		ProtocolVersion: FederationProtocolVersion,
		MessageID:       messageID,
		Type:            messageType,
		SourceNodeID:    s.nodeID,
		TargetNodeID:    targetNodeID,
		QuestID:         questID,
		SettlementID:    settlementID,
		Payload:         append([]byte(nil), payload...),
		CreatedAt:       now.UnixMilli(),
		ExpiresAt:       expiresAt.UnixMilli(),
	}
	canonical, err := canonicalFederationEnvelope(envelope)
	if err != nil {
		return nil, err
	}
	envelope.Signature = base64.RawURLEncoding.EncodeToString(ed25519.Sign(s.privateKey, canonical))
	return envelope, nil
}

// VerifyFederationEnvelope verifies an expected node's signature and transport invariants.
func VerifyFederationEnvelope(envelope *FederationEnvelope, sourcePublicKey ed25519.PublicKey) error {
	if err := validateFederationEnvelope(envelope); err != nil {
		return err
	}
	if len(sourcePublicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("%w: invalid source public key", ErrInvalidFederationEnvelope)
	}
	signature, err := base64.RawURLEncoding.DecodeString(envelope.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("%w: malformed signature", ErrInvalidFederationEnvelope)
	}
	canonical, err := canonicalFederationEnvelope(envelope)
	if err != nil {
		return err
	}
	if !ed25519.Verify(sourcePublicKey, canonical, signature) {
		return fmt.Errorf("%w: signature verification failed", ErrInvalidFederationEnvelope)
	}
	return nil
}

// FederationInbox verifies and accepts each signed envelope once until it expires.
// It is intentionally in-memory; real settlement requires a durable idempotency store.
type FederationInbox struct {
	localNodeID string
	mu          sync.Mutex
	processed   map[string]int64
}

// NewFederationInbox creates an inbox that accepts envelopes addressed only to localNodeID.
func NewFederationInbox(localNodeID string) (*FederationInbox, error) {
	if !validFederationID(localNodeID) {
		return nil, fmt.Errorf("%w: invalid local node ID", ErrInvalidFederationEnvelope)
	}
	return &FederationInbox{localNodeID: localNodeID, processed: make(map[string]int64)}, nil
}

func (i *FederationInbox) Accept(envelope *FederationEnvelope, sourcePublicKey ed25519.PublicKey) error {
	if i == nil {
		return fmt.Errorf("%w: inbox is nil", ErrInvalidFederationEnvelope)
	}
	if err := VerifyFederationEnvelope(envelope, sourcePublicKey); err != nil {
		return err
	}
	if envelope.TargetNodeID != i.localNodeID {
		return fmt.Errorf("%w: envelope is addressed to another node", ErrInvalidFederationEnvelope)
	}
	key := envelope.SourceNodeID + ":" + envelope.MessageID
	now := time.Now().UnixMilli()
	i.mu.Lock()
	defer i.mu.Unlock()
	for processedKey, expiry := range i.processed {
		if expiry <= now {
			delete(i.processed, processedKey)
		}
	}
	if _, exists := i.processed[key]; exists {
		return ErrFederationReplay
	}
	i.processed[key] = envelope.ExpiresAt
	return nil
}

func validateFederationEnvelope(envelope *FederationEnvelope) error {
	if envelope == nil || envelope.ProtocolVersion != FederationProtocolVersion || !validFederationMessageType(envelope.Type) {
		return ErrInvalidFederationEnvelope
	}
	if !validFederationID(envelope.MessageID) || !validFederationID(envelope.SourceNodeID) || !validFederationID(envelope.TargetNodeID) || !validFederationID(envelope.QuestID) || !validFederationID(envelope.SettlementID) {
		return ErrInvalidFederationEnvelope
	}
	if len(envelope.Payload) > maxFederationPayloadBytes || envelope.CreatedAt <= 0 || envelope.ExpiresAt <= envelope.CreatedAt {
		return ErrInvalidFederationEnvelope
	}
	if envelope.ExpiresAt <= time.Now().UnixMilli() {
		return ErrExpiredFederationEnvelope
	}
	return nil
}

func canonicalFederationEnvelope(envelope *FederationEnvelope) ([]byte, error) {
	if envelope == nil {
		return nil, ErrInvalidFederationEnvelope
	}
	return json.Marshal(struct {
		ProtocolVersion string                `json:"protocol_version"`
		MessageID       string                `json:"message_id"`
		Type            FederationMessageType `json:"type"`
		SourceNodeID    string                `json:"source_node_id"`
		TargetNodeID    string                `json:"target_node_id"`
		QuestID         string                `json:"quest_id"`
		SettlementID    string                `json:"settlement_id"`
		Payload         []byte                `json:"payload"`
		CreatedAt       int64                 `json:"created_at"`
		ExpiresAt       int64                 `json:"expires_at"`
	}{
		ProtocolVersion: envelope.ProtocolVersion,
		MessageID:       envelope.MessageID,
		Type:            envelope.Type,
		SourceNodeID:    envelope.SourceNodeID,
		TargetNodeID:    envelope.TargetNodeID,
		QuestID:         envelope.QuestID,
		SettlementID:    envelope.SettlementID,
		Payload:         envelope.Payload,
		CreatedAt:       envelope.CreatedAt,
		ExpiresAt:       envelope.ExpiresAt,
	})
}

func federationMessageID() (string, error) {
	randomBytes := make([]byte, 18)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generate federation message ID: %w", err)
	}
	return "fed_" + base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

func validFederationMessageType(messageType FederationMessageType) bool {
	return messageType == FederationSettlementIntent || messageType == FederationSettlementReceipt
}

func validFederationID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '.' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}
