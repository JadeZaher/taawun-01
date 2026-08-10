package ethics

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ComplianceResult holds the result of a Taqwa security & ethics audit.
type ComplianceResult struct {
	Passed      bool     `json:"passed"`
	Violations  []string `json:"violations"`
	Guardrail   string   `json:"guardrail"`
	Suggestions []string `json:"suggestions"`
}

// HaramCheckEngine scans app prompts, configuration specs, and generated code artifacts for non-compliant logic.
type HaramCheckEngine struct {
	prohibitedKeywords []string
	ribaPatterns       []*regexp.Regexp
}

// NewHaramCheckEngine creates a new ethics scanner pre-loaded with Islamic jurisprudence compliance patterns.
func NewHaramCheckEngine() *HaramCheckEngine {
	return &HaramCheckEngine{
		prohibitedKeywords: []string{
			"gambling", "casino", "poker", "betting", "lottery",
			"liquor", "alcohol", "winery", "brewery",
			"pork", "swine",
			"adult", "pornography",
			"interest loan", "usury", "payday loan",
		},
		ribaPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)\b(compound\s+interest|interest\s*rate\s*>\s*0|annual\s+percentage\s+rate|apr\s*=\s*\d+)\b`),
			regexp.MustCompile(`(?i)\b(late\s+fee\s*\+\s*interest|penalty\s*accumulates\s*interest)\b`),
			regexp.MustCompile(`(?i)\b(calculateInterest\s*\(|compoundingInterest\s*\()\b`),
		},
	}
}

// AuditPrompt checks a user prompt before generating code.
func (e *HaramCheckEngine) AuditPrompt(prompt string) (*ComplianceResult, error) {
	lower := strings.ToLower(prompt)
	var violations []string

	for _, word := range e.prohibitedKeywords {
		if strings.Contains(lower, word) {
			violations = append(violations, fmt.Sprintf("Prohibited domain concept detected: '%s'", word))
		}
	}

	for _, pat := range e.ribaPatterns {
		if pat.MatchString(prompt) {
			violations = append(violations, "Riba (interest-based financial calculation) detected in requirements")
		}
	}

	if len(violations) > 0 {
		return &ComplianceResult{
			Passed:      false,
			Violations:  violations,
			Guardrail:   "Haram-Check AI Guardrail (Surah Al-Ma'idah 5:2)",
			Suggestions: []string{"Use Murabaha or Mudarabah profit-sharing models instead of Riba interest calculations."},
		}, nil
	}

	return &ComplianceResult{
		Passed:    true,
		Guardrail: "Haram-Check AI Guardrail",
	}, nil
}

// AuditCode checks generated source code for non-compliant algorithms.
func (e *HaramCheckEngine) AuditCode(code string) (*ComplianceResult, error) {
	var violations []string

	for _, pat := range e.ribaPatterns {
		if pat.MatchString(code) {
			violations = append(violations, "Riba calculation logic detected in generated source code")
		}
	}

	if len(violations) > 0 {
		return &ComplianceResult{
			Passed:     false,
			Violations: violations,
			Guardrail:  "Code AST Compliance Audit",
		}, errors.New("code failed ethical compliance check")
	}

	return &ComplianceResult{
		Passed:    true,
		Guardrail: "Code AST Compliance Audit",
	}, nil
}
