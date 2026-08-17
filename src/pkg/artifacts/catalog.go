package artifacts

import "sort"

const (
	ModuleRegistration     = "iftar-registration"
	ModuleAnnouncements    = "announcements"
	ModuleDonationCampaign = "donation-campaign"
	ModuleShuraGovernance  = "shura-governance"
	ModuleComplianceReview = "compliance-source-review"
	ModuleBazaarLifecycle  = "bazaar-template-lifecycle"
	ModuleZakat            = "zakat"
	ModuleQardHasan        = "qard-hasan"
	ModuleVolunteerStipend = "volunteer-stipend"
	ModuleSandboxEscrow    = "sandbox-escrow"
	ModuleRevenueSplit     = "revenue-split"

	DataConvergentBrowserOnly = "convergent-browser-only"
	DataTransactionalServer   = "transactional-server"
)

type templateDefinition struct {
	Identity       TemplateIdentity
	Title          string
	Description    string
	AllowedModules []string
	Slots          []string
}

// TemplateCatalogEntry is a defensive public view of one curated template.
type TemplateCatalogEntry struct {
	Identity       TemplateIdentity `json:"identity"`
	Title          string           `json:"title"`
	Description    string           `json:"description"`
	AllowedModules []string         `json:"allowedModules"`
	Slots          []string         `json:"slots"`
}

var templateCatalog = map[string]templateDefinition{
	TemplateCommunityIftar: {
		Identity:    TemplateIdentity{ID: TemplateCommunityIftar, Version: TemplateCommunityIftarVersion},
		Title:       "Community iftar",
		Description: "A focused registration, announcements, fundraising, and volunteer coordination preset.",
		AllowedModules: []string{
			ModuleRegistration,
			ModuleAnnouncements,
			ModuleDonationCampaign,
			ModuleVolunteerStipend,
		},
		Slots: []string{"hero", "announcements", "registration", "donation-campaign", "volunteer-stipend", "compliance-note"},
	},
	TemplateCommunityWorkspace: {
		Identity:    TemplateIdentity{ID: TemplateCommunityWorkspace, Version: TemplateCommunityWorkspaceVersion},
		Title:       "Community workspace",
		Description: "The complete modular community operations, governance, compliance, and ethical-finance workspace.",
		AllowedModules: []string{
			ModuleRegistration,
			ModuleAnnouncements,
			ModuleDonationCampaign,
			ModuleShuraGovernance,
			ModuleComplianceReview,
			ModuleBazaarLifecycle,
			ModuleZakat,
			ModuleQardHasan,
			ModuleVolunteerStipend,
			ModuleSandboxEscrow,
			ModuleRevenueSplit,
		},
		Slots: []string{"hero", "module-grid", "activity", "governance", "finance", "compliance-note"},
	},
	TemplateBazaarCooperative: {
		Identity:    TemplateIdentity{ID: TemplateBazaarCooperative, Version: TemplateBazaarCooperativeVersion},
		Title:       "Bazaar cooperative",
		Description: "A curated ethical marketplace lifecycle with Shura oversight and sandbox financial flows.",
		AllowedModules: []string{
			ModuleAnnouncements,
			ModuleShuraGovernance,
			ModuleComplianceReview,
			ModuleBazaarLifecycle,
			ModuleQardHasan,
			ModuleVolunteerStipend,
			ModuleSandboxEscrow,
			ModuleRevenueSplit,
		},
		Slots: []string{"hero", "bazaar", "governance", "finance", "compliance-note"},
	},
}

// ModuleDescriptor is a declarative module contract, never executable code.
type ModuleDescriptor struct {
	ContractVersion      string            `json:"contractVersion"`
	ID                   string            `json:"id"`
	Version              string            `json:"version"`
	Title                string            `json:"title"`
	Capabilities         []string          `json:"capabilities"`
	Routes               []RouteDescriptor `json:"routes"`
	Events               []EventDescriptor `json:"events"`
	DataClassifications  []string          `json:"dataClassifications"`
	AllowedServerSignals []string          `json:"allowedServerSignals"`
}

// RouteDescriptor names a renderer-owned surface route, not a backend endpoint.
type RouteDescriptor struct {
	ID            string `json:"id"`
	Path          string `json:"path"`
	Surface       string `json:"surface"`
	ServerBinding string `json:"serverBinding"`
}

// EventDescriptor defines the only host-facing event names a module may emit.
type EventDescriptor struct {
	Name      string `json:"name"`
	Direction string `json:"direction"`
	Payload   string `json:"payload"`
}

var moduleCatalog = map[string]ModuleDescriptor{
	ModuleRegistration: {
		ID:      ModuleRegistration,
		Version: "1.0.0",
		Title:   "Iftar registration",
		Capabilities: []string{
			"registration.capture-local",
			"registration.view-local",
		},
		Routes: []RouteDescriptor{{
			ID:            "registration",
			Path:          "/register",
			Surface:       "module",
			ServerBinding: "runtime-injected",
		}},
		Events: []EventDescriptor{{
			Name:      "taawun.registration.changed",
			Direction: "module-to-host",
			Payload:   "registration-change-v1",
		}},
		DataClassifications:  []string{"community-registrations"},
		AllowedServerSignals: []string{"taawun_registration_status", "taawun_notice"},
	},
	ModuleAnnouncements: {
		ID:      ModuleAnnouncements,
		Version: "1.0.0",
		Title:   "Community announcements",
		Capabilities: []string{
			"announcement.compose-local",
			"announcement.view-local",
		},
		Routes: []RouteDescriptor{{
			ID:            "announcements",
			Path:          "/announcements",
			Surface:       "module",
			ServerBinding: "runtime-injected",
		}},
		Events: []EventDescriptor{{
			Name:      "taawun.announcement.changed",
			Direction: "module-to-host",
			Payload:   "announcement-change-v1",
		}},
		DataClassifications:  []string{"community-announcements"},
		AllowedServerSignals: []string{"taawun_announcement_status", "taawun_notice"},
	},
	ModuleDonationCampaign: {
		ID:      ModuleDonationCampaign,
		Version: "1.0.0",
		Title:   "Donation campaign",
		Capabilities: []string{
			"donation.campaign-view",
			"donation.sandbox-intent-request",
		},
		Routes: []RouteDescriptor{{
			ID:            "donation-campaign",
			Path:          "/donate",
			Surface:       "module",
			ServerBinding: "runtime-injected-sandbox-only",
		}},
		Events: []EventDescriptor{{
			Name:      "taawun.donation.sandbox-requested",
			Direction: "module-to-host",
			Payload:   "sandbox-donation-intent-v1",
		}},
		DataClassifications: []string{
			"donation-campaign-projection",
			"donation-intents",
		},
		AllowedServerSignals: []string{"taawun_donation_status", "taawun_notice"},
	},
	ModuleShuraGovernance: {
		ID: ModuleShuraGovernance, Version: "1.0.0", Title: "Shura governance",
		Capabilities:         []string{"shura.proposal-compose", "shura.deliberation-view", "shura.decision-record"},
		Routes:               []RouteDescriptor{{ID: "shura", Path: "/shura", Surface: "module", ServerBinding: "runtime-injected"}},
		Events:               []EventDescriptor{{Name: "taawun.shura.proposal-submitted", Direction: "module-to-host", Payload: "shura-proposal-v1"}},
		DataClassifications:  []string{"shura-proposals", "shura-decisions"},
		AllowedServerSignals: []string{"taawun_shura_status", "taawun_notice"},
	},
	ModuleComplianceReview: {
		ID: ModuleComplianceReview, Version: "1.0.0", Title: "Compliance and source review",
		Capabilities:         []string{"compliance.source-view", "compliance.review-request", "compliance.status-view"},
		Routes:               []RouteDescriptor{{ID: "compliance-review", Path: "/compliance", Surface: "module", ServerBinding: "runtime-injected"}},
		Events:               []EventDescriptor{{Name: "taawun.compliance.review-requested", Direction: "module-to-host", Payload: "compliance-review-request-v1"}},
		DataClassifications:  []string{"compliance-source-reviews"},
		AllowedServerSignals: []string{"taawun_compliance_status", "taawun_notice"},
	},
	ModuleBazaarLifecycle: {
		ID: ModuleBazaarLifecycle, Version: "1.0.0", Title: "Bazaar template lifecycle",
		Capabilities:         []string{"bazaar.template-draft", "bazaar.review-request", "bazaar.publication-view"},
		Routes:               []RouteDescriptor{{ID: "bazaar", Path: "/bazaar", Surface: "module", ServerBinding: "runtime-injected"}},
		Events:               []EventDescriptor{{Name: "taawun.bazaar.review-requested", Direction: "module-to-host", Payload: "bazaar-template-review-v1"}},
		DataClassifications:  []string{"bazaar-template-drafts", "bazaar-publications"},
		AllowedServerSignals: []string{"taawun_bazaar_status", "taawun_notice"},
	},
	ModuleZakat: {
		ID: ModuleZakat, Version: "1.0.0", Title: "Zakat workspace",
		Capabilities:         []string{"zakat.calculate-local", "zakat.eligibility-reference", "zakat.sandbox-intent-request"},
		Routes:               []RouteDescriptor{{ID: "zakat", Path: "/zakat", Surface: "module", ServerBinding: "runtime-injected-sandbox-only"}},
		Events:               []EventDescriptor{{Name: "taawun.zakat.sandbox-requested", Direction: "module-to-host", Payload: "zakat-sandbox-intent-v1"}},
		DataClassifications:  []string{"zakat-calculation-inputs", "zakat-intents"},
		AllowedServerSignals: []string{"taawun_zakat_status", "taawun_notice"},
	},
	ModuleQardHasan: {
		ID: ModuleQardHasan, Version: "1.0.0", Title: "Qard Hasan",
		Capabilities:         []string{"qard-hasan.request-draft", "qard-hasan.terms-view", "qard-hasan.sandbox-submit"},
		Routes:               []RouteDescriptor{{ID: "qard-hasan", Path: "/qard-hasan", Surface: "module", ServerBinding: "runtime-injected-sandbox-only"}},
		Events:               []EventDescriptor{{Name: "taawun.qard-hasan.sandbox-requested", Direction: "module-to-host", Payload: "qard-hasan-request-v1"}},
		DataClassifications:  []string{"qard-hasan-requests"},
		AllowedServerSignals: []string{"taawun_qard_hasan_status", "taawun_notice"},
	},
	ModuleVolunteerStipend: {
		ID: ModuleVolunteerStipend, Version: "1.0.0", Title: "Volunteer stipend",
		Capabilities:         []string{"stipend.assignment-draft", "stipend.terms-view", "stipend.sandbox-submit"},
		Routes:               []RouteDescriptor{{ID: "volunteer-stipend", Path: "/stipends", Surface: "module", ServerBinding: "runtime-injected-sandbox-only"}},
		Events:               []EventDescriptor{{Name: "taawun.stipend.sandbox-requested", Direction: "module-to-host", Payload: "volunteer-stipend-v1"}},
		DataClassifications:  []string{"volunteer-stipends"},
		AllowedServerSignals: []string{"taawun_stipend_status", "taawun_notice"},
	},
	ModuleSandboxEscrow: {
		ID: ModuleSandboxEscrow, Version: "1.0.0", Title: "Sandbox escrow",
		Capabilities:         []string{"escrow.terms-view", "escrow.sandbox-intent-request", "escrow.status-view"},
		Routes:               []RouteDescriptor{{ID: "sandbox-escrow", Path: "/escrow", Surface: "module", ServerBinding: "runtime-injected-sandbox-only"}},
		Events:               []EventDescriptor{{Name: "taawun.escrow.sandbox-requested", Direction: "module-to-host", Payload: "sandbox-escrow-intent-v1"}},
		DataClassifications:  []string{"sandbox-escrow-intents"},
		AllowedServerSignals: []string{"taawun_escrow_status", "taawun_notice"},
	},
	ModuleRevenueSplit: {
		ID: ModuleRevenueSplit, Version: "1.0.0", Title: "Revenue split",
		Capabilities:         []string{"revenue-split.rule-draft", "revenue-split.preview", "revenue-split.sandbox-submit"},
		Routes:               []RouteDescriptor{{ID: "revenue-split", Path: "/revenue-splits", Surface: "module", ServerBinding: "runtime-injected-sandbox-only"}},
		Events:               []EventDescriptor{{Name: "taawun.revenue-split.sandbox-requested", Direction: "module-to-host", Payload: "revenue-split-v1"}},
		DataClassifications:  []string{"revenue-split-rules"},
		AllowedServerSignals: []string{"taawun_revenue_split_status", "taawun_notice"},
	},
}

func selectedModules(ids []string) []ModuleDescriptor {
	modules := make([]ModuleDescriptor, 0, len(ids))
	for _, id := range ids {
		module := moduleCatalog[id]
		module.ContractVersion = ManifestContractVersion
		module.Capabilities = append([]string(nil), module.Capabilities...)
		module.Routes = append([]RouteDescriptor(nil), module.Routes...)
		module.Events = append([]EventDescriptor(nil), module.Events...)
		module.DataClassifications = append([]string(nil), module.DataClassifications...)
		module.AllowedServerSignals = append([]string(nil), module.AllowedServerSignals...)
		modules = append(modules, module)
	}
	return modules
}

// ListTemplates returns the curated template catalog in stable ID order.
func ListTemplates() []TemplateCatalogEntry {
	ids := make([]string, 0, len(templateCatalog))
	for id := range templateCatalog {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	entries := make([]TemplateCatalogEntry, 0, len(ids))
	for _, id := range ids {
		definition := templateCatalog[id]
		entries = append(entries, TemplateCatalogEntry{
			Identity: definition.Identity, Title: definition.Title, Description: definition.Description,
			AllowedModules: append([]string(nil), definition.AllowedModules...),
			Slots:          append([]string(nil), definition.Slots...),
		})
	}
	return entries
}

// GetTemplate returns one curated template without exposing mutable catalog state.
func GetTemplate(id string) (TemplateCatalogEntry, bool) {
	definition, ok := templateCatalog[id]
	if !ok {
		return TemplateCatalogEntry{}, false
	}
	return TemplateCatalogEntry{
		Identity: definition.Identity, Title: definition.Title, Description: definition.Description,
		AllowedModules: append([]string(nil), definition.AllowedModules...),
		Slots:          append([]string(nil), definition.Slots...),
	}, true
}

// ListModules returns the curated module catalog in stable ID order.
func ListModules() []ModuleDescriptor {
	ids := make([]string, 0, len(moduleCatalog))
	for id := range moduleCatalog {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return selectedModules(ids)
}

// GetModule returns one curated module descriptor without exposing mutable catalog state.
func GetModule(id string) (ModuleDescriptor, bool) {
	if _, ok := moduleCatalog[id]; !ok {
		return ModuleDescriptor{}, false
	}
	return selectedModules([]string{id})[0], true
}
