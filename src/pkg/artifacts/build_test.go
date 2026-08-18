package artifacts

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestBuildPublishesCompleteContentAddressedBundle(t *testing.T) {
	builder := newTestBuilder(t)
	request := validBuildRequest()

	result, err := builder.Build(context.Background(), request)
	if err != nil {
		t.Fatalf("Build() returned an error: %v", err)
	}
	if filepath.Base(result.Directory) != result.ContentHash || len(result.ContentHash) != sha256.Size*2 {
		t.Fatalf("bundle is not addressed by its content hash: %#v", result)
	}
	if !strings.HasPrefix(result.ArtifactID, "art_") || result.Manifest.ArtifactID != result.ArtifactID {
		t.Fatalf("artifact id = %q", result.ArtifactID)
	}
	if result.Manifest.CreatedAt != builder.now() {
		t.Fatalf("createdAt = %s, want %s", result.Manifest.CreatedAt, builder.now())
	}

	wantFiles := []string{
		"app.css",
		"assets/datastar-v1.0.2.js",
		"cards/announcements.html",
		"cards/donation-campaign.html",
		"cards/iftar-registration.html",
		"components.json",
		"components/announcements.json",
		"components/donation-campaign.json",
		"components/iftar-registration.json",
		"embed-loader.js",
		"embed.html",
		"index.html",
		"manifest.json",
		"modules/announcements.json",
		"modules/donation-campaign.json",
		"modules/iftar-registration.json",
		"render-adapters.json",
		"template.json",
		"theme.css",
	}
	entries := bundleFilePaths(t, result.Directory)
	if !reflect.DeepEqual(entries, wantFiles) {
		t.Fatalf("bundle files = %v, want %v", entries, wantFiles)
	}
	for _, digest := range result.Manifest.Files {
		contents := readBundleFile(t, result.Directory, digest.Path)
		sum := sha256.Sum256(contents)
		if got := hex.EncodeToString(sum[:]); got != digest.SHA256 || len(contents) != digest.Bytes {
			t.Fatalf("digest for %s = (%s, %d), want (%s, %d)", digest.Path, got, len(contents), digest.SHA256, digest.Bytes)
		}
	}

	if result.Manifest.ContractVersion != ManifestContractVersion || result.Manifest.Template.ID != TemplateCommunityIftar {
		t.Fatalf("manifest contract/template = %#v", result.Manifest)
	}
	if result.Manifest.Runtime.Path != datastarRuntimePath || result.Manifest.Runtime.Version != datastarRuntimeVersion || result.Manifest.Runtime.SHA256 != datastarRuntimeSHA256 || !result.Manifest.Runtime.Bundled {
		t.Fatalf("runtime requirement = %#v", result.Manifest.Runtime)
	}
	runtime, err := builder.ReadFile(context.Background(), result.ContentHash, datastarRuntimeBundlePath)
	if err != nil {
		t.Fatalf("ReadFile(bundled runtime) returned an error: %v", err)
	}
	runtimeDigest := sha256.Sum256(runtime.Contents)
	if got := hex.EncodeToString(runtimeDigest[:]); got != datastarRuntimeSHA256 {
		t.Fatalf("bundled runtime digest = %s, want %s", got, datastarRuntimeSHA256)
	}
	index := html.UnescapeString(string(readBundleFile(t, result.Directory, "index.html")))
	if !strings.Contains(index, `src="`+datastarRuntimePath+`" integrity="`+datastarSRI()+`"`) {
		t.Fatal("standalone preview does not retain the exact bundled runtime SRI")
	}
	if err := VerifyManifestSignature(result.Manifest, builder.privateKey.Public().(ed25519.PublicKey)); err != nil {
		t.Fatalf("VerifyManifestSignature() returned an error: %v", err)
	}
	tamperedManifest := result.Manifest
	tamperedManifest.Authorization.Subject.ID = "user:999"
	if err := VerifyManifestSignature(tamperedManifest, builder.privateKey.Public().(ed25519.PublicKey)); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("tampered signature error = %v, want ErrInvalidSignature", err)
	}
	if result.Manifest.Authorization.Subject.WorkspaceID != request.WorkspaceID || result.Manifest.Authorization.Subject.ID != request.Subject.ID || result.Manifest.Authorization.SignerKeyID != builder.keyID {
		t.Fatalf("signed authorization binding = %#v", result.Manifest.Authorization)
	}
	if len(result.Manifest.Compliance.References) == 0 || result.Manifest.Compliance.Status != "reference-only-pending-qualified-review" {
		t.Fatalf("compliance metadata = %#v", result.Manifest.Compliance)
	}
	if result.Manifest.Financial.Status != "sandbox-only" || result.Manifest.Financial.Custody != "none" || result.Manifest.Financial.Settlement != "none" {
		t.Fatalf("financial boundary = %#v", result.Manifest.Financial)
	}
	if !strings.Contains(result.Manifest.Financial.Disclaimer, "does not create a financial ledger") {
		t.Fatalf("financial disclaimer = %q", result.Manifest.Financial.Disclaimer)
	}

	theme := string(readBundleFile(t, result.Directory, "theme.css"))
	if !strings.Contains(theme, "--taawun-color-accent: #0F766E") {
		t.Fatalf("theme does not contain configured accent: %s", theme)
	}
	for _, token := range result.Manifest.Theme.TokenNames {
		if !strings.Contains(theme, token+":") || !strings.HasPrefix(token, "--taawun-") {
			t.Fatalf("theme token %q is missing or not namespaced", token)
		}
	}
	appCSS := string(readBundleFile(t, result.Directory, "app.css"))
	if !strings.Contains(appCSS, baselineContrastCSS) {
		t.Fatal("hero baseline must keep an opaque high-contrast surface above every accent gradient")
	}
}

func TestAccentForegroundAlwaysMeetsNormalTextContrast(t *testing.T) {
	accents := []string{
		"#57A68E", // cockpit default
		"#F4ECDD", // light
		"#777777", // mid-luminance fallback edge
		"#0F766E", // dark
		"#161A17", // very dark
	}
	for _, accent := range accents {
		foreground := contrastColor(accent)
		ratio := contrastRatio(relativeLuminance(accent), relativeLuminance(foreground))
		if ratio < 4.5 {
			t.Errorf("accent %s selected %s at %.2f:1, want at least 4.5:1", accent, foreground, ratio)
		}
	}
	if got := contrastColor("#57A68E"); got != "#161A17" {
		t.Fatalf("default accent foreground = %s, want Swiss ink", got)
	}
	if ratio := contrastRatio(relativeLuminance("#161A17"), relativeLuminance("#FFFFFF")); ratio < 4.5 {
		t.Fatalf("baseline ink/white contrast = %.2f:1, want at least 4.5:1", ratio)
	}
}

func TestBuildIsStableAndNeverOverwritesExistingArtifact(t *testing.T) {
	builder := newTestBuilder(t)
	firstRequest := validBuildRequest()
	first, err := builder.Build(context.Background(), firstRequest)
	if err != nil {
		t.Fatalf("first Build() returned an error: %v", err)
	}
	manifestBefore := readBundleFile(t, first.Directory, "manifest.json")

	builder.now = func() time.Time { return time.Date(2026, 8, 17, 3, 4, 5, 0, time.UTC) }
	secondRequest := firstRequest
	secondRequest.Modules = []string{ModuleDonationCampaign, ModuleRegistration, ModuleAnnouncements}
	secondRequest.AllowedOrigins.Embedders = []string{"https://community.example"}
	second, err := builder.Build(context.Background(), secondRequest)
	if err != nil {
		t.Fatalf("second Build() returned an error: %v", err)
	}
	if second.ContentHash != first.ContentHash || second.ArtifactID != first.ArtifactID || second.Directory != first.Directory {
		t.Fatalf("equivalent build was not deduplicated: first=%#v second=%#v", first, second)
	}
	if after := readBundleFile(t, first.Directory, "manifest.json"); !reflect.DeepEqual(after, manifestBefore) {
		t.Fatal("equivalent build overwrote the immutable manifest")
	}

	changedRequest := firstRequest
	changedRequest.Theme.AccentColor = "#7C3AED"
	changed, err := builder.Build(context.Background(), changedRequest)
	if err != nil {
		t.Fatalf("changed Build() returned an error: %v", err)
	}
	if changed.ContentHash == first.ContentHash || changed.Directory == first.Directory {
		t.Fatal("output-affecting theme change reused the old content address")
	}

	rootEntries, err := os.ReadDir(builder.root)
	if err != nil {
		t.Fatalf("ReadDir(root): %v", err)
	}
	for _, entry := range rootEntries {
		if strings.HasPrefix(entry.Name(), ".taawun-build-") {
			t.Fatalf("temporary build directory was left behind: %s", entry.Name())
		}
	}
}

func TestBuildDetectsTamperingInsteadOfOverwriting(t *testing.T) {
	builder := newTestBuilder(t)
	request := validBuildRequest()
	first, err := builder.Build(context.Background(), request)
	if err != nil {
		t.Fatalf("Build() returned an error: %v", err)
	}
	themePath := filepath.Join(first.Directory, "theme.css")
	if err := os.WriteFile(themePath, []byte("tampered\n"), 0o644); err != nil {
		t.Fatalf("tamper test fixture: %v", err)
	}

	_, err = builder.Build(context.Background(), request)
	if !errors.Is(err, ErrArtifactConflict) {
		t.Fatalf("Build() error = %v, want ErrArtifactConflict", err)
	}
	if got := string(readBundleFile(t, first.Directory, "theme.css")); got != "tampered\n" {
		t.Fatalf("Build() overwrote existing content: %q", got)
	}
}

func TestManifestDefinesExactSignedDatastarOriginAndCSPContract(t *testing.T) {
	builder := newTestBuilder(t)
	request := validBuildRequest()
	request.AllowedOrigins = OriginPolicy{
		Surfaces:    []string{"https://cards.example"},
		Embedders:   []string{"https://z.example", "https://a.example"},
		Connections: []string{"https://events.example", "https://api.example"},
		Resources:   []string{"https://cdn.example"},
	}
	result, err := builder.Build(context.Background(), request)
	if err != nil {
		t.Fatalf("Build() returned an error: %v", err)
	}

	wantOrigins := OriginPolicy{
		Surfaces:    []string{"https://cards.example"},
		Embedders:   []string{"https://a.example", "https://z.example"},
		Connections: []string{"https://api.example", "https://events.example"},
		Resources:   []string{"https://cdn.example"},
	}
	if !reflect.DeepEqual(result.Manifest.Security.AllowedOrigins, wantOrigins) {
		t.Fatalf("allowed origins = %#v, want exact normalized origins %#v", result.Manifest.Security.AllowedOrigins, wantOrigins)
	}

	modeByID := make(map[RenderMode]RenderModeDescriptor)
	for _, mode := range result.Manifest.RenderModes {
		modeByID[mode.ID] = mode
	}
	if !modeByID[RenderModeStandalone].UsesDatastar || !modeByID[RenderModeEmbed].UsesDatastar {
		t.Fatalf("standalone/embed render metadata = %#v", modeByID)
	}
	if len(modeByID) != 2 {
		t.Fatalf("unexpected render modes = %#v", modeByID)
	}

	policies := make(map[RenderMode]CSPPolicy)
	for _, entry := range result.Manifest.Security.CSP {
		policies[entry.RenderMode] = entry.Policy
	}
	for _, mode := range []RenderMode{RenderModeStandalone, RenderModeEmbed} {
		if !contains(policies[mode].ScriptSrc, "'unsafe-eval'") {
			t.Fatalf("%s CSP must explicitly support Datastar expression evaluation: %#v", mode, policies[mode])
		}
		for _, origin := range wantOrigins.Connections {
			if !contains(policies[mode].ConnectSrc, origin) {
				t.Fatalf("%s connect-src omits %s: %#v", mode, origin, policies[mode].ConnectSrc)
			}
		}
	}
	if !reflect.DeepEqual(policies[RenderModeEmbed].FrameAncestors, wantOrigins.Embedders) {
		t.Fatalf("embed frame ancestors = %v, want %v", policies[RenderModeEmbed].FrameAncestors, wantOrigins.Embedders)
	}
	if !reflect.DeepEqual(result.Manifest.Authorization.ApprovedDomains, []string{"cards.example"}) {
		t.Fatalf("approved domains must derive only from surface hosts: %v", result.Manifest.Authorization.ApprovedDomains)
	}
}

func TestBundleTreatsConfigurationAsDataAndContainsNoUserCode(t *testing.T) {
	builder := newTestBuilder(t)
	request := validBuildRequest()
	request.AppName = `Community "quoted"; alert(1)`
	request.OrganizationName = `Mercy's Center (North)`

	result, err := builder.Build(context.Background(), request)
	if err != nil {
		t.Fatalf("Build() returned an error: %v", err)
	}
	for _, relativePath := range bundleFilePaths(t, result.Directory) {
		extension := filepath.Ext(relativePath)
		if extension != ".json" && extension != ".css" && extension != ".html" && extension != ".js" {
			t.Fatalf("bundle contains executable or unknown artifact %q", relativePath)
		}
		contents := readBundleFile(t, result.Directory, relativePath)
		if extension == ".json" && !json.Valid(contents) {
			t.Fatalf("%s is not valid data-only JSON", relativePath)
		}
		if (extension == ".css" || extension == ".js") && (strings.Contains(string(contents), request.AppName) || strings.Contains(string(contents), request.OrganizationName)) {
			t.Fatalf("user input leaked into executable asset: %s", relativePath)
		}
	}

	var descriptor TemplateDescriptor
	if err := json.Unmarshal(readBundleFile(t, result.Directory, "template.json"), &descriptor); err != nil {
		t.Fatalf("decode template descriptor: %v", err)
	}
	if descriptor.Configuration.AppName != request.AppName || descriptor.Configuration.OrganizationName != request.OrganizationName {
		t.Fatalf("configuration did not round-trip as JSON data: %#v", descriptor.Configuration)
	}
	adapters := string(readBundleFile(t, result.Directory, "render-adapters.json"))
	for _, forbidden := range []string{"<script", "javascript:", "http://", "mcp-app", "appbridge"} {
		if strings.Contains(strings.ToLower(adapters), forbidden) {
			t.Fatalf("render adapter bakes unsafe code or a direct endpoint %q", forbidden)
		}
	}
	index := string(readBundleFile(t, result.Directory, "index.html"))
	if !strings.Contains(index, "Community &#34;quoted&#34;; alert(1)") || strings.Contains(index, `<script>alert(1)</script>`) {
		t.Fatalf("user slot was not safely escaped in preview HTML: %s", index)
	}
	if !strings.Contains(index, datastarRuntimePath) || strings.Contains(index, "cdn.") {
		t.Fatalf("preview does not use the pinned same-origin runtime: %s", index)
	}
}

func TestComponentDocumentsDriveEscapedSignedOutputDeterministically(t *testing.T) {
	builder := newTestBuilder(t)
	request := validBuildRequest()
	request.Components = []ComponentInstance{
		{ID: ModuleRegistration, Type: ModuleRegistration, Data: defaultComponentDocument(ModuleRegistration)},
		{ID: ModuleAnnouncements, Type: ModuleAnnouncements, Data: json.RawMessage(`{"zeta":{"b":2,"a":1},"summary":"<script>alert(1)</script>","title":"Trusted <updates>","audiences":["families",3,true]}`)},
		{ID: ModuleDonationCampaign, Type: ModuleDonationCampaign, Data: defaultComponentDocument(ModuleDonationCampaign)},
	}
	first, err := builder.Build(context.Background(), request)
	if err != nil {
		t.Fatalf("Build(component documents): %v", err)
	}
	if first.Manifest.ContractVersion != ManifestContractVersion || first.Manifest.Signature.ContractVersion != SignatureContractVersion || len(first.Manifest.Components) != 3 {
		t.Fatalf("v2 component manifest = %#v", first.Manifest)
	}
	var announcement ComponentManifest
	for _, component := range first.Manifest.Components {
		if component.Type == ModuleAnnouncements {
			announcement = component
		}
	}
	wantDocument := `{"audiences":["families",3,true],"summary":"\u003cscript\u003ealert(1)\u003c/script\u003e","title":"Trusted \u003cupdates\u003e","zeta":{"a":1,"b":2}}`
	if string(announcement.Data) != wantDocument || announcement.DocumentSHA256 != componentDocumentDigest(announcement.Data) {
		t.Fatalf("announcement binding = %#v", announcement)
	}
	if got := string(readBundleFile(t, first.Directory, announcement.DocumentPath)); got != wantDocument {
		t.Fatalf("stored document = %q", got)
	}
	index := string(readBundleFile(t, first.Directory, "index.html"))
	if strings.Contains(index, "<script>alert(1)</script>") || !strings.Contains(index, "&lt;script&gt;alert(1)&lt;/script&gt;") || !strings.Contains(index, "Trusted &lt;updates&gt;") || !strings.Contains(index, "audiences") {
		t.Fatalf("component data was not generically escaped: %s", index)
	}

	reordered := request
	reordered.Components = []ComponentInstance{
		{ID: ModuleDonationCampaign, Type: ModuleDonationCampaign, Data: defaultComponentDocument(ModuleDonationCampaign)},
		{ID: ModuleAnnouncements, Type: ModuleAnnouncements, Data: json.RawMessage(`{"title":"Trusted <updates>","audiences":["families",3,true],"zeta":{"a":1,"b":2},"summary":"<script>alert(1)</script>"}`)},
		{ID: ModuleRegistration, Type: ModuleRegistration, Data: defaultComponentDocument(ModuleRegistration)},
	}
	second, err := builder.Build(context.Background(), reordered)
	if err != nil || second.ContentHash != first.ContentHash || second.ArtifactID != first.ArtifactID {
		t.Fatalf("canonical rebuild = %#v err=%v, want %s", second, err, first.ContentHash)
	}

	changed := request
	changed.Components = append([]ComponentInstance(nil), request.Components...)
	changed.Components[1].Data = json.RawMessage(`{"summary":"A changed summary","title":"Trusted updates"}`)
	third, err := builder.Build(context.Background(), changed)
	if err != nil || third.ContentHash == first.ContentHash {
		t.Fatalf("changed component hash = %#v err=%v", third, err)
	}
}

func TestOpenRejectsMissingOrTamperedV2ComponentDocuments(t *testing.T) {
	for _, test := range []struct {
		name   string
		tamper func(string, Manifest)
	}{
		{name: "missing", tamper: func(directory string, manifest Manifest) {
			_ = os.Remove(filepath.Join(directory, filepath.FromSlash(manifest.Components[0].DocumentPath)))
		}},
		{name: "tampered", tamper: func(directory string, manifest Manifest) {
			_ = os.WriteFile(filepath.Join(directory, filepath.FromSlash(manifest.Components[0].DocumentPath)), []byte(`{"summary":"tampered","title":"tampered"}`), 0o644)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			builder := newTestBuilder(t)
			result, err := builder.Build(context.Background(), validBuildRequest())
			if err != nil {
				t.Fatalf("Build(): %v", err)
			}
			test.tamper(result.Directory, result.Manifest)
			if _, err := builder.Open(context.Background(), result.ContentHash); !errors.Is(err, ErrArtifactConflict) {
				t.Fatalf("Open() error = %v, want artifact conflict", err)
			}
		})
	}
}

func TestOpenRetainsLegacyV1SignatureCompatibility(t *testing.T) {
	builder := newTestBuilder(t)
	createdAt := builder.now()
	origins := OriginPolicy{Surfaces: []string{"https://community.example"}}
	manifest := Manifest{
		ContractVersion: ManifestContractVersionV1, WorkspaceID: 42, AppName: "Legacy card",
		Template: TemplateIdentity{ID: TemplateCommunityIftar, Version: "1.0.0"},
		Security: SecurityPolicy{AllowedOrigins: origins},
		Authorization: BundleAuthorization{
			Version: SignatureContractVersionV1, Subject: BundleSubject{ID: "user:7", UserID: 7, WorkspaceID: 42},
			AllowedOrigins: origins, ApprovedDomains: []string{"community.example"}, ExpiresAt: createdAt.Add(time.Hour),
			SignerKeyID: builder.keyID, Lifecycle: BundleLifecyclePreview,
		},
	}
	contentHash, err := semanticManifestHash(manifest)
	if err != nil {
		t.Fatalf("legacy semantic hash: %v", err)
	}
	manifest.ArtifactID = "art_legacy_v1"
	manifest.ContentHash = contentHash
	manifest.CreatedAt = createdAt
	manifest.Signature = BundleSignature{ContractVersion: SignatureContractVersionV1, Algorithm: "Ed25519", KeyID: builder.keyID, SignedFields: append([]string(nil), signedManifestFieldsV1...)}
	payload, err := signaturePayload(manifest)
	if err != nil {
		t.Fatalf("legacy signature payload: %v", err)
	}
	manifest.Signature.Value = base64.RawURLEncoding.EncodeToString(ed25519.Sign(builder.privateKey, payload))
	directory, err := safeJoin(builder.root, contentHash)
	if err != nil || os.Mkdir(directory, 0o755) != nil {
		t.Fatalf("create legacy fixture: path=%s err=%v", directory, err)
	}
	encoded, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(directory, "manifest.json"), encoded, 0o644); err != nil {
		t.Fatalf("write legacy manifest: %v", err)
	}
	opened, err := builder.Open(context.Background(), contentHash)
	if err != nil || opened.Manifest.ContractVersion != ManifestContractVersionV1 || len(opened.Manifest.Components) != 0 {
		t.Fatalf("Open(legacy) = %#v err=%v", opened, err)
	}

	componentBearing := manifest
	componentBearing.Components = []ComponentManifest{{ID: ModuleAnnouncements, Type: ModuleAnnouncements, Data: defaultComponentDocument(ModuleAnnouncements)}}
	if err := VerifyManifestSignature(componentBearing, builder.privateKey.Public().(ed25519.PublicKey)); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("component-bearing v1 verification = %v", err)
	}
}

func TestCatalogCoversFullMVPAndBuildsModularCards(t *testing.T) {
	wantModules := []string{
		ModuleRegistration, ModuleAnnouncements, ModuleDonationCampaign, ModuleShuraGovernance,
		ModuleComplianceReview, ModuleBazaarLifecycle, ModuleZakat, ModuleQardHasan,
		ModuleVolunteerStipend, ModuleSandboxEscrow, ModuleRevenueSplit,
	}
	request := validBuildRequest()
	request.TemplateID = TemplateCommunityWorkspace
	request.Modules = wantModules
	request.Lifecycle = BundleLifecyclePublished
	request.ExpiresAt = time.Date(2035, 1, 1, 0, 0, 0, 0, time.UTC)
	result, err := newTestBuilder(t).Build(context.Background(), request)
	if err != nil {
		t.Fatalf("Build(full catalog) returned an error: %v", err)
	}
	for _, moduleID := range wantModules {
		if _, ok := GetModule(moduleID); !ok {
			t.Fatalf("GetModule(%q) returned no curated module", moduleID)
		}
		if _, err := os.Stat(filepath.Join(result.Directory, "cards", moduleID+".html")); err != nil {
			t.Fatalf("modular card %s was not generated: %v", moduleID, err)
		}
	}
	if len(ListTemplates()) < 3 || len(ListModules()) != len(wantModules) {
		t.Fatalf("public catalog incomplete: templates=%d modules=%d", len(ListTemplates()), len(ListModules()))
	}
}

func TestExpiredBundleRemainsOpenForAuditButFailsServingCheck(t *testing.T) {
	builder := newTestBuilder(t)
	result, err := builder.Build(context.Background(), validBuildRequest())
	if err != nil {
		t.Fatalf("Build() returned an error: %v", err)
	}
	builder.now = func() time.Time { return result.Manifest.Authorization.ExpiresAt.Add(time.Hour) }
	if _, err := builder.Open(context.Background(), result.ContentHash); err != nil {
		t.Fatalf("Open() hid expired audit history: %v", err)
	}
	if err := CheckManifestExpiry(result.Manifest, builder.now()); !errors.Is(err, ErrArtifactExpired) {
		t.Fatalf("CheckManifestExpiry() error = %v, want ErrArtifactExpired", err)
	}
}

func TestBuildPathStaysInsideOutputRoot(t *testing.T) {
	builder := newTestBuilder(t)
	result, err := builder.Build(context.Background(), validBuildRequest())
	if err != nil {
		t.Fatalf("Build() returned an error: %v", err)
	}
	relative, err := filepath.Rel(builder.root, result.Directory)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		t.Fatalf("bundle path %q escapes root %q", result.Directory, builder.root)
	}
	if _, err := safeJoin(builder.root, "../escape"); !errors.Is(err, ErrInvalidOutputRoot) {
		t.Fatalf("safeJoin traversal error = %v, want ErrInvalidOutputRoot", err)
	}
}

func newTestBuilder(t *testing.T) *Builder {
	t.Helper()
	seed := sha256.Sum256([]byte("taawun artifact test signer"))
	privateKey := ed25519.NewKeyFromSeed(seed[:])
	builder, err := NewSignedBuilder(filepath.Join(t.TempDir(), "artifacts"), SigningConfig{KeyID: "test-key-1", PrivateKey: privateKey})
	if err != nil {
		t.Fatalf("NewBuilder() returned an error: %v", err)
	}
	fixed := time.Date(2026, 8, 16, 18, 30, 0, 0, time.UTC)
	builder.now = func() time.Time { return fixed }
	return builder
}

func bundleFilePaths(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		t.Fatalf("walk bundle: %v", err)
	}
	sort.Strings(paths)
	return paths
}

func readBundleFile(t *testing.T, root, relativePath string) []byte {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relativePath)))
	if err != nil {
		t.Fatalf("read %s: %v", relativePath, err)
	}
	return contents
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
