package artifacts

import (
	"time"

	"taawun/pkg/ethics"
)

const ManifestContractVersion = "taawun.artifact/v1"
const SignatureContractVersion = "taawun.bundle-signature/v1"

// RenderMode names a supported renderer while keeping the bundle renderer-neutral.
type RenderMode string

const (
	RenderModeStandalone RenderMode = "standalone"
	RenderModeEmbed      RenderMode = "embed"
)

// Manifest is the complete declarative trust and delivery contract for a bundle.
type Manifest struct {
	ContractVersion      string                 `json:"contractVersion"`
	ArtifactID           string                 `json:"artifactId"`
	ContentHash          string                 `json:"contentHash"`
	CreatedAt            time.Time              `json:"createdAt"`
	WorkspaceID          int                    `json:"workspaceId"`
	AppName              string                 `json:"appName"`
	Template             TemplateIdentity       `json:"template"`
	RenderModes          []RenderModeDescriptor `json:"renderModes"`
	DefaultEmbedBoundary string                 `json:"defaultEmbedBoundary"`
	Modules              []ModuleDescriptor     `json:"modules"`
	DataClassifications  []DataClassification   `json:"dataClassifications"`
	AllowedServerSignals []string               `json:"allowedServerSignals"`
	Security             SecurityPolicy         `json:"security"`
	Theme                ThemeManifest          `json:"theme"`
	Runtime              RuntimeRequirement     `json:"runtime"`
	StateSplit           StateSplit             `json:"stateSplit"`
	Relay                RelayRequirement       `json:"relay"`
	Financial            FinancialBoundary      `json:"financial"`
	Compliance           ComplianceBoundary     `json:"compliance"`
	Authorization        BundleAuthorization    `json:"authorization"`
	Signature            BundleSignature        `json:"signature"`
	Files                []FileDigest           `json:"files"`
}

type TemplateIdentity struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type RenderModeDescriptor struct {
	ID           RenderMode `json:"id"`
	Engine       string     `json:"engine"`
	Boundary     string     `json:"boundary"`
	Transport    string     `json:"transport"`
	HostContract string     `json:"hostContract"`
	Adapter      string     `json:"adapter"`
	UsesDatastar bool       `json:"usesDatastar"`
}

type DataClassification struct {
	ID              string `json:"id"`
	Classification  string `json:"classification"`
	Owner           string `json:"owner"`
	Persistence     string `json:"persistence"`
	Synchronization string `json:"synchronization"`
	Description     string `json:"description"`
}

type SecurityPolicy struct {
	AllowedOrigins OriginPolicy    `json:"allowedOrigins"`
	CSP            []RenderModeCSP `json:"csp"`
}

type RenderModeCSP struct {
	RenderMode RenderMode `json:"renderMode"`
	Policy     CSPPolicy  `json:"policy"`
	Rationale  string     `json:"rationale"`
}

type CSPPolicy struct {
	DefaultSrc     []string `json:"defaultSrc"`
	ScriptSrc      []string `json:"scriptSrc"`
	StyleSrc       []string `json:"styleSrc"`
	ConnectSrc     []string `json:"connectSrc"`
	ImageSrc       []string `json:"imageSrc"`
	FontSrc        []string `json:"fontSrc"`
	FrameAncestors []string `json:"frameAncestors"`
	ObjectSrc      []string `json:"objectSrc"`
	BaseURI        []string `json:"baseUri"`
	FormAction     []string `json:"formAction"`
}

type ThemeManifest struct {
	Stylesheet string   `json:"stylesheet"`
	TokenNames []string `json:"tokenNames"`
}

type RuntimeRequirement struct {
	Name                 string       `json:"name"`
	Version              string       `json:"version"`
	Path                 string       `json:"path"`
	SHA256               string       `json:"sha256"`
	RequiredBy           []RenderMode `json:"requiredBy"`
	Bundled              bool         `json:"bundled"`
	PackagingRequirement string       `json:"packagingRequirement"`
}

type StateSplit struct {
	Statement string `json:"statement"`
	Browser   string `json:"browser"`
	Server    string `json:"server"`
	Relay     string `json:"relay"`
}

type RelayRequirement struct {
	RequiredForLocalUse    bool   `json:"requiredForLocalUse"`
	RequiredForCrossDevice bool   `json:"requiredForCrossDevice"`
	Purpose                string `json:"purpose"`
}

type FinancialBoundary struct {
	Status     string `json:"status"`
	Custody    string `json:"custody"`
	Settlement string `json:"settlement"`
	Disclaimer string `json:"disclaimer"`
}

type ComplianceBoundary struct {
	Madhhab    ethics.Madhhab        `json:"madhhab"`
	Status     string                `json:"status"`
	Disclaimer string                `json:"disclaimer"`
	References []ComplianceReference `json:"references"`
}

type ComplianceReference struct {
	ID       string                        `json:"id"`
	Title    string                        `json:"title"`
	Citation string                        `json:"citation"`
	Status   ethics.ComplianceRecordStatus `json:"status"`
}

type BundleAuthorization struct {
	Version         string          `json:"version"`
	Subject         BundleSubject   `json:"subject"`
	AllowedOrigins  OriginPolicy    `json:"allowedOrigins"`
	ApprovedDomains []string        `json:"approvedDomains"`
	ExpiresAt       time.Time       `json:"expiresAt"`
	SignerKeyID     string          `json:"signerKeyId"`
	Lifecycle       BundleLifecycle `json:"lifecycle"`
	RevocationCheck string          `json:"revocationCheck"`
}

type BundleSubject struct {
	ID          string `json:"id"`
	UserID      int    `json:"userId"`
	WorkspaceID int    `json:"workspaceId"`
}

type BundleSignature struct {
	ContractVersion string   `json:"contractVersion"`
	Algorithm       string   `json:"algorithm"`
	KeyID           string   `json:"keyId"`
	Value           string   `json:"value"`
	SignedFields    []string `json:"signedFields"`
}

type FileDigest struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
}

// TemplateDescriptor is portable configuration consumed by any approved renderer.
type TemplateDescriptor struct {
	ContractVersion string           `json:"contractVersion"`
	Template        TemplateIdentity `json:"template"`
	Configuration   TemplateConfig   `json:"configuration"`
	RenderModes     []RenderMode     `json:"renderModes"`
	ModuleRefs      []string         `json:"moduleRefs"`
	Slots           []string         `json:"slots"`
}

type TemplateConfig struct {
	WorkspaceID      int            `json:"workspaceId"`
	AppName          string         `json:"appName"`
	OrganizationName string         `json:"organizationName"`
	City             string         `json:"city"`
	Madhhab          ethics.Madhhab `json:"madhhab"`
}

// RenderAdapterCatalog is renderer metadata consumed by the serving layer.
type RenderAdapterCatalog struct {
	ContractVersion string                 `json:"contractVersion"`
	Adapters        []RenderModeDescriptor `json:"adapters"`
	ServerSignals   []string               `json:"serverSignals"`
}
