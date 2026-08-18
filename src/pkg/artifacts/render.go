package artifacts

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
)

func componentIDs(components []ComponentInstance) []string {
	ids := make([]string, len(components))
	for index, component := range components {
		ids[index] = component.ID
	}
	return ids
}

var themeTokenNames = []string{
	"--taawun-color-accent",
	"--taawun-color-on-accent",
	"--taawun-color-gold",
	"--taawun-color-ink",
	"--taawun-color-surface",
	"--taawun-color-surface-muted",
	"--taawun-color-text",
	"--taawun-color-text-muted",
	"--taawun-color-border",
	"--taawun-font-sans",
	"--taawun-font-display",
	"--taawun-radius-card",
	"--taawun-radius-control",
	"--taawun-shadow-card",
	"--taawun-space-1",
	"--taawun-space-2",
	"--taawun-space-3",
	"--taawun-space-4",
	"--taawun-space-6",
	"--taawun-space-8",
}

const (
	datastarRuntimeName       = "Datastar"
	datastarRuntimeVersion    = "1.0.2"
	datastarRuntimePath       = "/assets/datastar-v1.0.2.js"
	datastarRuntimeBundlePath = "assets/datastar-v1.0.2.js"
	datastarRuntimeSHA256     = "2837d87acf6ee0ba8e4e63765926c25a98d63883b02f88be194a86b81d3fd24a"
)

var dataCatalog = map[string]DataClassification{
	"community-registrations": {
		ID:              "community-registrations",
		Classification:  DataConvergentBrowserOnly,
		Owner:           "browser",
		Persistence:     "browser-owned IndexedDB supplied by the renderer",
		Synchronization: "convergent operations through an approved renderer adapter",
		Description:     "Iftar attendance responses; never included in the generated bundle.",
	},
	"community-announcements": {
		ID:              "community-announcements",
		Classification:  DataConvergentBrowserOnly,
		Owner:           "browser",
		Persistence:     "browser-owned IndexedDB supplied by the renderer",
		Synchronization: "convergent operations through an approved renderer adapter",
		Description:     "Community announcements; never included in the generated bundle.",
	},
	"donation-campaign-projection": {
		ID:              "donation-campaign-projection",
		Classification:  DataTransactionalServer,
		Owner:           "authenticated application server",
		Persistence:     "server-owned sandbox projection",
		Synchronization: "allowlisted server signal only",
		Description:     "Read-only sandbox campaign status; the bundle is not a ledger.",
	},
	"donation-intents": {
		ID:              "donation-intents",
		Classification:  DataTransactionalServer,
		Owner:           "authenticated application server",
		Persistence:     "server-owned sandbox intent store",
		Synchronization: "explicit host-mediated request and allowlisted status signal",
		Description:     "Sandbox-only intent metadata; never persisted by the generated bundle.",
	},
	"shura-proposals": {
		ID: "shura-proposals", Classification: DataConvergentBrowserOnly, Owner: "browser",
		Persistence: "browser-owned IndexedDB supplied by the renderer", Synchronization: "convergent operations through an approved renderer adapter",
		Description: "Draft proposals and deliberation notes; final decisions are server-owned.",
	},
	"shura-decisions": {
		ID: "shura-decisions", Classification: DataTransactionalServer, Owner: "authenticated application server",
		Persistence: "server-owned governance record", Synchronization: "allowlisted server signal only",
		Description: "Recorded Shura outcomes with an auditable server lifecycle.",
	},
	"compliance-source-reviews": {
		ID: "compliance-source-reviews", Classification: DataTransactionalServer, Owner: "authenticated application server",
		Persistence: "server-owned cited review record", Synchronization: "allowlisted server signal only",
		Description: "Source citations, reviewer state, and qualified-review requests; never a generated fatwa.",
	},
	"bazaar-template-drafts": {
		ID: "bazaar-template-drafts", Classification: DataConvergentBrowserOnly, Owner: "browser",
		Persistence: "browser-owned IndexedDB supplied by the renderer", Synchronization: "convergent operations through an approved renderer adapter",
		Description: "Curated marketplace template drafts before review.",
	},
	"bazaar-publications": {
		ID: "bazaar-publications", Classification: DataTransactionalServer, Owner: "authenticated application server",
		Persistence: "server-owned publication lifecycle", Synchronization: "allowlisted server signal only",
		Description: "Reviewed Bazaar publication state and immutable release references.",
	},
	"zakat-calculation-inputs": {
		ID: "zakat-calculation-inputs", Classification: DataConvergentBrowserOnly, Owner: "browser",
		Persistence: "browser-owned IndexedDB supplied by the renderer", Synchronization: "local calculation only",
		Description: "Private calculation inputs; results remain estimates pending qualified review.",
	},
	"zakat-intents": {
		ID: "zakat-intents", Classification: DataTransactionalServer, Owner: "authenticated application server",
		Persistence: "server-owned sandbox intent store", Synchronization: "allowlisted server signal only",
		Description: "Sandbox-only Zakat distribution intent metadata.",
	},
	"qard-hasan-requests": {
		ID: "qard-hasan-requests", Classification: DataTransactionalServer, Owner: "authenticated application server",
		Persistence: "server-owned sandbox request store", Synchronization: "allowlisted server signal only",
		Description: "Interest-free assistance requests with explicit terms and approval state.",
	},
	"volunteer-stipends": {
		ID: "volunteer-stipends", Classification: DataTransactionalServer, Owner: "authenticated application server",
		Persistence: "server-owned sandbox stipend record", Synchronization: "allowlisted server signal only",
		Description: "Volunteer stipend assignments and explicit terms; no bundle custody.",
	},
	"sandbox-escrow-intents": {
		ID: "sandbox-escrow-intents", Classification: DataTransactionalServer, Owner: "authenticated application server",
		Persistence: "server-owned sandbox intent store", Synchronization: "allowlisted server signal only",
		Description: "Non-custodial escrow previews only; no settlement occurs in the artifact.",
	},
	"revenue-split-rules": {
		ID: "revenue-split-rules", Classification: DataTransactionalServer, Owner: "authenticated application server",
		Persistence: "server-owned sandbox rule store", Synchronization: "allowlisted server signal only",
		Description: "Explicit sandbox split rules and previews; no live disbursement.",
	},
}

func assembleBundle(request BuildRequest, signerKeyID string) (Manifest, map[string][]byte, error) {
	modules := selectedModules(request.Modules)
	components := cloneComponents(request.Components)
	templateDefinition := templateCatalog[request.TemplateID]
	renderModes := renderModeDescriptors()
	signals := allowedServerSignals(modules)
	data := selectedDataClassifications(modules)
	security := buildSecurityPolicy(request.AllowedOrigins)
	references, err := complianceReferences(request.Madhhab)
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("retrieve compliance references: %w", err)
	}

	template := TemplateDescriptor{
		ContractVersion: ManifestContractVersion,
		Template:        templateDefinition.Identity,
		Configuration: TemplateConfig{
			WorkspaceID:      request.WorkspaceID,
			AppName:          request.AppName,
			OrganizationName: request.OrganizationName,
			City:             request.City,
			Madhhab:          request.Madhhab,
		},
		RenderModes:   []RenderMode{RenderModeStandalone, RenderModeEmbed},
		ModuleRefs:    append([]string(nil), request.Modules...),
		ComponentRefs: componentIDs(components),
		Slots:         append([]string(nil), templateDefinition.Slots...),
	}

	runtime, err := bundledDatastarRuntime()
	if err != nil {
		return Manifest{}, nil, err
	}
	files := make(map[string][]byte, 8+len(modules))
	files[datastarRuntimeBundlePath] = runtime
	files["theme.css"] = renderThemeCSS(request.Theme.AccentColor)
	if files["template.json"], err = marshalDocument(template); err != nil {
		return Manifest{}, nil, fmt.Errorf("encode template descriptor: %w", err)
	}
	adapters := RenderAdapterCatalog{
		ContractVersion: ManifestContractVersion,
		Adapters:        renderModes,
		ServerSignals:   signals,
	}
	if files["render-adapters.json"], err = marshalDocument(adapters); err != nil {
		return Manifest{}, nil, fmt.Errorf("encode render adapters: %w", err)
	}
	for _, module := range modules {
		encoded, err := marshalDocument(module)
		if err != nil {
			return Manifest{}, nil, fmt.Errorf("encode module %s: %w", module.ID, err)
		}
		files["modules/"+module.ID+".json"] = encoded
	}
	componentAggregate, err := json.Marshal(components)
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("encode component aggregate: %w", err)
	}
	files["components.json"] = componentAggregate
	for _, component := range components {
		files["components/"+component.ID+".json"] = append([]byte(nil), component.Data...)
	}
	previewFiles, err := renderPreviewDocuments(request, templateDefinition, modules, references, security)
	if err != nil {
		return Manifest{}, nil, err
	}
	for path, contents := range previewFiles {
		files[path] = contents
	}

	manifest := Manifest{
		ContractVersion:      ManifestContractVersion,
		WorkspaceID:          request.WorkspaceID,
		AppName:              request.AppName,
		Template:             template.Template,
		RenderModes:          renderModes,
		DefaultEmbedBoundary: "isolated-iframe",
		Modules:              modules,
		Components:           componentManifests(components),
		DataClassifications:  data,
		AllowedServerSignals: signals,
		Security:             security,
		Theme: ThemeManifest{
			Stylesheet: "theme.css",
			TokenNames: append([]string(nil), themeTokenNames...),
		},
		Runtime: RuntimeRequirement{
			Name: datastarRuntimeName, Version: datastarRuntimeVersion, Path: datastarRuntimePath,
			SHA256: datastarRuntimeSHA256, RequiredBy: []RenderMode{RenderModeStandalone, RenderModeEmbed}, Bundled: true,
			PackagingRequirement: "This immutable bundle includes the exact same-origin versioned runtime at /assets/datastar-v1.0.2.js; serving verifies its signed manifest file digest before returning it.",
		},
		StateSplit: StateSplit{
			Statement: "Convergent community records belong to browser storage; transactional financial records belong to the authenticated server; relay nodes carry opaque coordination traffic only.",
			Browser:   "Renderer-owned IndexedDB may hold convergent registrations and announcements; no runtime records are generated into this bundle.",
			Server:    "Authenticated application servers own transactional sandbox donation intents and projections.",
			Relay:     "The relay is optional for local use and carries opaque signaling only; it is never storage or a financial settlement system.",
		},
		Relay: RelayRequirement{
			RequiredForLocalUse:    false,
			RequiredForCrossDevice: true,
			Purpose:                "Opaque peer coordination for cross-device convergent state only.",
		},
		Financial: FinancialBoundary{
			Status:     "sandbox-only",
			Custody:    "none",
			Settlement: "none",
			Disclaimer: "Preview only: this artifact does not collect, hold, transfer, or settle funds and does not create a financial ledger.",
		},
		Compliance: ComplianceBoundary{
			Madhhab:    request.Madhhab,
			Status:     "reference-only-pending-qualified-review",
			Disclaimer: "Seed reference material only; not a fatwa or scholar approval. Obtain qualified review before reliance.",
			References: references,
		},
		Authorization: BundleAuthorization{
			Version:         SignatureContractVersion,
			Subject:         BundleSubject{ID: request.Subject.ID, UserID: request.Subject.UserID, WorkspaceID: request.WorkspaceID},
			AllowedOrigins:  request.AllowedOrigins,
			ApprovedDomains: approvedDomains(request.AllowedOrigins),
			ExpiresAt:       request.ExpiresAt.UTC(),
			SignerKeyID:     signerKeyID,
			Lifecycle:       request.Lifecycle,
			RevocationCheck: "The serving adapter must check the signer-controlled revocation registry before every uncached authorization decision.",
		},
	}
	return manifest, files, nil
}

func renderModeDescriptors() []RenderModeDescriptor {
	return []RenderModeDescriptor{
		{
			ID:           RenderModeStandalone,
			Engine:       "datastar",
			Boundary:     "dedicated-https-origin",
			Transport:    "https-sse",
			HostContract: "The standalone host injects authenticated Datastar routes; this bundle contains no endpoint URL.",
			Adapter:      "host-supplied-datastar",
			UsesDatastar: true,
		},
		{
			ID:           RenderModeEmbed,
			Engine:       "datastar",
			Boundary:     "isolated-iframe",
			Transport:    "https-sse",
			HostContract: "The embed host uses an isolated iframe; a custom-element wrapper may be layered over the same contract.",
			Adapter:      "host-supplied-datastar",
			UsesDatastar: true,
		},
	}
}

func buildSecurityPolicy(origins OriginPolicy) SecurityPolicy {
	connectionSources := append([]string{"'self'"}, origins.Connections...)
	resourceSources := append([]string{"'self'", "data:"}, origins.Resources...)
	embedAncestors := append([]string(nil), origins.Embedders...)
	if len(embedAncestors) == 0 {
		embedAncestors = []string{"'none'"}
	}
	shared := func(script, connect, ancestors []string) CSPPolicy {
		return CSPPolicy{
			DefaultSrc:     []string{"'none'"},
			ScriptSrc:      script,
			StyleSrc:       []string{"'self'"},
			ConnectSrc:     connect,
			ImageSrc:       append([]string(nil), resourceSources...),
			FontSrc:        append([]string(nil), resourceSources...),
			FrameAncestors: ancestors,
			ObjectSrc:      []string{"'none'"},
			BaseURI:        []string{"'none'"},
			FormAction:     []string{"'none'"},
		}
	}
	return SecurityPolicy{
		AllowedOrigins: origins,
		CSP: []RenderModeCSP{
			{
				RenderMode: RenderModeStandalone,
				Policy: shared(
					[]string{"'self'", "'unsafe-eval'"},
					connectionSources,
					[]string{"'none'"},
				),
				Rationale: "Datastar evaluates declarative expressions; unsafe-eval is limited to this dedicated HTTPS surface and external scripts remain disallowed.",
			},
			{
				RenderMode: RenderModeEmbed,
				Policy: shared(
					[]string{"'self'", "'unsafe-eval'"},
					connectionSources,
					embedAncestors,
				),
				Rationale: "Datastar evaluates declarative expressions inside the isolated iframe; unsafe-eval is not granted to the parent page.",
			},
		},
	}
}

func selectedDataClassifications(modules []ModuleDescriptor) []DataClassification {
	seen := make(map[string]struct{})
	ids := make([]string, 0)
	for _, module := range modules {
		for _, id := range module.DataClassifications {
			if _, exists := seen[id]; !exists {
				seen[id] = struct{}{}
				ids = append(ids, id)
			}
		}
	}
	sort.Strings(ids)
	data := make([]DataClassification, 0, len(ids))
	for _, id := range ids {
		data = append(data, dataCatalog[id])
	}
	return data
}

func allowedServerSignals(modules []ModuleDescriptor) []string {
	seen := make(map[string]struct{})
	for _, module := range modules {
		for _, signal := range module.AllowedServerSignals {
			seen[signal] = struct{}{}
		}
	}
	signals := make([]string, 0, len(seen))
	for signal := range seen {
		signals = append(signals, signal)
	}
	sort.Strings(signals)
	return signals
}

func renderThemeCSS(accent string) []byte {
	accent = resolvedAccentColor(accent)
	return []byte(fmt.Sprintf(`:where(.taawun-surface, [data-taawun-theme]) {
  --taawun-color-accent: %s;
  --taawun-color-on-accent: %s;
	--taawun-color-gold: #C7942C;
	--taawun-color-ink: #161A17;
  --taawun-color-surface: #F4ECDD;
  --taawun-color-surface-muted: #E8DEC9;
  --taawun-color-text: #161A17;
  --taawun-color-text-muted: #4E554E;
  --taawun-color-border: #161A17;
  --taawun-font-sans: Arial, "Helvetica Neue", ui-sans-serif, system-ui, sans-serif;
	--taawun-font-display: "Arial Black", "Helvetica Neue", Arial, sans-serif;
  --taawun-radius-card: 0.25rem;
  --taawun-radius-control: 0.125rem;
  --taawun-shadow-card: 6px 6px 0 #161A17;
  --taawun-space-1: 0.25rem;
  --taawun-space-2: 0.5rem;
  --taawun-space-3: 0.75rem;
  --taawun-space-4: 1rem;
  --taawun-space-6: 1.5rem;
  --taawun-space-8: 2rem;
}
`, accent, contrastColor(accent)))
}

func contrastColor(hexColor string) string {
	background := relativeLuminance(hexColor)
	if contrastRatio(background, relativeLuminance("#FFFFFF")) >= 4.5 {
		return "#FFFFFF"
	}
	if contrastRatio(background, relativeLuminance("#161A17")) >= 4.5 {
		return "#161A17"
	}
	return "#000000"
}

func relativeLuminance(hexColor string) float64 {
	components := make([]float64, 0, 3)
	for offset := 1; offset < len(hexColor); offset += 2 {
		value, _ := strconv.ParseUint(hexColor[offset:offset+2], 16, 8)
		channel := float64(value) / 255
		if channel <= 0.04045 {
			channel /= 12.92
		} else {
			channel = math.Pow((channel+0.055)/1.055, 2.4)
		}
		components = append(components, channel)
	}
	return 0.2126*components[0] + 0.7152*components[1] + 0.0722*components[2]
}

func contrastRatio(left, right float64) float64 {
	if right > left {
		left, right = right, left
	}
	return (left + 0.05) / (right + 0.05)
}
