package financial

import (
	"crypto/ed25519"
	"errors"
	"testing"
	"time"
)

func TestFederationEnvelopeVerifiesAndBindsItsContents(t *testing.T) {
	signer, publicKey, err := GenerateFederationSigner("mosque-alpha")
	if err != nil {
		t.Fatalf("generate signer: %v", err)
	}
	envelope, err := signer.SignSettlementIntent("charity-beta", "quest-relief-1", "settlement-1", []byte(`{"amount_cents":5000}`), time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("sign settlement intent: %v", err)
	}
	if err := VerifyFederationEnvelope(envelope, publicKey); err != nil {
		t.Fatalf("verify signed intent: %v", err)
	}
	if envelope.ExpiresAt <= envelope.CreatedAt {
		t.Fatalf("expected serialized expiry after creation, got %d <= %d", envelope.ExpiresAt, envelope.CreatedAt)
	}
	envelope.TargetNodeID = "attacker-node"
	if err := VerifyFederationEnvelope(envelope, publicKey); !errors.Is(err, ErrInvalidFederationEnvelope) {
		t.Fatalf("verify tampered intent error = %v, want ErrInvalidFederationEnvelope", err)
	}
}

func TestFederationInboxRejectsReplay(t *testing.T) {
	signer, publicKey, err := GenerateFederationSigner("mosque-alpha")
	if err != nil {
		t.Fatalf("generate signer: %v", err)
	}
	envelope, err := signer.SignSettlementReceipt("charity-beta", "quest-relief-1", "settlement-1", nil, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("sign settlement receipt: %v", err)
	}
	inbox, err := NewFederationInbox("charity-beta")
	if err != nil {
		t.Fatalf("create federation inbox: %v", err)
	}
	if err := inbox.Accept(envelope, publicKey); err != nil {
		t.Fatalf("accept first envelope: %v", err)
	}
	if err := inbox.Accept(envelope, publicKey); !errors.Is(err, ErrFederationReplay) {
		t.Fatalf("accept replay error = %v, want ErrFederationReplay", err)
	}
}

func TestFederationInboxRejectsEnvelopeForAnotherNode(t *testing.T) {
	signer, publicKey, err := GenerateFederationSigner("mosque-alpha")
	if err != nil {
		t.Fatalf("generate signer: %v", err)
	}
	envelope, err := signer.SignSettlementIntent("charity-beta", "quest-relief-1", "settlement-1", nil, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("sign settlement intent: %v", err)
	}
	inbox, err := NewFederationInbox("different-node")
	if err != nil {
		t.Fatalf("create federation inbox: %v", err)
	}
	if err := inbox.Accept(envelope, publicKey); !errors.Is(err, ErrInvalidFederationEnvelope) {
		t.Fatalf("accept wrong target error = %v, want ErrInvalidFederationEnvelope", err)
	}
}

func TestFederationEnvelopeRejectsInvalidInputs(t *testing.T) {
	if _, err := NewFederationSigner("bad node", make(ed25519.PrivateKey, ed25519.PrivateKeySize)); !errors.Is(err, ErrInvalidFederationEnvelope) {
		t.Fatalf("NewFederationSigner invalid node error = %v", err)
	}
	signer, _, err := GenerateFederationSigner("mosque-alpha")
	if err != nil {
		t.Fatalf("generate signer: %v", err)
	}
	if _, err := signer.SignSettlementIntent("charity-beta", "quest-1", "settlement-1", nil, time.Now().Add(-time.Minute)); !errors.Is(err, ErrExpiredFederationEnvelope) {
		t.Fatalf("expired intent error = %v, want ErrExpiredFederationEnvelope", err)
	}
}
