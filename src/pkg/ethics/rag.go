package ethics

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// Madhhab identifies the jurisprudential baseline selected by a workspace.
type Madhhab string

const (
	MadhhabHanafi  Madhhab = "hanafi"
	MadhhabShafii  Madhhab = "shafii"
	MadhhabMaliki  Madhhab = "maliki"
	MadhhabHanbali Madhhab = "hanbali"
)

var (
	// ErrInvalidMadhhab is returned when a workspace has not selected a supported baseline.
	ErrInvalidMadhhab = errors.New("unsupported madhhab")
	// ErrEmptyComplianceQuery is returned when no searchable terms are supplied.
	ErrEmptyComplianceQuery = errors.New("compliance query must contain searchable terms")
	// ErrInvalidComplianceLimit is returned when a retrieval limit is not positive.
	ErrInvalidComplianceLimit = errors.New("compliance retrieval limit must be positive")
	// ErrInvalidComplianceRecord is returned when a corpus record cannot be safely indexed.
	ErrInvalidComplianceRecord = errors.New("invalid compliance corpus record")
)

// ComplianceRecordStatus describes the curation state of a corpus record.
// A status never represents scholar approval or a fatwa.
type ComplianceRecordStatus string

const (
	// ComplianceRecordStatusSeed marks reference material awaiting qualified review.
	ComplianceRecordStatusSeed ComplianceRecordStatus = "seed"
)

// ComplianceRecord is a tagged piece of reference guidance for generation-time context.
// It is not a ruling and must not be represented to users as scholar-approved advice.
type ComplianceRecord struct {
	ID       string                 `json:"id"`
	Madhhab  Madhhab                `json:"madhhab"`
	Title    string                 `json:"title"`
	Guidance string                 `json:"guidance"`
	Citation string                 `json:"citation"`
	Status   ComplianceRecordStatus `json:"status"`
	Keywords []string               `json:"keywords"`
}

// ComplianceCorpus keeps a validated, immutable in-memory reference corpus.
type ComplianceCorpus struct {
	records []ComplianceRecord
}

// NewComplianceCorpus validates and copies records so callers cannot mutate retrieval state.
func NewComplianceCorpus(records []ComplianceRecord) (*ComplianceCorpus, error) {
	if len(records) == 0 {
		return nil, fmt.Errorf("%w: corpus cannot be empty", ErrInvalidComplianceRecord)
	}

	corpus := &ComplianceCorpus{records: make([]ComplianceRecord, len(records))}
	seenIDs := make(map[string]struct{}, len(records))
	for i, record := range records {
		if err := validateComplianceRecord(record, seenIDs); err != nil {
			return nil, err
		}
		corpus.records[i] = cloneComplianceRecord(record)
	}
	return corpus, nil
}

// NewSeedComplianceCorpus returns the built-in reference-only seed corpus.
// The seed material is deliberately labelled as unapproved reference context.
func NewSeedComplianceCorpus() *ComplianceCorpus {
	corpus, err := NewComplianceCorpus(seedComplianceRecords())
	if err != nil {
		panic(fmt.Sprintf("invalid built-in compliance corpus: %v", err))
	}
	return corpus
}

// Records returns a defensive copy of the corpus records in insertion order.
func (c *ComplianceCorpus) Records() []ComplianceRecord {
	if c == nil {
		return nil
	}
	records := make([]ComplianceRecord, len(c.records))
	for i, record := range c.records {
		records[i] = cloneComplianceRecord(record)
	}
	return records
}

// Retrieve returns deterministic, relevant records for the workspace's selected madhhab.
func (c *ComplianceCorpus) Retrieve(query string, madhhab Madhhab, limit int) ([]ComplianceRecord, error) {
	if c == nil {
		return nil, fmt.Errorf("%w: corpus is nil", ErrInvalidComplianceRecord)
	}
	if !madhhab.Valid() {
		return nil, fmt.Errorf("%w: %q", ErrInvalidMadhhab, madhhab)
	}
	if limit <= 0 {
		return nil, ErrInvalidComplianceLimit
	}

	terms := complianceTerms(query)
	if len(terms) == 0 {
		return nil, ErrEmptyComplianceQuery
	}

	type scoredRecord struct {
		record ComplianceRecord
		score  int
	}
	matched := make([]scoredRecord, 0, len(c.records))
	for _, record := range c.records {
		if record.Madhhab != madhhab {
			continue
		}
		score := complianceScore(record, terms)
		if score > 0 {
			matched = append(matched, scoredRecord{record: record, score: score})
		}
	}

	sort.Slice(matched, func(i, j int) bool {
		if matched[i].score != matched[j].score {
			return matched[i].score > matched[j].score
		}
		return matched[i].record.ID < matched[j].record.ID
	})
	if len(matched) > limit {
		matched = matched[:limit]
	}

	results := make([]ComplianceRecord, len(matched))
	for i, match := range matched {
		results[i] = cloneComplianceRecord(match.record)
	}
	return results, nil
}

// InjectContext prepends relevant reference context before an LLM prompt.
// The supplied prompt remains unchanged when no records match.
func (c *ComplianceCorpus) InjectContext(prompt string, madhhab Madhhab, limit int) (string, []ComplianceRecord, error) {
	records, err := c.Retrieve(prompt, madhhab, limit)
	if err != nil {
		return "", nil, err
	}
	if len(records) == 0 {
		return prompt, records, nil
	}

	var context strings.Builder
	context.WriteString("[Taawun compliance reference context]\n")
	context.WriteString("This material is reference-only, not a fatwa or scholar-approved ruling. Require qualified scholar review before relying on it.\n")
	context.WriteString("Selected madhhab baseline: ")
	context.WriteString(string(madhhab))
	context.WriteString("\n")
	for _, record := range records {
		fmt.Fprintf(&context, "- %s (status: %s; citation: %s): %s\n", record.Title, record.Status, record.Citation, record.Guidance)
	}
	context.WriteString("[/Taawun compliance reference context]\n\n")
	context.WriteString(prompt)
	return context.String(), records, nil
}

// Valid reports whether m is one of the supported workspace baselines.
func (m Madhhab) Valid() bool {
	switch m {
	case MadhhabHanafi, MadhhabShafii, MadhhabMaliki, MadhhabHanbali:
		return true
	default:
		return false
	}
}

// ParseMadhhab converts a user-facing madhhab label into its canonical value.
func ParseMadhhab(value string) (Madhhab, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "'", "")
	switch normalized {
	case "hanafi":
		return MadhhabHanafi, nil
	case "shafii", "shafai", "shafi'i":
		return MadhhabShafii, nil
	case "maliki":
		return MadhhabMaliki, nil
	case "hanbali":
		return MadhhabHanbali, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidMadhhab, value)
	}
}

func validateComplianceRecord(record ComplianceRecord, seenIDs map[string]struct{}) error {
	if strings.TrimSpace(record.ID) == "" || strings.TrimSpace(record.Title) == "" || strings.TrimSpace(record.Guidance) == "" || strings.TrimSpace(record.Citation) == "" {
		return fmt.Errorf("%w: id, title, guidance, and citation are required", ErrInvalidComplianceRecord)
	}
	if !record.Madhhab.Valid() {
		return fmt.Errorf("%w: %q", ErrInvalidMadhhab, record.Madhhab)
	}
	if record.Status != ComplianceRecordStatusSeed {
		return fmt.Errorf("%w: unsupported record status %q", ErrInvalidComplianceRecord, record.Status)
	}
	if len(complianceTerms(strings.Join(record.Keywords, " "))) == 0 {
		return fmt.Errorf("%w: at least one searchable keyword is required", ErrInvalidComplianceRecord)
	}
	if _, exists := seenIDs[record.ID]; exists {
		return fmt.Errorf("%w: duplicate id %q", ErrInvalidComplianceRecord, record.ID)
	}
	seenIDs[record.ID] = struct{}{}
	return nil
}

func cloneComplianceRecord(record ComplianceRecord) ComplianceRecord {
	copy := record
	copy.Keywords = append([]string(nil), record.Keywords...)
	return copy
}

func complianceScore(record ComplianceRecord, terms []string) int {
	searchable := make(map[string]struct{}, len(complianceTerms(record.Title))+len(complianceTerms(record.Guidance))+len(record.Keywords))
	for _, token := range complianceTerms(strings.Join([]string{record.ID, record.Title, record.Guidance, record.Citation}, " ")) {
		searchable[token] = struct{}{}
	}
	for _, keyword := range record.Keywords {
		for _, token := range complianceTerms(keyword) {
			searchable[token] = struct{}{}
		}
	}

	score := 0
	for _, term := range terms {
		if _, found := searchable[term]; found {
			score++
		}
	}
	return score
}

func complianceTerms(value string) []string {
	terms := make([]string, 0)
	var token strings.Builder
	flush := func() {
		if token.Len() > 0 {
			terms = append(terms, token.String())
			token.Reset()
		}
	}
	for _, character := range strings.ToLower(value) {
		if unicode.IsLetter(character) || unicode.IsNumber(character) {
			token.WriteRune(character)
			continue
		}
		flush()
	}
	flush()
	return terms
}

func seedComplianceRecords() []ComplianceRecord {
	return []ComplianceRecord{
		{
			ID:       "hanafi-interest-free-financing",
			Madhhab:  MadhhabHanafi,
			Title:    "Interest-bearing lending requires review",
			Guidance: "Do not generate compound-interest or APR lending flows. Escalate financing structures for qualified review and make contractual terms explicit.",
			Citation: "Qur'an 2:275-279",
			Status:   ComplianceRecordStatusSeed,
			Keywords: []string{"interest", "loan", "apr", "compound", "financing", "riba"},
		},
		{
			ID:       "hanafi-transparent-marketplace-terms",
			Madhhab:  MadhhabHanafi,
			Title:    "Marketplace terms must be explicit",
			Guidance: "Show price, deliverables, capacity limits, and a buyer confirmation step before a marketplace settlement is submitted.",
			Citation: "Qur'an 4:29",
			Status:   ComplianceRecordStatusSeed,
			Keywords: []string{"marketplace", "price", "terms", "checkout", "settlement", "escrow"},
		},
		{
			ID:       "shafii-interest-free-financing",
			Madhhab:  MadhhabShafii,
			Title:    "Interest-bearing lending requires review",
			Guidance: "Do not generate compound-interest or APR lending flows. Escalate financing structures for qualified review and make contractual terms explicit.",
			Citation: "Qur'an 2:275-279",
			Status:   ComplianceRecordStatusSeed,
			Keywords: []string{"interest", "loan", "apr", "compound", "financing", "riba"},
		},
		{
			ID:       "shafii-transparent-marketplace-terms",
			Madhhab:  MadhhabShafii,
			Title:    "Marketplace terms must be explicit",
			Guidance: "Show price, deliverables, capacity limits, and a buyer confirmation step before a marketplace settlement is submitted.",
			Citation: "Qur'an 4:29",
			Status:   ComplianceRecordStatusSeed,
			Keywords: []string{"marketplace", "price", "terms", "checkout", "settlement", "escrow"},
		},
		{
			ID:       "maliki-interest-free-financing",
			Madhhab:  MadhhabMaliki,
			Title:    "Interest-bearing lending requires review",
			Guidance: "Do not generate compound-interest or APR lending flows. Escalate financing structures for qualified review and make contractual terms explicit.",
			Citation: "Qur'an 2:275-279",
			Status:   ComplianceRecordStatusSeed,
			Keywords: []string{"interest", "loan", "apr", "compound", "financing", "riba"},
		},
		{
			ID:       "maliki-transparent-marketplace-terms",
			Madhhab:  MadhhabMaliki,
			Title:    "Marketplace terms must be explicit",
			Guidance: "Show price, deliverables, capacity limits, and a buyer confirmation step before a marketplace settlement is submitted.",
			Citation: "Qur'an 4:29",
			Status:   ComplianceRecordStatusSeed,
			Keywords: []string{"marketplace", "price", "terms", "checkout", "settlement", "escrow"},
		},
		{
			ID:       "hanbali-interest-free-financing",
			Madhhab:  MadhhabHanbali,
			Title:    "Interest-bearing lending requires review",
			Guidance: "Do not generate compound-interest or APR lending flows. Escalate financing structures for qualified review and make contractual terms explicit.",
			Citation: "Qur'an 2:275-279",
			Status:   ComplianceRecordStatusSeed,
			Keywords: []string{"interest", "loan", "apr", "compound", "financing", "riba"},
		},
		{
			ID:       "hanbali-transparent-marketplace-terms",
			Madhhab:  MadhhabHanbali,
			Title:    "Marketplace terms must be explicit",
			Guidance: "Show price, deliverables, capacity limits, and a buyer confirmation step before a marketplace settlement is submitted.",
			Citation: "Qur'an 4:29",
			Status:   ComplianceRecordStatusSeed,
			Keywords: []string{"marketplace", "price", "terms", "checkout", "settlement", "escrow"},
		},
	}
}
