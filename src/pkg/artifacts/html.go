package artifacts

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"sort"
	"strings"
)

const baselineContrastCSS = `.taawun-baseline{border-color:#FFFFFF;color:#FFFFFF;background:var(--taawun-color-ink)}`

type previewView struct {
	Language         string
	Title            string
	AppName          string
	OrganizationName string
	City             string
	Madhhab          string
	Mode             string
	AssetPrefix      string
	RuntimePath      string
	RuntimeIntegrity string
	CSP              string
	Cards            []template.HTML
}

type cardView struct {
	ID               string
	Title            string
	OrganizationName string
	City             string
	Madhhab          string
	References       []ComplianceReference
	Summary          string
	CustomFields     []componentFieldView
}

type componentFieldView struct {
	Key   string
	Value string
}

func renderPreviewDocuments(request BuildRequest, definition templateDefinition, modules []ModuleDescriptor, references []ComplianceReference, security SecurityPolicy) (map[string][]byte, error) {
	standaloneCSP := cspForMode(security, RenderModeStandalone)
	embedCSP := cspForMode(security, RenderModeEmbed)
	standaloneCards, err := renderCards(request, modules, references)
	if err != nil {
		return nil, err
	}
	standalone := previewView{
		Language: "en", Title: definition.Title, AppName: request.AppName,
		OrganizationName: request.OrganizationName, City: request.City, Madhhab: string(request.Madhhab),
		Mode: "standalone", AssetPrefix: ".", RuntimePath: datastarRuntimePath,
		RuntimeIntegrity: datastarSRI(), CSP: cspHeaderValue(standaloneCSP), Cards: standaloneCards,
	}
	indexHTML, err := executeHTML(documentTemplate, standalone)
	if err != nil {
		return nil, fmt.Errorf("render standalone preview: %w", err)
	}

	embed := standalone
	embed.Mode = "embed"
	embed.CSP = cspHeaderValue(embedCSP)
	embedHTML, err := executeHTML(documentTemplate, embed)
	if err != nil {
		return nil, fmt.Errorf("render embed preview: %w", err)
	}

	files := map[string][]byte{
		"index.html":      indexHTML,
		"embed.html":      embedHTML,
		"app.css":         []byte(applicationCSS + baselineContrastCSS),
		"embed-loader.js": renderEmbedLoader(request.Modules),
	}
	for _, module := range modules {
		card, err := renderCard(module.ID, request, references)
		if err != nil {
			return nil, err
		}
		view := previewView{
			Language: "en", Title: module.Title, AppName: request.AppName,
			OrganizationName: request.OrganizationName, City: request.City, Madhhab: string(request.Madhhab),
			Mode: "card", AssetPrefix: "..", RuntimePath: datastarRuntimePath,
			RuntimeIntegrity: datastarSRI(), CSP: cspHeaderValue(embedCSP), Cards: []template.HTML{card},
		}
		encoded, err := executeHTML(documentTemplate, view)
		if err != nil {
			return nil, fmt.Errorf("render card %s: %w", module.ID, err)
		}
		files["cards/"+module.ID+".html"] = encoded
	}
	return files, nil
}

func renderCards(request BuildRequest, modules []ModuleDescriptor, references []ComplianceReference) ([]template.HTML, error) {
	cards := make([]template.HTML, 0, len(modules))
	for _, module := range modules {
		card, err := renderCard(module.ID, request, references)
		if err != nil {
			return nil, err
		}
		cards = append(cards, card)
	}
	return cards, nil
}

func renderCard(moduleID string, request BuildRequest, references []ComplianceReference) (template.HTML, error) {
	module, supported := moduleCatalog[moduleID]
	if !supported {
		return "", fmt.Errorf("render unsupported module %q", moduleID)
	}
	component, found := componentForType(request.Components, moduleID)
	if !found {
		return "", fmt.Errorf("component %q has no canonical document", moduleID)
	}
	title, summary, custom, err := componentCardData(component)
	if err != nil {
		return "", fmt.Errorf("render component %s: %w", moduleID, err)
	}
	view := cardView{
		ID: module.ID, Title: title, OrganizationName: request.OrganizationName,
		City: request.City, Madhhab: string(request.Madhhab), References: references, Summary: summary, CustomFields: custom,
	}
	cardTemplate, ok := cardTemplates[moduleID]
	if !ok {
		return "", fmt.Errorf("module %q has no curated card template", moduleID)
	}
	encoded, err := executeHTML(cardTemplate, view)
	if err != nil {
		return "", fmt.Errorf("render module %s: %w", moduleID, err)
	}
	document, err := executeHTML(componentDocumentTemplate, view)
	if err != nil {
		return "", fmt.Errorf("render component document %s: %w", moduleID, err)
	}
	return template.HTML(append(encoded, document...)), nil
}

func componentForType(components []ComponentInstance, moduleID string) (ComponentInstance, bool) {
	for _, component := range components {
		if component.Type == moduleID {
			return component, true
		}
	}
	return ComponentInstance{}, false
}

func componentCardData(component ComponentInstance) (string, string, []componentFieldView, error) {
	decoder := json.NewDecoder(bytes.NewReader(component.Data))
	decoder.UseNumber()
	var document map[string]any
	if err := decoder.Decode(&document); err != nil {
		return "", "", nil, err
	}
	title, _ := document["title"].(string)
	summary, _ := document["summary"].(string)
	keys := make([]string, 0, len(document))
	for key := range document {
		if key != "title" && key != "summary" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	custom := make([]componentFieldView, 0, len(keys))
	for _, key := range keys {
		value := document[key]
		formatted, ok := value.(string)
		if !ok {
			encoded, err := json.Marshal(value)
			if err != nil {
				return "", "", nil, err
			}
			formatted = string(encoded)
		}
		custom = append(custom, componentFieldView{Key: key, Value: formatted})
	}
	return title, summary, custom, nil
}

func executeHTML(source string, value any) ([]byte, error) {
	template, err := template.New("artifact").Parse(source)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	if err := template.Execute(&output, value); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func renderEmbedLoader(moduleIDs []string) []byte {
	ids := append([]string(nil), moduleIDs...)
	sort.Strings(ids)
	allowedJSON, _ := json.Marshal(ids)
	return []byte(fmt.Sprintf(`(() => {
  "use strict";
  const script = document.currentScript;
  const base = new URL("./", script.src);
  const allowed = new Set(%s);
  class TaawunCard extends HTMLElement {
    connectedCallback() {
      if (this.firstChild) return;
      const requested = this.getAttribute("module");
      const target = requested && allowed.has(requested) ? "cards/" + requested + ".html" : "embed.html";
      const frame = document.createElement("iframe");
      frame.src = new URL(target, base).href;
      frame.title = this.getAttribute("title") || "Taawun community card";
      frame.loading = "lazy";
      frame.sandbox = "allow-forms allow-scripts allow-same-origin";
      frame.referrerPolicy = "no-referrer";
	  frame.width = "100%%";
	  frame.height = "640";
	  frame.setAttribute("frameborder", "0");
      this.append(frame);
    }
  }
  if (!customElements.get("taawun-card")) customElements.define("taawun-card", TaawunCard);
})();
`, allowedJSON))
}

func cspForMode(security SecurityPolicy, mode RenderMode) CSPPolicy {
	for _, entry := range security.CSP {
		if entry.RenderMode == mode {
			return entry.Policy
		}
	}
	return CSPPolicy{DefaultSrc: []string{"'none'"}}
}

func cspHeaderValue(policy CSPPolicy) string {
	directives := []struct {
		name   string
		values []string
	}{
		{"default-src", policy.DefaultSrc},
		{"script-src", policy.ScriptSrc},
		{"style-src", policy.StyleSrc},
		{"connect-src", policy.ConnectSrc},
		{"img-src", policy.ImageSrc},
		{"font-src", policy.FontSrc},
		{"object-src", policy.ObjectSrc},
		{"base-uri", policy.BaseURI},
		{"form-action", policy.FormAction},
	}
	parts := make([]string, 0, len(directives))
	for _, directive := range directives {
		if len(directive.values) > 0 {
			parts = append(parts, directive.name+" "+strings.Join(directive.values, " "))
		}
	}
	return strings.Join(parts, "; ")
}

func datastarSRI() string {
	digest, _ := hex.DecodeString(datastarRuntimeSHA256)
	return "sha256-" + base64.StdEncoding.EncodeToString(digest)
}

const documentTemplate = `<!doctype html>
<html lang="{{.Language}}">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <meta name="referrer" content="no-referrer">
  <meta http-equiv="Content-Security-Policy" content="{{.CSP}}">
  <title>{{.AppName}} · {{.Title}}</title>
  <link rel="stylesheet" href="{{.AssetPrefix}}/theme.css">
  <link rel="stylesheet" href="{{.AssetPrefix}}/app.css">
  <script type="module" src="{{.RuntimePath}}" integrity="{{.RuntimeIntegrity}}"></script>
</head>
<body class="taawun-surface taawun-mode-{{.Mode}}" data-taawun-theme
  data-signals="{_notice: 'Preview ready', _registered: false, _attendee: '', _partySize: 1, _zakatAssets: 0, _zakatEstimate: 0}">
  <header class="taawun-hero">
    <div><span class="taawun-kicker">{{.OrganizationName}} · {{.City}}</span><h1>{{.AppName}}</h1></div>
    <span class="taawun-baseline">{{.Madhhab}} reference baseline</span>
  </header>
  <aside class="taawun-notice" role="status" aria-live="polite" data-text="$_notice">Preview ready</aside>
  <main class="taawun-grid">{{range .Cards}}{{.}}{{end}}</main>
  <footer><strong>Taawun sandbox</strong><span>No custody, ledger, transfer, or settlement. Compliance sources require qualified review.</span></footer>
</body>
</html>
`

const componentDocumentTemplate = `<aside class="taawun-card taawun-component-document" data-component-id="{{.ID}}"><div class="taawun-card-head"><span>Component data</span><h3>Signed content</h3></div><p>{{.Summary}}</p>{{if .CustomFields}}<dl>{{range .CustomFields}}<dt>{{.Key}}</dt><dd>{{.Value}}</dd>{{end}}</dl>{{else}}<small>No custom fields in this component.</small>{{end}}</aside>`

var cardTemplates = map[string]string{
	ModuleRegistration:     `<section class="taawun-card" id="card-iftar-registration"><div class="taawun-card-head"><span>Community</span><h2>{{.Title}}</h2></div><form data-on:submit="$_registered = true; $_notice = 'Registration preview saved for this tab'" class="taawun-form"><label>Name<input required maxlength="80" autocomplete="name" data-bind:_attendee></label><label>Party size<input required type="number" min="1" max="20" value="1" data-bind:_party-size></label><button type="submit">Preview registration</button><p class="taawun-success" data-show="$_registered">JazakAllahu khayran—your preview registration is ready.</p></form></section>`,
	ModuleAnnouncements:    `<section class="taawun-card" id="card-announcements"><div class="taawun-card-head"><span>Updates</span><h2>{{.Title}}</h2></div><article class="taawun-announcement"><strong>Iftar doors open 30 minutes before Maghrib</strong><p>Volunteer check-in and family seating details can be published here.</p></article><button type="button" data-on:click="$_notice = 'Announcement composer is ready for an approved host binding'">Compose announcement</button></section>`,
	ModuleDonationCampaign: `<section class="taawun-card" id="card-donation-campaign"><div class="taawun-card-head"><span>Sandbox only</span><h2>{{.Title}}</h2></div><div class="taawun-progress"><span class="taawun-progress-value"></span></div><p><strong>$3,100 previewed</strong> of a $5,000 community goal.</p><button type="button" data-on:click="$_notice = 'Sandbox preview only — no funds moved and no ledger was created'">Preview donation intent</button><small>No custody, transfer, or settlement occurs in this card.</small></section>`,
	ModuleShuraGovernance:  `<section class="taawun-card" id="card-shura-governance"><div class="taawun-card-head"><span>Governance</span><h2>{{.Title}}</h2></div><article><strong>Extend weekly food pantry hours</strong><p>Review evidence, discuss impact, and record a transparent decision.</p></article><div class="taawun-actions"><button type="button" data-on:click="$_notice = 'Shura proposal draft opened'">Open proposal</button><span>Decision record is server-owned</span></div></section>`,
	ModuleComplianceReview: `<section class="taawun-card" id="card-compliance-source-review"><div class="taawun-card-head"><span>Reference only</span><h2>{{.Title}}</h2></div><ul class="taawun-sources">{{range .References}}<li><strong>{{.Title}}</strong><span>{{.Citation}} · {{.Status}}</span></li>{{end}}</ul><button type="button" data-on:click="$_notice = 'Qualified review requested — no scholar approval is claimed'">Request qualified review</button><small>Seed references are not a fatwa or scholar approval.</small></section>`,
	ModuleBazaarLifecycle:  `<section class="taawun-card" id="card-bazaar-template-lifecycle"><div class="taawun-card-head"><span>Bazaar</span><h2>{{.Title}}</h2></div><ol class="taawun-steps"><li class="active">Draft</li><li>Source review</li><li>Shura review</li><li>Published</li></ol><button type="button" data-on:click="$_notice = 'Bazaar template moved to local review preview'">Preview review</button></section>`,
	ModuleZakat:            `<section class="taawun-card" id="card-zakat"><div class="taawun-card-head"><span>Estimate</span><h2>{{.Title}}</h2></div><label>Eligible assets<input type="number" min="0" step="0.01" data-bind:_zakat-assets data-on:input="$_zakatEstimate = Math.max(0, $_zakatAssets * 0.025)"></label><p class="taawun-amount">Estimated Zakat: <strong data-text="'$' + $_zakatEstimate.toFixed(2)">$0.00</strong></p><small>Illustrative estimate only; confirm nisab, eligibility, and timing with a qualified reviewer.</small></section>`,
	ModuleQardHasan:        `<section class="taawun-card" id="card-qard-hasan"><div class="taawun-card-head"><span>Interest-free</span><h2>{{.Title}}</h2></div><p>Draft a benevolent assistance request with explicit amount, purpose, repayment expectations, and approvals.</p><button type="button" data-on:click="$_notice = 'Qard Hasan sandbox request drafted with no interest or APR'">Draft sandbox request</button><small>No interest, APR, or live disbursement.</small></section>`,
	ModuleVolunteerStipend: `<section class="taawun-card" id="card-volunteer-stipend"><div class="taawun-card-head"><span>Volunteer support</span><h2>{{.Title}}</h2></div><p>Record a transparent scope, hours, amount, and approver before a sandbox stipend request.</p><button type="button" data-on:click="$_notice = 'Volunteer stipend terms opened for review'">Review stipend terms</button><small>Stipends are server-approved sandbox records; this card does not pay anyone.</small></section>`,
	ModuleSandboxEscrow:    `<section class="taawun-card" id="card-sandbox-escrow"><div class="taawun-card-head"><span>Non-custodial preview</span><h2>{{.Title}}</h2></div><p>Preview milestones, release conditions, dispute contacts, and mutual confirmation.</p><button type="button" data-on:click="$_notice = 'Escrow preview created — no assets were held or settled'">Preview escrow intent</button><small>Taawun does not hold funds or assert settlement.</small></section>`,
	ModuleRevenueSplit:     `<section class="taawun-card" id="card-revenue-split"><div class="taawun-card-head"><span>Explicit terms</span><h2>{{.Title}}</h2></div><div class="taawun-split"><span>Provider 80%</span><span>Community fund 20%</span></div><button type="button" data-on:click="$_notice = 'Revenue split preview totals 100% — no disbursement occurred'">Preview split</button><small>Rules require confirmation and server approval before any external settlement.</small></section>`,
}

const applicationCSS = `*{box-sizing:border-box}body{margin:0;background:var(--taawun-color-surface-muted);color:var(--taawun-color-text);font-family:var(--taawun-font-sans);line-height:1.5}.taawun-hero{position:relative;display:flex;align-items:center;justify-content:space-between;gap:var(--taawun-space-6);overflow:hidden;padding:clamp(1.5rem,5vw,4rem);border-bottom:3px solid var(--taawun-color-ink);color:var(--taawun-color-on-accent);background:linear-gradient(112deg,var(--taawun-color-accent) 0 70%,var(--taawun-color-gold) 70% 76%,var(--taawun-color-ink) 76%)}.taawun-kicker,.taawun-card-head span{font-size:.75rem;font-weight:900;letter-spacing:.14em;text-transform:uppercase}.taawun-card-head span{display:inline-block;padding:.15rem .4rem;color:var(--taawun-color-ink);background:var(--taawun-color-gold)}.taawun-hero h1,.taawun-card h2{font-family:var(--taawun-font-display);letter-spacing:-.035em;text-transform:uppercase}.taawun-hero h1{margin:.35rem 0 0;font-size:clamp(2rem,5vw,4.5rem);line-height:.95}.taawun-baseline{padding:.5rem .8rem;border:2px solid currentColor;border-radius:var(--taawun-radius-control);font-weight:800;text-transform:uppercase}.taawun-notice{margin:1rem auto 0;max-width:76rem;padding:.8rem 1rem;border:2px solid var(--taawun-color-border);border-radius:var(--taawun-radius-control);background:var(--taawun-color-surface);box-shadow:3px 3px 0 var(--taawun-color-gold)}.taawun-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,20rem),1fr));gap:var(--taawun-space-8);max-width:80rem;margin:auto;padding:clamp(1rem,4vw,3rem)}.taawun-card{display:flex;flex-direction:column;gap:var(--taawun-space-4);min-height:18rem;padding:var(--taawun-space-6);border:2px solid var(--taawun-color-border);border-radius:var(--taawun-radius-card);background:var(--taawun-color-surface);box-shadow:var(--taawun-shadow-card)}.taawun-card h2{margin:.3rem 0;font-size:1.45rem;line-height:1.05}.taawun-card p{margin:.25rem 0;color:var(--taawun-color-text-muted)}.taawun-card small{display:block;color:var(--taawun-color-text-muted)}.taawun-card button{align-self:flex-start;padding:.7rem 1rem;border:2px solid var(--taawun-color-ink);border-radius:var(--taawun-radius-control);color:var(--taawun-color-on-accent);background:var(--taawun-color-accent);box-shadow:3px 3px 0 var(--taawun-color-ink);font:inherit;font-weight:900;letter-spacing:.04em;text-transform:uppercase;cursor:pointer}.taawun-card button:active{transform:translate(2px,2px);box-shadow:1px 1px 0 var(--taawun-color-ink)}.taawun-card input{width:100%;padding:.7rem;border:2px solid var(--taawun-color-border);border-radius:var(--taawun-radius-control);background:#fffdf7;font:inherit}.taawun-form{display:grid;gap:var(--taawun-space-3)}.taawun-form label,.taawun-card>label{display:grid;gap:.35rem;font-weight:800}.taawun-success{padding:.6rem;border:2px solid var(--taawun-color-accent);background:#e3f1df}.taawun-announcement{padding:1rem;border-left:6px solid var(--taawun-color-accent);background:var(--taawun-color-surface-muted)}.taawun-progress{height:.75rem;overflow:hidden;border:2px solid var(--taawun-color-ink);background:var(--taawun-color-surface-muted)}.taawun-progress span{display:block;height:100%;background:var(--taawun-color-accent)}.taawun-progress-value{width:62%}.taawun-actions,.taawun-split{display:flex;align-items:center;justify-content:space-between;gap:1rem}.taawun-actions span,.taawun-split span{font-size:.86rem;color:var(--taawun-color-text-muted)}.taawun-sources{display:grid;gap:.6rem;margin:0;padding:0;list-style:none}.taawun-sources li{display:grid;padding:.7rem;border:2px solid var(--taawun-color-border);border-radius:var(--taawun-radius-control)}.taawun-sources span{font-size:.82rem;color:var(--taawun-color-text-muted)}.taawun-steps{display:grid;grid-template-columns:repeat(4,1fr);gap:.3rem;padding:0;list-style:none;font-size:.78rem}.taawun-steps li{padding:.45rem;text-align:center;border:1px solid var(--taawun-color-ink);background:var(--taawun-color-surface-muted)}.taawun-steps .active{color:var(--taawun-color-on-accent);background:var(--taawun-color-accent)}.taawun-amount{font-size:1.15rem}footer{display:flex;justify-content:center;gap:.7rem;flex-wrap:wrap;padding:1.5rem;border-top:2px solid var(--taawun-color-ink);color:var(--taawun-color-text-muted);text-align:center}.taawun-mode-card .taawun-hero,.taawun-mode-card .taawun-notice,.taawun-mode-card footer{display:none}.taawun-mode-card .taawun-grid{display:block;padding:0}.taawun-mode-card .taawun-card{border:0;box-shadow:none}@media(max-width:42rem){.taawun-hero{align-items:flex-start;flex-direction:column;background:var(--taawun-color-accent)}.taawun-actions,.taawun-split{align-items:flex-start;flex-direction:column}.taawun-steps{grid-template-columns:1fr 1fr}}
`
