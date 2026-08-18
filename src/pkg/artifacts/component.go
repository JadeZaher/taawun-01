package artifacts

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	MaxComponentDocumentBytes  = 8 << 10
	MaxComponentDocumentsBytes = 32 << 10
	MaxComponentDocumentDepth  = 6
	MaxComponentKeyBytes       = 64
	MaxComponentObjectFields   = 32
	MaxComponentArrayItems     = 32
	MaxComponentTotalKeys      = 128
	MaxComponentStringRunes    = 2048
	MaxComponentNumberBytes    = 64
)

// ComponentInstance is one curated module instance with a canonical JSON data object.
type ComponentInstance struct {
	ID   string          `json:"id"`
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// ComponentManifest binds a component document to its exact manifest-listed file.
type ComponentManifest struct {
	ID             string          `json:"id"`
	Type           string          `json:"type"`
	Data           json.RawMessage `json:"data"`
	DocumentPath   string          `json:"documentPath"`
	DocumentSHA256 string          `json:"documentSha256"`
}

// ComponentFieldDescriptor is an approachable declared field, not an executable schema.
type ComponentFieldDescriptor struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	ValueType   string `json:"valueType"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	MaxLength   int    `json:"maxLength,omitempty"`
}

// ComponentDocumentPolicy describes the fixed validation boundary exposed with the catalog.
type ComponentDocumentPolicy struct {
	ContractVersion   string   `json:"contractVersion"`
	RootType          string   `json:"rootType"`
	StableIDRule      string   `json:"stableIdRule"`
	KeyGrammar        string   `json:"keyGrammar"`
	NumberFormat      string   `json:"numberFormat"`
	MaxComponentBytes int      `json:"maxComponentBytes"`
	MaxTotalBytes     int      `json:"maxTotalBytes"`
	MaxDepth          int      `json:"maxDepth"`
	MaxKeyBytes       int      `json:"maxKeyBytes"`
	MaxObjectFields   int      `json:"maxObjectFields"`
	MaxArrayItems     int      `json:"maxArrayItems"`
	MaxTotalKeys      int      `json:"maxTotalKeys"`
	MaxStringRunes    int      `json:"maxStringRunes"`
	MaxNumberBytes    int      `json:"maxNumberBytes"`
	AllowedValueTypes []string `json:"allowedValueTypes"`
	ReservedKeyParts  []string `json:"reservedKeyParts"`
	CustomFields      string   `json:"customFields"`
}

// ComponentValidationError is safe to expose; it never contains a document value.
type ComponentValidationError struct {
	ComponentID string `json:"componentId,omitempty"`
	Key         string `json:"key,omitempty"`
	Reason      string `json:"reason"`
}

func (e *ComponentValidationError) Error() string {
	if e == nil {
		return ErrInvalidBuildRequest.Error()
	}
	safe := e.SafeDetails()
	return fmt.Sprintf("%s: component=%q key=%q reason=%s", ErrInvalidBuildRequest, safe.ComponentID, safe.Key, safe.Reason)
}

func (e *ComponentValidationError) Unwrap() error { return ErrInvalidBuildRequest }

// SafeDetails bounds validation metadata before it crosses an HTTP, MCP, or log boundary.
func (e *ComponentValidationError) SafeDetails() ComponentValidationError {
	if e == nil {
		return ComponentValidationError{Reason: "invalid_component_document"}
	}
	reason := e.Reason
	if !safeReason(reason) {
		reason = "invalid_component_document"
	}
	return ComponentValidationError{
		ComponentID: safeComponentErrorID(e.ComponentID),
		Key:         safeComponentErrorKey(e.Key, reason),
		Reason:      reason,
	}
}

// ComponentPolicy returns the stable catalog-visible component document bounds.
func ComponentPolicy() ComponentDocumentPolicy {
	return ComponentDocumentPolicy{
		ContractVersion: ManifestContractVersion, RootType: "object", StableIDRule: "one instance per allowed module; id equals type",
		KeyGrammar:        "ASCII letter first; then ASCII letters, digits, hyphen, or underscore",
		NumberFormat:      "base-10 JSON integer or minimal decimal; no exponent, negative zero, or trailing fractional zero",
		MaxComponentBytes: MaxComponentDocumentBytes, MaxTotalBytes: MaxComponentDocumentsBytes,
		MaxDepth: MaxComponentDocumentDepth, MaxKeyBytes: MaxComponentKeyBytes, MaxObjectFields: MaxComponentObjectFields,
		MaxArrayItems: MaxComponentArrayItems, MaxTotalKeys: MaxComponentTotalKeys, MaxStringRunes: MaxComponentStringRunes,
		MaxNumberBytes:    MaxComponentNumberBytes,
		AllowedValueTypes: []string{"null", "boolean", "number", "string", "array", "object"},
		ReservedKeyParts:  componentReservedKeyFragments(),
		CustomFields:      "Unknown safe keys are rendered as escaped reference data; they never become code, routes, origins, permissions, or authority.",
	}
}

func componentError(componentID, key, reason string) error {
	safe := (&ComponentValidationError{ComponentID: componentID, Key: key, Reason: reason}).SafeDetails()
	return &safe
}

// CanonicalizeSuppliedComponents validates only explicitly supplied components.
// It intentionally does not synthesize defaults, preserving legacy composition hashes.
func CanonicalizeSuppliedComponents(templateID string, modules []string, components []ComponentInstance) ([]ComponentInstance, error) {
	if components == nil {
		if len(modules) > 0 {
			return nil, validateModules(templateID, modules)
		}
		return nil, fmt.Errorf("%w: at least one module or component is required", ErrInvalidBuildRequest)
	}
	if len(components) == 0 {
		return nil, componentError("", "", "components_required")
	}
	return resolveComponents(templateID, modules, components, false)
}

// ResolveBuildRequest produces the canonical v2 build request, including default documents for legacy modules-only input.
func ResolveBuildRequest(request BuildRequest) (BuildRequest, error) {
	resolved := request
	components, err := resolveComponents(request.TemplateID, request.Modules, request.Components, true)
	if err != nil {
		return BuildRequest{}, err
	}
	resolved.Components = components
	resolved.Modules = make([]string, len(components))
	for index, component := range components {
		resolved.Modules[index] = component.Type
	}
	return resolved, nil
}

func resolveComponents(templateID string, modules []string, supplied []ComponentInstance, defaults bool) ([]ComponentInstance, error) {
	definition, supported := templateCatalog[templateID]
	if !supported {
		return nil, fmt.Errorf("%w: %q", ErrInvalidTemplate, templateID)
	}
	if len(modules) > 0 {
		if err := validateModules(templateID, modules); err != nil {
			return nil, err
		}
	}
	allowed := make(map[string]struct{}, len(definition.AllowedModules))
	for _, moduleID := range definition.AllowedModules {
		allowed[moduleID] = struct{}{}
	}
	if supplied == nil {
		if !defaults || len(modules) == 0 {
			return nil, fmt.Errorf("%w: at least one component is required", ErrInvalidBuildRequest)
		}
		result := make([]ComponentInstance, 0, len(modules))
		for _, moduleID := range modules {
			result = append(result, ComponentInstance{ID: moduleID, Type: moduleID, Data: defaultComponentDocument(moduleID)})
		}
		sortComponents(result)
		return result, nil
	}
	if len(supplied) == 0 {
		return nil, componentError("", "", "components_required")
	}

	seen := make(map[string]struct{}, len(supplied))
	totalBytes := 0
	result := make([]ComponentInstance, 0, len(supplied))
	for _, component := range supplied {
		if component.ID == "" || component.ID != component.Type || !validComponentIdentifier(component.ID) {
			return nil, componentError(component.ID, "", "invalid_component_identity")
		}
		if _, exists := allowed[component.Type]; !exists {
			return nil, componentError(component.ID, "", "component_not_allowed_by_template")
		}
		if _, exists := moduleCatalog[component.Type]; !exists {
			return nil, componentError(component.ID, "", "unknown_component_type")
		}
		if _, duplicate := seen[component.Type]; duplicate {
			return nil, componentError(component.ID, "", "duplicate_component_type")
		}
		seen[component.Type] = struct{}{}
		canonical, err := canonicalComponentDocument(component.ID, component.Data)
		if err != nil {
			return nil, err
		}
		if err := validateDeclaredComponentFields(component.ID, canonical); err != nil {
			return nil, err
		}
		totalBytes += len(canonical)
		if totalBytes > MaxComponentDocumentsBytes {
			return nil, componentError(component.ID, "", "total_documents_too_large")
		}
		result = append(result, ComponentInstance{ID: component.ID, Type: component.Type, Data: canonical})
	}
	if len(modules) > 0 {
		if len(seen) != len(modules) {
			return nil, componentError("", "", "module_component_set_mismatch")
		}
		for _, moduleID := range modules {
			if _, exists := seen[moduleID]; !exists {
				return nil, componentError(moduleID, "", "module_component_set_mismatch")
			}
		}
	}
	sortComponents(result)
	return result, nil
}

func validateDeclaredComponentFields(componentID string, document json.RawMessage) error {
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil {
		return componentError(componentID, "", "invalid_json")
	}
	limits := map[string]int{"title": 120, "summary": 600}
	for key, limit := range limits {
		value, exists := object[key]
		text, stringValue := value.(string)
		if !exists || !stringValue || strings.TrimSpace(text) == "" {
			return componentError(componentID, key, "declared_field_required")
		}
		if utf8.RuneCountInString(text) > limit {
			return componentError(componentID, key, "declared_field_too_long")
		}
	}
	return nil
}

func validComponentIdentifier(value string) bool {
	if value == "" || len(value) > MaxComponentKeyBytes {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-' {
			continue
		}
		return false
	}
	return value[0] >= 'a' && value[0] <= 'z'
}

func sortComponents(components []ComponentInstance) {
	sort.Slice(components, func(i, j int) bool { return components[i].Type < components[j].Type })
}

func cloneComponents(components []ComponentInstance) []ComponentInstance {
	if components == nil {
		return nil
	}
	cloned := make([]ComponentInstance, len(components))
	for index, component := range components {
		cloned[index] = component
		cloned[index].Data = append(json.RawMessage(nil), component.Data...)
	}
	return cloned
}

type documentValidationState struct {
	componentID string
	totalKeys   int
}

func canonicalComponentDocument(componentID string, document json.RawMessage) (json.RawMessage, error) {
	if len(document) == 0 {
		return nil, componentError(componentID, "", "document_required")
	}
	if len(document) > MaxComponentDocumentBytes {
		return nil, componentError(componentID, "", "document_too_large")
	}
	if !utf8.Valid(document) {
		return nil, componentError(componentID, "", "invalid_utf8")
	}
	if !validJSONUnicodeScalars(document) {
		return nil, componentError(componentID, "", "invalid_unicode_scalar")
	}
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()
	state := &documentValidationState{componentID: componentID}
	value, err := decodeDocumentValue(decoder, state, 1, "")
	if err != nil {
		return nil, err
	}
	if _, object := value.(map[string]any); !object {
		return nil, componentError(componentID, "", "root_object_required")
	}
	if token, err := decoder.Token(); !errors.Is(err, io.EOF) || token != nil {
		return nil, componentError(componentID, "", "invalid_json")
	}
	canonical, err := json.Marshal(value)
	if err != nil || len(canonical) > MaxComponentDocumentBytes {
		return nil, componentError(componentID, "", "document_too_large")
	}
	return json.RawMessage(canonical), nil
}

// validJSONUnicodeScalars rejects escapes that encoding/json would replace with U+FFFD.
func validJSONUnicodeScalars(document []byte) bool {
	inString := false
	for index := 0; index < len(document); index++ {
		switch document[index] {
		case '"':
			inString = !inString
		case '\\':
			if !inString || index+1 >= len(document) {
				continue
			}
			escape := document[index+1]
			if escape != 'u' {
				index++
				continue
			}
			code, ok := jsonUnicodeEscape(document, index)
			if !ok {
				continue
			}
			if code >= 0xdc00 && code <= 0xdfff {
				return false
			}
			if code >= 0xd800 && code <= 0xdbff {
				lowIndex := index + 6
				low, paired := jsonUnicodeEscape(document, lowIndex)
				if !paired || low < 0xdc00 || low > 0xdfff {
					return false
				}
				index = lowIndex + 5
				continue
			}
			index += 5
		}
	}
	return true
}

func jsonUnicodeEscape(document []byte, slashIndex int) (uint16, bool) {
	if slashIndex < 0 || slashIndex+5 >= len(document) || document[slashIndex] != '\\' || document[slashIndex+1] != 'u' {
		return 0, false
	}
	var value uint16
	for index := slashIndex + 2; index <= slashIndex+5; index++ {
		digit := document[index]
		value <<= 4
		switch {
		case digit >= '0' && digit <= '9':
			value |= uint16(digit - '0')
		case digit >= 'a' && digit <= 'f':
			value |= uint16(digit-'a') + 10
		case digit >= 'A' && digit <= 'F':
			value |= uint16(digit-'A') + 10
		default:
			return 0, false
		}
	}
	return value, true
}

func decodeDocumentValue(decoder *json.Decoder, state *documentValidationState, depth int, path string) (any, error) {
	if depth > MaxComponentDocumentDepth {
		return nil, componentError(state.componentID, path, "depth_exceeded")
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, componentError(state.componentID, path, "invalid_json")
	}
	switch value := token.(type) {
	case json.Delim:
		switch value {
		case '{':
			object := make(map[string]any)
			for decoder.More() {
				if len(object) >= MaxComponentObjectFields {
					return nil, componentError(state.componentID, path, "object_field_limit")
				}
				keyToken, keyErr := decoder.Token()
				key, ok := keyToken.(string)
				if keyErr != nil || !ok {
					return nil, componentError(state.componentID, path, "invalid_json")
				}
				keyPath := key
				if path != "" {
					keyPath = path + "." + key
				}
				if _, duplicate := object[key]; duplicate {
					return nil, componentError(state.componentID, keyPath, "duplicate_key")
				}
				if reason := componentKeyRejection(key); reason != "" {
					return nil, componentError(state.componentID, keyPath, reason)
				}
				state.totalKeys++
				if state.totalKeys > MaxComponentTotalKeys {
					return nil, componentError(state.componentID, keyPath, "total_key_limit")
				}
				child, childErr := decodeDocumentValue(decoder, state, depth+1, keyPath)
				if childErr != nil {
					return nil, childErr
				}
				object[key] = child
			}
			if closing, closeErr := decoder.Token(); closeErr != nil || closing != json.Delim('}') {
				return nil, componentError(state.componentID, path, "invalid_json")
			}
			return object, nil
		case '[':
			array := make([]any, 0)
			for decoder.More() {
				if len(array) >= MaxComponentArrayItems {
					return nil, componentError(state.componentID, path, "array_item_limit")
				}
				child, childErr := decodeDocumentValue(decoder, state, depth+1, path)
				if childErr != nil {
					return nil, childErr
				}
				array = append(array, child)
			}
			if closing, closeErr := decoder.Token(); closeErr != nil || closing != json.Delim(']') {
				return nil, componentError(state.componentID, path, "invalid_json")
			}
			return array, nil
		default:
			return nil, componentError(state.componentID, path, "invalid_json")
		}
	case string:
		if !utf8.ValidString(value) {
			return nil, componentError(state.componentID, path, "invalid_utf8")
		}
		if utf8.RuneCountInString(value) > MaxComponentStringRunes {
			return nil, componentError(state.componentID, path, "string_too_long")
		}
		for _, character := range value {
			if unicode.IsControl(character) && character != '\n' && character != '\t' {
				return nil, componentError(state.componentID, path, "control_character")
			}
		}
		return value, nil
	case json.Number:
		if len(value.String()) > MaxComponentNumberBytes {
			return nil, componentError(state.componentID, path, "number_too_long")
		}
		if !canonicalJSONNumber(value.String()) {
			return nil, componentError(state.componentID, path, "non_canonical_number")
		}
		return value, nil
	case bool, nil:
		return value, nil
	default:
		return nil, componentError(state.componentID, path, "invalid_json_type")
	}
}

func componentKeyRejection(key string) string {
	if key == "" || len(key) > MaxComponentKeyBytes || !utf8.ValidString(key) {
		return "invalid_key"
	}
	normalized := strings.ToLower(strings.NewReplacer("-", "", "_", "").Replace(key))
	if strings.HasPrefix(normalized, "on") {
		return "reserved_key"
	}
	for _, word := range componentReservedKeyFragments() {
		if strings.Contains(normalized, word) {
			return "reserved_key"
		}
	}
	for index, character := range key {
		if index == 0 && !asciiLetter(character) {
			return "invalid_key"
		}
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '-' || character == '_' {
			continue
		}
		return "invalid_key"
	}
	return ""
}

func asciiLetter(character rune) bool {
	return character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
}

func componentReservedKeyFragments() []string {
	return []string{
		"proto", "prototype", "constructor", "script", "html", "css", "style", "endpoint", "url", "uri",
		"origin", "surface", "embedder", "connection", "resource", "authority", "permission", "capability",
		"workspace", "principal", "subject", "user", "actor", "signer", "signature", "auth", "lifecycle",
		"expiry", "expires", "shura", "finance", "payment", "settlement", "escrow",
		"artifactid", "contenthash", "manifestdigest", "contractversion", "templateid", "moduleid", "componentid",
		"token", "password", "credential", "secret", "cookie", "apikey",
	}
}

func canonicalJSONNumber(value string) bool {
	if value == "-0" || strings.ContainsAny(value, "eE") {
		return false
	}
	if point := strings.IndexByte(value, '.'); point >= 0 {
		fraction := value[point+1:]
		return fraction != "" && !strings.HasSuffix(fraction, "0")
	}
	return true
}

func safeReason(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || character == '_' {
			continue
		}
		return false
	}
	return true
}

func safeComponentErrorID(value string) string {
	if !validComponentIdentifier(value) {
		return ""
	}
	return value
}

func safeComponentErrorKey(value, reason string) string {
	if value == "" || len(value) > 192 || reason == "reserved_key" || reason == "invalid_key" {
		return ""
	}
	for _, segment := range strings.Split(value, ".") {
		if componentKeyRejection(segment) != "" {
			return ""
		}
	}
	return value
}

func componentDocumentDigest(document json.RawMessage) string {
	digest := sha256.Sum256(document)
	return hex.EncodeToString(digest[:])
}

func componentInstancesEqual(left, right []ComponentInstance) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].ID != right[index].ID || left[index].Type != right[index].Type || !bytes.Equal(left[index].Data, right[index].Data) {
			return false
		}
	}
	return true
}

func componentManifests(components []ComponentInstance) []ComponentManifest {
	result := make([]ComponentManifest, 0, len(components))
	for _, component := range components {
		result = append(result, ComponentManifest{
			ID: component.ID, Type: component.Type, Data: append(json.RawMessage(nil), component.Data...),
			DocumentPath: "components/" + component.ID + ".json", DocumentSHA256: componentDocumentDigest(component.Data),
		})
	}
	return result
}

func componentInstancesFromManifest(components []ComponentManifest) []ComponentInstance {
	result := make([]ComponentInstance, 0, len(components))
	for _, component := range components {
		result = append(result, ComponentInstance{ID: component.ID, Type: component.Type, Data: append(json.RawMessage(nil), component.Data...)})
	}
	return result
}

func validateManifestComponents(manifest Manifest) error {
	if manifest.ContractVersion == ManifestContractVersionV1 {
		if len(manifest.Components) != 0 {
			return ErrInvalidSignature
		}
		return nil
	}
	if manifest.ContractVersion != ManifestContractVersion || len(manifest.Components) == 0 {
		return ErrInvalidSignature
	}
	definition, exists := templateCatalog[manifest.Template.ID]
	if !exists || manifest.Template.Version != definition.Identity.Version {
		return ErrInvalidSignature
	}
	moduleIDs := make([]string, 0, len(manifest.Modules))
	for _, module := range manifest.Modules {
		moduleIDs = append(moduleIDs, module.ID)
	}
	if !reflect.DeepEqual(manifest.Modules, selectedModules(moduleIDs)) {
		return ErrInvalidSignature
	}
	instances := componentInstancesFromManifest(manifest.Components)
	canonical, err := resolveComponents(manifest.Template.ID, moduleIDs, instances, false)
	if err != nil || !componentInstancesEqual(canonical, instances) {
		return ErrInvalidSignature
	}
	aggregate, err := json.Marshal(instances)
	if err != nil {
		return ErrInvalidSignature
	}
	fileDigests := make(map[string]FileDigest, len(manifest.Files))
	for _, file := range manifest.Files {
		fileDigests[file.Path] = file
	}
	aggregateFile, exists := fileDigests["components.json"]
	if !exists || aggregateFile.SHA256 != componentDocumentDigest(aggregate) || aggregateFile.Bytes != len(aggregate) {
		return ErrInvalidSignature
	}
	for index, component := range manifest.Components {
		if component.DocumentPath != "components/"+component.ID+".json" || component.DocumentSHA256 != componentDocumentDigest(component.Data) {
			return ErrInvalidSignature
		}
		file, exists := fileDigests[component.DocumentPath]
		if !exists || file.SHA256 != component.DocumentSHA256 || file.Bytes != len(component.Data) {
			return ErrInvalidSignature
		}
		if index > 0 && manifest.Components[index-1].Type >= component.Type {
			return ErrInvalidSignature
		}
	}
	return nil
}

// ValidateManifestComponents checks the versioned component/data/module/template binding.
func ValidateManifestComponents(manifest Manifest) error {
	return validateManifestComponents(manifest)
}

func validateStoredComponentDocuments(directory string, manifest Manifest) error {
	if err := validateManifestComponents(manifest); err != nil {
		return err
	}
	if manifest.ContractVersion == ManifestContractVersionV1 {
		return nil
	}
	instances := componentInstancesFromManifest(manifest.Components)
	aggregate, err := json.Marshal(instances)
	if err != nil {
		return ErrInvalidSignature
	}
	aggregatePath, err := safeJoin(directory, "components.json")
	if err != nil {
		return ErrInvalidSignature
	}
	storedAggregate, err := os.ReadFile(aggregatePath)
	if err != nil || !bytes.Equal(storedAggregate, aggregate) {
		return fmt.Errorf("%w: component aggregate mismatch", ErrArtifactConflict)
	}
	for _, component := range manifest.Components {
		path, pathErr := safeJoin(directory, component.DocumentPath)
		if pathErr != nil {
			return ErrInvalidSignature
		}
		stored, readErr := os.ReadFile(path)
		if readErr != nil || !bytes.Equal(stored, component.Data) {
			return fmt.Errorf("%w: component document mismatch", ErrArtifactConflict)
		}
	}
	return nil
}
