package artifacts

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestComponentDocumentsCanonicalizeAndResolveLegacyDefaults(t *testing.T) {
	legacy := validBuildRequest()
	resolved, err := ResolveBuildRequest(legacy)
	if err != nil {
		t.Fatalf("ResolveBuildRequest(legacy): %v", err)
	}
	if len(resolved.Components) != len(legacy.Modules) || resolved.Components[0].ID != resolved.Components[0].Type {
		t.Fatalf("legacy components = %#v", resolved.Components)
	}

	left := []ComponentInstance{{ID: ModuleAnnouncements, Type: ModuleAnnouncements, Data: json.RawMessage(`{"title":"Updates","summary":"Tonight","meta":{"b":2,"a":1},"items":[true,null,1.25]}`)}}
	right := []ComponentInstance{{ID: ModuleAnnouncements, Type: ModuleAnnouncements, Data: json.RawMessage(` { "items" : [ true, null, 1.25 ], "meta" : {"a":1,"b":2}, "summary":"Tonight", "title":"Updates" } `)}}
	first, err := CanonicalizeSuppliedComponents(TemplateCommunityIftar, []string{ModuleAnnouncements}, left)
	if err != nil {
		t.Fatalf("canonicalize first: %v", err)
	}
	second, err := CanonicalizeSuppliedComponents(TemplateCommunityIftar, []string{ModuleAnnouncements}, right)
	if err != nil {
		t.Fatalf("canonicalize second: %v", err)
	}
	if !componentInstancesEqual(first, second) || !bytes.Equal(first[0].Data, []byte(`{"items":[true,null,1.25],"meta":{"a":1,"b":2},"summary":"Tonight","title":"Updates"}`)) {
		t.Fatalf("canonical documents differ: %s / %s", first[0].Data, second[0].Data)
	}
}

func TestExplicitEmptyComponentsNeverMeanOmittedLegacyComponents(t *testing.T) {
	request := validBuildRequest()
	request.Components = []ComponentInstance{}
	if _, err := ResolveBuildRequest(request); !errors.Is(err, ErrInvalidBuildRequest) {
		t.Fatalf("ResolveBuildRequest(explicit empty) error = %v", err)
	}
	if _, err := CanonicalizeSuppliedComponents(request.TemplateID, request.Modules, []ComponentInstance{}); !errors.Is(err, ErrInvalidBuildRequest) {
		t.Fatalf("CanonicalizeSuppliedComponents(explicit empty) error = %v", err)
	}
	request.Components = nil
	resolved, err := ResolveBuildRequest(request)
	if err != nil || len(resolved.Components) != len(request.Modules) {
		t.Fatalf("omitted legacy components = %#v err=%v", resolved.Components, err)
	}
}

func TestComponentDocumentValidationRejectsUnsafeAndUnboundedData(t *testing.T) {
	tests := []struct {
		name   string
		data   string
		reason string
	}{
		{name: "duplicate", data: `{"title":"A","summary":"B","note":1,"note":2}`, reason: "duplicate_key"},
		{name: "prototype", data: `{"title":"A","summary":"B","__proto__":{}}`, reason: "reserved_key"},
		{name: "nested authority", data: `{"title":"A","summary":"B","safe":{"signerKey":"x"}}`, reason: "reserved_key"},
		{name: "event handler", data: `{"title":"A","summary":"B","onClick":"x"}`, reason: "reserved_key"},
		{name: "bad key", data: `{"title":"A","summary":"B","not.ok":1}`, reason: "invalid_key"},
		{name: "leading hyphen", data: `{"title":"A","summary":"B","-note":1}`, reason: "invalid_key"},
		{name: "leading underscore", data: `{"title":"A","summary":"B","_note":1}`, reason: "invalid_key"},
		{name: "root array", data: `[]`, reason: "root_object_required"},
		{name: "depth", data: `{"title":"A","summary":"B","a":{"b":{"c":{"d":{"e":{"f":1}}}}}}`, reason: "depth_exceeded"},
		{name: "array", data: `{"title":"A","summary":"B","items":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]}`, reason: "array_item_limit"},
		{name: "control", data: `{"title":"A","summary":"B","note":"\u0000"}`, reason: "control_character"},
		{name: "missing declared", data: `{"title":"A"}`, reason: "declared_field_required"},
		{name: "wrong declared type", data: `{"title":"A","summary":3}`, reason: "declared_field_required"},
		{name: "decimal trailing zero", data: `{"title":"A","summary":"B","amount":1.0}`, reason: "non_canonical_number"},
		{name: "exponent", data: `{"title":"A","summary":"B","amount":1e0}`, reason: "non_canonical_number"},
		{name: "negative zero", data: `{"title":"A","summary":"B","amount":-0}`, reason: "non_canonical_number"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := CanonicalizeSuppliedComponents(TemplateCommunityIftar, []string{ModuleAnnouncements}, []ComponentInstance{{ID: ModuleAnnouncements, Type: ModuleAnnouncements, Data: json.RawMessage(test.data)}})
			var validation *ComponentValidationError
			if !errors.As(err, &validation) || validation.Reason != test.reason || validation.ComponentID != ModuleAnnouncements {
				t.Fatalf("validation = %#v err=%v, want %s", validation, err, test.reason)
			}
			if strings.Contains(err.Error(), test.data) {
				t.Fatalf("error leaked document: %v", err)
			}
		})
	}

	tooLarge := json.RawMessage(`{"title":"A","summary":"B","note":"` + strings.Repeat("x", MaxComponentDocumentBytes) + `"}`)
	_, err := CanonicalizeSuppliedComponents(TemplateCommunityIftar, []string{ModuleAnnouncements}, []ComponentInstance{{ID: ModuleAnnouncements, Type: ModuleAnnouncements, Data: tooLarge}})
	var validation *ComponentValidationError
	if !errors.As(err, &validation) || validation.Reason != "document_too_large" {
		t.Fatalf("oversize validation = %#v err=%v", validation, err)
	}
}

func TestComponentDocumentUnicodeScalarsRejectSubstitutionAndPreserveValidPairs(t *testing.T) {
	for _, test := range []struct {
		name string
		data string
	}{
		{name: "root high surrogate", data: `{"title":"Unicode","summary":"\ud800"}`},
		{name: "root low surrogate", data: `{"title":"Unicode","summary":"\udfff"}`},
		{name: "nested high surrogate", data: `{"title":"Unicode","summary":"Safe","details":{"note":"\ud800"}}`},
		{name: "nested low surrogate", data: `{"title":"Unicode","summary":"Safe","items":[{"note":"\udc00"}]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := CanonicalizeSuppliedComponents(TemplateCommunityIftar, []string{ModuleAnnouncements}, []ComponentInstance{{
				ID: ModuleAnnouncements, Type: ModuleAnnouncements, Data: json.RawMessage(test.data),
			}})
			var validation *ComponentValidationError
			if !errors.As(err, &validation) || validation.Reason != "invalid_unicode_scalar" || validation.Key != "" {
				t.Fatalf("unicode validation = %#v err=%v", validation, err)
			}
			if strings.Contains(err.Error(), test.data) || strings.Contains(err.Error(), "d800") || strings.Contains(err.Error(), "dc00") || strings.Contains(err.Error(), "dfff") {
				t.Fatalf("unicode validation leaked raw value: %v", err)
			}
		})
	}

	escaped := []ComponentInstance{{ID: ModuleAnnouncements, Type: ModuleAnnouncements, Data: json.RawMessage(`{"title":"Unicode","summary":"\ud83d\ude00","details":{"note":"\uD83D\uDE80"}}`)}}
	literal := []ComponentInstance{{ID: ModuleAnnouncements, Type: ModuleAnnouncements, Data: json.RawMessage(`{"title":"Unicode","summary":"😀","details":{"note":"🚀"}}`)}}
	first, err := CanonicalizeSuppliedComponents(TemplateCommunityIftar, []string{ModuleAnnouncements}, escaped)
	if err != nil {
		t.Fatalf("escaped surrogate pairs: %v", err)
	}
	second, err := CanonicalizeSuppliedComponents(TemplateCommunityIftar, []string{ModuleAnnouncements}, literal)
	if err != nil {
		t.Fatalf("literal Unicode scalars: %v", err)
	}
	want := `{"details":{"note":"🚀"},"summary":"😀","title":"Unicode"}`
	if !componentInstancesEqual(first, second) || string(first[0].Data) != want {
		t.Fatalf("valid Unicode canonicalization = %s / %s, want %s", first[0].Data, second[0].Data, want)
	}
}

func TestComponentReservedKeysCoverCredentialAndArtifactVariants(t *testing.T) {
	for _, key := range []string{
		"artifactId", "CONTENT_HASH", "manifest-digest", "Contract_Version", "templateId", "module-id", "component_ID",
		"token", "Pass_Word", "credential", "clientSecret", "session-cookie", "api_key",
	} {
		t.Run(key, func(t *testing.T) {
			document := json.RawMessage(`{"title":"A","summary":"B","outer":{"` + key + `":"private"}}`)
			_, err := CanonicalizeSuppliedComponents(TemplateCommunityIftar, []string{ModuleAnnouncements}, []ComponentInstance{{ID: ModuleAnnouncements, Type: ModuleAnnouncements, Data: document}})
			var validation *ComponentValidationError
			if !errors.As(err, &validation) || validation.Reason != "reserved_key" || validation.Key != "" || strings.Contains(err.Error(), key) {
				t.Fatalf("reserved key validation = %#v err=%v", validation, err)
			}
		})
	}
}

func TestComponentValidationErrorMetadataIsBoundedAndRedacted(t *testing.T) {
	secretID := strings.Repeat("secret-token-", 40)
	secretKey := strings.Repeat("credential.", 40) + "password"
	error := &ComponentValidationError{ComponentID: secretID, Key: secretKey, Reason: "reserved_key"}
	safe := error.SafeDetails()
	if safe.ComponentID != "" || safe.Key != "" || safe.Reason != "reserved_key" || strings.Contains(error.Error(), "secret") || strings.Contains(error.Error(), "password") {
		t.Fatalf("safe details = %#v error=%q", safe, error.Error())
	}
}

func TestComponentModuleRelationshipIsExact(t *testing.T) {
	document := defaultComponentDocument(ModuleAnnouncements)
	tests := []struct {
		name       string
		modules    []string
		components []ComponentInstance
	}{
		{name: "id type mismatch", modules: []string{ModuleAnnouncements}, components: []ComponentInstance{{ID: "custom", Type: ModuleAnnouncements, Data: document}}},
		{name: "set mismatch", modules: []string{ModuleAnnouncements, ModuleRegistration}, components: []ComponentInstance{{ID: ModuleAnnouncements, Type: ModuleAnnouncements, Data: document}}},
		{name: "duplicate", modules: nil, components: []ComponentInstance{{ID: ModuleAnnouncements, Type: ModuleAnnouncements, Data: document}, {ID: ModuleAnnouncements, Type: ModuleAnnouncements, Data: document}}},
		{name: "incompatible", modules: nil, components: []ComponentInstance{{ID: ModuleZakat, Type: ModuleZakat, Data: defaultComponentDocument(ModuleZakat)}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := CanonicalizeSuppliedComponents(TemplateCommunityIftar, test.modules, test.components); !errors.Is(err, ErrInvalidBuildRequest) {
				t.Fatalf("error = %v, want invalid build request", err)
			}
		})
	}
}

func TestCatalogExposesDefensiveDocumentContracts(t *testing.T) {
	first := ListModules()
	if len(first) != 11 || len(first[0].DocumentFields) != 2 || first[0].DefaultDocument["title"] == nil {
		t.Fatalf("catalog document contract = %#v", first[0])
	}
	first[0].DocumentFields[0].Label = "tampered"
	first[0].DefaultDocument["title"] = "tampered"
	second := ListModules()
	if second[0].DocumentFields[0].Label == "tampered" || reflect.DeepEqual(first[0].DefaultDocument, second[0].DefaultDocument) {
		t.Fatal("catalog returned mutable document metadata")
	}
	policy := ComponentPolicy()
	if policy.MaxComponentBytes != MaxComponentDocumentBytes || policy.StableIDRule == "" || policy.KeyGrammar == "" || policy.NumberFormat == "" || !reflect.DeepEqual(policy.ReservedKeyParts, componentReservedKeyFragments()) {
		t.Fatalf("component policy = %#v", policy)
	}
	policy.ReservedKeyParts[0] = "tampered"
	if ComponentPolicy().ReservedKeyParts[0] == "tampered" {
		t.Fatal("component policy returned mutable reserved-key metadata")
	}
}
