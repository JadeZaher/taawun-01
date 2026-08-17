package shura

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Role defines capability scopes for P2P workspace members.
type Role string

const (
	RoleArchitect  Role = "Architect"  // Full CRDT write + AZOA quest execution & template publish
	RoleMaintainer Role = "Maintainer" // Write access to convergent state (page copy, schedules)
	RoleViewer     Role = "Viewer"     // Read-only access & public interaction
)

// CapabilityToken represents a signed P2P capability token issued by Taawun SSO.
type CapabilityToken struct {
	TokenID        string    `json:"token_id"`
	SubjectID      string    `json:"subject_id"`
	WorkspaceID    string    `json:"workspace_id"`
	Role           Role      `json:"role"`
	CRDTWriteScope []string  `json:"crdt_write_scope"`
	AzoaQuestScope []string  `json:"azoa_quest_scope"`
	ExpiresAt      time.Time `json:"expires_at"`
	Signature      string    `json:"signature"`
}

// TokenVerifier checks capability tokens against actions.
type TokenVerifier struct {
	IssuerPublicKey string
}

// NewTokenVerifier creates a verifier.
func NewTokenVerifier(publicKey string) *TokenVerifier {
	return &TokenVerifier{IssuerPublicKey: publicKey}
}

// CanWriteCRDT checks if the token grants write access to a target CRDT collection.
func (v *TokenVerifier) CanWriteCRDT(token *CapabilityToken, collection string) error {
	if time.Now().After(token.ExpiresAt) {
		return errors.New("capability token expired")
	}

	if token.Role == RoleArchitect {
		return nil
	}

	if token.Role == RoleMaintainer {
		// Maintainers can edit convergent state collections
		for _, scope := range token.CRDTWriteScope {
			if scope == "*" || scope == collection || strings.HasPrefix(collection, "convergent:") {
				return nil
			}
		}
	}

	return fmt.Errorf("role '%s' unauthorized to write to collection '%s'", token.Role, collection)
}

// CanExecuteAzoaQuest checks if the token grants execution permission for an AZOA quest.
func (v *TokenVerifier) CanExecuteAzoaQuest(token *CapabilityToken, questAction string) error {
	if time.Now().After(token.ExpiresAt) {
		return errors.New("capability token expired")
	}

	if token.Role == RoleArchitect {
		return nil
	}

	return fmt.Errorf("role '%s' unauthorized to trigger financial AZOA quest '%s'", token.Role, questAction)
}
