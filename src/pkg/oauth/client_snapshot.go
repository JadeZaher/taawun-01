package oauth

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

// clientSnapshot freezes the validated CIMD or registered-client contract for one grant lineage.
type clientSnapshot struct {
	Fingerprint string
	Client      Client
}

type clientSnapshotPayload struct {
	ID                      string   `json:"id"`
	Name                    string   `json:"name"`
	RedirectURIs            []string `json:"redirectUris"`
	GrantTypes              []string `json:"grantTypes"`
	ResponseTypes           []string `json:"responseTypes"`
	TokenEndpointAuthMethod string   `json:"tokenEndpointAuthMethod"`
	ClientURI               string   `json:"clientUri"`
}

func newClientSnapshot(client Client) (clientSnapshot, error) {
	payload := clientSnapshotPayload{
		ID: client.ID, Name: client.Name, RedirectURIs: append([]string(nil), client.RedirectURIs...),
		GrantTypes: append([]string(nil), client.GrantTypes...), ResponseTypes: append([]string(nil), client.ResponseTypes...),
		TokenEndpointAuthMethod: client.TokenEndpointAuthMethod, ClientURI: client.ClientURI,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return clientSnapshot{}, err
	}
	digest := sha256.Sum256(data)
	client.Disabled = false
	return clientSnapshot{Fingerprint: hex.EncodeToString(digest[:]), Client: client}, nil
}

func encodeClientSnapshot(snapshot clientSnapshot) (string, error) {
	payload := clientSnapshotPayload{
		ID: snapshot.Client.ID, Name: snapshot.Client.Name, RedirectURIs: snapshot.Client.RedirectURIs,
		GrantTypes: snapshot.Client.GrantTypes, ResponseTypes: snapshot.Client.ResponseTypes,
		TokenEndpointAuthMethod: snapshot.Client.TokenEndpointAuthMethod, ClientURI: snapshot.Client.ClientURI,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func decodeClientSnapshot(fingerprint, raw string) (*clientSnapshot, error) {
	var payload clientSnapshotPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}
	validated, err := validateClientDefinition(Client{
		ID: payload.ID, Name: payload.Name, RedirectURIs: payload.RedirectURIs, GrantTypes: payload.GrantTypes,
		ResponseTypes: payload.ResponseTypes, TokenEndpointAuthMethod: payload.TokenEndpointAuthMethod, ClientURI: payload.ClientURI,
	})
	if err != nil {
		return nil, err
	}
	snapshot, err := newClientSnapshot(validated)
	if err != nil || snapshot.Fingerprint != fingerprint {
		return nil, errors.New("client metadata snapshot integrity check failed")
	}
	return &snapshot, nil
}
