package ethics

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestParseMadhhab(t *testing.T) {
	tests := []struct {
		input string
		want  Madhhab
	}{
		{input: "Hanafi", want: MadhhabHanafi},
		{input: " Shafi'i ", want: MadhhabShafii},
		{input: "maliki", want: MadhhabMaliki},
		{input: "HANBALI", want: MadhhabHanbali},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := ParseMadhhab(test.input)
			if err != nil {
				t.Fatalf("ParseMadhhab(%q) returned an error: %v", test.input, err)
			}
			if got != test.want {
				t.Fatalf("ParseMadhhab(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}

	if _, err := ParseMadhhab("unknown"); !errors.Is(err, ErrInvalidMadhhab) {
		t.Fatalf("ParseMadhhab(unknown) error = %v, want ErrInvalidMadhhab", err)
	}
}

func TestNewComplianceCorpusValidatesAndDefensivelyCopiesRecords(t *testing.T) {
	valid := testComplianceRecord("one", MadhhabHanafi, "loan")
	corpus, err := NewComplianceCorpus([]ComplianceRecord{valid})
	if err != nil {
		t.Fatalf("NewComplianceCorpus() returned an error: %v", err)
	}

	valid.Keywords[0] = "changed"
	records := corpus.Records()
	if records[0].Keywords[0] != "loan" {
		t.Fatalf("corpus retained caller mutation: %v", records[0].Keywords)
	}
	records[0].Keywords[0] = "mutated-copy"
	if corpus.Records()[0].Keywords[0] != "loan" {
		t.Fatal("Records returned a mutable internal slice")
	}

	tests := []struct {
		name    string
		records []ComplianceRecord
		wantErr error
	}{
		{name: "empty", records: nil, wantErr: ErrInvalidComplianceRecord},
		{name: "missing citation", records: []ComplianceRecord{{ID: "one", Madhhab: MadhhabHanafi, Title: "Title", Guidance: "Guidance", Status: ComplianceRecordStatusSeed, Keywords: []string{"loan"}}}, wantErr: ErrInvalidComplianceRecord},
		{name: "invalid madhhab", records: []ComplianceRecord{testComplianceRecord("one", Madhhab("other"), "loan")}, wantErr: ErrInvalidMadhhab},
		{name: "invalid status", records: []ComplianceRecord{{ID: "one", Madhhab: MadhhabHanafi, Title: "Title", Guidance: "Guidance", Citation: "Citation", Status: "approved", Keywords: []string{"loan"}}}, wantErr: ErrInvalidComplianceRecord},
		{name: "missing keywords", records: []ComplianceRecord{{ID: "one", Madhhab: MadhhabHanafi, Title: "Title", Guidance: "Guidance", Citation: "Citation", Status: ComplianceRecordStatusSeed}}, wantErr: ErrInvalidComplianceRecord},
		{name: "duplicate id", records: []ComplianceRecord{testComplianceRecord("same", MadhhabHanafi, "loan"), testComplianceRecord("same", MadhhabShafii, "loan")}, wantErr: ErrInvalidComplianceRecord},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewComplianceCorpus(test.records)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("NewComplianceCorpus() error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestComplianceCorpusRetrieveFiltersRanksAndIsDeterministic(t *testing.T) {
	corpus, err := NewComplianceCorpus([]ComplianceRecord{
		testComplianceRecord("zeta", MadhhabHanafi, "loan interest"),
		testComplianceRecord("alpha", MadhhabHanafi, "loan interest"),
		testComplianceRecord("shafii", MadhhabShafii, "loan interest"),
		testComplianceRecord("market", MadhhabHanafi, "marketplace checkout"),
	})
	if err != nil {
		t.Fatalf("NewComplianceCorpus() returned an error: %v", err)
	}

	got, err := corpus.Retrieve("loan interest", MadhhabHanafi, 10)
	if err != nil {
		t.Fatalf("Retrieve() returned an error: %v", err)
	}
	if gotIDs := recordIDs(got); !reflect.DeepEqual(gotIDs, []string{"alpha", "zeta"}) {
		t.Fatalf("Retrieve() ids = %v, want [alpha zeta]", gotIDs)
	}

	limited, err := corpus.Retrieve("loan interest", MadhhabHanafi, 1)
	if err != nil {
		t.Fatalf("Retrieve() returned an error: %v", err)
	}
	if gotIDs := recordIDs(limited); !reflect.DeepEqual(gotIDs, []string{"alpha"}) {
		t.Fatalf("limited Retrieve() ids = %v, want [alpha]", gotIDs)
	}

	noMatches, err := corpus.Retrieve("unrelated gardening", MadhhabHanafi, 10)
	if err != nil {
		t.Fatalf("Retrieve() returned an error: %v", err)
	}
	if len(noMatches) != 0 {
		t.Fatalf("Retrieve() returned %v for an unrelated query, want no matches", noMatches)
	}

	got[0].Keywords[0] = "changed"
	again, err := corpus.Retrieve("loan interest", MadhhabHanafi, 10)
	if err != nil {
		t.Fatalf("Retrieve() returned an error: %v", err)
	}
	if again[0].Keywords[0] != "loan" {
		t.Fatal("Retrieve returned a mutable internal record")
	}
}

func TestComplianceCorpusRetrieveRejectsInvalidInputs(t *testing.T) {
	corpus, err := NewComplianceCorpus([]ComplianceRecord{testComplianceRecord("one", MadhhabHanafi, "loan")})
	if err != nil {
		t.Fatalf("NewComplianceCorpus() returned an error: %v", err)
	}

	tests := []struct {
		name    string
		corpus  *ComplianceCorpus
		query   string
		madhhab Madhhab
		limit   int
		wantErr error
	}{
		{name: "nil corpus", corpus: nil, query: "loan", madhhab: MadhhabHanafi, limit: 1, wantErr: ErrInvalidComplianceRecord},
		{name: "invalid madhhab", corpus: corpus, query: "loan", madhhab: "other", limit: 1, wantErr: ErrInvalidMadhhab},
		{name: "empty query", corpus: corpus, query: "---", madhhab: MadhhabHanafi, limit: 1, wantErr: ErrEmptyComplianceQuery},
		{name: "invalid limit", corpus: corpus, query: "loan", madhhab: MadhhabHanafi, limit: 0, wantErr: ErrInvalidComplianceLimit},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.corpus.Retrieve(test.query, test.madhhab, test.limit)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Retrieve() error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestInjectContextPrependsReferenceDisclaimerAndCitation(t *testing.T) {
	corpus, err := NewComplianceCorpus([]ComplianceRecord{testComplianceRecord("loan-record", MadhhabHanafi, "loan interest")})
	if err != nil {
		t.Fatalf("NewComplianceCorpus() returned an error: %v", err)
	}
	prompt := "Build a loan calculator without interest."

	injected, records, err := corpus.InjectContext(prompt, MadhhabHanafi, 2)
	if err != nil {
		t.Fatalf("InjectContext() returned an error: %v", err)
	}
	if len(records) != 1 || records[0].ID != "loan-record" {
		t.Fatalf("InjectContext() records = %v, want loan-record", records)
	}
	if !strings.HasSuffix(injected, prompt) {
		t.Fatalf("injected prompt does not preserve prompt suffix: %q", injected)
	}
	for _, want := range []string{
		"[Taawun compliance reference context]",
		"not a fatwa or scholar-approved ruling",
		"status: seed",
		"citation: Test citation",
		"Selected madhhab baseline: hanafi",
	} {
		if !strings.Contains(injected, want) {
			t.Fatalf("injected context missing %q: %q", want, injected)
		}
	}

	unchanged, noMatches, err := corpus.InjectContext("Build a garden planner.", MadhhabHanafi, 2)
	if err != nil {
		t.Fatalf("InjectContext() returned an error: %v", err)
	}
	if unchanged != "Build a garden planner." || len(noMatches) != 0 {
		t.Fatalf("unmatched InjectContext() = (%q, %v), want original prompt and no records", unchanged, noMatches)
	}
}

func TestSeedComplianceCorpusHasTaggedCitedReferenceRecords(t *testing.T) {
	corpus := NewSeedComplianceCorpus()
	records := corpus.Records()
	if len(records) == 0 {
		t.Fatal("NewSeedComplianceCorpus() returned no records")
	}

	seenMadhhab := make(map[Madhhab]bool)
	for _, record := range records {
		seenMadhhab[record.Madhhab] = true
		if record.Status != ComplianceRecordStatusSeed {
			t.Fatalf("seed record %q status = %q, want seed", record.ID, record.Status)
		}
		if strings.TrimSpace(record.Citation) == "" {
			t.Fatalf("seed record %q has no citation", record.ID)
		}
	}
	for _, madhhab := range []Madhhab{MadhhabHanafi, MadhhabShafii, MadhhabMaliki, MadhhabHanbali} {
		if !seenMadhhab[madhhab] {
			t.Fatalf("seed corpus has no %s records", madhhab)
		}
	}
}

func testComplianceRecord(id string, madhhab Madhhab, keywords string) ComplianceRecord {
	return ComplianceRecord{
		ID:       id,
		Madhhab:  madhhab,
		Title:    "Test guidance",
		Guidance: "Use explicit terms and request qualified review.",
		Citation: "Test citation",
		Status:   ComplianceRecordStatusSeed,
		Keywords: strings.Fields(keywords),
	}
}

func recordIDs(records []ComplianceRecord) []string {
	ids := make([]string, len(records))
	for i, record := range records {
		ids[i] = record.ID
	}
	return ids
}
