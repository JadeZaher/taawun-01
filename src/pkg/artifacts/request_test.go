package artifacts

import (
	"errors"
	"strings"
	"testing"
	"time"

	"taawun/pkg/ethics"
)

func TestValidateRequest(t *testing.T) {
	valid := validBuildRequest()
	if err := ValidateRequest(valid); err != nil {
		t.Fatalf("ValidateRequest() returned an error for valid input: %v", err)
	}

	tests := []struct {
		name    string
		mutate  func(*BuildRequest)
		wantErr error
	}{
		{name: "unknown template", mutate: func(r *BuildRequest) { r.TemplateID = "custom" }, wantErr: ErrInvalidTemplate},
		{name: "workspace missing", mutate: func(r *BuildRequest) { r.WorkspaceID = 0 }, wantErr: ErrInvalidBuildRequest},
		{name: "app name missing", mutate: func(r *BuildRequest) { r.AppName = "" }, wantErr: ErrInvalidBuildRequest},
		{name: "organization padded", mutate: func(r *BuildRequest) { r.OrganizationName = " Masjid " }, wantErr: ErrInvalidBuildRequest},
		{name: "city control character", mutate: func(r *BuildRequest) { r.City = "Denver\nColorado" }, wantErr: ErrInvalidBuildRequest},
		{name: "madhhab unknown", mutate: func(r *BuildRequest) { r.Madhhab = "other" }, wantErr: ErrInvalidBuildRequest},
		{name: "short accent", mutate: func(r *BuildRequest) { r.Theme.AccentColor = "#abc" }, wantErr: ErrInvalidBuildRequest},
		{name: "accent injection", mutate: func(r *BuildRequest) { r.Theme.AccentColor = "red;display:none" }, wantErr: ErrInvalidBuildRequest},
		{name: "missing modules", mutate: func(r *BuildRequest) { r.Modules = nil }, wantErr: ErrInvalidBuildRequest},
		{name: "unknown module", mutate: func(r *BuildRequest) { r.Modules = []string{"custom"} }, wantErr: ErrInvalidBuildRequest},
		{name: "module incompatible with template", mutate: func(r *BuildRequest) { r.Modules = []string{ModuleZakat} }, wantErr: ErrInvalidBuildRequest},
		{name: "duplicate module", mutate: func(r *BuildRequest) { r.Modules = []string{ModuleRegistration, ModuleRegistration} }, wantErr: ErrInvalidBuildRequest},
		{name: "origin path", mutate: func(r *BuildRequest) { r.AllowedOrigins.Embedders = []string{"https://example.com/path"} }, wantErr: ErrInvalidBuildRequest},
		{name: "insecure remote origin", mutate: func(r *BuildRequest) { r.AllowedOrigins.Connections = []string{"http://example.com"} }, wantErr: ErrInvalidBuildRequest},
		{name: "wildcard origin", mutate: func(r *BuildRequest) { r.AllowedOrigins.Resources = []string{"https://*.example.com"} }, wantErr: ErrInvalidBuildRequest},
		{name: "missing surface", mutate: func(r *BuildRequest) { r.AllowedOrigins.Surfaces = nil }, wantErr: ErrInvalidBuildRequest},
		{name: "missing subject", mutate: func(r *BuildRequest) { r.Subject = SubjectBinding{} }, wantErr: ErrInvalidBuildRequest},
		{name: "invalid lifecycle", mutate: func(r *BuildRequest) { r.Lifecycle = "forever" }, wantErr: ErrInvalidBuildRequest},
		{name: "overlong app", mutate: func(r *BuildRequest) { r.AppName = strings.Repeat("a", 81) }, wantErr: ErrInvalidBuildRequest},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := valid
			if test.mutate != nil {
				test.mutate(&request)
			}
			if err := ValidateRequest(request); !errors.Is(err, test.wantErr) {
				t.Fatalf("ValidateRequest() error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestDecodeBuildRequestIsStrict(t *testing.T) {
	request, err := DecodeBuildRequest(strings.NewReader(`{
  "workspaceId": 42,
  "appName": "Community Iftar",
  "organizationName": "Mercy Community Center",
  "city": "Denver",
  "madhhab": "hanafi",
  "templateId": "community-iftar",
  "theme": {},
  "modules": ["iftar-registration"],
  "allowedOrigins": {"surfaces": ["https://community.example"], "connections": ["https://app.example"]},
  "subject": {"id":"user:7","userId":7},
  "expiresAt":"2026-09-01T00:00:00Z",
  "lifecycle":"preview"
}`))
	if err != nil {
		t.Fatalf("DecodeBuildRequest() returned an error: %v", err)
	}
	if request.WorkspaceID != 42 || request.TemplateID != TemplateCommunityIftar {
		t.Fatalf("DecodeBuildRequest() = %#v", request)
	}

	for _, body := range []string{
		`{"workspaceId":42,"unknown":true}`,
		`{} {}`,
	} {
		if _, err := DecodeBuildRequest(strings.NewReader(body)); !errors.Is(err, ErrInvalidBuildRequest) {
			t.Fatalf("DecodeBuildRequest(%q) error = %v, want ErrInvalidBuildRequest", body, err)
		}
	}
}

func TestValidateRequestRejectsPathAndMarkupInput(t *testing.T) {
	for _, unsafe := range []string{"../outside", `folder\\outside`, "<script>", "two..dots"} {
		t.Run(unsafe, func(t *testing.T) {
			request := validBuildRequest()
			request.AppName = unsafe
			if err := ValidateRequest(request); !errors.Is(err, ErrInvalidBuildRequest) {
				t.Fatalf("ValidateRequest() error = %v, want ErrInvalidBuildRequest", err)
			}
		})
	}
}

func validBuildRequest() BuildRequest {
	return BuildRequest{
		WorkspaceID:      42,
		AppName:          "Community Iftar",
		OrganizationName: "Mercy Community Center",
		City:             "Denver",
		Madhhab:          ethics.MadhhabHanafi,
		TemplateID:       TemplateCommunityIftar,
		Theme:            ThemeRequest{AccentColor: "#0F766E"},
		Modules:          []string{ModuleRegistration, ModuleAnnouncements, ModuleDonationCampaign},
		AllowedOrigins: OriginPolicy{
			Surfaces:    []string{"https://community.example"},
			Embedders:   []string{"https://community.example"},
			Connections: []string{"https://app.example"},
			Resources:   []string{"https://assets.example"},
		},
		Subject:   SubjectBinding{ID: "user:7", UserID: 7},
		ExpiresAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		Lifecycle: BundleLifecyclePreview,
	}
}
